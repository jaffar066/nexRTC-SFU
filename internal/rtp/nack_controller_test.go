package rtp

import (
	"testing"
	"time"

	"github.com/jaffar066/nexrtc/internal/rtcp"
)

func TestNACKControllerPollAndSend(t *testing.T) {
	// --------------------------------------------------
	// 1. Create stream manager with short NACK delay
	// --------------------------------------------------

	config := NACKSchedulerConfig{
		InitialDelay:  10 * time.Millisecond,
		RetryInterval: 20 * time.Millisecond,
		MaxRetries:    3,
	}

	scheduler := NewNACKScheduler(config)

	streams := NewStreamManagerWithNACKScheduler(
		scheduler,
	)

	// --------------------------------------------------
	// 2. Create RTCP receiver
	// --------------------------------------------------

	receiver, err := NewRTCPTransport("127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to create RTCP receiver: %v", err)
	}
	defer receiver.Close()

	// --------------------------------------------------
	// 3. Create NACK sender
	// --------------------------------------------------

	sender := NewNACKSender(
		receiver,
		99999,
		receiver.LocalAddr(),
	)

	// --------------------------------------------------
	// 4. Create controller
	// --------------------------------------------------

	controller := NewNACKController(
		streams,
		sender,
	)

	// --------------------------------------------------
	// 5. Schedule missing RTP packet
	// --------------------------------------------------

	now := time.Now()

	streams.NACKScheduler().Schedule(
		12345,
		[]uint16{102},
		now,
	)

	// --------------------------------------------------
	// 6. Poll too early
	// --------------------------------------------------

	requests, err := controller.PollAndSend(
		now.Add(5 * time.Millisecond),
	)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(requests) != 0 {
		t.Fatalf(
			"expected no NACK before delay, got %d",
			len(requests),
		)
	}

	// --------------------------------------------------
	// 7. Poll after initial delay
	// --------------------------------------------------

	received := make(chan []byte, 1)
	errors := make(chan error, 1)

	go func() {
		data, _, err := receiver.Receive()
		if err != nil {
			errors <- err
			return
		}

		received <- data
	}()

	requests, err = controller.PollAndSend(
		now.Add(11 * time.Millisecond),
	)

	if err != nil {
		t.Fatalf("failed to send NACK: %v", err)
	}

	if len(requests) != 1 {
		t.Fatalf(
			"expected 1 NACK request, got %d",
			len(requests),
		)
	}

	if requests[0].SSRC != 12345 {
		t.Fatalf(
			"expected SSRC 12345, got %d",
			requests[0].SSRC,
		)
	}

	if len(requests[0].Sequences) != 1 {
		t.Fatalf(
			"expected 1 sequence, got %d",
			len(requests[0].Sequences),
		)
	}

	if requests[0].Sequences[0] != 102 {
		t.Fatalf(
			"expected sequence 102, got %d",
			requests[0].Sequences[0],
		)
	}

	// --------------------------------------------------
	// 8. Verify actual RTCP packet
	// --------------------------------------------------

	select {
	case data := <-received:

		nack, err := rtcp.UnmarshalNACK(data)
		if err != nil {
			t.Fatalf(
				"failed to parse received NACK: %v",
				err,
			)
		}

		if nack.SenderSSRC != 99999 {
			t.Fatalf(
				"expected sender SSRC 99999, got %d",
				nack.SenderSSRC,
			)
		}

		if nack.MediaSSRC != 12345 {
			t.Fatalf(
				"expected media SSRC 12345, got %d",
				nack.MediaSSRC,
			)
		}

		if len(nack.Pairs) != 1 {
			t.Fatalf(
				"expected 1 NACK pair, got %d",
				len(nack.Pairs),
			)
		}

		if nack.Pairs[0].PID != 102 {
			t.Fatalf(
				"expected PID 102, got %d",
				nack.Pairs[0].PID,
			)
		}

	case err := <-errors:
		t.Fatalf("RTCP receiver error: %v", err)

	case <-time.After(time.Second):
		t.Fatal("timed out waiting for RTCP NACK")
	}
}
