package opus

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"io/ioutil"
	"os"
)

// OpusReader is used to take an OGG file and write RTP packets
type OpusReader struct {
	stream                  io.Reader
	fd                      *os.File
	sampleRate              uint32
	channelCount            uint16
	serial                  uint32
	pageIndex               uint32
	checksumTable           *crc32.Table
	previousGranulePosition uint64
	currentSampleLen        float32
	currentSamples          uint32
	currentSegment          uint8
	segments                uint8
	currentSample           uint8
	segmentMap              map[uint8]uint8
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
	reader.segmentMap = make(map[uint8]uint8)
	return reader, nil
}

func (i *OpusReader) readOpusHead() (uint32, error) {
	var plen uint32
	var version uint8
	magic := make([]byte, 8)
	if err := binary.Read(i.stream, binary.LittleEndian, &magic); err != err {
		return 0, err
	}
	if bytes.Compare(magic, []byte("OpusHead")) != 0 {
		return 0, errors.New("Wrong Opus Head")
	}

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

	if err := binary.Read(i.stream, binary.LittleEndian, &i.sampleRate); err != err {
		return 0, err
	}
	plen += 4
	fmt.Printf("SamlpleRate: %v\n", i.sampleRate)
	//Skipping OutputGain
	io.CopyN(ioutil.Discard, i.stream, 2)
	plen += 2
	var channelMap uint8
	if err := binary.Read(i.stream, binary.LittleEndian, &channelMap); err != err {
		return 0, err
	}
	plen += 2
	fmt.Printf("ChannelMap : %v\n", channelMap)
	//if channelMap (Mapping family) is different than 0, next 4 bytes contain channel mapping configuration

	if channelMap != 0 {
		io.CopyN(ioutil.Discard, i.stream, 4)
		plen += 4
	}
	return plen, nil
}

func (i *OpusReader) readOpusTags() (uint32, error) {
	var plen uint32
	var vendorLen uint32
	magic := make([]byte, 8)
	if err := binary.Read(i.stream, binary.LittleEndian, &magic); err != err {
		return 0, err
	}
	if bytes.Compare(magic, []byte("OpusTags")) != 0 {
		fmt.Printf("Incorrect magic \"OpusTags\" : %s %v\n", string(magic), hex.EncodeToString(magic))
		return 0, errors.New("Wrong Opus Tags")
	}

	if err := binary.Read(i.stream, binary.LittleEndian, &vendorLen); err != err {
		return 0, err
	}
	fmt.Printf("VendorLen: %v\n", vendorLen)
	vendorName := make([]byte, vendorLen)
	if err := binary.Read(i.stream, binary.LittleEndian, &vendorName); err != err {
		return 0, err
	}
	fmt.Printf("Vendor Name: %v\n", string(vendorName))

	var userCommentLen uint32
	if err := binary.Read(i.stream, binary.LittleEndian, &userCommentLen); err != err {
		return 0, err
	}
	fmt.Printf("userCommentLen: %v\n", userCommentLen)
	userComment := make([]byte, userCommentLen)
	if err := binary.Read(i.stream, binary.LittleEndian, &userComment); err != err {
		return 0, err
	}
	fmt.Printf("UserComment: %v\n", string(userComment))
	plen = 16 + vendorLen + userCommentLen
	return plen, nil

}

func (i *OpusReader) getPage() ([]byte, error) {
	payload := make([]byte, 1)
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
	var granulePosition uint64
	if err := binary.Read(i.stream, binary.LittleEndian, &granulePosition); err != err {
		return payload, err
	}
	fmt.Printf("i.granulePosition: %v\n", granulePosition)
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

	i.previousGranulePosition = granulePosition

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

	if i.pageIndex == 0 {

		_, err := i.readOpusHead()
		if err != nil {
			fmt.Printf("Read Headers Error : %v\n", err)
		}
	} else if i.pageIndex == 1 {
		plen, err := i.readOpusTags()
		if err != nil {
			fmt.Printf("ReadTags Error : %v\n", err)
		}
		// we are not interested in tags (metadata?)
		io.CopyN(ioutil.Discard, i.stream, int64(payloadLen-plen))

	} else {
		tmpPacket := make([]byte, payloadLen)
		binary.Read(i.stream, binary.LittleEndian, &tmpPacket)
		fmt.Printf("an audio frame\n")
		//Reading the TOC byte - we need to know  the frame duration.
		fmt.Printf("============= TOC : % 08b \n", tmpPacket[0])
		return tmpPacket, nil
	}

	fmt.Printf("payloadLen: %v\n", payloadLen)

	return payload, nil
}

