package rtp

import (
	"testing"
	"time"
)

func TestNACKSchedulerWaitsBeforeFirstNACK(t *testing.T) {
	config := DefaultNACKSchedulerConfig()
	scheduler := NewNACKScheduler(config)

	start := time.Now()

	scheduler.Schedule(
		1234,
		[]uint16{101},
		start,
	)

	requests := scheduler.Poll(
		start.Add(10 * time.Millisecond),
	)

	if len(requests) != 0 {
		t.Fatalf("expected no NACK before delay")
	}
}

func TestNACKSchedulerGeneratesNACKAfterDelay(t *testing.T) {
	config := DefaultNACKSchedulerConfig()
	scheduler := NewNACKScheduler(config)

	start := time.Now()

	scheduler.Schedule(
		1234,
		[]uint16{101},
		start,
	)

	requests := scheduler.Poll(
		start.Add(20 * time.Millisecond),
	)

	if len(requests) != 1 {
		t.Fatalf("expected 1 NACK request, got %d", len(requests))
	}

	if requests[0].SSRC != 1234 {
		t.Fatalf("expected SSRC 1234, got %d", requests[0].SSRC)
	}

	if len(requests[0].Sequences) != 1 {
		t.Fatalf("expected 1 sequence, got %d",
			len(requests[0].Sequences))
	}

	if requests[0].Sequences[0] != 101 {
		t.Fatalf("expected sequence 101, got %d",
			requests[0].Sequences[0])
	}
}

func TestNACKSchedulerRecoveryCancelsNACK(t *testing.T) {
	config := DefaultNACKSchedulerConfig()
	scheduler := NewNACKScheduler(config)

	start := time.Now()

	scheduler.Schedule(
		1234,
		[]uint16{101},
		start,
	)

	// Packet arrives late before NACK delay expires.
	scheduler.Recover(1234, 101)

	requests := scheduler.Poll(
		start.Add(20 * time.Millisecond),
	)

	if len(requests) != 0 {
		t.Fatalf("expected NACK to be cancelled")
	}

	if scheduler.PendingCount() != 0 {
		t.Fatalf("expected no pending packets")
	}
}

func TestNACKSchedulerRetry(t *testing.T) {
	config := DefaultNACKSchedulerConfig()
	scheduler := NewNACKScheduler(config)

	start := time.Now()

	scheduler.Schedule(
		1234,
		[]uint16{101},
		start,
	)

	// First NACK.
	requests := scheduler.Poll(
		start.Add(20 * time.Millisecond),
	)

	if len(requests) != 1 {
		t.Fatalf("expected first NACK")
	}

	// Too early for retry.
	requests = scheduler.Poll(
		start.Add(40 * time.Millisecond),
	)

	if len(requests) != 0 {
		t.Fatalf("expected no retry yet")
	}

	// Retry is now due.
	requests = scheduler.Poll(
		start.Add(70 * time.Millisecond),
	)

	if len(requests) != 1 {
		t.Fatalf("expected retry NACK")
	}
}

func TestNACKSchedulerMultipleMissingPackets(t *testing.T) {
	config := DefaultNACKSchedulerConfig()
	scheduler := NewNACKScheduler(config)

	start := time.Now()

	scheduler.Schedule(
		1234,
		[]uint16{105, 102, 104, 103},
		start,
	)

	requests := scheduler.Poll(
		start.Add(20 * time.Millisecond),
	)

	if len(requests) != 1 {
		t.Fatalf("expected one grouped request")
	}

	expected := []uint16{102, 103, 104, 105}

	if len(requests[0].Sequences) != len(expected) {
		t.Fatalf("expected %d sequences, got %d",
			len(expected),
			len(requests[0].Sequences))
	}

	for i := range expected {
		if requests[0].Sequences[i] != expected[i] {
			t.Fatalf(
				"expected sequence %d at index %d, got %d",
				expected[i],
				i,
				requests[0].Sequences[i],
			)
		}
	}
}

func TestNACKSchedulerSeparatesSSRC(t *testing.T) {
	config := DefaultNACKSchedulerConfig()
	scheduler := NewNACKScheduler(config)

	start := time.Now()

	scheduler.Schedule(
		1111,
		[]uint16{101},
		start,
	)

	scheduler.Schedule(
		2222,
		[]uint16{201},
		start,
	)

	requests := scheduler.Poll(
		start.Add(20 * time.Millisecond),
	)

	if len(requests) != 2 {
		t.Fatalf("expected 2 requests, got %d", len(requests))
	}

	if requests[0].SSRC != 1111 {
		t.Fatalf("expected first SSRC 1111, got %d",
			requests[0].SSRC)
	}

	if requests[1].SSRC != 2222 {
		t.Fatalf("expected second SSRC 2222, got %d",
			requests[1].SSRC)
	}
}

func TestNACKSchedulerDuplicateSchedule(t *testing.T) {
	config := DefaultNACKSchedulerConfig()
	scheduler := NewNACKScheduler(config)

	start := time.Now()

	scheduler.Schedule(
		1234,
		[]uint16{101},
		start,
	)

	scheduler.Schedule(
		1234,
		[]uint16{101},
		start,
	)

	if scheduler.PendingCount() != 1 {
		t.Fatalf("expected duplicate to be ignored")
	}
}

func TestNACKSchedulerMaxRetries(t *testing.T) {
	config := NACKSchedulerConfig{
		InitialDelay:  10 * time.Millisecond,
		RetryInterval: 10 * time.Millisecond,
		MaxRetries:    2,
	}

	scheduler := NewNACKScheduler(config)

	start := time.Now()

	scheduler.Schedule(
		1234,
		[]uint16{101},
		start,
	)

	// Retry #1.
	requests := scheduler.Poll(
		start.Add(10 * time.Millisecond),
	)

	if len(requests) != 1 {
		t.Fatalf("expected first NACK")
	}

	// Retry #2.
	requests = scheduler.Poll(
		start.Add(20 * time.Millisecond),
	)

	if len(requests) != 1 {
		t.Fatalf("expected second NACK")
	}

	// Max retries reached; remove entry.
	requests = scheduler.Poll(
		start.Add(30 * time.Millisecond),
	)

	if len(requests) != 0 {
		t.Fatalf("expected no NACK after max retries")
	}

	if scheduler.PendingCount() != 0 {
		t.Fatalf("expected packet to be removed")
	}
}
