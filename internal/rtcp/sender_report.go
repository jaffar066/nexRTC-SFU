package rtcp

import (
	"encoding/binary"
	"errors"
)

const SenderReportType uint8 = 200

const SenderReportSize = 28

type SenderReport struct {
	SSRC uint32

	NTPSec  uint32
	NTPFrac uint32

	RTPTimestamp uint32

	SenderPacketCount uint32
	SenderOctetCount  uint32
}

func (sr *SenderReport) Marshal() ([]byte, error) {
	data := make([]byte, SenderReportSize)

	// Version = 2
	data[0] = 2 << 6

	// Report Count = 0
	data[0] |= 0

	// Packet Type = 200
	data[1] = SenderReportType

	// RTCP length:
	// (28 / 4) - 1 = 6
	binary.BigEndian.PutUint16(data[2:4], 6)

	// SSRC
	binary.BigEndian.PutUint32(data[4:8], sr.SSRC)

	// NTP timestamp
	binary.BigEndian.PutUint32(data[8:12], sr.NTPSec)
	binary.BigEndian.PutUint32(data[12:16], sr.NTPFrac)

	// RTP timestamp
	binary.BigEndian.PutUint32(data[16:20], sr.RTPTimestamp)

	// Sender packet count
	binary.BigEndian.PutUint32(data[20:24], sr.SenderPacketCount)

	// Sender octet count
	binary.BigEndian.PutUint32(data[24:28], sr.SenderOctetCount)

	return data, nil
}

func UnmarshalSenderReport(data []byte) (*SenderReport, error) {
	if len(data) < SenderReportSize {
		return nil, errors.New("sender report too short")
	}

	if data[0]>>6 != 2 {
		return nil, errors.New("unsupported RTCP version")
	}

	if data[1] != SenderReportType {
		return nil, errors.New("not a sender report")
	}

	length := binary.BigEndian.Uint16(data[2:4])

	if length < 6 {
		return nil, errors.New("invalid sender report length")
	}

	return &SenderReport{
		SSRC:              binary.BigEndian.Uint32(data[4:8]),
		NTPSec:            binary.BigEndian.Uint32(data[8:12]),
		NTPFrac:           binary.BigEndian.Uint32(data[12:16]),
		RTPTimestamp:      binary.BigEndian.Uint32(data[16:20]),
		SenderPacketCount: binary.BigEndian.Uint32(data[20:24]),
		SenderOctetCount:  binary.BigEndian.Uint32(data[24:28]),
	}, nil
}