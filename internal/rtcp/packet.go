package rtcp

import (
	"encoding/binary"
	"errors"
)

const HeaderSize = 4

type Packet struct {
	Version uint8
	Padding bool
	Count   uint8
	Type    uint8
	Length  uint16
	Payload []byte
}

func (p *Packet) Marshal() ([]byte, error) {
	if p.Version == 0 {
		p.Version = 2
	}

	if p.Version != 2 {
		return nil, errors.New("unsupported RTCP version")
	}

	if len(p.Payload)%4 != 0 {
		return nil, errors.New("RTCP payload must be 32-bit aligned")
	}

	length := uint16((HeaderSize + len(p.Payload)) / 4 - 1)

	data := make([]byte, HeaderSize+len(p.Payload))

	data[0] = (p.Version & 0x03) << 6

	if p.Padding {
		data[0] |= 1 << 5
	}

	data[0] |= p.Count & 0x1F

	data[1] = p.Type

	binary.BigEndian.PutUint16(data[2:4], length)

	copy(data[4:], p.Payload)

	return data, nil
}

func Unmarshal(data []byte) (*Packet, error) {
	if len(data) < HeaderSize {
		return nil, errors.New("RTCP packet too short")
	}

	version := (data[0] >> 6) & 0x03

	if version != 2 {
		return nil, errors.New("unsupported RTCP version")
	}

	padding := (data[0] & 0x20) != 0
	count := data[0] & 0x1F
	packetType := data[1]

	length := binary.BigEndian.Uint16(data[2:4])

	expectedSize := (int(length) + 1) * 4

	if expectedSize > len(data) {
		return nil, errors.New("RTCP packet length exceeds buffer")
	}

	payloadSize := expectedSize - HeaderSize

	payload := make([]byte, payloadSize)

	copy(payload, data[HeaderSize:expectedSize])

	return &Packet{
		Version: 2,
		Padding: padding,
		Count:   count,
		Type:    packetType,
		Length:  length,
		Payload: payload,
	}, nil
}