package rtp

import "time"

type RTCPStats struct {
	FractionLost     uint8
	CumulativeLost   int32
	HighestSeqNumber uint32
	Jitter           uint32
	LastSenderReport uint32
	DelaySinceLastSR uint32
}

type StreamStats struct {
	PacketsReceived uint64
	PacketsLost     uint64
	BytesReceived   uint64

	HighestSequence uint32

	LossPercentage float64
}

type Stream struct {
	SSRC uint32

	SequenceTracker *SequenceTracker
	JitterTracker   *JitterTracker

	RTCPStats   RTCPStats
	StreamStats StreamStats
}

type StreamManager struct {
	streams map[uint32]*Stream

	nackScheduler *NACKScheduler
}

func NewStreamManager() *StreamManager {
	return NewStreamManagerWithNACKScheduler(
		NewNACKScheduler(
			DefaultNACKSchedulerConfig(),
		),
	)
}

func NewStreamManagerWithNACKScheduler(
	scheduler *NACKScheduler,
) *StreamManager {

	if scheduler == nil {
		scheduler = NewNACKScheduler(
			DefaultNACKSchedulerConfig(),
		)
	}

	return &StreamManager{
		streams:       make(map[uint32]*Stream),
		nackScheduler: scheduler,
	}
}

func (m *StreamManager) GetOrCreate(
	ssrc uint32,
	clockRate uint32,
) *Stream {

	if m.streams == nil {
		m.streams = make(map[uint32]*Stream)
	}

	if stream, ok := m.streams[ssrc]; ok {
		return stream
	}

	stream := NewStream(
		ssrc,
		clockRate,
	)

	m.streams[ssrc] = stream

	return stream
}

func NewStream(
	ssrc uint32,
	clockRate uint32,
) *Stream {

	return &Stream{
		SSRC: ssrc,

		SequenceTracker: NewSequenceTracker(),

		JitterTracker: NewJitterTracker(
			clockRate,
		),

		RTCPStats: RTCPStats{},
	}
}

func (m *StreamManager) ProcessPacket(
	packet *Packet,
	arrivalTime float64,
	clockRate uint32,
) []uint16 {

	if packet == nil {
		return nil
	}

	stream := m.GetOrCreate(
		packet.SSRC,
		clockRate,
	)

	// Process sequence number.
	result := stream.SequenceTracker.PushResult(
		packet.SequenceNumber,
	)

	// If this packet was previously missing,
	// remove it from the NACK scheduler.
	if result.Recovered {
		m.nackScheduler.Recover(
			packet.SSRC,
			packet.SequenceNumber,
		)
	}

	// Schedule newly detected missing packets.
	if len(result.Missing) > 0 {
		m.nackScheduler.Schedule(
			packet.SSRC,
			result.Missing,
			time.Now(),
		)
	}

	// Update jitter.
	stream.JitterTracker.Update(
		packet.Timestamp,
		arrivalTime,
	)

	// Update received packet statistics.
	stream.StreamStats.PacketsReceived++

	// Count payload bytes.
	stream.StreamStats.BytesReceived += uint64(
		len(packet.Payload),
	)

	// Store extended sequence number.
	stream.StreamStats.HighestSequence =
		result.ExtendedSeq

	// Count newly detected missing packets.
	stream.StreamStats.PacketsLost += uint64(
		len(result.Missing),
	)

	// Calculate total packets seen.
	totalPackets :=
		stream.StreamStats.PacketsReceived +
			stream.StreamStats.PacketsLost

	if totalPackets > 0 {
		stream.StreamStats.LossPercentage =
			float64(stream.StreamStats.PacketsLost) /
				float64(totalPackets) *
				100
	}

	return result.Missing
}

// NACKScheduler returns the scheduler used by this manager.
func (m *StreamManager) NACKScheduler() *NACKScheduler {
	return m.nackScheduler
}

// PollNACKs returns NACK requests that are ready to be sent.
//
// The caller supplies the current time so the scheduler remains
// deterministic and easy to test.
func (m *StreamManager) PollNACKs(
	now time.Time,
) []NACKRequest {
	return m.nackScheduler.Poll(now)
}
