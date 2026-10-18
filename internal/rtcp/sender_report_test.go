package rtcp

import "testing"

func TestSenderReport(t *testing.T) {
	report := &SenderReport{
		SSRC:              12345,
		NTPSec:            1000,
		NTPFrac:           500,
		RTPTimestamp:      90000,
		SenderPacketCount: 1500,
		SenderOctetCount:  500000,
		Blocks: []ReceiverReportBlock{
			{
				SSRC:             54321,
				FractionLost:     4,
				CumulativeLost:   -3,
				HighestSeqNumber: 900,
				Jitter:           7,
				LastSenderReport: 8,
				DelaySinceLastSR: 9,
			},
		},
	}

	data, err := report.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	parsed, err := UnmarshalSenderReport(data)
	if err != nil {
		t.Fatal(err)
	}

	if parsed.SSRC != report.SSRC {
		t.Fatalf("SSRC mismatch")
	}

	if parsed.NTPSec != report.NTPSec {
		t.Fatalf("NTP seconds mismatch")
	}

	if parsed.NTPFrac != report.NTPFrac {
		t.Fatalf("NTP fraction mismatch")
	}

	if parsed.RTPTimestamp != report.RTPTimestamp {
		t.Fatalf("RTP timestamp mismatch")
	}

	if parsed.SenderPacketCount != report.SenderPacketCount {
		t.Fatalf("packet count mismatch")
	}

	if parsed.SenderOctetCount != report.SenderOctetCount {
		t.Fatalf("octet count mismatch")
	}
	if len(parsed.Blocks) != 1 || parsed.Blocks[0] != report.Blocks[0] {
		t.Fatalf("report blocks = %+v, want %+v", parsed.Blocks, report.Blocks)
	}
}

func TestSenderReportRejectsCountLengthMismatch(t *testing.T) {
	data := make([]byte, SenderReportSize)
	data[0] = 2<<6 | 1
	data[1] = SenderReportType
	data[3] = 6
	if _, err := UnmarshalSenderReport(data); err == nil {
		t.Fatal("expected error for sender report count/length mismatch")
	}
}
