package rtp

import (
	"testing"

	"github.com/jaffar066/nexrtc/internal/rtcp"
)

func TestReceiverReportHandler(t *testing.T) {
	streams := NewStreamManager()

	handler := NewReceiverReportHandler(streams)

	report := &rtcp.ReceiverReport{
		SSRC: 9999,

		Blocks: []rtcp.ReceiverReportBlock{
			{
				SSRC:              12345,
				FractionLost:      10,
				CumulativeLost:    25,
				HighestSeqNumber:  5000,
				Jitter:            1800,
				LastSenderReport: 100,
				DelaySinceLastSR:  200,
			},
		},
	}

	data, err := report.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	stats, err := handler.HandlePacket(data)

	if err != nil {
		t.Fatal(err)
	}

	if len(stats) != 1 {
		t.Fatalf(
			"expected 1 report block, got %d",
			len(stats),
		)
	}

	result := stats[0]

	if result.SSRC != 12345 {
		t.Fatalf(
			"expected SSRC 12345, got %d",
			result.SSRC,
		)
	}

	if result.FractionLost != 10 {
		t.Fatalf(
			"expected fraction lost 10, got %d",
			result.FractionLost,
		)
	}

	if result.CumulativeLost != 25 {
		t.Fatalf(
			"expected cumulative lost 25, got %d",
			result.CumulativeLost,
		)
	}

	if result.HighestSeqNumber != 5000 {
		t.Fatalf(
			"expected highest sequence 5000, got %d",
			result.HighestSeqNumber,
		)
	}

	if result.Jitter != 1800 {
		t.Fatalf(
			"expected jitter 1800, got %d",
			result.Jitter,
		)
	}

	if result.LastSenderReport != 100 {
		t.Fatalf(
			"expected LSR 100, got %d",
			result.LastSenderReport,
		)
	}

	if result.DelaySinceLastSR != 200 {
		t.Fatalf(
			"expected DLSR 200, got %d",
			result.DelaySinceLastSR,
		)
	}
}