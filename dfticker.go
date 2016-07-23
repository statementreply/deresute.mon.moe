package main

import (
	"fmt"
	"os"
	"os/exec"
	"sync"
	"time"
)

var lock sync.Mutex
var wg sync.WaitGroup
var _isRunning bool
var lastRun = time.Unix(0, 0)
var sleepDuration = time.Minute * 2

// python ../deresuteme/main2.py
// ./df
func main() {
	ticker := time.NewTicker(time.Second * 1)
	var q, q0 time.Duration
	var mod time.Duration
	var r time.Duration
	r = time.Minute * 2
	mod = time.Minute * 15
	//r = time.Second * 2
	//mod = time.Second * 15
	q0 = (time.Duration(time.Now().UnixNano()) - r) / mod
	for {
		select {
		case t := <-ticker.C:
			//fmt.Println(t.String(), _isRunning, lastRun.String())
			q = (time.Duration(t.UnixNano()) - r) / mod
			if (q > q0) || NeedToRun() {
				fmt.Println("runCommand", t.String())
				q0 = q
				runCommand()
			}
		}
	}
	wg.Wait()
}

func runCommand() {
	//c := exec.Command("timeout", "300", "python", "../deresuteme/main2.py")
	c := exec.Command("timeout", "300", "./df")
	c.Stdin = nil
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	wg.Add(1)
	go func() {
		defer wg.Done()
		if !IsRunning() {
			SetRunning()
			c.Run()
			SetFinished()
			fmt.Println("current:", time.Now().String())
			lock.Lock()
			fmt.Println("lastRun:", lastRun.String())
			lock.Unlock()
		}
	}()
}

func IsRunning() bool {
	lock.Lock()
	ret := _isRunning
	lock.Unlock()
	return ret
}

func SetRunning() {
	lock.Lock()
	_isRunning = true
	lastRun = time.Now()
	lock.Unlock()
}

func SetFinished() {
	lock.Lock()
	_isRunning = false
	lock.Unlock()
}

func NeedToRun() bool {
	lock.Lock()
	diff := time.Now().Sub(lastRun)
	lock.Unlock()
	ret := false
	if diff > sleepDuration {
		ret = true
	}
	return ret
}
package main

import (
	"apiclient"
	"datafetcher"
	"log"
	"os"
	"path"
	"time"
)

var SECRET_FILE string = "secret.yaml"
var BASE string = path.Dir(os.Args[0])
var RANK_CACHE_DIR string = BASE + "/data/rank/"

func main() {
	log.Println("dfnew", os.Args[0])
	//rand.Seed(time.Now().Unix())
	client := apiclient.NewApiClientFromConfig(SECRET_FILE)

	log.Println(datafetcher.GetLocalTimestamp())
	log.Println(datafetcher.RoundTimestamp(time.Now()).String())

	key_point := [][2]int{
		[2]int{1, 1},
		[2]int{1, 501},     // pt ranking emblem-1
		[2]int{1, 2001},    // tier 1
		[2]int{1, 5001},    // emblem-2
		[2]int{1, 10001},   // tier 2
		[2]int{1, 20001},   // tier 3
		[2]int{1, 50001},   // tier 4-old
		[2]int{1, 60001},   // tier 4
		[2]int{1, 100001},  // tier 5-old
		[2]int{1, 120001},  // tier 5
		[2]int{1, 300001},  // tier 6
		[2]int{1, 500001},  // tier 7
		[2]int{1, 1000001}, // tier 8
		[2]int{2, 1},       // score ranking top
		[2]int{2, 5001},    // tier 1
		[2]int{2, 10001},   // tier 2
		[2]int{2, 40001},   // tier 3
		[2]int{2, 50001},   // tier 4
	}
	// extra data points
	for index := 0; index < 61; index++ {
		key_point = append(key_point, [2]int{1, index*10000 + 1})
		key_point = append(key_point, [2]int{2, index*10000 + 1})
	}
	df := datafetcher.NewDataFetcher(client, key_point, RANK_CACHE_DIR)
	//client.LoadCheck()
	err := df.Run()
	if err != nil {
		log.Println(err)
	}
}
