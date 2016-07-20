package main

import (
	"time"
	"fmt"
	"os"
	"os/exec"
	"sync"
)

var lock sync.Mutex
var _isRunning bool
var lastRun = time.Unix(0, 0)
var sleepDuration = time.Minute * 2

// python ../deresuteme/main2.py
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
			fmt.Println(t.String())
			q = (time.Duration(t.UnixNano()) - r) / mod
			if (q > q0) || NeedToRun() {
				fmt.Println("runCommand", t.String())
				q0 = q
				runCommand()
			}
		}
	}
}

func runCommand() {
	c := exec.Command("python", "../deresuteme/main2.py")
	c.Stdin = nil
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	go func() {
		if !IsRunning() {
			SetRunning()
			c.Run()
			SetFinished()
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
	_isRunning = true
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
