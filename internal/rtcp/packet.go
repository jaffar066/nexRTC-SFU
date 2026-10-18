package rtcp

import (
	"encoding/binary"
	"errors"
	"fmt"
)

const HeaderSize = 4

type Packet struct {
	Version     uint8
	Padding     bool
	PaddingSize uint8
	Count       uint8
	Type        uint8
	Length      uint16
	Payload     []byte
}

func (p *Packet) Marshal() ([]byte, error) {
	if p == nil {
		return nil, errors.New("RTCP packet is nil")
	}
	version := p.Version
	if version == 0 {
		version = 2
	}
	if version != 2 {
		return nil, fmt.Errorf("unsupported RTCP version: %d", version)
	}
	if p.Count > 31 {
		return nil, fmt.Errorf("RTCP count/FMT exceeds 31: %d", p.Count)
	}
	if !p.Padding && len(p.Payload)%4 != 0 {
		return nil, errors.New("RTCP payload must be 32-bit aligned")
	}
	if !p.Padding && p.PaddingSize != 0 {
		return nil, errors.New("RTCP padding size provided without padding flag")
	}

	paddingSize := int(p.PaddingSize)
	if p.Padding && paddingSize == 0 {
		paddingSize = 4 - len(p.Payload)%4
	}
	if p.Padding && (paddingSize == 0 || (len(p.Payload)+paddingSize)%4 != 0) {
		return nil, errors.New("RTCP padding does not align the packet to 32 bits")
	}
	if paddingSize > 255 {
		return nil, errors.New("RTCP padding size exceeds 255 bytes")
	}

	packetSize := HeaderSize + len(p.Payload) + paddingSize
	if packetSize/4-1 > int(^uint16(0)) {
		return nil, errors.New("RTCP packet exceeds maximum encoded length")
	}
	length := uint16(packetSize/4 - 1)
	data := make([]byte, packetSize)

	data[0] = version << 6

	if p.Padding {
		data[0] |= 1 << 5
	}

	data[0] |= p.Count

	data[1] = p.Type

	binary.BigEndian.PutUint16(data[2:4], length)

	copy(data[4:], p.Payload)
	if paddingSize > 0 {
		data[len(data)-1] = byte(paddingSize)
	}

	return data, nil
}

func Unmarshal(data []byte) (*Packet, error) {
	packet, size, err := unmarshalOne(data)
	if err != nil {
		return nil, err
	}
	if size != len(data) {
		return nil, fmt.Errorf("RTCP packet has %d trailing bytes", len(data)-size)
	}
	return packet, nil
}

// UnmarshalCompound parses a concatenated sequence of RTCP packets.
func UnmarshalCompound(data []byte) ([]*Packet, error) {
	if len(data) == 0 {
		return nil, errors.New("RTCP compound packet is empty")
	}
	packets := make([]*Packet, 0, 1)
	for len(data) > 0 {
		packet, size, err := unmarshalOne(data)
		if err != nil {
			return nil, err
		}
		if packet.Padding && size != len(data) {
			return nil, errors.New("RTCP padding is only allowed on the final compound packet")
		}
		packets = append(packets, packet)
		data = data[size:]
	}
	return packets, nil
}

func unmarshalOne(data []byte) (*Packet, int, error) {
	if len(data) < HeaderSize {
		return nil, 0, errors.New("RTCP packet too short")
	}

	version := (data[0] >> 6) & 0x03

	if version != 2 {
		return nil, 0, fmt.Errorf("unsupported RTCP version: %d", version)
	}

	padding := (data[0] & 0x20) != 0
	count := data[0] & 0x1F
	packetType := data[1]

	length := binary.BigEndian.Uint16(data[2:4])
	expectedSize := (int(length) + 1) * 4

	if expectedSize < HeaderSize || expectedSize > len(data) {
		return nil, 0, errors.New("RTCP packet length exceeds buffer")
	}

	payloadSize := expectedSize - HeaderSize
	payloadEnd := expectedSize
	paddingSize := uint8(0)
	if padding {
		paddingSize = data[expectedSize-1]
		if paddingSize == 0 {
			return nil, 0, errors.New("RTCP padding count is zero")
		}
		if int(paddingSize) > payloadSize {
			return nil, 0, errors.New("RTCP padding exceeds packet payload")
		}
		payloadEnd -= int(paddingSize)
	}

	packet := &Packet{
		Version:     2,
		Padding:     padding,
		PaddingSize: paddingSize,
		Count:       count,
		Type:        packetType,
		Length:      length,
		Payload:     append([]byte(nil), data[HeaderSize:payloadEnd]...),
	}
	return packet, expectedSize, nil
}
