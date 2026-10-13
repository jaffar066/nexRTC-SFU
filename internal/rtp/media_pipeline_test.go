package rtp

import (
	"net"
	"testing"
	"time"
)

func TestMediaPipelineStartAndStop(t *testing.T) {
	rtpReceiver, err := NewUDPReceiver("127.0.0.1:0")
	if err != nil {
		t.Fatalf("create RTP receiver: %v", err)
	}

	streams := NewStreamManager()

	rtcpTransport, err := NewRTCPTransport("127.0.0.1:0")
	if err != nil {
		rtpReceiver.Close()
		t.Fatalf("create RTCP transport: %v", err)
	}
	defer rtcpTransport.Close()

	sender := NewNACKSender(
		rtcpTransport,
		99999,
		rtcpTransport.LocalAddr(),
	)

	controller := NewNACKController(streams, sender)

	pipeline, err := NewMediaPipeline(
		rtpReceiver.Conn,
		streams,
		90000,
		controller,
		10*time.Millisecond,
	)
	if err != nil {
		rtpReceiver.Close()
		t.Fatalf("create media pipeline: %v", err)
	}

	if err := pipeline.Start(); err != nil {
		rtpReceiver.Close()
		t.Fatalf("start media pipeline: %v", err)
	}

	if err := pipeline.Start(); err == nil {
		pipeline.Stop()
		t.Fatal("second Start() should return an error")
	}

	pipeline.Stop()

	// Stop should be safe to call again.
	pipeline.Stop()
}

func TestMediaPipelineRejectsNilConnections(t *testing.T) {
	streams := NewStreamManager()

	transport, err := NewRTCPTransport("127.0.0.1:0")
	if err != nil {
		t.Fatalf("create RTCP transport: %v", err)
	}
	defer transport.Close()

	controller := NewNACKController(
		streams,
		NewNACKSender(
			transport,
			99999,
			transport.LocalAddr(),
		),
	)

	_, err = NewMediaPipeline(
		nil,
		streams,
		90000,
		controller,
		10*time.Millisecond,
	)
	if err == nil {
		t.Fatal("expected error for nil RTP connection")
	}

	var nilStreams *StreamManager
	conn, err := net.ListenUDP(
		"udp",
		&net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 0},
	)
	if err != nil {
		t.Fatalf("create UDP connection: %v", err)
	}
	defer conn.Close()

	_, err = NewMediaPipeline(
		conn,
		nilStreams,
		90000,
		controller,
		10*time.Millisecond,
	)
	if err == nil {
		t.Fatal("expected error for nil stream manager")
	}
}
