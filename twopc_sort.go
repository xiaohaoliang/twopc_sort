package main

import (
	"math/rand"
	"sync"
	"sync/atomic"
	"time"
)

var (
	// number of client
	clientNums  = 2
	// number of messages, simpify program implementation
	messageNums = 20
	dataStreaming  []chan data

	token int64
	step int64
	
	maxSleepInterval int64 = 5
	maxGap int64 = 10

	wg sync.WaitGroup
)

type data struct {
	kind     string
	prepare int64
	commit  int64
}

func init() {
	dataStreaming = make([]chan data, clientNums)
	for i := 0; i < clientNums; i++ {
		dataStreaming[i] = make(chan data, messageNums)
	}
}

/* please implement sort function */
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
		sort()
	}()
	wg.Wait()
}

/*
 * 1 assume dataStreamings are endless => we have infinitely many datas;
 * because it's a simulation program, it has a limited number of datas, but the assumption is not shoul be satisfied
 * 2 sort commit kind of datas that are from multiple dataStreamings by commit ascending 
 * and output them in the fastest way you can think
 */
func sort() {

}

func generateDatas(index int) {
	for i := 0; i < messageNums; i++ {
		dataPrepare := data{
			kind: "prepare",
			prepare: incrementToken(),
		}
		sleep()
		dataStreaming[index] <- dataPrepare

		sleep()

		dataCommit := data{
			kind: "commit",
			prepare: dataPrepare.prepare,
			commit:  incrementToken(),
		}
		sleep()
		dataStreaming[index] <- dataCommit
	}
}

func incrementToken() int64 {
	return atomic.AddInt64(&token, rand.Int63()%maxGap+1)
}

func sleep() {
	interval := atomic.AddInt64(&step, 3)%maxSleepInterval + 1
	waitTime := time.Duration(rand.Int63()%interval)
	time.Sleep(waitTime * time.Second)
}
