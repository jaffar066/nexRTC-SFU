package rtp

import (
	"bytes"
	"encoding/binary"
	"testing"
)

func TestRTPPacketRoundTripCSRCsExtensionsAndPadding(t *testing.T) {
	original := &Packet{
		Version:          2,
		Padding:          true,
		Extension:        true,
		CSRCs:            []uint32{0x01020304, 0xA0B0C0D0},
		ExtensionProfile: 0xBEDE,
		ExtensionData:    []byte{1, 2, 3, 4, 5, 6, 7, 8},
		PaddingSize:      4,
		Marker:           true,
		PayloadType:      111,
		SequenceNumber:   0x1234,
		Timestamp:        0x10203040,
		SSRC:             0x50607080,
		Payload:          []byte("encoded media"),
	}

	data, err := original.Marshal()
	if err != nil {
		t.Fatalf("marshal packet: %v", err)
	}
	packet, err := Unmarshal(data)
	if err != nil {
		t.Fatalf("unmarshal packet: %v", err)
	}

	if packet.Version != original.Version ||
		packet.Padding != original.Padding ||
		packet.Extension != original.Extension ||
		packet.CSRCCount != uint8(len(original.CSRCs)) ||
		packet.ExtensionProfile != original.ExtensionProfile ||
		packet.PaddingSize != original.PaddingSize ||
		packet.Marker != original.Marker ||
		packet.PayloadType != original.PayloadType ||
		packet.SequenceNumber != original.SequenceNumber ||
		packet.Timestamp != original.Timestamp ||
		packet.SSRC != original.SSRC {
		t.Fatalf("packet header does not round trip: %+v", packet)
	}
	if !equalUint32s(packet.CSRCs, original.CSRCs) {
		t.Fatalf("CSRCs = %v, want %v", packet.CSRCs, original.CSRCs)
	}
	if !bytes.Equal(packet.ExtensionData, original.ExtensionData) {
		t.Fatalf("extension data = %v, want %v", packet.ExtensionData, original.ExtensionData)
	}
	if !bytes.Equal(packet.Payload, original.Payload) {
		t.Fatalf("payload = %q, want %q", packet.Payload, original.Payload)
	}

	remarshaled, err := packet.Marshal()
	if err != nil {
		t.Fatalf("remarshal packet: %v", err)
	}
	reparsed, err := Unmarshal(remarshaled)
	if err != nil {
		t.Fatalf("reparse remarshal: %v", err)
	}
	if !equalUint32s(reparsed.CSRCs, packet.CSRCs) ||
		!bytes.Equal(reparsed.ExtensionData, packet.ExtensionData) ||
		!bytes.Equal(reparsed.Payload, packet.Payload) ||
		reparsed.PaddingSize != packet.PaddingSize {
		t.Fatalf("packet components changed after remarshal: %+v", reparsed)
	}
}

func TestRTPUnmarshalRejectsMalformedHeaders(t *testing.T) {
	validHeader := make([]byte, HeaderSize)
	validHeader[0] = rtpVersion << 6

	tests := []struct {
		name string
		data []byte
	}{
		{name: "short fixed header", data: validHeader[:HeaderSize-1]},
		{name: "unsupported version", data: append([]byte{1 << 6}, validHeader[1:]...)},
		{
			name: "incomplete CSRC list",
			data: append([]byte{(rtpVersion << 6) | 1}, validHeader[1:]...),
		},
		{
			name: "incomplete extension header",
			data: append([]byte{(rtpVersion << 6) | 0x10}, validHeader[1:]...),
		},
		{
			name: "incomplete extension data",
			data: append(append([]byte{(rtpVersion << 6) | 0x10}, validHeader[1:]...), 0xBE, 0xDE, 0, 1),
		},
		{
			name: "zero padding length",
			data: append([]byte{(rtpVersion << 6) | 0x20}, validHeader[1:]...),
		},
		{
			name: "padding exceeds packet payload",
			data: append(append([]byte{(rtpVersion << 6) | 0x20}, validHeader[1:]...), 13),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if packet, err := Unmarshal(test.data); err == nil {
				t.Fatalf("Unmarshal() = %+v, want an error", packet)
			}
		})
	}
}

