package rtcp

import "testing"

func TestReceiverReport(t *testing.T) {
	report := &ReceiverReport{
		SSRC: 9999,
		Blocks: []ReceiverReportBlock{
			{
				SSRC:             12345,
				FractionLost:     10,
				CumulativeLost:   5,
				HighestSeqNumber: 100,
				Jitter:           250,
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

func TestReceiverReportSignedCumulativeLoss(t *testing.T) {
	tests := []int32{-1, -8388608, 0, 8388607}
	for _, lost := range tests {
		report := &ReceiverReport{
			SSRC: 1,
			Blocks: []ReceiverReportBlock{
				{SSRC: 2, CumulativeLost: lost},
			},
		}
		data, err := report.Marshal()
		if err != nil {
			t.Fatalf("marshal cumulative loss %d: %v", lost, err)
		}
		parsed, err := UnmarshalReceiverReport(data)
		if err != nil {
			t.Fatalf("unmarshal cumulative loss %d: %v", lost, err)
		}
		if got := parsed.Blocks[0].CumulativeLost; got != lost {
			t.Errorf("cumulative loss = %d, want %d", got, lost)
		}
	}
}

func TestReceiverReportRejectsCountLengthMismatch(t *testing.T) {
	data := make([]byte, 8)
	data[0] = 2<<6 | 1
	data[1] = ReceiverReportType
	data[3] = 1
	if _, err := UnmarshalReceiverReport(data); err == nil {
		t.Fatal("expected error for report count/length mismatch")
	}
}

func TestReceiverReportMarshalRejectsOutOfRangeCumulativeLoss(t *testing.T) {
	for _, lost := range []int32{-8388609, 8388608} {
		report := &ReceiverReport{
			Blocks: []ReceiverReportBlock{{CumulativeLost: lost}},
		}
		if _, err := report.Marshal(); err == nil {
			t.Errorf("Marshal() accepted cumulative loss %d", lost)
		}
	}
}
