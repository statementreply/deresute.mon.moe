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

// source code from gopacket examples

// capture from network interface:
//sudo setcap cap_net_raw,cap_net_admin=eip ./pcapdump

//./pcapdump  -f 'tcp and port 80' -i eth0

// Copyright 2012 Google, Inc. All rights reserved.
//
// Use of this source code is governed by a BSD-style license
// that can be found in the LICENSE file in the root of the source
// tree.

// This binary provides sample code for using the gopacket TCP assembler and TCP
// stream reader.  It reads packets off the wire and reconstructs HTTP requests
// it sees, logging them.
package main

import (
	"apiclient"
	//"bytes"
	"bufio"
	//"encoding/hex"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"resource_mgr"
	"sync"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcap"
	"github.com/google/gopacket/tcpassembly"
	"github.com/google/gopacket/tcpassembly/tcpreader"
	"gopkg.in/yaml.v2"
	"util"
)

var fname = flag.String("r", "", "Filename to read from, overrides -i")
var iface = flag.String("i", "", "Interface to get packets from")
var snaplen = flag.Int("s", 1600, "SnapLen for pcap packet capture")
var filter = flag.String("f", "tcp", "BPF filter for pcap")
var logAllPackets = flag.Bool("v", false, "Logs every packet in great detail")
var showAllHTTP = flag.Bool("a", false, "Show every http request/response")
var isDebug = flag.Bool("d", false, "(debug) show types")
var showYAML = flag.Bool("y", false, "(debug) show yaml")
var filterHost = flag.String("h", "", "filter host")
var wg sync.WaitGroup

// use lock to prevent concurrent rw
// map[net-flow: IP pair]
//   map[transport-flow: port pair]
var pendingRequest map[gopacket.Flow]map[gopacket.Flow]*http.Request = make(map[gopacket.Flow]map[gopacket.Flow]*http.Request)
var pendingRequestLock sync.RWMutex

// for printing to stdout, maybe use log
var outputLock sync.Mutex

var mgr *resource_mgr.ResourceMgr

func addRequest(net, transport gopacket.Flow, req *http.Request) {
	//log.Println("ADD", net, transport)
	pendingRequestLock.RLock()
	_, ok := pendingRequest[net]
	pendingRequestLock.RUnlock()
	if !ok {
		pendingRequestLock.Lock()
		pendingRequest[net] = make(map[gopacket.Flow]*http.Request)
		pendingRequestLock.Unlock()
	}
	pendingRequestLock.Lock()
	pendingRequest[net][transport] = req
	pendingRequestLock.Unlock()
}

func matchRequest(net, transport gopacket.Flow) *http.Request {
	rnet := net.Reverse()
	rtransport := transport.Reverse()
	pendingRequestLock.RLock()
	_, ok := pendingRequest[rnet]
	pendingRequestLock.RUnlock()
	if !ok {
		return nil
	}
	pendingRequestLock.RLock()
	req, ok := pendingRequest[rnet][rtransport]
	pendingRequestLock.RUnlock()
	if !ok {
		return nil
	}
	//log.Println("matched req ", rnet, rtransport, req)
	pendingRequestLock.Lock()
	//log.Println("DEL", rnet, rtransport)
	delete(pendingRequest[rnet], rtransport)
	pendingRequestLock.Unlock()
	return req
}

// Build a simple HTTP request parser using tcpassembly.StreamFactory and tcpassembly.Stream interfaces

// httpStreamFactory implements tcpassembly.StreamFactory
type httpStreamFactory struct{}

// httpStream will handle the actual decoding of http requests.
type httpStream struct {
	net, transport gopacket.Flow
	r              tcpreader.ReaderStream
}

func (h *httpStreamFactory) New(net, transport gopacket.Flow) tcpassembly.Stream {
	//log.Println("New stream", net, transport)
	hstream := &httpStream{
		net:       net,
		transport: transport,
		r:         tcpreader.NewReaderStream(),
	}
	wg.Add(1)
	go hstream.run() // Important... we must guarantee that data from the reader stream is read.

	// ReaderStream implements tcpassembly.Stream, so we can return a pointer to it.
	return &hstream.r
}

func (h *httpStream) run() {
	defer wg.Done()
	buf := bufio.NewReader(&h.r)

	header, err := buf.Peek(4)
	if err != nil {
		if err != io.EOF {
			log.Printf("cannot peek 4 bytes <%s> %s", string(header), err)
		}
	} else if string(header) == "HTTP" { // guess: HTTP response
		for {
			req := matchRequest(h.net, h.transport)
			resp, err := http.ReadResponse(buf, req)
			// FIXME: why io.ErrUnexpectedEOF
			// FIXME: ignore io.EOF io.ErrUnexpectedEOF and other errors
			if err != nil {
				//log.Printf("Error reading stream %s %s : %#v\n", h.net, h.transport, err)
				break
			} else {
				processHTTP("Resp", req, resp.Body, h)
			}
		}
	} else { // guess: HTTP request
		for {
			req, err := http.ReadRequest(buf)
			if err != nil {
				//log.Printf("Error reading stream %s %s : %#v\n", h.net, h.transport, err)
				break
			} else {
				addRequest(h.net, h.transport, req)
				processHTTP("Req", req, req.Body, h)
			}
		}
	}
	// We must read until we see an EOF... very important!
	tcpreader.DiscardBytesToEOF(buf)
	h.r.Close()
	return
}