func TestRTPMarshalRejectsInvalidFields(t *testing.T) {
	tests := []struct {
		name   string
		packet *Packet
	}{
		{name: "nil packet", packet: nil},
		{name: "unsupported version", packet: &Packet{Version: 1}},
		{name: "payload type out of range", packet: &Packet{Version: 2, PayloadType: 128}},
		{
			name: "CSRC count mismatch",
			packet: &Packet{
				Version:   2,
				CSRCCount: 1,
			},
		},
		{
			name: "too many CSRCs",
			packet: &Packet{
				Version: 2,
				CSRCs:   make([]uint32, 16),
			},
		},
		{
			name: "extension bytes without extension flag",
			packet: &Packet{
				Version:       2,
				ExtensionData: []byte{1, 2, 3, 4},
			},
		},
		{
			name: "unaligned extension data",
			packet: &Packet{
				Version:       2,
				Extension:     true,
				ExtensionData: []byte{1},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if data, err := test.packet.Marshal(); err == nil {
				t.Fatalf("Marshal() = %v, want an error", data)
			}
		})
	}
}

func FuzzRTPUnmarshalNeverPanics(f *testing.F) {
	f.Add([]byte{})
	f.Add(make([]byte, HeaderSize))
	f.Add([]byte{
		0x90, 0x60, 0, 1, 0, 0, 0, 1, 0, 0, 0, 1,
		0xBE, 0xDE, 0, 1, 1, 2, 3, 4,
	})
	f.Add([]byte{
		0xA0, 0x60, 0, 1, 0, 0, 0, 1, 0, 0, 0, 1,
		0x01, 0x02, 0x03, 0x04,
	})

	f.Fuzz(func(t *testing.T, data []byte) {
		packet, err := Unmarshal(data)
		if err != nil {
			return
		}

		encoded, err := packet.Marshal()
		if err != nil {
			t.Fatalf("marshal parsed packet: %v", err)
		}
		reparsed, err := Unmarshal(encoded)
		if err != nil {
			t.Fatalf("unmarshal marshaled packet: %v", err)
		}
		if reparsed.SSRC != packet.SSRC ||
			reparsed.SequenceNumber != packet.SequenceNumber ||
			reparsed.Timestamp != packet.Timestamp ||
			reparsed.PayloadType != packet.PayloadType ||
			reparsed.PaddingSize != packet.PaddingSize {
			t.Fatalf("packet fields changed after round trip: before=%+v after=%+v", packet, reparsed)
		}
	})
}

func equalUint32s(left, right []uint32) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}

func TestRTPUnmarshalExtensionLengthUsesWords(t *testing.T) {
	data := make([]byte, HeaderSize+4+8+3)
	data[0] = rtpVersion<<6 | 0x10
	binary.BigEndian.PutUint16(data[HeaderSize:HeaderSize+2], 0x1000)
	binary.BigEndian.PutUint16(data[HeaderSize+2:HeaderSize+4], 2)
	copy(data[HeaderSize+4:], []byte{9, 8, 7, 6, 5, 4, 3, 2, 1, 2, 3})

	packet, err := Unmarshal(data)
	if err != nil {
		t.Fatalf("unmarshal extension packet: %v", err)
	}
	if packet.ExtensionProfile != 0x1000 {
		t.Fatalf("extension profile = 0x%04x, want 0x1000", packet.ExtensionProfile)
	}
	if !bytes.Equal(packet.ExtensionData, []byte{9, 8, 7, 6, 5, 4, 3, 2}) {
		t.Fatalf("extension data = %v", packet.ExtensionData)
	}
	if !bytes.Equal(packet.Payload, []byte{1, 2, 3}) {
		t.Fatalf("payload = %v, want [1 2 3]", packet.Payload)
	}
}
