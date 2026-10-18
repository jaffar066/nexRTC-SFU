package rtp

import (
	"errors"
	"fmt"
	"log"
	"net"
	"sync"
	"time"
)

const temporaryReadRetryDelay = 10 * time.Millisecond

type MediaPipeline struct {
	receiver     *RTPReceiveLoop
	feedbackLoop *RTCPFeedbackLoop

	mu       sync.Mutex
	running  bool
	started  bool
	stopCh   chan struct{}
	doneCh   chan struct{}
	errors   chan error
	stopOnce sync.Once
}

// NewMediaPipeline creates a pipeline that takes ownership of rtpConn after Start succeeds.
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

// NewMediaPipelineWithForwarder creates a forwarding pipeline that takes ownership of rtpConn
// after Start succeeds.
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
		errors: make(chan error, 1),
	}, nil
}

func (p *MediaPipeline) Start() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.started {
		if p.running {
			return fmt.Errorf("media pipeline is already running")
		}
		return fmt.Errorf("media pipeline cannot be restarted after it stops")
	}

	if p.receiver == nil || p.receiver.conn == nil {
		return fmt.Errorf("RTP receiver connection is nil")
	}

	if p.feedbackLoop == nil {
		return fmt.Errorf("RTCP feedback loop is nil")
	}

	stopCh := make(chan struct{})
	doneCh := make(chan struct{})
	if err := p.feedbackLoop.Start(); err != nil {
		return fmt.Errorf("start RTCP feedback loop: %w", err)
	}

	p.stopCh = stopCh
	p.doneCh = doneCh
	p.started = true
	p.running = true
	p.stopOnce = sync.Once{}

	go p.receivePackets(stopCh, doneCh)

	return nil
}

// Errors returns the channel used to report a terminal RTP socket read error.
func (p *MediaPipeline) Errors() <-chan error {
	return p.errors
}

func (p *MediaPipeline) receivePackets(
	stopCh chan struct{},
	doneCh chan struct{},
) {
	defer close(doneCh)
	defer func() {
		p.mu.Lock()
		p.running = false
		p.mu.Unlock()
	}()

	for {
		select {
		case <-stopCh:
			return
		default:
		}

		_, err := p.receiver.ReceiveOnce()
		if err == nil {
			continue
		}

		select {
		case <-stopCh:
			return
		default:
		}

		var readErr *rtpReadError
		if !errors.As(err, &readErr) {
			log.Printf("RTP media pipeline packet error: %v", err)
			continue
		}

		var networkErr net.Error
		if errors.As(readErr, &networkErr) && networkErr.Temporary() {
			timer := time.NewTimer(temporaryReadRetryDelay)
			select {
			case <-stopCh:
				timer.Stop()
				return
			case <-timer.C:
				continue
			}
		}

		terminalErr := fmt.Errorf("RTP media pipeline stopped: %w", err)
		log.Printf("%v", terminalErr)
		p.errors <- terminalErr
		p.stopSocket(stopCh)
		p.feedbackLoop.Stop()
		return
	}
}

func (p *MediaPipeline) stopSocket(stopCh chan struct{}) {
	p.stopOnce.Do(func() {
		close(stopCh)
		if err := p.receiver.conn.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
			log.Printf("close RTP media pipeline socket: %v", err)
		}
	})
}

func (p *MediaPipeline) Stop() {
	p.mu.Lock()
	if !p.started {
		p.mu.Unlock()
		return
	}

	stopCh := p.stopCh
	doneCh := p.doneCh
	p.mu.Unlock()

	// The pipeline owns this socket after Start succeeds. Closing it unblocks ReadFromUDP.
	p.stopSocket(stopCh)
	<-doneCh
	p.feedbackLoop.Stop()
}
