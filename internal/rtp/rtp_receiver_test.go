package rtp

import (
	"testing"
	"time"
)

func TestRTPReceiveLoopDetectsMissingPacket(t *testing.T) {
	// -----------------------------------------
	// Receiver
	// -----------------------------------------

	receiver, err := NewUDPReceiver("127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to create receiver: %v", err)
	}
	defer receiver.Close()

	// -----------------------------------------
	// Stream manager
	// -----------------------------------------

	scheduler := NewNACKScheduler(
		NACKSchedulerConfig{
			InitialDelay:  10 * time.Millisecond,
			RetryInterval: 20 * time.Millisecond,
			MaxRetries:    3,
		},
	)

	streams := NewStreamManagerWithNACKScheduler(
		scheduler,
	)

	// -----------------------------------------
	// RTP receive loop
	// -----------------------------------------

	loop := NewRTPReceiveLoop(
		receiver.Conn,
		streams,
		90000,
	)

	// -----------------------------------------
	// Sender
	// -----------------------------------------

	sender, err := NewUDPSender(
		receiver.Conn.LocalAddr().String(),
	)
	if err != nil {
		t.Fatalf("failed to create sender: %v", err)
	}
	defer sender.Close()

	// -----------------------------------------
	// Send packet 100
	// -----------------------------------------

	packet100 := &Packet{
		Version:        2,
		PayloadType:    96,
		SequenceNumber: 100,
		Timestamp:      9000,
		SSRC:           12345,
		Payload:        []byte("frame-100"),
	}

	// -----------------------------------------
	// Send packet 101
	// -----------------------------------------

	packet101 := &Packet{
		Version:        2,
		PayloadType:    96,
		SequenceNumber: 101,
		Timestamp:      9900,
		SSRC:           12345,
		Payload:        []byte("frame-101"),
	}

	// -----------------------------------------
	// Skip packet 102
	// -----------------------------------------

	// Send packet 103
	packet103 := &Packet{
		Version:        2,
		PayloadType:    96,
		SequenceNumber: 103,
		Timestamp:      11700,
		SSRC:           12345,
		Payload:        []byte("frame-103"),
	}

	// -----------------------------------------
	// Receive packets in background
	// -----------------------------------------

	received := make(chan *Packet, 3)
	errors := make(chan error, 1)

	go func() {
		for i := 0; i < 3; i++ {
			packet, err := loop.ReceiveOnce()
			if err != nil {
				errors <- err
				return
			}

			received <- packet
		}
	}()

	// -----------------------------------------
	// Send RTP packets
	// -----------------------------------------

	if err := sender.Send(packet100); err != nil {
		t.Fatalf("failed to send packet 100: %v", err)
	}

	if err := sender.Send(packet101); err != nil {
		t.Fatalf("failed to send packet 101: %v", err)
	}

	if err := sender.Send(packet103); err != nil {
		t.Fatalf("failed to send packet 103: %v", err)
	}

	// -----------------------------------------
	// Verify packets received
	// -----------------------------------------

	for i := 0; i < 3; i++ {
		select {
		case packet := <-received:
			if packet == nil {
				t.Fatal("received nil packet")
			}

		case err := <-errors:
			t.Fatalf("receive loop error: %v", err)

		case <-time.After(time.Second):
			t.Fatal("timed out waiting for RTP packet")
		}
	}

	// -----------------------------------------
	// Verify missing packet 102
	// -----------------------------------------

	if !streams.NACKScheduler().IsPending(
		12345,
		102,
	) {
		t.Fatal(
			"expected packet 102 to be scheduled for NACK",
		)
	}
}
