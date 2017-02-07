// Copyright 2016 GUO Yixuan <culy.gyx@gmail.com>
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License version 3 as
// published by the Free Software Foundation.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"sync"
	"time"
)

var wg sync.WaitGroup

var updatingFilter = regexp.MustCompile("(^\\s*$)|UPDATING")
var waitFilter = regexp.MustCompile("WAITING")
var resultFilter = regexp.MustCompile("【結果発表】")
var emptyFilter = regexp.MustCompile("EMPTY")

type Periodic struct {
	cache_filename string
	url            string
	interval       time.Duration
	div, rem       time.Duration
	dryrun         bool
}

func main() {
	fmt.Println("go version of twitter ticker")
	log.Println("go version of twitter ticker")
	twitter1 := &Periodic{
		cache_filename: "cached_status",
		url:            "https://deresuteborder.mon.moe/twitter",
		interval:       30 * time.Second,
		div:            15 * 60 * time.Second,
		rem:            (2*60 + 15) * time.Second,
		dryrun:         false,
	}
	twitter2 := &Periodic{
		cache_filename: "cached_status_emblem",
		url:            "https://deresuteborder.mon.moe/twitter_emblem",
		interval:       30 * time.Second,
		div:            60 * 60 * time.Second,
		rem:            3 * 60 * time.Second,
		dryrun:         false,
	}
	twitter3 := &Periodic{
		cache_filename: "cached_status_trophy",
		url:            "https://deresuteborder.mon.moe/twitter_trophy",
		interval:       30 * time.Second,
		div:            60 * 60 * time.Second,
		rem:            165 * time.Second,
		dryrun:         false,
	}
	wg.Add(3)
	go twitter1.Run()
	go twitter2.Run()
	go twitter3.Run()
	// wait
	wg.Wait()
}

func (p *Periodic) Run() {
	defer wg.Done()
	ticker := time.NewTicker(time.Second * 1)
	//quotient := (time.Duration(time.Now().UnixNano()) - p.rem) / p.div;
	quotient := time.Duration(0)

	// too complex state transition
	// the presence of a cache file indicates that the post action SUCCEEDED
	// FIXME: must parse twitter content to determine timestamp
	for {
		select {
		case _ = <-ticker.C: // discard return value
			quotient_new := (time.Duration(time.Now().UnixNano()) - p.rem) / p.div
			if quotient_new <= quotient {
				continue
			}
			for {
				content, err := ioutil.ReadFile(p.cache_filename)
				if err != nil {
					log.Println(p.url, "cannot read cache_file", err)
					content = []byte("")
				}
				//log.Println("content is", string(content))
				var body, result []byte

				resp, err := http.Get(p.url)
				if err != nil {
					log.Println(p.url, "cannot get url", err)
					goto Retry
				}

				body, err = ioutil.ReadAll(resp.Body)
				resp.Body.Close()
				if err != nil {
					log.Println(p.url, "cannot read respbody", err)
					goto Retry
				}

				//log.Println("body is", string(body))
				if waitFilter.Match(body) {
					log.Println(p.url, "wait filter matched")
					goto Finish
				}
				if resultFilter.Match(content) && resultFilter.Match(body) {
					log.Println(p.url, "don't post final twice")
					goto Finish
				}

				if bytes.Equal(body, content) {
					log.Println(p.url, "body content equal")
					goto Retry
				}
				if updatingFilter.Match(body) {
					log.Println(p.url, "updatingfilter match")
					goto Retry
				}
				if emptyFilter.Match(body) {
					log.Println(p.url, "empty response")
					goto Finish
				}

				err = ioutil.WriteFile(p.cache_filename, body, 0644)
				if err != nil {
					log.Println(p.url, "cannot write file", err)
					// FIXME
					goto Retry
				}

				if p.dryrun {
					goto Finish
				}

				result, err = exec.Command("perl", "twitter.pl", string(body)).CombinedOutput()
				log.Println(string(result))
				if err != nil {
					log.Println(p.url, "error occured", err)
					err = os.Remove(p.cache_filename)
					if err != nil {
						log.Println(p.url, "cannot remove file", p.cache_filename)
					}
					goto Retry
				}

			Finish:
				quotient = quotient_new
				break
			Retry: // continue block
				log.Println(p.url, "in retry")
				time.Sleep(p.interval)
			}
		}
	}
}
