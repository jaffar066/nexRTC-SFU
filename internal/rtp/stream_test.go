package rtp

import (
	"testing"
)

func TestStreamManagerGetOrCreate(t *testing.T) {
	manager := NewStreamManager()

	ssrc := uint32(12345)
	clockRate := uint32(90000)

	stream := manager.GetOrCreate(
		ssrc,
		clockRate,
	)

	if stream == nil {
		t.Fatal("expected stream, got nil")
	}

	if stream.SSRC != ssrc {
		t.Fatalf(
			"expected SSRC %d, got %d",
			ssrc,
			stream.SSRC,
		)
	}

	if stream.SequenceTracker == nil {
		t.Fatal("expected SequenceTracker, got nil")
	}

	if stream.JitterTracker == nil {
		t.Fatal("expected JitterTracker, got nil")
	}
}

func TestStreamManagerReturnsSameStream(t *testing.T) {
	manager := NewStreamManager()

	ssrc := uint32(12345)
	clockRate := uint32(90000)

	stream1 := manager.GetOrCreate(
		ssrc,
		clockRate,
	)

	stream2 := manager.GetOrCreate(
		ssrc,
		clockRate,
	)

	if stream1 != stream2 {
		t.Fatal("expected same stream for same SSRC")
	}
}

func TestStreamManagerCreatesDifferentStreams(t *testing.T) {
	manager := NewStreamManager()

	clockRate := uint32(90000)

	stream1 := manager.GetOrCreate(
		1111,
		clockRate,
	)

	stream2 := manager.GetOrCreate(
		2222,
		clockRate,
	)

	if stream1 == stream2 {
		t.Fatal("expected different streams for different SSRCs")
	}

	if stream1.SSRC == stream2.SSRC {
		t.Fatal("expected different SSRCs")
	}
}

func TestStreamStatistics(t *testing.T) {
	manager := NewStreamManager()

	ssrc := uint32(12345)
	clockRate := uint32(90000)

	packets := []*Packet{
		{
			Version:        2,
			SequenceNumber: 1,
			Timestamp:      0,
			SSRC:           ssrc,
			Payload:        []byte("packet-1"),
		},
		{
			Version:        2,
			SequenceNumber: 2,
			Timestamp:      3000,
			SSRC:           ssrc,
			Payload:        []byte("packet-2"),
		},
		{
			Version:        2,
			SequenceNumber: 4,
			Timestamp:      9000,
			SSRC:           ssrc,
			Payload:        []byte("packet-4"),
		},
	}

	arrivalTime := 0.0

	for _, packet := range packets {
		manager.ProcessPacket(
			packet,
			arrivalTime,
			clockRate,
		)

		arrivalTime += 0.033333
	}

	stream := manager.GetOrCreate(
		ssrc,
		clockRate,
	)

	// We received packets 1, 2 and 4.
	if stream.StreamStats.PacketsReceived != 3 {
		t.Fatalf(
			"expected 3 received packets, got %d",
			stream.StreamStats.PacketsReceived,
		)
	}

	// Packet 3 was missing.
	if stream.StreamStats.PacketsLost != 1 {
		t.Fatalf(
			"expected 1 lost packet, got %d",
			stream.StreamStats.PacketsLost,
		)
	}

	// Highest sequence number should be 4.
	if stream.StreamStats.HighestSequence != 4 {
		t.Fatalf(
			"expected highest sequence 4, got %d",
			stream.StreamStats.HighestSequence,
		)
	}

	// Total packets = 3 received + 1 lost = 4.
	// Loss = 1 / 4 * 100 = 25%.
	expectedLoss := 25.0

	if stream.StreamStats.LossPercentage != expectedLoss {
		t.Fatalf(
			"expected loss %.2f%%, got %.2f%%",
			expectedLoss,
			stream.StreamStats.LossPercentage,
		)
	}

	// packet-1 = 8 bytes
	// packet-2 = 8 bytes
	// packet-4 = 8 bytes
	expectedBytes := uint64(24)

	if stream.StreamStats.BytesReceived != expectedBytes {
		t.Fatalf(
			"expected %d bytes, got %d",
			expectedBytes,
			stream.StreamStats.BytesReceived,
		)
	}
}

func TestStreamStatisticsNoPacketLoss(t *testing.T) {
	manager := NewStreamManager()

	ssrc := uint32(9999)
	clockRate := uint32(90000)

	packets := []*Packet{
		{
			Version:        2,
			SequenceNumber: 1,
			Timestamp:      0,
			SSRC:           ssrc,
			Payload:        []byte("hello"),
		},
		{
			Version:        2,
			SequenceNumber: 2,
			Timestamp:      3000,
			SSRC:           ssrc,
			Payload:        []byte("world"),
		},
		{
			Version:        2,
			SequenceNumber: 3,
			Timestamp:      6000,
			SSRC:           ssrc,
			Payload:        []byte("test"),
		},
	}

	arrivalTime := 0.0

	for _, packet := range packets {
		manager.ProcessPacket(
			packet,
			arrivalTime,
			clockRate,
		)

		arrivalTime += 0.033333
	}

	stream := manager.GetOrCreate(
		ssrc,
		clockRate,
	)

	if stream.StreamStats.PacketsReceived != 3 {
		t.Fatalf(
			"expected 3 received packets, got %d",
			stream.StreamStats.PacketsReceived,
		)
	}

	if stream.StreamStats.PacketsLost != 0 {
		t.Fatalf(
			"expected 0 lost packets, got %d",
			stream.StreamStats.PacketsLost,
		)
	}

	if stream.StreamStats.LossPercentage != 0 {
		t.Fatalf(
			"expected 0%% packet loss, got %.2f%%",
			stream.StreamStats.LossPercentage,
		)
	}
}