package rtp

import (
	"testing"

	"github.com/jaffar066/nexrtc/internal/rtcp"
)

type mockPacketSender struct {
	packets []*Packet
}

func (m *mockPacketSender) Send(packet *Packet) error {
	m.packets = append(m.packets, packet)
	return nil
}

func TestExpandNACKPair(t *testing.T) {
	pair := rtcp.NACKPair{
		PID: 100,
		BLP: 0,
	}

	// 102, 103 and 105 are missing.
	pair.BLP |= 1 << 1
	pair.BLP |= 1 << 2
	pair.BLP |= 1 << 4

	sequences := ExpandNACKPair(pair)

	expected := []uint16{
		100,
		102,
		103,
		105,
	}

	if len(sequences) != len(expected) {
		t.Fatalf(
			"expected %d sequences, got %d",
			len(expected),
			len(sequences),
		)
	}

	for i := range expected {
		if sequences[i] != expected[i] {
			t.Fatalf(
				"expected sequence %d at index %d, got %d",
				expected[i],
				i,
				sequences[i],
			)
		}
	}
}

func TestRetransmissionManager(t *testing.T) {
	buffer := NewRetransmissionBuffer(100)

	sender := &mockPacketSender{}

	manager := NewRetransmissionManager(
		buffer,
		sender,
	)

	ssrc := uint32(12345)

	// Store packets 100, 102, 103 and 105.
	sequences := []uint16{
		100,
		102,
		103,
		105,
	}

	for _, sequence := range sequences {
		packet := &Packet{
			Version:        2,
			PayloadType:    96,
			SequenceNumber: sequence,
			Timestamp:      uint32(sequence) * 3000,
			SSRC:           ssrc,
			Payload:        []byte("video packet"),
		}

		buffer.Store(packet)
	}

	// NACK:
	//
	// PID = 100
	//
	// Missing:
	// 100
	// 102
	// 103
	// 105
	//
	// BLP bits:
	// bit 1 -> 102
	// bit 2 -> 103
	// bit 4 -> 105

	nack := &rtcp.NACK{
		SenderSSRC: 9999,
		MediaSSRC:  ssrc,
		Pairs: []rtcp.NACKPair{
			{
				PID: 100,
				BLP: (1 << 1) |
					(1 << 2) |
					(1 << 4),
			},
		},
	}

	count, err := manager.HandleNACK(
		ssrc,
		nack,
	)

	if err != nil {
		t.Fatal(err)
	}

	if count != 4 {
		t.Fatalf(
			"expected 4 retransmitted packets, got %d",
			count,
		)
	}

	if len(sender.packets) != 4 {
		t.Fatalf(
			"expected 4 packets sent, got %d",
			len(sender.packets),
		)
	}

	expected := []uint16{
		100,
		102,
		103,
		105,
	}

	for i, packet := range sender.packets {
		if packet.SequenceNumber != expected[i] {
			t.Fatalf(
				"expected sequence %d, got %d",
				expected[i],
				packet.SequenceNumber,
			)
		}
	}
}

func TestRetransmissionManagerMissingPacket(t *testing.T) {
	buffer := NewRetransmissionBuffer(100)

	sender := &mockPacketSender{}

	manager := NewRetransmissionManager(
		buffer,
		sender,
	)

	ssrc := uint32(12345)

	// Only packet 100 exists.
	buffer.Store(&Packet{
		Version:        2,
		PayloadType:    96,
		SequenceNumber: 100,
		SSRC:           ssrc,
		Payload:        []byte("packet 100"),
	})

	// Request 100, 101 and 102.
	nack := &rtcp.NACK{
		MediaSSRC: ssrc,
		Pairs: []rtcp.NACKPair{
			{
				PID: 100,
				BLP: 0b0000000000000011,
			},
		},
	}

	count, err := manager.HandleNACK(
		ssrc,
		nack,
	)

	if err != nil {
		t.Fatal(err)
	}

	// Only packet 100 exists.
	if count != 1 {
		t.Fatalf(
			"expected 1 retransmitted packet, got %d",
			count,
		)
	}
}