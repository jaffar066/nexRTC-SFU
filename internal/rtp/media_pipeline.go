package rtp

import (
	"fmt"
	"log"
	"net"
	"sync"
	"time"
)

type MediaPipeline struct {
	receiver     *RTPReceiveLoop
	feedbackLoop *RTCPFeedbackLoop

	mu      sync.Mutex
	running bool
	stopCh  chan struct{}
	doneCh  chan struct{}
}

func NewMediaPipeline(
	rtpConn *net.UDPConn,
	streams *StreamManager,
	clockRate uint32,
	controller *NACKController,
	feedbackInterval time.Duration,
) (*MediaPipeline, error) {
	return newMediaPipeline(
		rtpConn,
		streams,
		clockRate,
		controller,
		feedbackInterval,
		nil,
	)
}

func NewMediaPipelineWithForwarder(
	rtpConn *net.UDPConn,
	streams *StreamManager,
	clockRate uint32,
	controller *NACKController,
	feedbackInterval time.Duration,
	forwarder RTPPacketForwarder,
) (*MediaPipeline, error) {
	if forwarder == nil {
		return nil, fmt.Errorf("RTP packet forwarder is nil")
	}

	return newMediaPipeline(
		rtpConn,
		streams,
		clockRate,
		controller,
		feedbackInterval,
		forwarder,
	)
}

func newMediaPipeline(
	rtpConn *net.UDPConn,
	streams *StreamManager,
	clockRate uint32,
	controller *NACKController,
	feedbackInterval time.Duration,
	forwarder RTPPacketForwarder,
) (*MediaPipeline, error) {
	if rtpConn == nil {
		return nil, fmt.Errorf("RTP UDP connection is nil")
	}

	if streams == nil {
		return nil, fmt.Errorf("stream manager is nil")
	}

	if controller == nil {
		return nil, fmt.Errorf("NACK controller is nil")
	}

	return &MediaPipeline{
		receiver: NewRTPReceiveLoopWithForwarder(
			rtpConn,
			streams,
			clockRate,
			forwarder,
		),
		feedbackLoop: NewRTCPFeedbackLoop(
			controller,
			feedbackInterval,
		),
	}, nil
}

func (p *MediaPipeline) Start() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.running {
		return fmt.Errorf("media pipeline is already running")
	}

	if err := p.feedbackLoop.Start(); err != nil {
		return fmt.Errorf("start RTCP feedback loop: %w", err)
	}

	p.stopCh = make(chan struct{})
	p.doneCh = make(chan struct{})
	p.running = true

	go p.receivePackets(p.stopCh, p.doneCh)

	return nil
}

func (p *MediaPipeline) receivePackets(
	stopCh <-chan struct{},
	doneCh chan<- struct{},
) {
	defer close(doneCh)

	for {
		select {
		case <-stopCh:
			return
		default:
		}

		_, err := p.receiver.ReceiveOnce()
		if err != nil {
			select {
			case <-stopCh:
				return
			default:
				log.Printf("RTP media pipeline receive error: %v", err)
				continue
			}
		}
	}
}

func (p *MediaPipeline) Stop() {
	p.mu.Lock()

	if !p.running {
		p.mu.Unlock()
		return
	}

	stopCh := p.stopCh
	doneCh := p.doneCh

	p.running = false

	p.mu.Unlock()

	close(stopCh)

	// Unblock ReadFromUDP so the receiver goroutine
	// can exit. This pipeline owns the supplied socket.
	_ = p.receiver.conn.Close()

	<-doneCh
	p.feedbackLoop.Stop()
}
