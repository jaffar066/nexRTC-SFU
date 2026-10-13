package rtcp

import (
	"encoding/binary"
	"errors"
)

const ReceiverReportType uint8 = 201

const ReceiverReportBlockSize = 24

type ReceiverReportBlock struct {
	SSRC uint32

	FractionLost       uint8
	CumulativeLost     int32
	HighestSeqNumber   uint32
	Jitter             uint32
	LastSenderReport   uint32
	DelaySinceLastSR   uint32
}

type ReceiverReport struct {
	SSRC   uint32
	Blocks []ReceiverReportBlock
}

func (rr *ReceiverReport) Marshal() ([]byte, error) {
	if len(rr.Blocks) > 31 {
		return nil, errors.New("too many receiver report blocks")
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
		// SSRC of RTP source
		binary.BigEndian.PutUint32(
			data[offset:offset+4],
			block.SSRC,
		)

		// Fraction lost
		data[offset+4] = block.FractionLost

		// Cumulative packets lost
		lost := block.CumulativeLost & 0xFFFFFF

		data[offset+5] = byte(lost >> 16)
		data[offset+6] = byte(lost >> 8)
		data[offset+7] = byte(lost)

		// Highest sequence number
		binary.BigEndian.PutUint32(
			data[offset+8:offset+12],
			block.HighestSeqNumber,
		)

		// Jitter
		binary.BigEndian.PutUint32(
			data[offset+12:offset+16],
			block.Jitter,
		)

		// Last Sender Report
		binary.BigEndian.PutUint32(
			data[offset+16:offset+20],
			block.LastSenderReport,
		)

		// Delay Since Last Sender Report
		binary.BigEndian.PutUint32(
			data[offset+20:offset+24],
			block.DelaySinceLastSR,
		)

		offset += ReceiverReportBlockSize
	}

	return data, nil
}

func UnmarshalReceiverReport(data []byte) (*ReceiverReport, error) {
	if len(data) < 8 {
		return nil, errors.New("receiver report too short")
	}

	if data[0]>>6 != 2 {
		return nil, errors.New("unsupported RTCP version")
	}

	if data[1] != ReceiverReportType {
		return nil, errors.New("not a receiver report")
	}

	count := int(data[0] & 0x1F)

	expectedSize := 8 + count*ReceiverReportBlockSize

	if len(data) < expectedSize {
		return nil, errors.New("receiver report is incomplete")
	}

	report := &ReceiverReport{
		SSRC: binary.BigEndian.Uint32(data[4:8]),
		Blocks: make([]ReceiverReportBlock, 0, count),
	}

	offset := 8

	for i := 0; i < count; i++ {
		block := ReceiverReportBlock{
			SSRC: binary.BigEndian.Uint32(
				data[offset : offset+4],
			),

			FractionLost: data[offset+4],

			CumulativeLost: int32(
				int32(data[offset+5])<<16 |
					int32(data[offset+6])<<8 |
					int32(data[offset+7]),
			),

			HighestSeqNumber: binary.BigEndian.Uint32(
				data[offset+8 : offset+12],
			),

			Jitter: binary.BigEndian.Uint32(
				data[offset+12 : offset+16],
			),

			LastSenderReport: binary.BigEndian.Uint32(
				data[offset+16 : offset+20],
			),

			DelaySinceLastSR: binary.BigEndian.Uint32(
				data[offset+20 : offset+24],
			),
		}

		report.Blocks = append(report.Blocks, block)

		offset += ReceiverReportBlockSize
	}

	return report, nil
}