package rtp

import (
	"testing"

	"github.com/jaffar066/nexrtc/internal/rtcp"
)

func TestRTCPHandlerNACK(t *testing.T) {
	buffer := NewRetransmissionBuffer(100)

	sender := &mockPacketSender{}

	manager := NewRetransmissionManager(
		buffer,
		sender,
	)

	handler := NewRTCPHandler(manager)

	ssrc := uint32(12345)

	// Store RTP packets.
	for _, sequence := range []uint16{
		100,
		102,
		103,
	} {
		buffer.Store(&Packet{
			Version:        2,
			PayloadType:    96,
			SequenceNumber: sequence,
			Timestamp:      uint32(sequence) * 3000,
			SSRC:           ssrc,
			Payload:        []byte("video packet"),
		})
	}

	// Create NACK.
	nack := &rtcp.NACK{
		SenderSSRC: 9999,
		MediaSSRC:  ssrc,
		Pairs: []rtcp.NACKPair{
			{
				PID: 100,

				// 102 and 103 are missing.
				//
				// bit 1 -> 102
				// bit 2 -> 103
				BLP: (1 << 1) | (1 << 2),
			},
		},
	}

	// Convert NACK into raw RTCP bytes.
	data, err := nack.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	// Feed raw RTCP packet to handler.
	count, err := handler.HandlePacket(data)

	if err != nil {
		t.Fatal(err)
	}

	// Expected:
	//
	// 100
	// 102
	// 103
	//
	// = 3 packets retransmitted

	if count != 3 {
		t.Fatalf(
			"expected 3 retransmitted packets, got %d",
			count,
		)
	}

	if len(sender.packets) != 3 {
		t.Fatalf(
			"expected 3 sent packets, got %d",
			len(sender.packets),
		)
	}

	expected := []uint16{
		100,
		102,
		103,
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