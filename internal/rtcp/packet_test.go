package rtcp

import (
	"bytes"
	"testing"
)

func TestRTCPPacket(t *testing.T) {
	packet := &Packet{
		Version: 2,
		Type:    200,
		Count:   0,
		Payload: []byte{1, 2, 3, 4},
	}

	data, err := packet.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	parsed, err := Unmarshal(data)
	if err != nil {
		t.Fatal(err)
	}

	if parsed.Version != 2 {
		t.Fatalf("expected version 2, got %d", parsed.Version)
	}

	if parsed.Type != 200 {
		t.Fatalf("expected type 200, got %d", parsed.Type)
	}

	if !bytes.Equal(parsed.Payload, packet.Payload) {
		t.Fatalf("payload mismatch")
	}
}