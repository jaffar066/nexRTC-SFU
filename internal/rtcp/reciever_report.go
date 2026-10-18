package rtcp

import (
	"encoding/binary"
	"errors"
)

const ReceiverReportType uint8 = 201

const ReceiverReportBlockSize = 24

type ReceiverReportBlock struct {
	SSRC uint32

	FractionLost     uint8
	CumulativeLost   int32
	HighestSeqNumber uint32
	Jitter           uint32
	LastSenderReport uint32
	DelaySinceLastSR uint32
}

type ReceiverReport struct {
	SSRC   uint32
	Blocks []ReceiverReportBlock
}

func (rr *ReceiverReport) Marshal() ([]byte, error) {
	if rr == nil {
		return nil, errors.New("receiver report is nil")
	}
	if len(rr.Blocks) > 31 {
		return nil, errors.New("too many receiver report blocks")
	}
	for _, block := range rr.Blocks {
		if block.CumulativeLost < -8388608 || block.CumulativeLost > 8388607 {
			return nil, errors.New("cumulative packet loss exceeds signed 24-bit range")
		}
	}

	size := 8 + len(rr.Blocks)*ReceiverReportBlockSize

	data := make([]byte, size)

	// Version = 2
	data[0] = 2 << 6

	// Report Count
	data[0] |= uint8(len(rr.Blocks)) & 0x1F

	// Packet Type = 201
	data[1] = ReceiverReportType

	// RTCP length is number of 32-bit words minus 1.
	length := uint16(size/4 - 1)

	binary.BigEndian.PutUint16(data[2:4], length)

	// Receiver SSRC
	binary.BigEndian.PutUint32(data[4:8], rr.SSRC)

	offset := 8

	for _, block := range rr.Blocks {
		marshalReceiverReportBlock(data[offset:offset+ReceiverReportBlockSize], block)
		offset += ReceiverReportBlockSize
	}

	return data, nil
}

func marshalReceiverReportBlock(data []byte, block ReceiverReportBlock) {
	binary.BigEndian.PutUint32(data[0:4], block.SSRC)
	data[4] = block.FractionLost
	lost := block.CumulativeLost & 0xFFFFFF
	data[5] = byte(lost >> 16)
	data[6] = byte(lost >> 8)
	data[7] = byte(lost)
	binary.BigEndian.PutUint32(data[8:12], block.HighestSeqNumber)
	binary.BigEndian.PutUint32(data[12:16], block.Jitter)
	binary.BigEndian.PutUint32(data[16:20], block.LastSenderReport)
	binary.BigEndian.PutUint32(data[20:24], block.DelaySinceLastSR)
}

func unmarshalReceiverReportBlock(data []byte) ReceiverReportBlock {
	lost := int32(data[5])<<16 | int32(data[6])<<8 | int32(data[7])
	if lost&0x800000 != 0 {
		lost |= ^int32(0xFFFFFF)
	}
	return ReceiverReportBlock{
		SSRC:             binary.BigEndian.Uint32(data[0:4]),
		FractionLost:     data[4],
		CumulativeLost:   lost,
		HighestSeqNumber: binary.BigEndian.Uint32(data[8:12]),
		Jitter:           binary.BigEndian.Uint32(data[12:16]),
		LastSenderReport: binary.BigEndian.Uint32(data[16:20]),
		DelaySinceLastSR: binary.BigEndian.Uint32(data[20:24]),
	}
}

func UnmarshalReceiverReport(data []byte) (*ReceiverReport, error) {
	packet, err := Unmarshal(data)
	if err != nil {
		return nil, err
	}
	if packet.Type != ReceiverReportType {
		return nil, errors.New("not a receiver report")
	}
	count := int(packet.Count)
	expectedPayloadSize := 4 + count*ReceiverReportBlockSize
	if len(packet.Payload) != expectedPayloadSize {
		return nil, errors.New("receiver report payload length does not match report count")
	}
	report := &ReceiverReport{
		SSRC:   binary.BigEndian.Uint32(packet.Payload[0:4]),
		Blocks: make([]ReceiverReportBlock, 0, count),
	}

	offset := 4

	for i := 0; i < count; i++ {
		report.Blocks = append(
			report.Blocks,
			unmarshalReceiverReportBlock(packet.Payload[offset:offset+ReceiverReportBlockSize]),
		)
		offset += ReceiverReportBlockSize
	}

	return report, nil
}
