package rtp

import (
	"math"
	"testing"
)

func TestJitterValueMilliseconds(t *testing.T) {
	// Video RTP clock rate = 90000 Hz.
	tracker := NewJitterTracker(90000)

	// First packet establishes baseline.
	tracker.Update(
		0,
		0,
	)

	// Second packet:
	//
	// RTP timestamp difference = 9000 ticks
	// Expected media time = 100 ms
	//
	// Arrival time = 110 ms
	//
	// Difference = 10 ms
	// In 90000 Hz:
	// 10 ms = 900 RTP ticks
	tracker.Update(
		9000,
		0.110,
	)

	jitterMs := tracker.ValueMilliseconds()

	if jitterMs <= 0 {
		t.Fatalf(
			"expected jitter > 0, got %.2f ms",
			jitterMs,
		)
	}
}

func TestJitterMillisecondsConversion(t *testing.T) {
	tracker := NewJitterTracker(90000)

	// Directly set jitter for conversion testing.
	tracker.jitter = 900

	expected := 10.0

	actual := tracker.ValueMilliseconds()

	if math.Abs(actual-expected) > 0.0001 {
		t.Fatalf(
			"expected %.2f ms, got %.2f ms",
			expected,
			actual,
		)
	}
}

func TestAudioJitterMillisecondsConversion(t *testing.T) {
	// Opus commonly uses 48000 Hz RTP clock rate.
	tracker := NewJitterTracker(48000)

	// 480 RTP ticks = 10 ms.
	tracker.jitter = 480

	expected := 10.0

	actual := tracker.ValueMilliseconds()

	if math.Abs(actual-expected) > 0.0001 {
		t.Fatalf(
			"expected %.2f ms, got %.2f ms",
			expected,
			actual,
		)
	}
}

func TestJitterZeroClockRate(t *testing.T) {
	tracker := NewJitterTracker(0)

	tracker.jitter = 100

	actual := tracker.ValueMilliseconds()

	if actual != 0 {
		t.Fatalf(
			"expected 0 ms for zero clock rate, got %.2f",
			actual,
		)
	}
}

