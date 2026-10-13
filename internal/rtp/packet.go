package rtp

import (
	"encoding/binary"
	"errors"
)

// HeaderSize is the minimum RTP header size in bytes.
const HeaderSize = 12

type Packet struct {
	Version        uint8
	Padding        bool
	Extension      bool
	CSRCCount      uint8
	Marker         bool
	PayloadType    uint8
	SequenceNumber uint16
	Timestamp      uint32
	SSRC           uint32
	Payload        []byte
}

func (p *Packet) Marshal() ([]byte, error) {
	packet := make([]byte, HeaderSize+len(p.Payload))

	// Byte 0: Version (7-6) | Padding (5) | Extension (4) | CSRC Count (3-0)
	packet[0] = (p.Version & 0x03) << 6
	if p.Padding {
		packet[0] |= 1 << 5
	}
	if p.Extension {
		packet[0] |= 1 << 4
	}
	packet[0] |= p.CSRCCount & 0x0F

	// Byte 1: Marker (7) | Payload Type (6-0)
	packet[1] = p.PayloadType & 0x7F
	if p.Marker {
		packet[1] |= 1 << 7
	}

	// Bytes 2-3: Sequence Number
	binary.BigEndian.PutUint16(packet[2:4], p.SequenceNumber)

	// Bytes 4-7: Timestamp
	binary.BigEndian.PutUint32(packet[4:8], p.Timestamp)

	// Bytes 8-11: SSRC
	binary.BigEndian.PutUint32(packet[8:12], p.SSRC)

	// Media payload (e.g. VP8 or Opus encoded data)
	copy(packet[HeaderSize:], p.Payload)

	return packet, nil
}

func Unmarshal(data []byte) (*Packet, error) {
	if len(data) < HeaderSize {
		return nil, errors.New("RTP packet too short")
	}

	packet := &Packet{}

	// Byte 0: Version (7-6) | Padding (5) | Extension (4) | CSRC Count (3-0)
	packet.Version = (data[0] >> 6) & 0x03
	packet.Padding = (data[0] & 0x20) != 0
	packet.Extension = (data[0] & 0x10) != 0
	packet.CSRCCount = data[0] & 0x0F

	// Byte 1: Marker (7) | Payload Type (6-0)
	packet.Marker = (data[1] & 0x80) != 0
	packet.PayloadType = data[1] & 0x7F

	// Bytes 2-3: Sequence Number
	packet.SequenceNumber = binary.BigEndian.Uint16(data[2:4])

	// Bytes 4-7: Timestamp
	packet.Timestamp = binary.BigEndian.Uint32(data[4:8])

	// Bytes 8-11: SSRC
	packet.SSRC = binary.BigEndian.Uint32(data[8:12])

	// Remaining bytes: RTP payload (copied)
	packet.Payload = append([]byte(nil), data[HeaderSize:]...)

	return packet, nil
}