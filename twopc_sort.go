package main

import (
	"math/rand"
	"sync"
	"sync/atomic"
	"time"

	"fmt"
	"sort"
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

type simpleLimiter struct {
	rate float64

	burst int

	mu       sync.Mutex
	tokens   float64
	interval time.Duration
	// last is the last time the limiter's tokens field was updated
	last time.Time
}

// r, b must > 0
func newSimpleLimiter(r float64, b int) simpleLimiter {
	return simpleLimiter{
		rate:     r,
		burst:    b,
		interval: time.Nanosecond * time.Duration(1e9/r),
		last:     time.Now(),
	}
}

func acquire(lim *simpleLimiter) {
	lim.mu.Lock()
	defer lim.mu.Unlock()
	if lim.tokens >= 1 {
		lim.tokens--
		return
	}
	for {
		now := time.Now()
		elapsed := now.Sub(lim.last)
		deltaTokens := elapsed.Seconds() * float64(lim.rate)
		lim.tokens = lim.tokens + deltaTokens

		if burst := float64(lim.burst); lim.tokens > burst {
			lim.tokens = burst
		}

		lim.last = now

		if lim.tokens >= 1 {
			lim.tokens--
			return
		}
		sleepTime := lim.interval - elapsed
		if sleepTime < time.Millisecond {
			// lim.burst greater, speed faster
			sleepTime = sleepTime * time.Duration(lim.burst)
		}
		time.Sleep(sleepTime)
	}
}

type dataSlice struct {
	maxSendTime time.Time
	prepares    []int64
	commits     []data
	cur         int
	isSorted    bool
}

func isOver(oneDataSlice *dataSlice) bool {
	if oneDataSlice.cur == len(oneDataSlice.prepares) {
		return true
	}
	return false
}
func getCommitData(oneDataSlice *dataSlice) *data {
	return &(oneDataSlice.commits[oneDataSlice.cur])
}

type sortBuf struct {
	srcChan  *chan data
	bufDatas []dataSlice
	commits  map[int64]data
}

func nextMsg(oneSortbuf *sortBuf, thePrepare int64) bool {
	oneData := <-*oneSortbuf.srcChan

	//if commit msg
	if oneData.commit != 0 {
		oneSortbuf.commits[oneData.prepare] = oneData
		if oneData.prepare == thePrepare {
			return true
		}
		return false
	}
	//insert last dataSlice or new dataSlice
	lastDataSlice := &(oneSortbuf.bufDatas[len(oneSortbuf.bufDatas)-1])

	if !lastDataSlice.maxSendTime.Before(oneData.sendTime) {
		lastDataSlice.prepares = append(lastDataSlice.prepares, oneData.prepare)
	} else {
		lastDataSlice = &dataSlice{
			maxSendTime: oneData.sendTime.Add(time.Duration(2*maxSleepInterval) * time.Millisecond),
			prepares:    []int64{oneData.prepare},
			cur:         0,
			isSorted:    false,
		}

		oneSortbuf.bufDatas = append(oneSortbuf.bufDatas, *lastDataSlice)
	}

	return false

}

func readCommitData(oneSortbuf *sortBuf, thePrepare int64) data {
	if v, ok := oneSortbuf.commits[thePrepare]; ok {
		delete(oneSortbuf.commits, thePrepare)
		return v
	}
	for {
		if nextMsg(oneSortbuf, thePrepare) {
			break
		}
	}

	res := oneSortbuf.commits[thePrepare]
	delete(oneSortbuf.commits, thePrepare)
	return res
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
	sortAllDataStreaming := make(chan data, 10)

	sortDataStreaming0 := make(chan data, 10)
	sortDataStreaming1 := make(chan data, 10)

	go func() {
		sortDataStream(&dataStreaming[0], &sortDataStreaming0)
	}()
	go func() {
		sortDataStream(&dataStreaming[1], &sortDataStreaming1)
	}()
	mergeDataStreaming(&sortDataStreaming0, &sortDataStreaming1, &sortAllDataStreaming)

	limiter := newSimpleLimiter(20, 5)

	var i int64 = 0
	for {
		acquire(&limiter)

		oneData := <-sortAllDataStreaming
		i++
		fmt.Println(i, ", commit= ", oneData.commit, oneData.sendTime)
	}
}

func sortDataStream(inputChan *chan data, sortedChan *chan data) {
	//init
	oneSortbuf := sortBuf{
		srcChan: inputChan,
		commits: make(map[int64]data),
	}
	firstData := <-*inputChan
	firstDataSlice := dataSlice{
		maxSendTime: firstData.sendTime.Add(time.Duration(2*maxSleepInterval) * time.Millisecond),
		prepares:    []int64{firstData.prepare},
		cur:         0,
		isSorted:    false,
	}
	oneSortbuf.bufDatas = append(oneSortbuf.bufDatas, firstDataSlice)

	for {
		nextMsg(&oneSortbuf, 0)

		if len(oneSortbuf.bufDatas) > 3 {

			if isOver(&oneSortbuf.bufDatas[0]) {
				//delete the first dataSlice
				index := 0
				oneSortbuf.bufDatas = append(oneSortbuf.bufDatas[:index], oneSortbuf.bufDatas[index+1:]...)
				continue
			}
			// sort 3 dataSlice
			for i := 0; i <= 2; i++ {
				if !oneSortbuf.bufDatas[i].isSorted {
					//get all commit msg
					for _, onePrepare := range oneSortbuf.bufDatas[i].prepares {
						theCommit := readCommitData(&oneSortbuf, onePrepare)
						oneSortbuf.bufDatas[i].commits = append(oneSortbuf.bufDatas[i].commits, theCommit)
					}
					sort.Sort(Datas(oneSortbuf.bufDatas[i].commits))
					oneSortbuf.bufDatas[i].isSorted = true
				}
			}

			// merge  output
			//TODO: optimize  : loser tree
			for !isOver(&oneSortbuf.bufDatas[0]) {

				minIndex := 0

				if !isOver(&oneSortbuf.bufDatas[1]) {
					if getCommitData(&oneSortbuf.bufDatas[1]).commit < getCommitData(&oneSortbuf.bufDatas[minIndex]).commit {
						minIndex = 1
					}
				}
				if !isOver(&oneSortbuf.bufDatas[2]) {
					if getCommitData(&oneSortbuf.bufDatas[2]).commit < getCommitData(&oneSortbuf.bufDatas[minIndex]).commit {
						minIndex = 2
					}
				}

				*sortedChan <- *getCommitData(&oneSortbuf.bufDatas[minIndex])
				oneSortbuf.bufDatas[minIndex].cur++

			}
			//delete the first dataSlice
			index := 0
			oneSortbuf.bufDatas = append(oneSortbuf.bufDatas[:index], oneSortbuf.bufDatas[index+1:]...)

		}
	}

}

//Maybe use struct replace chan
func mergeDataStreaming(dataChan0 *chan data, dataChan1 *chan data, outputDataChan *chan data) {
	go func() {
		d0 := <-*dataChan0
		d1 := <-*dataChan1
		for {
			if d0.commit <= d1.commit {
				*outputDataChan <- d0
				d0 = <-*dataChan0
			} else {
				*outputDataChan <- d1
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
