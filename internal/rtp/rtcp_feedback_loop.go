package rtp

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"
)

type RTCPFeedbackLoop struct {
	controller *NACKController
	interval   time.Duration

	mu      sync.Mutex
	cancel  context.CancelFunc
	done    chan struct{}
	running bool
}

func NewRTCPFeedbackLoop(
	controller *NACKController,
	interval time.Duration,
) *RTCPFeedbackLoop {
	if interval <= 0 {
		interval = 10 * time.Millisecond
	}

	return &RTCPFeedbackLoop{
		controller: controller,
		interval:   interval,
	}
}

func (l *RTCPFeedbackLoop) Start() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.controller == nil {
		return fmt.Errorf("NACK controller is nil")
	}

	if l.running {
		return fmt.Errorf("RTCP feedback loop is already running")
	}

	ctx, cancel := context.WithCancel(context.Background())

	l.cancel = cancel
	l.done = make(chan struct{})
	l.running = true

	go l.run(ctx, l.done)

	return nil
}

func (l *RTCPFeedbackLoop) run(
	ctx context.Context,
	done chan struct{},
) {
	defer close(done)

	ticker := time.NewTicker(l.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return

		case now := <-ticker.C:
			_, err := l.controller.PollAndSend(now)
			if err != nil {
				log.Printf("RTCP feedback loop error: %v", err)
				continue
			}
		}
	}
}

func (l *RTCPFeedbackLoop) Stop() {
	l.mu.Lock()

	if !l.running {
		l.mu.Unlock()
		return
	}

	cancel := l.cancel
	done := l.done

	l.running = false
	l.cancel = nil
	l.done = nil

	l.mu.Unlock()

	cancel()
	<-done
}
