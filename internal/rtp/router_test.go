package rtp

import (
	"errors"
	"net"
	"strings"
	"testing"
	"time"
)

type routerTestSender struct {
	packets chan *Packet
	err     error
}

func (s *routerTestSender) Send(packet *Packet) error {
	if s.err != nil {
		return s.err
	}
	s.packets <- packet
	return nil
}

func TestRTPRouterForwardsOnlyToSubscribersOfPacketSSRC(t *testing.T) {
	router := NewRTPRouter()
	first := &routerTestSender{packets: make(chan *Packet, 1)}
	second := &routerTestSender{packets: make(chan *Packet, 1)}
	unrelated := &routerTestSender{packets: make(chan *Packet, 1)}

	if err := router.Subscribe(123, "peer-a", first); err != nil {
		t.Fatalf("subscribe peer-a: %v", err)
	}
	if err := router.Subscribe(123, "peer-b", second); err != nil {
		t.Fatalf("subscribe peer-b: %v", err)
	}
	if err := router.Subscribe(456, "peer-c", unrelated); err != nil {
		t.Fatalf("subscribe peer-c: %v", err)
	}

	packet := &Packet{SSRC: 123, SequenceNumber: 7, Payload: []byte("frame")}
	if err := router.Forward(packet); err != nil {
		t.Fatalf("forward packet: %v", err)
	}

	for name, sender := range map[string]*routerTestSender{
		"peer-a": first,
		"peer-b": second,
	} {
		select {
		case forwarded := <-sender.packets:
			if forwarded == packet {
				t.Errorf("%s received the original mutable packet", name)
			}
			if forwarded.SSRC != packet.SSRC ||
				forwarded.SequenceNumber != packet.SequenceNumber ||
				string(forwarded.Payload) != string(packet.Payload) {
				t.Errorf("%s received incorrect packet: %+v", name, forwarded)
			}
			forwarded.Payload[0] = 'X'
		default:
			t.Errorf("%s did not receive the packet", name)
		}
	}

	if len(unrelated.packets) != 0 {
		t.Fatal("subscriber to another SSRC received the packet")
	}
	if string(packet.Payload) != "frame" {
		t.Fatal("forwarding allowed a subscriber to mutate the source packet")
	}
}

func TestRTPRouterContinuesAfterSubscriberSendFailure(t *testing.T) {
	router := NewRTPRouter()
	failure := errors.New("send failed")
	failedSender := &routerTestSender{err: failure}
	goodSender := &routerTestSender{packets: make(chan *Packet, 1)}

	if err := router.Subscribe(9, "peer-a", failedSender); err != nil {
		t.Fatalf("subscribe peer-a: %v", err)
	}
	if err := router.Subscribe(9, "peer-b", goodSender); err != nil {
		t.Fatalf("subscribe peer-b: %v", err)
	}

	err := router.Forward(&Packet{SSRC: 9})
	if err == nil || !strings.Contains(err.Error(), `subscriber "peer-a"`) {
		t.Fatalf("expected contextual send error, got %v", err)
	}
	if !errors.Is(err, failure) {
		t.Fatalf("expected wrapped send error, got %v", err)
	}
	if len(goodSender.packets) != 1 {
		t.Fatal("router stopped forwarding after the first subscriber failure")
	}
}

func TestRTPRouterValidatesSubscriptionsAndUnsubscribes(t *testing.T) {
	router := NewRTPRouter()
	sender := &routerTestSender{packets: make(chan *Packet, 1)}

	if err := router.Subscribe(1, "", sender); err == nil {
		t.Fatal("expected empty subscriber ID to be rejected")
	}
	if err := router.Subscribe(1, "peer", nil); err == nil {
		t.Fatal("expected nil sender to be rejected")
	}
	if err := router.Subscribe(1, "peer", sender); err != nil {
		t.Fatalf("subscribe peer: %v", err)
	}
	if err := router.Subscribe(1, "peer", sender); err == nil {
		t.Fatal("expected duplicate subscription to be rejected")
	}
	if got := router.SubscriberCount(1); got != 1 {
		t.Fatalf("expected one subscriber, got %d", got)
	}

	router.Unsubscribe(1, "peer")
	if got := router.SubscriberCount(1); got != 0 {
		t.Fatalf("expected no subscribers after unsubscribe, got %d", got)
	}
}

func TestRTPReceiveLoopForwardsReceivedPackets(t *testing.T) {
	receiver, err := NewUDPReceiver("127.0.0.1:0")
	if err != nil {
		t.Fatalf("create RTP receiver: %v", err)
	}
	defer receiver.Close()

	forwarded := &routerTestSender{packets: make(chan *Packet, 1)}
	router := NewRTPRouter()
	if err := router.Subscribe(12345, "subscriber", forwarded); err != nil {
		t.Fatalf("subscribe forwarding destination: %v", err)
	}

	loop := NewRTPReceiveLoopWithForwarder(
		receiver.Conn,
		NewStreamManager(),
		90000,
		router,
	)
	upstream, err := NewUDPSender(receiver.Conn.LocalAddr().String())
	if err != nil {
		t.Fatalf("create upstream sender: %v", err)
	}
	defer upstream.Close()

	packet := &Packet{
		Version:        2,
		PayloadType:    96,
		SequenceNumber: 10,
		Timestamp:      9000,
		SSRC:           12345,
		Payload:        []byte("media"),
	}

	received := make(chan error, 1)
	go func() {
		_, receiveErr := loop.ReceiveOnce()
		received <- receiveErr
	}()

	if err := upstream.Send(packet); err != nil {
		t.Fatalf("send upstream RTP packet: %v", err)
	}

	select {
	case err := <-received:
		if err != nil {
			t.Fatalf("receive and forward RTP packet: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for RTP receive loop")
	}

	select {
	case got := <-forwarded.packets:
		if got.SSRC != packet.SSRC || string(got.Payload) != "media" {
			t.Fatalf("received incorrect forwarded packet: %+v", got)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for forwarded RTP packet")
	}
}

func TestNewMediaPipelineWithForwarderRejectsNil(t *testing.T) {
	conn, err := net.ListenUDP(
		"udp",
		&net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 0},
	)
	if err != nil {
		t.Fatalf("create UDP connection: %v", err)
	}
	defer conn.Close()

	_, err = NewMediaPipelineWithForwarder(
		conn,
		NewStreamManager(),
		90000,
		&NACKController{},
		time.Millisecond,
		nil,
	)
	if err == nil {
		t.Fatal("expected nil packet forwarder to be rejected")
	}
}
