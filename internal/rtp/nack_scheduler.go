package rtp

import (
	"sort"
	"sync"
	"time"
)

type NACKSchedulerConfig struct {
	// Wait this long before sending the first NACK.
	InitialDelay time.Duration

	// Wait this long between NACK retries.
	RetryInterval time.Duration

	// Maximum number of NACK transmissions for one packet.
	MaxRetries int
}

func DefaultNACKSchedulerConfig() NACKSchedulerConfig {
	return NACKSchedulerConfig{
		InitialDelay:  20 * time.Millisecond,
		RetryInterval: 50 * time.Millisecond,
		MaxRetries:    3,
	}
}

type nackEntry struct {
	SSRC           uint32
	SequenceNumber uint16

	FirstSeen time.Time
	LastSent  time.Time
	Retries   int
}

type NACKRequest struct {
	SSRC      uint32
	Sequences []uint16
}

type NACKScheduler struct {
	mu sync.Mutex

	config NACKSchedulerConfig

	pending map[RetransmissionKey]*nackEntry
}

func NewNACKScheduler(config NACKSchedulerConfig) *NACKScheduler {
	if config.InitialDelay <= 0 {
		config.InitialDelay = 20 * time.Millisecond
	}

	if config.RetryInterval <= 0 {
		config.RetryInterval = 50 * time.Millisecond
	}

	if config.MaxRetries <= 0 {
		config.MaxRetries = 3
	}

	return &NACKScheduler{
		config:  config,
		pending: make(map[RetransmissionKey]*nackEntry),
	}
}

// Schedule adds missing RTP packets to the NACK queue.
func (s *NACKScheduler) Schedule(
	ssrc uint32,
	missing []uint16,
	now time.Time,
) {
	if len(missing) == 0 {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for _, sequence := range missing {
		key := RetransmissionKey{
			SSRC:           ssrc,
			SequenceNumber: sequence,
		}

		// Already scheduled.
		if _, exists := s.pending[key]; exists {
			continue
		}

		s.pending[key] = &nackEntry{
			SSRC:           ssrc,
			SequenceNumber: sequence,
			FirstSeen:      now,
		}
	}
}

// Recover removes a packet from the NACK queue.
//
// This happens when a previously missing packet arrives late.
func (s *NACKScheduler) Recover(
	ssrc uint32,
	sequence uint16,
) {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := RetransmissionKey{
		SSRC:           ssrc,
		SequenceNumber: sequence,
	}

	delete(s.pending, key)
}

// Poll returns NACK requests that are ready to be sent.
//
// The caller provides the current time so tests can be deterministic.
func (s *NACKScheduler) Poll(now time.Time) []NACKRequest {
	s.mu.Lock()
	defer s.mu.Unlock()

	grouped := make(map[uint32][]uint16)

	for key, entry := range s.pending {

		// First NACK.
		if entry.Retries == 0 {
			if now.Sub(entry.FirstSeen) < s.config.InitialDelay {
				continue
			}
		} else {
			// Retry.
			if now.Sub(entry.LastSent) < s.config.RetryInterval {
				continue
			}
		}

		// Retry limit reached.
		if entry.Retries >= s.config.MaxRetries {
			delete(s.pending, key)
			continue
		}

		grouped[entry.SSRC] = append(
			grouped[entry.SSRC],
			entry.SequenceNumber,
		)

		entry.Retries++
		entry.LastSent = now
	}

	if len(grouped) == 0 {
		return nil
	}

	requests := make([]NACKRequest, 0, len(grouped))

	for ssrc, sequences := range grouped {
		sort.Slice(sequences, func(i, j int) bool {
			return sequences[i] < sequences[j]
		})

		requests = append(requests, NACKRequest{
			SSRC:      ssrc,
			Sequences: sequences,
		})
	}

	sort.Slice(requests, func(i, j int) bool {
		return requests[i].SSRC < requests[j].SSRC
	})

	return requests
}

// PendingCount returns the number of packets waiting for NACK.
func (s *NACKScheduler) PendingCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	return len(s.pending)
}

// IsPending checks whether a packet is currently scheduled.
func (s *NACKScheduler) IsPending(
	ssrc uint32,
	sequence uint16,
) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := RetransmissionKey{
		SSRC:           ssrc,
		SequenceNumber: sequence,
	}

	_, exists := s.pending[key]
	return exists
}
