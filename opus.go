package opus

import (
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"io"
	//       "math/rand"
	"bufio"
	"bytes"
	"encoding/hex"
	"io/ioutil"
	"os"
)

// OpusReader is used to take an OGG file and write RTP packets
type OpusReader struct {
	stream          io.Reader
	fd              *os.File
	sampleRate      uint32
	channelCount    uint16
	serial          uint32
	pageIndex       uint32
	checksumTable   *crc32.Table
	granulePosition uint64
	timestamp       uint32
}

// New builds a new OGG Opus reader
func NewFile(fileName string) (*OpusReader, error) {
	f, err := os.Open(fileName)
	if err != nil {
		return nil, err
	}
	reader := &OpusReader{}
	//  reader, err := NewWith(f, sampleRate, channelCount)
	//if err != nil {
	//        return nil, err
	//}
	reader.fd = f
	reader.stream = bufio.NewReader(f)
	_, err = reader.getPage()
	if err != nil {
		return reader, err
	}
	_, err = reader.getPage()
	if err != nil {
		return reader, err
	}
	return reader, nil
}

func (i *OpusReader) readOpusHead() (uint32, error) {
	var plen uint32
	var version uint8
	if err := binary.Read(i.stream, binary.LittleEndian, &version); err != err {
		return 0, err
	}
	plen += 1
	fmt.Printf("Version: %v\n", version)
	var channels uint8
	if err := binary.Read(i.stream, binary.LittleEndian, &channels); err != err {
		return 0, err
	}

	fmt.Printf("Channels: %v\n", channels)

	var preSkip uint16
	if err := binary.Read(i.stream, binary.LittleEndian, &preSkip); err != err {
		return 0, err
	}
	plen += 2
	fmt.Printf("preSkip: %v\n", preSkip)

	var sampleRate uint32
	if err := binary.Read(i.stream, binary.LittleEndian, &sampleRate); err != err {
		return 0, err
	}
	plen += 4
	fmt.Printf("SamlpleRate: %v\n", sampleRate)
	//Skipping OutputGain
	io.CopyN(ioutil.Discard, i.stream, 2)
	plen += 2
	var channelMap uint8
	if err := binary.Read(i.stream, binary.LittleEndian, &channelMap); err != err {
		return 0, err
	}
	plen += 2
	fmt.Printf("ChannelMap : %v\n", channelMap)

	return plen, nil
}

func (i *OpusReader) readOpusTags() (uint32, error) {
	var plen uint32
	var vendorLen uint32
	if err := binary.Read(i.stream, binary.LittleEndian, &vendorLen); err != err {
		return 0, err
	}
	fmt.Printf("VendorLen: %v\n", vendorLen)
	plen = plen + 4
	vendorName := make([]byte, vendorLen)
	if err := binary.Read(i.stream, binary.LittleEndian, &vendorName); err != err {
		return 0, err
	}
	plen = plen + vendorLen
	fmt.Printf("Vendor Name: %v\n", string(vendorName))

	var userCommentLen uint32
	if err := binary.Read(i.stream, binary.LittleEndian, &userCommentLen); err != err {
		return 0, err
	}
	fmt.Printf("userCommentLen: %v\n", userCommentLen)
	plen = plen + 4
	userComment := make([]byte, userCommentLen)
	if err := binary.Read(i.stream, binary.LittleEndian, &userComment); err != err {
		return 0, err
	}
	plen = plen + userCommentLen
	fmt.Printf("UserComment: %v\n", string(userComment))

	return plen, nil

}

func (i *OpusReader) getPage() ([]byte, error) {
	payload = make([]byte, 255)
	head := make([]byte, 4)
	if err := binary.Read(i.stream, binary.LittleEndian, &head); err != err {
		return payload, err
	}
	if bytes.Compare(head, []byte("OggS")) != 0 {
		return payload, fmt.Errorf("Incorrect page. Does not start with \"OggS\" : %s %v", string(head), hex.EncodeToString(head))
	}
	//Skipping Version
	io.CopyN(ioutil.Discard, i.stream, 1)
	var headerType uint8
	if err := binary.Read(i.stream, binary.LittleEndian, &headerType); err != err {
		return payload, err
	}
	fmt.Printf("headerType: %v\n", headerType)
	if err := binary.Read(i.stream, binary.LittleEndian, &i.granulePosition); err != err {
		return payload, err
	}
	fmt.Printf("i.granulePosition: %v\n", i.granulePosition)
	if err := binary.Read(i.stream, binary.LittleEndian, &i.serial); err != err {
		return payload, err
	}
	fmt.Printf("i.serial: %v\n", i.serial)
	if err := binary.Read(i.stream, binary.LittleEndian, &i.pageIndex); err != err {
		return payload, err
	}
	fmt.Printf("i.pageIndexl: %v\n", i.pageIndex)

	//skipping checksum
	io.CopyN(ioutil.Discard, i.stream, 4)

	var segments uint8
	if err := binary.Read(i.stream, binary.LittleEndian, &segments); err != err {
		return payload, err
	}
	var payloadLen uint32
	for x := 1; x <= int(segments); x++ {
		var segSize uint8
		if err := binary.Read(i.stream, binary.LittleEndian, &segSize); err != err {
			return payload, err
		}
		fmt.Printf("Seg %d size: %v\n", x, segSize)
		payloadLen = payloadLen + uint32(segSize)
	}

	fmt.Printf("payloadLen 0: %v\n", payloadLen)
	fmt.Printf("segments: %v\n", segments)
	magic := make([]byte, 8)
	if err := binary.Read(i.stream, binary.LittleEndian, &magic); err != err {
		return payload, err
	}
	if bytes.Compare(magic, []byte("OpusHead")) == 0 {
		plen, err := i.readOpusHead()
		if err != nil {
			fmt.Printf("Read Headers Error : %v\n", err)
		}

		fmt.Printf("plen : %v\n", plen)
	} else if bytes.Compare(magic, []byte("OpusTags")) == 0 {
		plen, err := i.readOpusTags()
		if err != nil {
			fmt.Printf("ReadTags Error : %v\n", err)
		}
		fmt.Printf("plen : %v\n", plen)
		io.CopyN(ioutil.Discard, i.stream, int64(payloadLen-30))
	} else {
		fmt.Printf("an audio frame\n")
		io.CopyN(payload, i.stream, int64(payloadLen-8))
	}

	fmt.Printf("payloadLen: %v\n", payloadLen)

	return payload, nil
}

func (i *OpusReader) GetSample() ([]byte, error) {
	payload, err := i.getPage()
	if err != nil {
		return nil, err
	}

	return payload, nil
}
