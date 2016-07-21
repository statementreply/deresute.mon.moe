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
	Magic  uint32
	Uncomp uint32
	Comp   uint32
	Unk    uint32
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
		log.Fatal(err)
	}
	log.Println(lh)
	var dst []byte
	copy(content[12:16], content[4:8])
	blk, err := lz4.Decode(dst, content[12:])
	log.Println(len(blk), err)
	log.Println(len(blk), cap(blk), len(dst), cap(dst))
	return blk

}
