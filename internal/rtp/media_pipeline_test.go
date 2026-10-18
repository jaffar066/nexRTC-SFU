package rtp

import (
	"net"
	"testing"
	"time"

	"github.com/jaffar066/nexrtc/internal/rtcp"
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

	if err := pipeline.Start(); err == nil {
		pipeline.Stop()
		t.Fatal("pipeline should not restart after Stop closes its owned socket")
	}
}

func TestMediaPipelineReportsTerminalReadError(t *testing.T) {
	rtpReceiver, err := NewUDPReceiver("127.0.0.1:0")
	if err != nil {
		t.Fatalf("create RTP receiver: %v", err)
	}
	if err := rtpReceiver.Close(); err != nil {
		t.Fatalf("close RTP receiver before pipeline start: %v", err)
	}

	rtcpTransport, err := NewRTCPTransport("127.0.0.1:0")
	if err != nil {
		t.Fatalf("create RTCP transport: %v", err)
	}
	defer rtcpTransport.Close()

	streams := NewStreamManager()
	pipeline, err := NewMediaPipeline(
		rtpReceiver.Conn,
		streams,
		90000,
		NewNACKController(
			streams,
			NewNACKSender(rtcpTransport, 1, rtcpTransport.LocalAddr()),
		),
		time.Millisecond,
	)
	if err != nil {
		t.Fatalf("create media pipeline: %v", err)
	}

	if err := pipeline.Start(); err != nil {
		t.Fatalf("start media pipeline: %v", err)
	}

	select {
	case err := <-pipeline.Errors():
		if err == nil {
			t.Fatal("expected terminal read error")
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for terminal read error")
	}

	pipeline.Stop()

	if err := pipeline.Start(); err == nil {
		t.Fatal("pipeline should not restart after a terminal read error")
	}
}

func TestMediaPipelineSendsNACKForMissingRTPPacket(t *testing.T) {
	rtpReceiver, err := NewUDPReceiver("127.0.0.1:0")
	if err != nil {
		t.Fatalf("create RTP receiver: %v", err)
	}

	rtcpReceiver, err := NewRTCPTransport("127.0.0.1:0")
	if err != nil {
		rtpReceiver.Close()
		t.Fatalf("create RTCP receiver: %v", err)
	}
	defer rtcpReceiver.Close()

	rtcpSender, err := NewRTCPTransport("127.0.0.1:0")
	if err != nil {
		rtpReceiver.Close()
		t.Fatalf("create RTCP sender: %v", err)
	}
	defer rtcpSender.Close()

	scheduler := NewNACKScheduler(NACKSchedulerConfig{
		InitialDelay:  10 * time.Millisecond,
		RetryInterval: time.Second,
		MaxRetries:    1,
	})
	streams := NewStreamManagerWithNACKScheduler(scheduler)
	controller := NewNACKController(
		streams,
		NewNACKSender(rtcpSender, 99999, rtcpReceiver.LocalAddr()),
	)
	pipeline, err := NewMediaPipeline(
		rtpReceiver.Conn,
		streams,
		90000,
		controller,
		time.Millisecond,
	)
	if err != nil {
		rtpReceiver.Close()
		t.Fatalf("create media pipeline: %v", err)
	}
	if err := pipeline.Start(); err != nil {
		rtpReceiver.Close()
		t.Fatalf("start media pipeline: %v", err)
	}
	defer pipeline.Stop()

	rtpSender, err := NewUDPSender(rtpReceiver.Conn.LocalAddr().String())
	if err != nil {
		t.Fatalf("create RTP sender: %v", err)
	}
	defer rtpSender.Close()

	for _, sequence := range []uint16{100, 101, 103} {
		if err := rtpSender.Send(&Packet{
			Version:        2,
			PayloadType:    96,
			SequenceNumber: sequence,
			Timestamp:      uint32(sequence) * 900,
			SSRC:           12345,
			Payload:        []byte("media"),
		}); err != nil {
			t.Fatalf("send RTP packet %d: %v", sequence, err)
		}
		time.Sleep(20 * time.Millisecond)
	}

	if err := rtcpReceiver.conn.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatalf("set RTCP read deadline: %v", err)
	}
	data, _, err := rtcpReceiver.Receive()
	if err != nil {
		t.Fatalf("receive serialized RTCP NACK: %v", err)
	}

	nack, err := rtcp.UnmarshalNACK(data)
	if err != nil {
		t.Fatalf("unmarshal serialized RTCP NACK: %v", err)
	}
	if nack.SenderSSRC != 99999 || nack.MediaSSRC != 12345 {
		t.Fatalf("unexpected NACK SSRCs: sender=%d media=%d", nack.SenderSSRC, nack.MediaSSRC)
	}
	if len(nack.Pairs) != 1 || nack.Pairs[0].PID != 102 {
		t.Fatalf("NACK pairs = %+v, want one PID 102 pair", nack.Pairs)
	}
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
