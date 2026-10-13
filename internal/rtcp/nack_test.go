package rtcp

import "testing"

func TestBuildNACKPairs(t *testing.T) {
	missing := []uint16{
		100,
		102,
		103,
		105,
	}

	pairs := BuildNACKPairs(missing)

	if len(pairs) != 1 {
		t.Fatalf("expected 1 NACK pair, got %d", len(pairs))
	}

	if pairs[0].PID != 100 {
		t.Fatalf("expected PID 100, got %d", pairs[0].PID)
	}

	// 102 -> bit 1
	// 103 -> bit 2
	// 105 -> bit 4
	expectedBLP := uint16(
		(1 << 1) |
			(1 << 2) |
			(1 << 4),
	)

	if pairs[0].BLP != expectedBLP {
		t.Fatalf(
			"expected BLP %d, got %d",
			expectedBLP,
			pairs[0].BLP,
		)
	}
}

func TestNACK(t *testing.T) {
	nack := &NACK{
		SenderSSRC: 9999,
		MediaSSRC:  12345,
		Pairs: []NACKPair{
			{
				PID: 100,
				BLP: 0x0016,
			},
		},
	}

	data, err := nack.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	parsed, err := UnmarshalNACK(data)
	if err != nil {
		t.Fatal(err)
	}

	if parsed.SenderSSRC != nack.SenderSSRC {
		t.Fatalf("sender SSRC mismatch")
	}

	if parsed.MediaSSRC != nack.MediaSSRC {
		t.Fatalf("media SSRC mismatch")
	}

	if len(parsed.Pairs) != 1 {
		t.Fatalf("expected 1 pair")
	}

	if parsed.Pairs[0].PID != nack.Pairs[0].PID {
		t.Fatalf("PID mismatch")
	}

	if parsed.Pairs[0].BLP != nack.Pairs[0].BLP {
		t.Fatalf("BLP mismatch")
	}
}