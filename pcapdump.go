// source code from gopacket examples

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
	"os"
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
var filter = flag.String("f", "tcp", "BPF filter for pcap")
var logAllPackets = flag.Bool("v", false, "Logs every packet in great detail")
var wg sync.WaitGroup
var pendingRequest map[gopacket.Flow]map[gopacket.Flow]*http.Request = make(map[gopacket.Flow]map[gopacket.Flow]*http.Request)
var outputLock sync.Mutex

func addRequest(net, transport gopacket.Flow, req *http.Request) {
	//log.Println("ADD", net, transport)
	_, ok := pendingRequest[net]
	if !ok {
		pendingRequest[net] = make(map[gopacket.Flow]*http.Request)
	}
	pendingRequest[net][transport] = req
}

func matchRequest(net, transport gopacket.Flow) *http.Request {
	rnet := net.Reverse()
	rtransport := transport.Reverse()
	//log.Println("DEL", rnet, rtransport)
	_, ok := pendingRequest[rnet]
	if !ok {
		return nil
	}
	req, ok := pendingRequest[rnet][rtransport]
	if !ok {
		return nil
	}
	//log.Println("matched req ", rnet, rtransport, req)
	delete(pendingRequest[rnet], rtransport)
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
	//fmt.Println("WGADD", net, transport)
	wg.Add(1)
	go hstream.run() // Important... we must guarantee that data from the reader stream is read.

	// ReaderStream implements tcpassembly.Stream, so we can return a pointer to it.
	return &hstream.r
}

func (h *httpStream) run() {
	defer wg.Done()
	//defer fmt.Println("WGDONE", h.net, h.transport)
	buf := bufio.NewReader(&h.r)

	header, err := buf.Peek(4)
	if err != nil {
		log.Printf("cannot peek 4 bytes <%s> %s", string(header), err)
		tcpreader.DiscardBytesToEOF(buf)
		h.r.Close()
		return
	}
	//log.Printf("first four bytes is %#v\n", header)

	if string(header) == "HTTP" {  // guess: HTTP response
		for {
			req := matchRequest(h.net, h.transport)
			if req == nil {
				//log.Println("cannot match response to request", h.net, h.transport)
			}
			resp, err := http.ReadResponse(buf, req)
			// FIXME: why io.ErrUnexpectedEOF
			if (err == io.EOF) || (err == io.ErrUnexpectedEOF) {
				h.r.Close()
				return
			} else if err != nil {
				//log.Printf("%#v\n", err)
				//log.Println("Error reading stream", h.net, h.transport, ":", err)
				tcpreader.DiscardBytesToEOF(buf)
				h.r.Close()
				return
			} else {
				body, err := ioutil.ReadAll(resp.Body)
				resp.Body.Close()
				if err != nil {
					log.Println("x1", err)
					continue
				}
				if req == nil {
					continue
				}
				printHTTP("Resp", req, body)
			}
		}
	} else {   // guess: HTTP request
		for {
			req, err := http.ReadRequest(buf)
			if (err == io.EOF) || (err == io.ErrUnexpectedEOF) {
				// We must read until we see an EOF... very important!
				h.r.Close()
				return
			} else if err != nil {
				//log.Printf("%#v\n", err)
				//log.Println("Error reading stream", h.net, h.transport, ":", err)
				tcpreader.DiscardBytesToEOF(buf)
				h.r.Close()
				return
			} else {
				addRequest(h.net, h.transport, req)
				body, err := ioutil.ReadAll(req.Body)
				if err != nil {
					log.Fatal("x2", err)
				}
				req.Body.Close()
				printHTTP("Req", req, body)
			}
		}
	}
}

func printHTTP(t string, req *http.Request, body []byte) {
	Host := req.Host
	URL := req.URL
	var udid string
	list_udid, ok := req.Header["Udid"]
	if ok {
		udid = list_udid[0]
	} else {
		// cannot decrypt without UDID
		// print nothing
		return
	}

	outputLock.Lock()
	fmt.Println("==================================")
	fmt.Println(t+" URL: ", Host, " ", URL)
	fmt.Println("bodylen: ", len(body))
	msg_iv := apiclient.Unlolfuscate(udid)
	//fmt.Println("msg_iv ", msg_iv)
	content := apiclient.DecodeBody(body, msg_iv)
	yy, err := yaml.Marshal(content)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("dumplen: ", len(yy))
	fmt.Println(string(yy))
	outputLock.Unlock()
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
	} else {
		log.Fatalf("use ./http_packet -r $filename")
	}
	if err != nil {
		log.Fatal(err)
	}

	if err := handle.SetBPFFilter(*filter); err != nil {
		log.Fatal(err)
	}

	// Set up assembly
	streamFactory := &httpStreamFactory{}
	streamPool := tcpassembly.NewStreamPool(streamFactory)
	assembler := tcpassembly.NewAssembler(streamPool)

	log.Println("reading in packets")
	// Read in packets, pass to assembler.
	packetSource := gopacket.NewPacketSource(handle, handle.LinkType())
	packets := packetSource.Packets()
	for {
		packet := <-packets
		// A nil packet indicates the end of a pcap file.
		if packet == nil {
			//return
			break
		}
		if *logAllPackets {
			log.Println(packet)
		}
		if packet.NetworkLayer() == nil || packet.TransportLayer() == nil || packet.TransportLayer().LayerType() != layers.LayerTypeTCP {
			log.Println("Unusable packet")
			continue
		}
		tcp := packet.TransportLayer().(*layers.TCP)
		packetTimestamp := packet.Metadata().Timestamp
		assembler.AssembleWithTimestamp(packet.NetworkLayer().NetworkFlow(), tcp, packetTimestamp)
		assembler.FlushOlderThan(packetTimestamp.Add(time.Minute * -2))
	}
	assembler.FlushAll()
	//log.Print("wait", wg)
	wg.Wait()
	//log.Print("done")
}
