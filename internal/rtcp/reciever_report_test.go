package rtcp

import "testing"

func TestReceiverReport(t *testing.T) {
	report := &ReceiverReport{
		SSRC: 9999,
		Blocks: []ReceiverReportBlock{
			{
				SSRC:              12345,
				FractionLost:      10,
				CumulativeLost:    5,
				HighestSeqNumber:  100,
				Jitter:            250,
				LastSenderReport: 5000,
				DelaySinceLastSR: 100,
			},
		},
	}

	data, err := report.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	parsed, err := UnmarshalReceiverReport(data)
	if err != nil {
		t.Fatal(err)
	}

	if parsed.SSRC != report.SSRC {
		t.Fatalf("receiver SSRC mismatch")
	}

	if len(parsed.Blocks) != 1 {
		t.Fatalf("expected 1 report block, got %d", len(parsed.Blocks))
	}

	block := parsed.Blocks[0]
	expected := report.Blocks[0]

	if block.SSRC != expected.SSRC {
		t.Fatalf("SSRC mismatch")
	}

	if block.FractionLost != expected.FractionLost {
		t.Fatalf("fraction lost mismatch")
	}

	if block.CumulativeLost != expected.CumulativeLost {
		t.Fatalf("cumulative lost mismatch")
	}

	if block.HighestSeqNumber != expected.HighestSeqNumber {
		t.Fatalf("highest sequence mismatch")
	}

	if block.Jitter != expected.Jitter {
		t.Fatalf("jitter mismatch")
	}

	if block.LastSenderReport != expected.LastSenderReport {
		t.Fatalf("LSR mismatch")
	}

	if block.DelaySinceLastSR != expected.DelaySinceLastSR {
		t.Fatalf("DLSR mismatch")
	}
}