func processHTTP(t string, req *http.Request, bodyReader io.ReadCloser, h *httpStream) {
	body, err := ioutil.ReadAll(bodyReader)
	bodyReader.Close()
	if err != nil {
		log.Println("x2", err)
		return
	}

	if req == nil {
		return
	}

	Host := req.Host
	URL := req.URL
	var udid string
	list_udid, ok := req.Header["Udid"]
	var content map[string]interface{}
	var isDereAPI bool
	var msg_iv string
	if ok {
		udid = list_udid[0]
		msg_iv = apiclient.Unlolfuscate(udid)
		content = apiclient.DecodeBody(body, msg_iv)
		//yy, err := yaml.Marshal(content)
		if err != nil {
			log.Fatal("yaml error", err)
		}
		isDereAPI = true
	} else {
		// cannot decrypt without UDID
		// normal http packet
		// print request
	}

	var isResourceAPI bool
	isResourceAPI = Host == "storage.game.starlight-stage.jp"

	if *filterHost != "" {
		if *filterHost != Host {
			return
		}
	}

	if isDereAPI || isResourceAPI || *showAllHTTP {
		outputLock.Lock()
		fmt.Println("=======================================================")
		fmt.Println(t+" URL:", Host, URL, h.net, h.transport)
		if isDereAPI {
			fmt.Println("udid:", msg_iv)
			fmt.Println("user:", apiclient.Unlolfuscate(req.Header.Get("USER_ID")))
		}
		//fmt.Println("viewer_id:", content["viewer_id"])
		//fmt.Println("bodylen: ", len(body))
		//fmt.Println("msg_iv ", msg_iv)
		//fmt.Println("yamllen:", len(yy))
		//fmt.Println(string(yy))
		if content != nil {
			fmt.Println(content)
			fmt.Println("viewer_id:", content["viewer_id"])
			if *isDebug {
				fmt.Printf("%t\n", content)
			}
			//fmt.Printf("%v\n", content)
			//fmt.Printf("%#v\n", content)
			if _, ok := content["data_headers"]; ok {
				result_code := content["data_headers"].(map[interface{}]interface{})["result_code"]
				fmt.Printf("%T %#v\n", result_code, result_code)
				viewer_id := content["data_headers"].(map[interface{}]interface{})["viewer_id"]
				fmt.Println("viewer_id:", viewer_id)
			}
			if *showYAML {
				yy, _ := yaml.Marshal(content)
				fmt.Print(string(yy))
			}
		}
		if isResourceAPI {
			processResourceAPI(URL)
		}
		outputLock.Unlock()
	}
}

var rvFilter = regexp.MustCompile(`^/dl/(\d+)/`)
var rscFilter = regexp.MustCompile(`^/dl/resources/.*/([0-9a-f]+)$`)

func processResourceAPI(URL *url.URL) {
	path := URL.Path
	fmt.Printf("resource %s\n", path)
	submatch := rvFilter.FindStringSubmatch(path)
	if submatch != nil {
		fmt.Println("res_ver", submatch[1])
		mgr.Set_res_ver(submatch[1])
	}
	submatch = rscFilter.FindStringSubmatch(path)
	if submatch != nil {
		fmt.Println("rsc hash", submatch[1])
		name := mgr.ParseResource(submatch[1])
		fmt.Println("rsc name", name)
	}
}

func main() {
	log.SetOutput(os.Stderr)
	defer util.Run()()
	var handle *pcap.Handle
	var err error

	// Set up pcap packet capture
	if *fname != "" {
		log.Printf("Reading from pcap dump %q", *fname)
		handle, err = pcap.OpenOffline(*fname)
	} else if *iface != "" {
		log.Printf("Starting capture on interface %q", *iface)
		handle, err = pcap.OpenLive(*iface, int32(*snaplen), true, pcap.BlockForever)
	} else {
		log.Fatalf("use ./http_packet -r $filename or ./xxx -i eth0")
	}
	if err != nil {
		log.Fatal("pcap error", err)
	}

	if err := handle.SetBPFFilter(*filter); err != nil {
		log.Fatal("filter error", err)
	}

	// Set up assembly
	streamFactory := &httpStreamFactory{}
	streamPool := tcpassembly.NewStreamPool(streamFactory)
	assembler := tcpassembly.NewAssembler(streamPool)

	//log.Println("reading in packets")
	// Read in packets, pass to assembler.
	packetSource := gopacket.NewPacketSource(handle, handle.LinkType())
	packets := packetSource.Packets()
	ticker := time.Tick(time.Minute)

	conf := apiclient.ParseConfig("secret.yaml")
	mgr = resource_mgr.NewResourceMgr(conf.ResVer, "data/resourcesbeta")
PacketLoop:
	for {
		select {
		case packet := <-packets:
			// A nil packet indicates the end of a pcap file.
			if packet == nil {
				//return
				break PacketLoop
			}
			if *logAllPackets {
				//log.Println("logall", packet.Dump())
				log.Println("logall", packet)
			}
			if packet.NetworkLayer() == nil || packet.TransportLayer() == nil || packet.TransportLayer().LayerType() != layers.LayerTypeTCP {
				log.Println("Unusable packet")
				continue
			}
			tcp := packet.TransportLayer().(*layers.TCP)
			packetTimestamp := packet.Metadata().Timestamp
			assembler.AssembleWithTimestamp(packet.NetworkLayer().NetworkFlow(), tcp, packetTimestamp)
			assembler.FlushOlderThan(packetTimestamp.Add(time.Minute * -2))

		case <-ticker:
			assembler.FlushOlderThan(time.Now().Add(time.Minute * -2))
		}
	}
	// close all connections
	assembler.FlushAll()
	wg.Wait()
}
