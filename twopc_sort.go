package main

import (
	"math/rand"
	"sync"
	"sync/atomic"
	"time"
)

var (
	maxClientNums  = 2
	maxMessageNums = 20
	dataStreaming  []chan data

	token int64
	step int64
	
	maxGap int64 = 10

	wg sync.WaitGroup
)

type data struct {
	kind     string
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

/*
 * 1 assume dataStreamings are endless => we have infinitely many datas;
 * because it's a simulation program, it has a limited number of datas, but the assumption is not shoul be satisfied
 * 2 sort commit kind of datas that are from multiple dataStreamings by commit ascending 
 * and output them in the fastest way you can think
 */
func sort() {

}

func generateDatas(index int) {
	for i := 0; i < maxMessageNums; i++ {
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
	interval := atomic.AddInt64(&step, 3)%5 + 1
	waitTime := time.Duration(rand.Int63()%interval + 1)
	time.Sleep(waitTime * time.Second)
}
