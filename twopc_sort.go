package main

import (
	"math/rand"
	"sync"
	"sync/atomic"
	"time"
)

var (
	maxClientNums  = 2
	maxMessageNums = 2
	dataStreaming  []chan data

	token int64
	
	maxGap int64 = 10
	sleepInterval int64 = 5

	wg sync.WaitGroup
)

type data struct {
	prepare int64
	commit  int64
}

func init() {
	dataStreaming = make([]chan data, maxClientNums)
	for i := 0; i < maxClientNums; i++ {
		dataStreaming[i] = make(chan data, maxMessageNums)
	}
}

/* please implement sort function */
func main() {
	wg.Add(maxClientNums*2 + 1)
	// genrateDatas and sort are parallel
	for i := 0; i < maxClientNums; i++ {
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

func generateDatas(index int) {
	for i := 0; i < maxMessageNums; i++ {
		dataPrepare := data{
			prepare: incrementToken(),
		}
		sleep()
		dataStreaming[index] <- dataPrepare

		sleep()

		dataCommit := data{
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
	waitTime := time.Duration(rand.Int63()%sleepInterval + 1)
	time.Sleep(waitTime * time.Second)
}

/*
 * 1 assume dataStreamings are endless => we have infinitely many dataCommits
 * 2 sort dataCommits that from multiple dataStreaming by commit ascending and output them in the fastest way you can think
 */
func sort() {

}
