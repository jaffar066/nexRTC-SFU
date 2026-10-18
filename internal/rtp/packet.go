package rtp

import (
	"encoding/binary"
	"errors"
	"fmt"
)

// HeaderSize is the minimum RTP header size in bytes.
const HeaderSize = 12

const (
	rtpVersion      = 2
	rtpMaxCSRCCount = 15
)

type Packet struct {
	Version          uint8
	Padding          bool
	Extension        bool
	CSRCCount        uint8
	CSRCs            []uint32
	ExtensionProfile uint16
	ExtensionData    []byte
	PaddingSize      uint8
	Marker           bool
	PayloadType      uint8
	SequenceNumber   uint16
	Timestamp        uint32
	SSRC             uint32
	Payload          []byte
}

func (p *Packet) Marshal() ([]byte, error) {
	if p == nil {
		return nil, errors.New("RTP packet is nil")
	}
	if p.Version != rtpVersion {
		return nil, fmt.Errorf("unsupported RTP version: %d", p.Version)
	}
	if p.PayloadType > 127 {
		return nil, fmt.Errorf("RTP payload type exceeds 127: %d", p.PayloadType)
	}
	if len(p.CSRCs) > rtpMaxCSRCCount {
		return nil, fmt.Errorf("RTP packet has too many CSRCs: %d", len(p.CSRCs))
	}
	if p.CSRCCount != 0 && int(p.CSRCCount) != len(p.CSRCs) {
		return nil, fmt.Errorf(
			"RTP CSRC count %d does not match CSRC list length %d",
			p.CSRCCount,
			len(p.CSRCs),
		)
	}
	if !p.Extension && (p.ExtensionProfile != 0 || len(p.ExtensionData) != 0) {
		return nil, errors.New("RTP extension data provided without extension flag")
	}
	if len(p.ExtensionData)%4 != 0 {
		return nil, errors.New("RTP extension data must be 32-bit aligned")
	}
	const maxExtensionDataSize = 0xFFFF * 4
	if len(p.ExtensionData) > maxExtensionDataSize {
		return nil, errors.New("RTP extension data is too large")
	}

	paddingSize := 0
	if p.Padding {
		paddingSize = int(p.PaddingSize)
		if paddingSize == 0 {
			paddingSize = 1
		}
	}

	csrcCount := len(p.CSRCs)
	extensionHeaderSize := 0
	if p.Extension {
		extensionHeaderSize = 4 + len(p.ExtensionData)
	}
	packetSize := HeaderSize + csrcCount*4 + extensionHeaderSize + len(p.Payload) + paddingSize
	packet := make([]byte, packetSize)

	packet[0] = rtpVersion << 6
	if p.Padding {
		packet[0] |= 1 << 5
	}
	if p.Extension {
		packet[0] |= 1 << 4
	}
	packet[0] |= uint8(csrcCount)

	packet[1] = p.PayloadType
	if p.Marker {
		packet[1] |= 1 << 7
	}
	binary.BigEndian.PutUint16(packet[2:4], p.SequenceNumber)
	binary.BigEndian.PutUint32(packet[4:8], p.Timestamp)
	binary.BigEndian.PutUint32(packet[8:12], p.SSRC)

	offset := HeaderSize
	for _, csrc := range p.CSRCs {
		binary.BigEndian.PutUint32(packet[offset:offset+4], csrc)
		offset += 4
	}

	if p.Extension {
		binary.BigEndian.PutUint16(packet[offset:offset+2], p.ExtensionProfile)
		binary.BigEndian.PutUint16(packet[offset+2:offset+4], uint16(len(p.ExtensionData)/4))
		offset += 4
		copy(packet[offset:], p.ExtensionData)
		offset += len(p.ExtensionData)
	}

	copy(packet[offset:], p.Payload)
	offset += len(p.Payload)
	if paddingSize > 0 {
		packet[len(packet)-1] = byte(paddingSize)
	}

	return packet, nil
}

func Unmarshal(data []byte) (*Packet, error) {
	if len(data) < HeaderSize {
		return nil, errors.New("RTP packet too short")
	}

	version := (data[0] >> 6) & 0x03
	if version != rtpVersion {
		return nil, fmt.Errorf("unsupported RTP version: %d", version)
	}

	csrcCount := data[0] & 0x0F
	offset := HeaderSize
	csrcBytes := int(csrcCount) * 4
	if len(data)-offset < csrcBytes {
		return nil, errors.New("RTP packet has incomplete CSRC list")
	}

	csrcs := make([]uint32, int(csrcCount))
	for i := range csrcs {
		csrcs[i] = binary.BigEndian.Uint32(data[offset : offset+4])
		offset += 4
	}

	hasExtension := data[0]&0x10 != 0
	var extensionProfile uint16
	var extensionData []byte
	if hasExtension {
		if len(data)-offset < 4 {
			return nil, errors.New("RTP packet has incomplete extension header")
		}

		extensionProfile = binary.BigEndian.Uint16(data[offset : offset+2])
		extensionWords := int(binary.BigEndian.Uint16(data[offset+2 : offset+4]))
		offset += 4
		extensionSize := extensionWords * 4
		if len(data)-offset < extensionSize {
			return nil, errors.New("RTP packet has incomplete extension data")
		}
		extensionData = append([]byte(nil), data[offset:offset+extensionSize]...)
		offset += extensionSize
	}

	padding := data[0]&0x20 != 0
	paddingSize := uint8(0)
	payloadEnd := len(data)
	if padding {
		paddingSize = data[len(data)-1]
		if paddingSize == 0 {
			return nil, errors.New("RTP padding count is zero")
		}
		if int(paddingSize) > len(data)-offset {
			return nil, errors.New("RTP padding exceeds packet payload")
		}
		payloadEnd -= int(paddingSize)
	}

	if payloadEnd < offset {
		return nil, errors.New("RTP packet has invalid payload and padding lengths")
	}

	return &Packet{
		Version:          version,
		Padding:          padding,
		Extension:        hasExtension,
		CSRCCount:        csrcCount,
		CSRCs:            csrcs,
		ExtensionProfile: extensionProfile,
		ExtensionData:    extensionData,
		PaddingSize:      paddingSize,
		Marker:           data[1]&0x80 != 0,
		PayloadType:      data[1] & 0x7F,
		SequenceNumber:   binary.BigEndian.Uint16(data[2:4]),
		Timestamp:        binary.BigEndian.Uint32(data[4:8]),
		SSRC:             binary.BigEndian.Uint32(data[8:12]),
		Payload:          append([]byte(nil), data[offset:payloadEnd]...),
	}, nil
}
