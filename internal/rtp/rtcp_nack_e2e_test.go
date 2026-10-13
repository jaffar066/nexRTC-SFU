package rtp

import (
	"net"
	"testing"
	"time"

	"github.com/jaffar066/nexrtc/internal/rtcp"
)

func TestRTCPNACKTriggersRTPRetransmission(t *testing.T) {
	// --------------------------------------------------
	// 1. Create RTP receiver
	// --------------------------------------------------

	probe, err := net.ListenUDP(
		"udp",
		&net.UDPAddr{
			IP: net.ParseIP("127.0.0.1"),
		},
	)
	if err != nil {
		t.Fatalf("failed to find RTP receiver address: %v", err)
	}

	rtpAddress := probe.LocalAddr().String()

	if err := probe.Close(); err != nil {
		t.Fatalf("failed to release RTP address: %v", err)
	}

	rtpReceiver, err := NewUDPReceiver(rtpAddress)
	if err != nil {
		t.Fatalf("failed to create RTP receiver: %v", err)
	}
	defer rtpReceiver.Close()

	// --------------------------------------------------
	// 2. Create RTP sender
	// --------------------------------------------------

	rtpSender, err := NewUDPSender(rtpAddress)
	if err != nil {
		t.Fatalf("failed to create RTP sender: %v", err)
	}
	defer rtpSender.Close()

	// --------------------------------------------------
	// 3. Store original RTP packet
	// --------------------------------------------------

	original := &Packet{
		Version:        2,
		PayloadType:    96,
		SequenceNumber: 102,
		Timestamp:      9000,
		SSRC:           12345,
		Payload:        []byte("video-frame-102"),
	}

	buffer := NewRetransmissionBuffer(100)

	buffer.Store(original)

	// --------------------------------------------------
	// 4. Create retransmission manager
	// --------------------------------------------------

	manager := NewRetransmissionManager(
		buffer,
		rtpSender,
	)

	// --------------------------------------------------
	// 5. Create RTCP handler
	// --------------------------------------------------

	handler := NewRTCPHandler(manager)

	// --------------------------------------------------
	// 6. Create RTCP transport
	// --------------------------------------------------

	rtcpTransport, err := NewRTCPTransport("127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to create RTCP transport: %v", err)
	}
	defer rtcpTransport.Close()

	// --------------------------------------------------
	// 7. Build RTCP NACK
	// --------------------------------------------------

	nack := &rtcp.NACK{
		SenderSSRC: 99999,
		MediaSSRC:  12345,
		Pairs: []rtcp.NACKPair{
			{
				PID: 102,
				BLP: 0,
			},
		},
	}

	nackData, err := nack.Marshal()
	if err != nil {
		t.Fatalf("failed to marshal NACK: %v", err)
	}

	// --------------------------------------------------
	// 8. Receive retransmitted RTP packet
	// --------------------------------------------------

	received := make(chan *Packet, 1)
	errors := make(chan error, 1)

	go func() {
		packet, _, err := rtpReceiver.Receive()
		if err != nil {
			errors <- err
			return
		}

		received <- packet
	}()

	// --------------------------------------------------
	// 9. Simulate RTCP network receive
	// --------------------------------------------------
	//
	// Instead of directly calling HandleNACK(),
	// feed the actual serialized RTCP packet into
	// RTCPHandler.
	//

	_, err = handler.HandlePacket(nackData)

	if err != nil {
		t.Fatalf("RTCP handler failed: %v", err)
	}

	// --------------------------------------------------
	// 10. Verify retransmitted RTP packet
	// --------------------------------------------------

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
		t.Fatalf("RTP receiver error: %v", err)

	case <-time.After(time.Second):
		t.Fatal(
			"timed out waiting for retransmitted RTP packet",
		)
	}
}
