package rtp

import (
	"fmt"
	"testing"
	"time"

	"github.com/jaffar066/nexrtc/internal/rtcp"
)

func TestNACKSender(t *testing.T) {
	receiver, err := NewRTCPTransport("127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to create receiver: %v", err)
	}
	defer receiver.Close()

	sender, err := NewRTCPTransport("127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to create sender: %v", err)
	}
	defer sender.Close()

	remoteAddress := receiver.conn.LocalAddr().String()

	nackSender := NewNACKSender(
		sender,
		99999,
		remoteAddress,
	)

	request := NACKRequest{
		SSRC:      12345,
		Sequences: []uint16{102},
	}

	done := make(chan error, 1)

	go func() {
		data, _, err := receiver.Receive()
		if err != nil {
			done <- err
			return
		}

		nack, err := rtcp.UnmarshalNACK(data)
		if err != nil {
			done <- err
			return
		}

		if nack.SenderSSRC != 99999 {
			done <- fmt.Errorf(
				"expected SenderSSRC 99999, got %d",
				nack.SenderSSRC,
			)
			return
		}

		if nack.MediaSSRC != 12345 {
			done <- fmt.Errorf(
				"expected MediaSSRC 12345, got %d",
				nack.MediaSSRC,
			)
			return
		}

		if len(nack.Pairs) != 1 {
			done <- fmt.Errorf(
				"expected 1 NACK pair, got %d",
				len(nack.Pairs),
			)
			return
		}

		if nack.Pairs[0].PID != 102 {
			done <- fmt.Errorf(
				"expected PID 102, got %d",
				nack.Pairs[0].PID,
			)
			return
		}

		done <- nil
	}()

	err = nackSender.Send(request)
	if err != nil {
		t.Fatalf("failed to send NACK: %v", err)
	}

	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}

	case <-time.After(time.Second):
		t.Fatal("timed out waiting for NACK")
	}
}
