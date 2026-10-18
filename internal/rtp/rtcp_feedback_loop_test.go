package rtp

import (
	"testing"
	"time"

	"github.com/jaffar066/nexrtc/internal/rtcp"
)

func TestRTCPFeedbackLoopAutomaticallySendsNACK(t *testing.T) {
	// Create a UDP receiver for outgoing RTCP feedback.
	transport, err := NewRTCPTransport("127.0.0.1:0")
	if err != nil {
		t.Fatalf("create RTCP transport: %v", err)
	}
	defer transport.Close()

	// Create the NACK scheduler with a short initial delay.
	scheduler := NewNACKScheduler(NACKSchedulerConfig{
		InitialDelay:  20 * time.Millisecond,
		RetryInterval: 100 * time.Millisecond,
		MaxRetries:    3,
	})

	streams := NewStreamManagerWithNACKScheduler(scheduler)

	sender := NewNACKSender(
		transport,
		99999,
		transport.LocalAddr(),
	)

	controller := NewNACKController(streams, sender)

	loop := NewRTCPFeedbackLoop(
		controller,
		5*time.Millisecond,
	)

	// Schedule a missing RTP packet.
	scheduler.Schedule(
		12345,
		[]uint16{102},
		time.Now(),
	)

	// Start the automatic feedback loop.
	if err := loop.Start(); err != nil {
		t.Fatalf("start feedback loop: %v", err)
	}
	defer loop.Stop()

	// Receive the generated RTCP NACK.
	if err := transport.conn.SetReadDeadline(
		time.Now().Add(time.Second),
	); err != nil {
		t.Fatalf("set read deadline: %v", err)
	}

	buffer := make([]byte, 1500)
	n, _, err := transport.conn.ReadFromUDP(buffer)
	if err != nil {
		t.Fatalf("receive RTCP NACK: %v", err)
	}

	data := buffer[:n]

	if len(data) < rtcp.HeaderSize {
		t.Fatalf("RTCP packet too short: %d bytes", len(data))
	}

	nack, err := rtcp.UnmarshalNACK(data)
	if err != nil {
		t.Fatalf("unmarshal RTCP NACK: %v", err)
	}

	if nack.SenderSSRC != 99999 {
		t.Errorf(
			"sender SSRC = %d, want 99999",
			nack.SenderSSRC,
		)
	}

	if nack.MediaSSRC != 12345 {
		t.Errorf(
			"media SSRC = %d, want 12345",
			nack.MediaSSRC,
		)
	}

	if len(nack.Pairs) != 1 {
		t.Fatalf(
			"NACK pairs = %d, want 1",
			len(nack.Pairs),
		)
	}

	if nack.Pairs[0].PID != 102 {
		t.Errorf(
			"NACK PID = %d, want 102",
			nack.Pairs[0].PID,
		)
	}
}

func TestRTCPFeedbackLoopRejectsRepeatedStart(t *testing.T) {
	transport, err := NewRTCPTransport("127.0.0.1:0")
	if err != nil {
		t.Fatalf("create RTCP transport: %v", err)
	}
	defer transport.Close()

	streams := NewStreamManager()

	sender := NewNACKSender(
		transport,
		99999,
		transport.LocalAddr(),
	)

	controller := NewNACKController(streams, sender)

	loop := NewRTCPFeedbackLoop(
		controller,
		10*time.Millisecond,
	)

	if err := loop.Start(); err != nil {
		t.Fatalf("first Start() failed: %v", err)
	}

	if err := loop.Start(); err == nil {
		loop.Stop()
		t.Fatal("second Start() should return an error")
	}

	loop.Stop()
}

func TestRTCPFeedbackLoopRejectsNilController(t *testing.T) {
	loop := NewRTCPFeedbackLoop(nil, time.Millisecond)
	if err := loop.Start(); err == nil {
		t.Fatal("expected Start() to reject a nil controller")
	}
}

func TestRTCPFeedbackLoopStopIsSafe(t *testing.T) {
	transport, err := NewRTCPTransport("127.0.0.1:0")
	if err != nil {
		t.Fatalf("create RTCP transport: %v", err)
	}
	defer transport.Close()

	controller := NewNACKController(
		NewStreamManager(),
		NewNACKSender(
			transport,
			99999,
			transport.LocalAddr(),
		),
	)

	loop := NewRTCPFeedbackLoop(
		controller,
		10*time.Millisecond,
	)

	// Stopping a loop that has not started should be safe.
	loop.Stop()

	if err := loop.Start(); err != nil {
		t.Fatalf("start feedback loop: %v", err)
	}

	loop.Stop()

	// Calling Stop() again should also be safe.
	loop.Stop()

	// Confirm the loop can be started again after it stops.
	if err := loop.Start(); err != nil {
		t.Fatalf("restart feedback loop: %v", err)
	}

	loop.Stop()
}
