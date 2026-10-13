package rtp

import (
	"testing"
	"time"

	"github.com/jaffar066/nexrtc/internal/rtcp"
)

func TestRTCPTransportNACK(t *testing.T) {
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

	address := receiver.conn.LocalAddr().String()

	nack := &rtcp.NACK{
		SenderSSRC: 99999,
		MediaSSRC:  12345,
		Pairs: []rtcp.NACKPair{
			{
				PID: 101,
				BLP: 0,
			},
		},
	}

	done := make(chan error, 1)

	go func() {
		data, _, err := receiver.Receive()
		if err != nil {
			done <- err
			return
		}

		parsed, err := rtcp.UnmarshalNACK(data)
		if err != nil {
			done <- err
			return
		}

		if parsed.SenderSSRC != 99999 {
			done <- errUnexpected(
				"SenderSSRC",
				99999,
				parsed.SenderSSRC,
			)
			return
		}

		if parsed.MediaSSRC != 12345 {
			done <- errUnexpected(
				"MediaSSRC",
				12345,
				parsed.MediaSSRC,
			)
			return
		}

		done <- nil
	}()

	if err := sender.Send(address, nack); err != nil {
		t.Fatalf("failed to send NACK: %v", err)
	}

	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for RTCP packet")
	}
}

type unexpectedError struct {
	field    string
	expected any
	actual   any
}

func (e *unexpectedError) Error() string {
	return e.field + " mismatch"
}

func errUnexpected(
	field string,
	expected any,
	actual any,
) error {
	return &unexpectedError{
		field:    field,
		expected: expected,
		actual:   actual,
	}
}
