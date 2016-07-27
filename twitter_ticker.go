package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os/exec"
	"regexp"
	"sync"
	"time"
)

var wg sync.WaitGroup

var updatingFilter = regexp.MustCompile("(^\\s*$)|UPDATING")
var waitFilter = regexp.MustCompile("WAITING")

type Periodic struct {
	cache_filename string
	url            string
	interval       time.Duration
	div, rem       time.Duration
}

func main() {
	fmt.Println("go version of twitter ticker")
	log.Println("go version of twitter ticker")
	twitter1 := &Periodic{
		cache_filename: "cached_status",
		url:            "https://deresuteborder.mon.moe/twitter",
		interval:       30 * time.Second,
		div:            15 * 60 * time.Second,
		rem:            2 * 60 * time.Second,
	}
	twitter2 := &Periodic{
		cache_filename: "cached_status_emblem",
		url:            "https://deresuteborder.mon.moe/twitter_emblem",
		interval:       30 * time.Second,
		div:            60 * 60 * time.Second,
		rem:            3 * 60 * time.Second,
	}
	wg.Add(2)
	go twitter1.Run()
	go twitter2.Run()
	// wait
	wg.Wait()
}

func (p *Periodic) Run() {
	defer wg.Done()
	ticker := time.NewTicker(time.Second * 1)
	//quotient := (time.Duration(time.Now().UnixNano()) - p.rem) / p.div;
	quotient := time.Duration(0)

	for {
		select {
		case t := <-ticker.C:
			_ = t
			quotient_new := (time.Duration(time.Now().UnixNano()) - p.rem) / p.div
			if quotient_new <= quotient {
				continue
			}
			for {
				content, err := ioutil.ReadFile(p.cache_filename)
				if err != nil {
					log.Println("cannot read cache_file", err)
					content = []byte("")
				}
				//log.Println("content is", string(content))
				var body, result []byte

				resp, err := http.Get(p.url)
				if err != nil {
					log.Println("cannot get url", err)
					goto Retry
				}

				body, err = ioutil.ReadAll(resp.Body)
				resp.Body.Close()
				if err != nil {
					log.Println("cannot read respbody", err)
					goto Retry
				}

				//log.Println("body is", string(body))
				if bytes.Equal(body, content) {
					goto Retry
				}
				if updatingFilter.Match(body) {
					goto Retry
				}
				if waitFilter.Match(body) {
					log.Println("wait filter matched")
					goto Finish
				}

				err = ioutil.WriteFile(p.cache_filename, body, 0644)
				if err != nil {
					log.Println("cannot write file", err)
				}
				result, err = exec.Command("perl", "twitter.pl", string(body)).CombinedOutput()
				log.Println(string(result))
				if err != nil {
					log.Println("error occured", err)
					goto Retry
				}

			Finish:
				quotient = quotient_new
				break
			Retry: // continue block
				time.Sleep(p.interval)
			}
		}
	}
}