func (i *OpusReader) getPageSingle() ([]byte, error) {
	if i.currentSegment == 0 {
		payload := make([]byte, 1)
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
		var granulePosition uint64
		if err := binary.Read(i.stream, binary.LittleEndian, &granulePosition); err != err {
			return payload, err
		}
		fmt.Printf("i.granulePosition: %v\n", granulePosition)
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

		if err := binary.Read(i.stream, binary.LittleEndian, &i.segments); err != err {
			return payload, err
		}
		i.previousGranulePosition = granulePosition

		var x uint8
		for x = 1; x <= i.segments; x++ {
			var segSize uint8
			if err := binary.Read(i.stream, binary.LittleEndian, &segSize); err != err {
				return payload, err
			}
			fmt.Printf("Seg %d size: %v\n", x, segSize)
			i.segmentMap[x] = segSize
		}
		i.currentSegment = 1
	}
	var currentPacketSize uint32

	if i.currentSegment == i.segments {
		i.currentSegment = 0
	} else {
		for i.segmentMap[i.currentSegment] == 255 {
			i.currentSegment += 1
			currentPacketSize += 255
		}
		//last lacing packet
		currentPacketSize += uint32(i.segmentMap[i.currentSegment])
		i.currentSegment += 1
	}

	fmt.Printf("reading segment %v  size: %v \n", i.currentSegment, i.segmentMap[i.currentSegment])

	tmpPacket := make([]byte, currentPacketSize)

	binary.Read(i.stream, binary.LittleEndian, &tmpPacket)
	//Reading the TOC byte - we need to know  the frame duration.
	if len(tmpPacket) > 0 {
		//shift 3 bits right to get a value of 5 leading bits. See https://tools.ietf.org/html/rfc6716
		toc := tmpPacket[0] >> 3
		fmt.Printf("============= TOC : % 08b \n", tmpPacket[0])
		fmt.Printf("============= TOC Dec value : %d \n", toc)
		fmt.Printf("============= frameSize : %f \n", getFrameSize(uint8(toc)))
		i.currentSampleLen = getFrameSize(uint8(toc))
	}
	return tmpPacket, nil
}

//Returns Frame size in ms based on Configuration number
func getFrameSize(toc uint8) float32 {
	var frameSize float32
	// https://tools.ietf.org/html/rfc6716
	switch toc {
	case 16, 20, 24, 28:
		frameSize = 2.5
	case 17, 21, 25, 29:
		frameSize = 5
	case 0, 4, 8, 12, 14, 18, 22, 26, 30:
		frameSize = 10
	case 1, 5, 9, 13, 15, 19, 23, 27, 31:
		frameSize = 20
	case 2, 6, 10:
		frameSize = 40
	case 3, 7, 11:
		frameSize = 60
	}
	return frameSize

}

func (i *OpusReader) GetSample() ([]byte, error) {
	payload, err := i.getPage()
	if err != nil {
		return nil, err
	}

	return payload, nil
}

func (i *OpusReader) GetSingleSample() ([]byte, error) {
	payload, err := i.getPageSingle()
	if err != nil {
		return nil, err
	}

	return payload, nil
}

func (i *OpusReader) calculateSampleDuration(deltaGranulePosition uint32) (uint32, error) {
	i.currentSamples = uint32(deltaGranulePosition)
	fmt.Printf("calculating sample duration for packets: %v\n", i.currentSamples)
	if i.sampleRate == 0 {
		return 0, errors.New("Wrong samplerate")
	}
	if i.segments == 0 {
		return 0, errors.New("Wrong number of segments")
	}

	deltaTime := i.currentSamples * 1000 / i.sampleRate / uint32(i.segments)
	return uint32(deltaTime), nil
}

func (i *OpusReader) GetCurrentSamples() uint32 {
	fmt.Printf("current samples: %v\n", i.currentSamples)
	return i.currentSamples
}

// Returns duration in ms of current sample
func (i *OpusReader) GetCurrentSampleDuration() float32 {
	fmt.Printf("current sample duration: %v\n", i.currentSampleLen)
	return i.currentSampleLen
}
