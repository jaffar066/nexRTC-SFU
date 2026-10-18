package rtcp

import (
	"bytes"
	"testing"
)

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

func TestBuildNACKPairsAcrossSequenceWrap(t *testing.T) {
	pairs := BuildNACKPairs([]uint16{65534, 0, 65535, 1, 1})
	want := []NACKPair{{PID: 65534, BLP: 0b0000000000000111}}
	if len(pairs) != len(want) || pairs[0] != want[0] {
		t.Fatalf("NACK pairs = %+v, want %+v", pairs, want)
	}
}

func TestBuildNACKPairsStartsNewPairAfterBLPWindow(t *testing.T) {
	pairs := BuildNACKPairs([]uint16{65530, 10, 65531, 11, 12})
	want := []NACKPair{
		{PID: 65530, BLP: 1 | (1 << 15)},
		{PID: 11, BLP: 1},
	}
	if len(pairs) != len(want) {
		t.Fatalf("NACK pairs = %+v, want %+v", pairs, want)
	}
	for i := range want {
		if pairs[i] != want[i] {
			t.Fatalf("NACK pair %d = %+v, want %+v", i, pairs[i], want[i])
		}
	}
}

func TestNACKWireVector(t *testing.T) {
	nack := &NACK{
		SenderSSRC: 0x01020304,
		MediaSSRC:  0x05060708,
		Pairs:      []NACKPair{{PID: 0xFFFE, BLP: 0x0003}},
	}
	want := []byte{
		0x81, 205, 0, 3,
		1, 2, 3, 4,
		5, 6, 7, 8,
		0xFF, 0xFE, 0, 3,
	}

	data, err := nack.Marshal()
	if err != nil {
		t.Fatalf("marshal NACK: %v", err)
	}
	if !bytes.Equal(data, want) {
		t.Fatalf("NACK wire bytes = %v, want %v", data, want)
	}
	parsed, err := UnmarshalNACK(want)
	if err != nil {
		t.Fatalf("unmarshal NACK wire vector: %v", err)
	}
	if parsed.SenderSSRC != nack.SenderSSRC ||
		parsed.MediaSSRC != nack.MediaSSRC ||
		len(parsed.Pairs) != 1 ||
		parsed.Pairs[0] != nack.Pairs[0] {
		t.Fatalf("parsed NACK = %+v", parsed)
	}
}

func TestUnmarshalNACKRejectsInvalidPackets(t *testing.T) {
	tests := []struct {
		name string
		data []byte
	}{
		{name: "wrong packet type", data: []byte{0x81, 200, 0, 3, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}},
		{name: "wrong FMT", data: []byte{0x82, 205, 0, 3, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}},
		{name: "no feedback pair", data: []byte{0x81, 205, 0, 2, 0, 0, 0, 0, 0, 0, 0, 0}},
		{name: "misaligned feedback payload", data: []byte{0x81, 205, 0, 3, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if nack, err := UnmarshalNACK(test.data); err == nil {
				t.Fatalf("UnmarshalNACK() = %+v, want an error", nack)
			}
		})
	}
}
