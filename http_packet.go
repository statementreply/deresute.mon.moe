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
	"bytes"
	"bufio"
	//"encoding/hex"
	"flag"
	"io"
	"io/ioutil"
	"log"
	//"os"
	"net/http"
	//"time"
	"sync"

	"github.com/google/gopacket"
	"util"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcap"
	"github.com/google/gopacket/tcpassembly"
	"github.com/google/gopacket/tcpassembly/tcpreader"
)

var fname = flag.String("r", "", "Filename to read from, overrides -i")
var filter = flag.String("f", "tcp", "BPF filter for pcap")
var logAllPackets = flag.Bool("v", false, "Logs every packet in great detail")
var wg sync.WaitGroup

// Build a simple HTTP request parser using tcpassembly.StreamFactory and tcpassembly.Stream interfaces

// httpStreamFactory implements tcpassembly.StreamFactory
type httpStreamFactory struct{}

// httpStream will handle the actual decoding of http requests.
type httpStream struct {
	net, transport gopacket.Flow
	r              tcpreader.ReaderStream
}

func (h *httpStreamFactory) New(net, transport gopacket.Flow) tcpassembly.Stream {
	log.Println("New stream", net, transport)
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
	all, err := ioutil.ReadAll(&(h.r))
	h.r.Close()
	if err != nil {
		log.Fatal(err)
	}
	//log.Println(h.net, " ", h.transport, "\n", hex.Dump(all))
	log.Println("Read ", h.net, " ", h.transport, " ", len(all))

	//buf := bufio.NewReader(&h.r)
	unbuf := bytes.NewReader(all)
	buf := bufio.NewReader(unbuf)

	header, err := buf.Peek(4)
	if err != nil {
		log.Printf("cannot peek 4 bytes <%s> %s", string(header), err)
		ioutil.ReadAll(buf)
		return
	}
	//log.Printf("first four bytes is %#v\n", header)
	if string(header) == "HTTP" {
		// guess: response
		//log.Printf("HTTP response")
		loop := 0
		for {
			loop += 1
			resp, err := http.ReadResponse(buf, nil)
			if err == io.EOF {
				log.Printf("loop %s %s %d br1\n", h.net, h.transport, loop)
				return
			} else if err != nil {
				log.Printf("loop %s %s %d br2\n", h.net, h.transport, loop)
				log.Println("Error reading stream", h.net, h.transport, ":", err)
			} else {
				log.Printf("loop %s %s %d br3\n", h.net, h.transport, loop)
				bodyBytes := tcpreader.DiscardBytesToEOF(resp.Body)
				resp.Body.Close()
				_ = bodyBytes
				//log.Println("Received response from stream", h.net, h.transport, ":", resp, "with", bodyBytes, "bytes in response body")
				log.Println("Received response from stream", h.net, h.transport, ":", "with", bodyBytes, "bytes in response body")

			}
		}
	} else {
		// guess: request
		loop := 0
		for {
			loop += 1
			req, err := http.ReadRequest(buf)
			if err == io.EOF {
				log.Printf("loop %s %s %d br1\n", h.net, h.transport, loop)
				// We must read until we see an EOF... very important!
				return
			} else if err != nil {
				log.Printf("loop %s %s %d br2\n", h.net, h.transport, loop)
				log.Println("Error reading stream", h.net, h.transport, ":", err)
			} else {
				log.Printf("loop %s %s %d br3\n", h.net, h.transport, loop)
				bodyBytes := tcpreader.DiscardBytesToEOF(req.Body)
				req.Body.Close()
				_ = bodyBytes
				//log.Println("Received request from stream", h.net, h.transport, ":", req, "with", bodyBytes, "bytes in request body")
				log.Println("Received request from stream", h.net, h.transport, ":","with", bodyBytes, "bytes in request body")
			}
		}
	}
}


func main() {
	//log.SetOutput(os.Stdout)
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
		//select {
		//case packet := <-packets:
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
			assembler.AssembleWithTimestamp(packet.NetworkLayer().NetworkFlow(), tcp, packet.Metadata().Timestamp)
		//}
	}
	log.Print("wait")
	wg.Wait()
	log.Print("done")
}
