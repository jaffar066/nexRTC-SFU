package rtp

import (
	"fmt"

	"github.com/jaffar066/nexrtc/internal/rtcp"
)

type ReceiverReportHandler struct {
	Streams *StreamManager
}

func NewReceiverReportHandler(
	streams *StreamManager,
) *ReceiverReportHandler {
	return &ReceiverReportHandler{
		Streams: streams,
	}
}

// ReceiverReportStats contains the latest RTCP
// Receiver Report information for an RTP stream.
type ReceiverReportStats struct {
	SSRC uint32

	FractionLost   uint8
	CumulativeLost int32

	HighestSeqNumber uint32
	Jitter           uint32

	LastSenderReport uint32
	DelaySinceLastSR uint32
}

// HandlePacket receives a raw RTCP Receiver Report.
func (h *ReceiverReportHandler) HandlePacket(
	data []byte,
) ([]ReceiverReportStats, error) {

	if len(data) < rtcp.HeaderSize {
		return nil, fmt.Errorf(
			"RTCP packet too short",
		)
	}

	if data[1] != rtcp.ReceiverReportType {
		return nil, fmt.Errorf(
			"not a Receiver Report",
		)
	}

	report, err := rtcp.UnmarshalReceiverReport(data)

	if err != nil {
		return nil, fmt.Errorf(
			"failed to parse Receiver Report: %w",
			err,
		)
	}

	if h.Streams == nil {
		return nil, fmt.Errorf(
			"stream manager is nil",
		)
	}

	stats := make(
		[]ReceiverReportStats,
		0,
		len(report.Blocks),
	)

	for _, block := range report.Blocks {

		// Find existing stream or create a new one.
		stream := h.Streams.GetOrCreate(
			block.SSRC,
			90000,
		)

		// Store latest RTCP statistics inside the stream.
		stream.RTCPStats = RTCPStats{
			FractionLost:     block.FractionLost,
			CumulativeLost:   block.CumulativeLost,
			HighestSeqNumber: block.HighestSeqNumber,
			Jitter:           block.Jitter,
			LastSenderReport: block.LastSenderReport,
			DelaySinceLastSR: block.DelaySinceLastSR,
		}

		// Also return the statistics to the caller.
		stats = append(
			stats,
			ReceiverReportStats{
				SSRC:              block.SSRC,
				FractionLost:      block.FractionLost,
				CumulativeLost:    block.CumulativeLost,
				HighestSeqNumber:  block.HighestSeqNumber,
				Jitter:            block.Jitter,
				LastSenderReport: block.LastSenderReport,
				DelaySinceLastSR: block.DelaySinceLastSR,
			},
		)
	}

	return stats, nil
}