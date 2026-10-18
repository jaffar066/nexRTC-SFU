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

func TestRTCPPacketUsesWireVector(t *testing.T) {
	packet := &Packet{
		Version: 2,
		Type:    200,
		Payload: []byte{1, 2, 3, 4},
	}
	want := []byte{0x80, 200, 0, 1, 1, 2, 3, 4}

	data, err := packet.Marshal()
	if err != nil {
		t.Fatalf("marshal packet: %v", err)
	}
	if !bytes.Equal(data, want) {
		t.Fatalf("wire bytes = %v, want %v", data, want)
	}
	parsed, err := Unmarshal(want)
	if err != nil {
		t.Fatalf("unmarshal wire vector: %v", err)
	}
	if parsed.Type != 200 || !bytes.Equal(parsed.Payload, []byte{1, 2, 3, 4}) {
		t.Fatalf("parsed packet = %+v", parsed)
	}
}

func TestRTCPUnmarshalRejectsMalformedPackets(t *testing.T) {
	tests := []struct {
		name string
		data []byte
	}{
		{name: "short header", data: []byte{0x80, 200, 0}},
		{name: "unsupported version", data: []byte{0x40, 200, 0, 0}},
		{name: "declared length exceeds input", data: []byte{0x80, 200, 0, 2}},
		{name: "trailing bytes", data: []byte{0x80, 200, 0, 0, 0, 0, 0, 0}},
		{name: "zero padding", data: []byte{0xA0, 200, 0, 1, 0, 0, 0, 0}},
		{name: "padding exceeds payload", data: []byte{0xA0, 200, 0, 1, 0, 0, 0, 5}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if packet, err := Unmarshal(test.data); err == nil {
				t.Fatalf("Unmarshal() = %+v, want an error", packet)
			}
		})
	}
}

func TestRTCPPacketPaddingRoundTrip(t *testing.T) {
	packet := &Packet{
		Version: 2,
		Padding: true,
		Type:    200,
		Payload: []byte{1, 2, 3},
	}
	data, err := packet.Marshal()
	if err != nil {
		t.Fatalf("marshal padded packet: %v", err)
	}
	parsed, err := Unmarshal(data)
	if err != nil {
		t.Fatalf("unmarshal padded packet: %v", err)
	}
	if parsed.PaddingSize != 1 || !bytes.Equal(parsed.Payload, packet.Payload) {
		t.Fatalf("parsed packet = %+v", parsed)
	}
}

func TestRTCPUnmarshalCompound(t *testing.T) {
	first := &Packet{Version: 2, Type: 200, Payload: []byte{1, 2, 3, 4}}
	last := &Packet{Version: 2, Padding: true, Type: 201, Payload: []byte{5, 6, 7, 8}}
	firstData, err := first.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	lastData, err := last.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	compound := append(firstData, lastData...)

	packets, err := UnmarshalCompound(compound)
	if err != nil {
		t.Fatalf("unmarshal compound packet: %v", err)
	}
	if len(packets) != 2 ||
		packets[0].Type != first.Type ||
		packets[1].Type != last.Type ||
		!packets[1].Padding {
		t.Fatalf("parsed compound packets = %+v", packets)
	}

	if _, err := Unmarshal(compound); err == nil {
		t.Fatal("Unmarshal accepted trailing compound packet data")
	}
	if _, err := UnmarshalCompound(append(lastData, firstData...)); err == nil {
		t.Fatal("compound parser accepted padding before a later packet")
	}
}

func FuzzRTCPUnmarshalNeverPanics(f *testing.F) {
	f.Add([]byte{})
	f.Add([]byte{0x80, 200, 0, 1, 1, 2, 3, 4})
	f.Add([]byte{0xA0, 200, 0, 1, 0, 0, 0, 4})
	f.Fuzz(func(t *testing.T, data []byte) {
		packet, err := Unmarshal(data)
		if err != nil {
			return
		}
		encoded, err := packet.Marshal()
		if err != nil {
			t.Fatalf("marshal parsed RTCP packet: %v", err)
		}
		if _, err := Unmarshal(encoded); err != nil {
			t.Fatalf("unmarshal serialized RTCP packet: %v", err)
		}
		_, _ = UnmarshalCompound(data)
	})
}
