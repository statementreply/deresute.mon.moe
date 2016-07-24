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
	Magic  uint32  // [0:4]
	Uncomp uint32  // [4:8]
	Comp   uint32  // [8:12]
	Unk    uint32  // [12:16]
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
	if lh.Uncomp > 1024 * 1024 * 1024 {
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
