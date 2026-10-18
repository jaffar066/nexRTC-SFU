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

	Blocks []ReceiverReportBlock
}

func (sr *SenderReport) Marshal() ([]byte, error) {
	if sr == nil {
		return nil, errors.New("sender report is nil")
	}
	if len(sr.Blocks) > 31 {
		return nil, errors.New("too many sender report blocks")
	}
	payload := make([]byte, 24+len(sr.Blocks)*ReceiverReportBlockSize)
	binary.BigEndian.PutUint32(payload[0:4], sr.SSRC)
	binary.BigEndian.PutUint32(payload[4:8], sr.NTPSec)
	binary.BigEndian.PutUint32(payload[8:12], sr.NTPFrac)
	binary.BigEndian.PutUint32(payload[12:16], sr.RTPTimestamp)
	binary.BigEndian.PutUint32(payload[16:20], sr.SenderPacketCount)
	binary.BigEndian.PutUint32(payload[20:24], sr.SenderOctetCount)

	offset := 24
	for _, block := range sr.Blocks {
		marshalReceiverReportBlock(payload[offset:offset+ReceiverReportBlockSize], block)
		offset += ReceiverReportBlockSize
	}

	return (&Packet{
		Version: 2,
		Count:   uint8(len(sr.Blocks)),
		Type:    SenderReportType,
		Payload: payload,
	}).Marshal()
}

func UnmarshalSenderReport(data []byte) (*SenderReport, error) {
	packet, err := Unmarshal(data)
	if err != nil {
		return nil, err
	}
	if packet.Type != SenderReportType {
		return nil, errors.New("not a sender report")
	}
	expectedPayloadSize := 24 + int(packet.Count)*ReceiverReportBlockSize
	if len(packet.Payload) != expectedPayloadSize {
		return nil, errors.New("sender report payload length does not match report count")
	}

	report := &SenderReport{
		SSRC:              binary.BigEndian.Uint32(packet.Payload[0:4]),
		NTPSec:            binary.BigEndian.Uint32(packet.Payload[4:8]),
		NTPFrac:           binary.BigEndian.Uint32(packet.Payload[8:12]),
		RTPTimestamp:      binary.BigEndian.Uint32(packet.Payload[12:16]),
		SenderPacketCount: binary.BigEndian.Uint32(packet.Payload[16:20]),
		SenderOctetCount:  binary.BigEndian.Uint32(packet.Payload[20:24]),
		Blocks:            make([]ReceiverReportBlock, 0, packet.Count),
	}
	for offset := 24; offset < len(packet.Payload); offset += ReceiverReportBlockSize {
		report.Blocks = append(report.Blocks, unmarshalReceiverReportBlock(packet.Payload[offset:offset+ReceiverReportBlockSize]))
	}
	return report, nil
}
