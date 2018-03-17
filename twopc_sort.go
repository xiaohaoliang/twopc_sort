package main

import (
	"math/rand"
	"sync"
	"sync/atomic"
	"time"

	"context"
	"fmt"
	"sort"

	"golang.org/x/time/rate"
)

var (
	// number of client
	clientNums = 2
	// number of messages, simpify program implementation
	messageNums = 20
	// assume dataStreaming has unlimited capacity
	dataStreaming []chan data

	token int64
	step  int64

	maxSleepInterval int64 = 5
	maxGap           int64 = 10

	wg sync.WaitGroup
)

type data struct {
	kind     string
	prepare  int64
	commit   int64
	sendTime time.Time
}

type Datas []data

func (ds Datas) Len() int {
	return len(ds)
}

func (ds Datas) Less(i, j int) bool {
	return ds[i].commit < ds[j].commit
}

func (ds Datas) Swap(i, j int) {
	temp := ds[i]
	ds[i] = ds[j]
	ds[j] = temp
}

type dataBuf struct {
	beginTime time.Time
	vector    []data
	cur       int
	nextData  data
}

func init() {
	dataStreaming = make([]chan data, clientNums)
	for i := 0; i < clientNums; i++ {
		dataStreaming[i] = make(chan data, messageNums)
	}
}

/* 1. please implement sort code
 *    u can add some auxiliary structures, variables and functions
 *    dont modify any definition
 * 2. implement flow control for the sort code
 */
func main() {
	wg.Add(clientNums*2 + 1)
	// genrateDatas and sort are parallel
	for i := 0; i < clientNums; i++ {
		go func(index int) {
			defer wg.Done()
			generateDatas(index)
		}(i)
		go func(index int) {
			defer wg.Done()
			generateDatas(index)
		}(i)
	}

	go func() {
		defer wg.Done()
		sortAndPrint()
	}()
	wg.Wait()
}

/*
 * 1 assume dataStreamings are endless => we have infinitely many datas;
 * because it's a simulation program, it has a limited number of datas, but the assumption is not shoul be satisfied
 * 2 sort commit kind of datas that are from multiple dataStreamings by commit ascending
 * and output them in the fastest way you can think
 */
func sortAndPrint() {
	// sorted all chan result
	sortAllDataStreaming := make(chan data, 1)

	sortDataStreaming0 := make(chan data, 1)
	sortDataStreaming1 := make(chan data, 1)
	go func() {
		sortDataStream(&dataStreaming[0], &sortDataStreaming0)
	}()
	go func() {
		sortDataStream(&dataStreaming[1], &sortDataStreaming1)
	}()
	mergeDataStreaming(&sortDataStreaming0, &sortDataStreaming1, &sortAllDataStreaming)

	l := rate.NewLimiter(20, 5)
	c, _ := context.WithCancel(context.TODO())
	fmt.Println(l.Limit(), l.Burst())
	var i int64 = 0
	for {
		l.Wait(c)
		oneData := <-sortAllDataStreaming
		i++
		fmt.Println(i, ", commit= ", oneData.commit, oneData.sendTime)
	}
}

func getCommitData(dataChan *chan data) data {
	for {
		oneData := <-*dataChan
		if oneData.commit != 0 {
			return oneData
		}
	}
}

func sortDataStream(inputChan *chan data, sortedChan *chan data) {

	oneData := getCommitData(inputChan)

	databuf1 := makeAndSortDataBuf(&oneData, inputChan)
	oneData = databuf1.nextData
	databuf2 := makeAndSortDataBuf(&oneData, inputChan)
	oneData = databuf2.nextData

	for {
		var minData data
		if getMinData(&databuf1, &databuf2, &minData) {
			*sortedChan <- minData
		} else {
			databuf1 = databuf2
			databuf2 = makeAndSortDataBuf(&oneData, inputChan)
			oneData = databuf2.nextData
		}
	}

}

func makeAndSortDataBuf(oneData *data, dataChan *chan data) dataBuf {
	datas := dataBuf{
		beginTime: oneData.sendTime,
		vector:    []data{*oneData},
		cur:       0,
	}
	for {
		d1 := getCommitData(dataChan)
		if d1.sendTime.Sub(datas.beginTime) <= time.Duration(maxSleepInterval)*time.Millisecond {
			datas.vector = append(datas.vector, d1)
		} else {
			datas.nextData = d1
			break
		}
	}
	sort.Sort(Datas(datas.vector))
	return datas
}

func getMinData(databuf1 *dataBuf, databuf2 *dataBuf, minData *data) bool {
	if databuf1.cur < len(databuf1.vector) {

		if databuf2.cur < len(databuf2.vector) {
			if databuf2.vector[databuf2.cur].commit <= databuf1.vector[databuf1.cur].commit {
				*minData = databuf2.vector[databuf2.cur]
				databuf2.cur++
				return true
			}
		}
		*minData = databuf1.vector[databuf1.cur]
		databuf1.cur++
		return true
	}
	return false
}

//TODO use struct replace chan
func mergeDataStreaming(dataChan0 *chan data, dataChan1 *chan data, sortedDataChan *chan data) {
	go func() {
		d0 := <-*dataChan0
		d1 := <-*dataChan1
		for {
			if d0.commit <= d1.commit {
				*sortedDataChan <- d0
				d0 = <-*dataChan0
			} else {
				*sortedDataChan <- d1
				d1 = <-*dataChan1
			}
		}
	}()
}

/*
 * generate prepare and commit datas.
 * assume max difference of send time between prepare and commit data is 2*maxSleepInterval(millisecond),
 * thus u would't think some extreme cases about thread starvation.
 */
func generateDatas(index int) {
	for i := 0; i < messageNums; i++ {
		prepare := incrementToken()
		sleep(maxSleepInterval)

		dataStreaming[index] <- data{
			kind:     "prepare",
			prepare:  prepare,
			sendTime: time.Now(),
		}
		sleep(maxSleepInterval)

		commit := incrementToken()
		sleep(maxSleepInterval)

		dataStreaming[index] <- data{
			kind:     "commit",
			prepare:  prepare,
			commit:   commit,
			sendTime: time.Now(),
		}
		sleep(10 * maxSleepInterval)
	}
}

func incrementToken() int64 {
	return atomic.AddInt64(&token, rand.Int63()%maxGap+1)
}

func sleep(factor int64) {
	interval := atomic.AddInt64(&step, 3)%factor + 1
	waitTime := time.Duration(rand.Int63() % interval)
	time.Sleep(waitTime * time.Millisecond)
}
