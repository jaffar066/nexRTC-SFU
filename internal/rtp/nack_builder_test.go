package rtp

import (
	"testing"

	"github.com/pion/rtcp"
)

func TestBuildRTCPNACK(t *testing.T) {
	request := NACKRequest{
		SSRC:      12345,
		Sequences: []uint16{101, 102, 103},
	}

	nack, err := BuildRTCPNACK(99999, request)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
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

	if nack.Pairs[0].PID != 101 {
		t.Fatalf(
			"expected PID 101, got %d",
			nack.Pairs[0].PID,
		)
	}
}

func TestBuildRTCPNACKEmpty(t *testing.T) {
	request := NACKRequest{
		SSRC:      12345,
		Sequences: nil,
	}

	_, err := BuildRTCPNACK(99999, request)

	if err == nil {
		t.Fatal("expected error for empty NACK request")
	}
}

func TestBuildRTCPNACKMarshal(t *testing.T) {
	request := NACKRequest{
		SSRC:      12345,
		Sequences: []uint16{101, 102, 103},
	}

	nack, err := BuildRTCPNACK(99999, request)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := nack.Marshal()
	if err != nil {
		t.Fatalf("failed to marshal NACK: %v", err)
	}

	if len(data) == 0 {
		t.Fatal("expected RTCP NACK bytes")
	}

	if data[0]>>6 != 2 {
		t.Fatalf(
			"expected RTCP version 2, got %d",
			data[0]>>6,
		)
	}

	if data[1] != byte(rtcp.TypeTransportSpecificFeedback) {
		t.Fatalf(
			"expected RTCP transport feedback type %d, got %d",
			rtcp.TypeTransportSpecificFeedback,
			data[1],
		)
	}
}
