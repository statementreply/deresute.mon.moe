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
//
// Ported from the Python implementation in deresute.me
//     <https://github.com/marcan/deresuteme>
//     Copyright 2016-2017 Hector Martin <marcan@marcan.st>
//     Licensed under the Apache License, Version 2.0

package resource_mgr

import (
	"io/ioutil"
	//"github.com/pierrec/lz4"
	// depends on github.com/pierrec/xxHash
	"bytes"
	"encoding/binary"
	lz4 "github.com/bkaradzic/go-lz4"
	"log"
)

type lz4Header struct {
	Magic  uint32 // [0:4]
	Uncomp uint32 // [4:8]
	Comp   uint32 // [8:12]
	Unk    uint32 // [12:16]
}

func Unlz4(fileName string) []byte {
	content, err := ioutil.ReadFile(fileName)
	if err != nil {
		return nil
	}
	br := bytes.NewReader(content)

	var lh lz4Header
	err = binary.Read(br, binary.LittleEndian, &lh)

	if err != nil {
		log.Println(err)
		return nil
	}
	log.Println("header", lh.Uncomp, lh)
	// 1G
	if lh.Uncomp > 1024*1024*1024 {
		log.Println("too large")
		return nil
	}
	var dst []byte
	copy(content[12:16], content[4:8])
	blk, err := lz4.Decode(dst, content[12:])
	if err != nil {
		return nil
	}
	//log.Println("blk", len(blk), err)
	//log.Println("blk", len(blk), cap(blk), len(dst), cap(dst))
	log.Println("uncomp", lh.Uncomp, len(blk))
	if int64(len(blk)) != int64(lh.Uncomp) {
		log.Println("size incorrect")
		return nil
	}
	return blk

}
