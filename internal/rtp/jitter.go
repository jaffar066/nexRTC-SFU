package rtp

import "math"

type JitterTracker struct {
	clockRate uint32

	initialized bool
	lastArrival float64
	lastRTP     uint32

	jitter float64
}

func NewJitterTracker(clockRate uint32) *JitterTracker {
	return &JitterTracker{
		clockRate: clockRate,
	}
}

// Update receives:
//
//	rtpTimestamp = RTP timestamp from the packet
//	arrivalTime  = packet arrival time in seconds
//
// It returns the current jitter in RTP timestamp units.
func (j *JitterTracker) Update(
	rtpTimestamp uint32,
	arrivalTime float64,
) uint32 {

	// First packet establishes our baseline.
	if !j.initialized {
		j.initialized = true
		j.lastArrival = arrivalTime
		j.lastRTP = rtpTimestamp

		return 0
	}

	// Convert arrival time from seconds into RTP clock ticks.
	arrivalRTP := arrivalTime * float64(j.clockRate)

	// Difference between current and previous arrival time.
	arrivalDifference :=
		arrivalRTP -
			j.lastArrival*float64(j.clockRate)

	// Difference between current and previous RTP timestamps.
	rtpDifference := float64(
		rtpTimestamp - j.lastRTP,
	)

	// D = |arrival difference - RTP timestamp difference|
	difference := math.Abs(
		arrivalDifference - rtpDifference,
	)

	// RFC-style exponentially smoothed jitter:
	//
	// J = J + (|D| - J) / 16
	j.jitter += (difference - j.jitter) / 16

	j.lastArrival = arrivalTime
	j.lastRTP = rtpTimestamp

	return uint32(j.jitter)
}

// Value returns the current jitter
// in RTP timestamp units.
func (j *JitterTracker) Value() uint32 {
	return uint32(j.jitter)
}

// ValueMilliseconds returns the current jitter
// converted into milliseconds.
func (j *JitterTracker) ValueMilliseconds() float64 {
	if j.clockRate == 0 {
		return 0
	}

	return (j.jitter / float64(j.clockRate)) * 1000
}
