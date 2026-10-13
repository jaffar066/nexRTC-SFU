package rtcp

import "testing"

func TestSenderReport(t *testing.T) {
	report := &SenderReport{
		SSRC:              12345,
		NTPSec:            1000,
		NTPFrac:           500,
		RTPTimestamp:      90000,
		SenderPacketCount: 1500,
		SenderOctetCount: 500000,
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
}