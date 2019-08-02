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

const DEFAULT_BUFFER_FOR_PLAYBACK_MS = 2500

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
	CurrentSampleDuration   uint32
	CurrentSampleLen        uint32
	CurrentFrames           uint32
	currentSamples          uint32
	currentSegment          uint8
	payloadLen              uint32
	segments                uint8
	currentSample           uint8
	segmentMap              map[uint8]uint8
}

type OpusSamples struct {
	Payload  []byte
	Frames   uint8
	Samples  uint32
	Duration uint32
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
	reader.segmentMap = make(map[uint8]uint8)
	reader.stream = bufio.NewReader(f)
	err = reader.getPage()
	if err != nil {
		return reader, err
	}
	err = reader.getPage()
	if err != nil {
		return reader, err
	}
	return reader, nil
}

func (i *OpusReader) readOpusHead() error {
	var version uint8
	magic := make([]byte, 8)
	if err := binary.Read(i.stream, binary.LittleEndian, &magic); err != err {
		return err
	}
	if bytes.Compare(magic, []byte("OpusHead")) != 0 {
		return errors.New("Wrong Opus Head")
	}

	if err := binary.Read(i.stream, binary.LittleEndian, &version); err != err {
		return err
	}
	var channels uint8
	if err := binary.Read(i.stream, binary.LittleEndian, &channels); err != err {
		return err
	}
	var preSkip uint16
	if err := binary.Read(i.stream, binary.LittleEndian, &preSkip); err != err {
		return err
	}
	if err := binary.Read(i.stream, binary.LittleEndian, &i.sampleRate); err != err {
		return err
	}
	//Skipping OutputGain
	io.CopyN(ioutil.Discard, i.stream, 2)
	var channelMap uint8
	if err := binary.Read(i.stream, binary.LittleEndian, &channelMap); err != err {
		return err
	}
	//if channelMap (Mapping family) is different than 0, next 4 bytes contain channel mapping configuration
	if channelMap != 0 {
		io.CopyN(ioutil.Discard, i.stream, 4)
	}
	return nil
}

func (i *OpusReader) readOpusTags() (uint32, error) {
	var plen uint32
	var vendorLen uint32
	magic := make([]byte, 8)
	if err := binary.Read(i.stream, binary.LittleEndian, &magic); err != err {
		return 0, err
	}
	if bytes.Compare(magic, []byte("OpusTags")) != 0 {
		return 0, errors.New("Wrong Opus Tags")
	}

	if err := binary.Read(i.stream, binary.LittleEndian, &vendorLen); err != err {
		return 0, err
	}
	vendorName := make([]byte, vendorLen)
	if err := binary.Read(i.stream, binary.LittleEndian, &vendorName); err != err {
		return 0, err
	}

	var userCommentLen uint32
	if err := binary.Read(i.stream, binary.LittleEndian, &userCommentLen); err != err {
		return 0, err
	}
	userComment := make([]byte, userCommentLen)
	if err := binary.Read(i.stream, binary.LittleEndian, &userComment); err != err {
		return 0, err
	}
	plen = 16 + vendorLen + userCommentLen
	return plen, nil

}

func (i *OpusReader) getPageHead() error {
	head := make([]byte, 4)
	if err := binary.Read(i.stream, binary.LittleEndian, &head); err != err {
		return err
	}
	if bytes.Compare(head, []byte("OggS")) != 0 {
		return fmt.Errorf("Incorrect page. Does not start with \"OggS\" : %s %v", string(head), hex.EncodeToString(head))
	}
	//Skipping Version
	io.CopyN(ioutil.Discard, i.stream, 1)
	var headerType uint8
	if err := binary.Read(i.stream, binary.LittleEndian, &headerType); err != err {
		return err
	}
	var granulePosition uint64
	if err := binary.Read(i.stream, binary.LittleEndian, &granulePosition); err != err {
		return err
	}
	if err := binary.Read(i.stream, binary.LittleEndian, &i.serial); err != err {
		return err
	}
	if err := binary.Read(i.stream, binary.LittleEndian, &i.pageIndex); err != err {
		return err
	}
	//skipping checksum
	io.CopyN(ioutil.Discard, i.stream, 4)

	if err := binary.Read(i.stream, binary.LittleEndian, &i.segments); err != err {
		return err
	}
	var x uint8
	// building a map of all segments
	i.payloadLen = 0
	for x = 1; x <= i.segments; x++ {
		var segSize uint8
		if err := binary.Read(i.stream, binary.LittleEndian, &segSize); err != err {
			return err
		}
		i.segmentMap[x] = segSize
		i.payloadLen += uint32(segSize)
	}

	return nil
}

func (i *OpusReader) getPage() error {
	err := i.getPageHead()
	if err != nil {
		return err
	}
	if i.pageIndex == 0 {
		err := i.readOpusHead()
		if err != nil {
			return err
		}
	} else if i.pageIndex == 1 {
		plen, err := i.readOpusTags()
		if err != nil {
			return err
		}
		// we are not interested in tags (metadata?)
		io.CopyN(ioutil.Discard, i.stream, int64(i.payloadLen-plen))

	} else {
		io.CopyN(ioutil.Discard, i.stream, int64(i.payloadLen))
	}

	return nil
}

func (i *OpusReader) GetSample() (*OpusSamples, error) {
	opusSamples := new(OpusSamples)
	if i.currentSegment == 0 {

		err := i.getPageHead()
		if err != nil {
			return opusSamples, err

		}
	}
	var currentPacketSize uint32
	// Iteraring throug all segments to check if there are lacing packets. If a segment is 255 bytes long, it means that there must be a following segment for the same packet (which may be again 255 bytes long)
	for i.segmentMap[i.currentSegment] == 255 {
		currentPacketSize += 255
		i.currentSegment += 1

	}
	// Adding either the last segments of lacing ones or a packet that fits only in one segment
	currentPacketSize += uint32(i.segmentMap[i.currentSegment])
	if i.currentSegment < i.segments {
		i.currentSegment += 1
	} else {
		i.currentSegment = 0
	}
	tmpPacket := make([]byte, currentPacketSize)
	opusSamples.Payload = tmpPacket
	binary.Read(i.stream, binary.LittleEndian, &tmpPacket)
	//Reading the TOC byte - we need to know  the frame duration.
	if len(tmpPacket) > 0 {
		//shift 3 bits right to get a value of 5 leading bits. See https://tools.ietf.org/html/rfc6716
		tmptoc := tmpPacket[0] & 255
		var frames uint8
		switch tmptoc & 3 {
		case 0:
			frames = 1
			break
		case 1:
		case 2:
			frames = 2
			break
		default:
			frames = tmpPacket[1] & 63
			break
		}
		opusSamples.Frames = frames
		tocConfig := tmpPacket[0] >> 3

		var length uint32
		length = uint32(tocConfig & 3)
		if tocConfig >= 16 {
			length = DEFAULT_BUFFER_FOR_PLAYBACK_MS << length
		} else if tocConfig >= 12 {
			length = 10000 << (length & 1)
		} else if length == 3 {
			length = 60000
		} else {
			length = 10000 << length
		}
		opusSamples.Duration = length
		duration := uint32(frames) * length
		if duration > 0 {
			opusSamples.Samples = 48000 / (1000000 / length)
		} else {
			opusSamples.Samples = 0
		}
	}
	return opusSamples, nil
}
