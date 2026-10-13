package rtp

import (
	"net"
	"testing"
	"time"

	"github.com/jaffar066/nexrtc/internal/rtcp"
)

func TestNACKTriggersRTPRetransmission(t *testing.T) {
	// Reserve an ephemeral UDP address, then bind the receiver to it.
	probe, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP("127.0.0.1")})
	if err != nil {
		t.Fatalf("failed to find receiver address: %v", err)
	}
	receiverAddr := probe.LocalAddr().String()
	if err := probe.Close(); err != nil {
		t.Fatalf("failed to release receiver address: %v", err)
	}

	// Receiver UDP socket.
	receiver, err := NewUDPReceiver(receiverAddr)
	if err != nil {
		t.Fatalf("failed to create receiver: %v", err)
	}
	defer receiver.Close()

	// Sender UDP socket.
	sender, err := NewUDPSender(
		receiverAddr,
	)
	if err != nil {
		t.Fatalf("failed to create sender: %v", err)
	}
	defer sender.Close()

	// Original RTP packet.
	original := &Packet{
		Version:        2,
		PayloadType:    96,
		SequenceNumber: 102,
		Timestamp:      9000,
		SSRC:           12345,
		Payload:        []byte("video-frame-102"),
	}

	// Store packet for possible retransmission.
	buffer := NewRetransmissionBuffer(100)

	buffer.Store(original)

	// Retransmission manager.
	manager := NewRetransmissionManager(
		buffer,
		sender,
	)

	// Build NACK requesting packet 102.
	nack := rtcp.NACK{
		SenderSSRC: 99999,
		MediaSSRC:  12345,
		Pairs: []rtcp.NACKPair{
			{
				PID: 102,
				BLP: 0,
			},
		},
	}

	// Receive retransmitted RTP packet in background.
	received := make(chan *Packet, 1)
	errors := make(chan error, 1)

	go func() {
		packet, _, err := receiver.Receive()
		if err != nil {
			errors <- err
			return
		}

		received <- packet
	}()

	// Ask retransmission manager to resend packet 102.
	_, err = manager.HandleNACK(
		nack.MediaSSRC,
		&nack,
	)

	if err != nil {
		t.Fatalf("failed to handle NACK: %v", err)
	}

	select {
	case packet := <-received:

		if packet.SequenceNumber != 102 {
			t.Fatalf(
				"expected sequence 102, got %d",
				packet.SequenceNumber,
			)
		}

		if packet.SSRC != 12345 {
			t.Fatalf(
				"expected SSRC 12345, got %d",
				packet.SSRC,
			)
		}

		if string(packet.Payload) != "video-frame-102" {
			t.Fatalf(
				"unexpected payload: %s",
				packet.Payload,
			)
		}

	case err := <-errors:
		t.Fatalf("receiver error: %v", err)

	case <-time.After(time.Second):
		t.Fatal("timed out waiting for retransmitted RTP packet")
	}
}
