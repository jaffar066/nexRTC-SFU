package rtp

import "testing"

func TestSequenceTrackerFirstPacket(t *testing.T) {
	tracker := NewSequenceTracker()

	missing := tracker.Push(100)

	if len(missing) != 0 {
		t.Fatalf("expected no missing packets, got %v", missing)
	}

	if tracker.HighestSequence() != 100 {
		t.Fatalf("expected highest sequence 100, got %d",
			tracker.HighestSequence())
	}

	if tracker.HighestExtendedSequence() != 100 {
		t.Fatalf("expected extended sequence 100, got %d",
			tracker.HighestExtendedSequence())
	}

	if tracker.ReceivedPackets() != 1 {
		t.Fatalf("expected 1 received packet, got %d",
			tracker.ReceivedPackets())
	}
}

func TestSequenceTrackerNormalPackets(t *testing.T) {
	tracker := NewSequenceTracker()

	tracker.Push(100)

	if missing := tracker.Push(101); len(missing) != 0 {
		t.Fatalf("expected no missing packets, got %v", missing)
	}

	if missing := tracker.Push(102); len(missing) != 0 {
		t.Fatalf("expected no missing packets, got %v", missing)
	}

	if tracker.HighestSequence() != 102 {
		t.Fatalf("expected highest sequence 102, got %d",
			tracker.HighestSequence())
	}
}

func TestSequenceTrackerMissingPacket(t *testing.T) {
	tracker := NewSequenceTracker()

	tracker.Push(100)

	missing := tracker.Push(102)

	if len(missing) != 1 {
		t.Fatalf("expected 1 missing packet, got %v", missing)
	}

	if missing[0] != 101 {
		t.Fatalf("expected missing packet 101, got %d", missing[0])
	}

	if !tracker.IsPendingMissing(101) {
		t.Fatal("expected packet 101 to be pending")
	}
}

func TestSequenceTrackerMultipleMissing(t *testing.T) {
	tracker := NewSequenceTracker()

	tracker.Push(100)

	missing := tracker.Push(105)

	expected := []uint16{101, 102, 103, 104}

	if len(missing) != len(expected) {
		t.Fatalf("expected %v, got %v", expected, missing)
	}

	for i := range expected {
		if missing[i] != expected[i] {
			t.Fatalf("expected missing[%d] = %d, got %d",
				i, expected[i], missing[i])
		}
	}
}

func TestSequenceTrackerDuplicate(t *testing.T) {
	tracker := NewSequenceTracker()

	tracker.Push(100)
	tracker.Push(101)
	tracker.Push(101)

	if tracker.DuplicatePackets() != 1 {
		t.Fatalf("expected 1 duplicate, got %d",
			tracker.DuplicatePackets())
	}

	if tracker.ReceivedPackets() != 2 {
		t.Fatalf("expected 2 received packets, got %d",
			tracker.ReceivedPackets())
	}
}

func TestSequenceTrackerLatePacket(t *testing.T) {
	tracker := NewSequenceTracker()

	tracker.Push(100)
	tracker.Push(102)

	if tracker.PendingMissingPackets() != 1 {
		t.Fatalf("expected 1 pending packet, got %d",
			tracker.PendingMissingPackets())
	}

	tracker.Push(101)

	if tracker.LatePackets() != 1 {
		t.Fatalf("expected 1 late packet, got %d",
			tracker.LatePackets())
	}

	if tracker.PendingMissingPackets() != 0 {
		t.Fatalf("expected no pending packets, got %d",
			tracker.PendingMissingPackets())
	}
}

func TestSequenceTrackerWrapAround(t *testing.T) {
	tracker := NewSequenceTracker()

	tracker.Push(65534)
	tracker.Push(65535)

	missing := tracker.Push(1)

	if len(missing) != 1 {
		t.Fatalf("expected 1 missing packet, got %v", missing)
	}

	if missing[0] != 0 {
		t.Fatalf("expected missing packet 0, got %d", missing[0])
	}

	if tracker.HighestSequence() != 1 {
		t.Fatalf("expected highest sequence 1, got %d",
			tracker.HighestSequence())
	}

	if tracker.HighestExtendedSequence() != 65537 {
		t.Fatalf("expected extended sequence 65537, got %d",
			tracker.HighestExtendedSequence())
	}
}

func TestSequenceTrackerWrapAroundWithoutLoss(t *testing.T) {
	tracker := NewSequenceTracker()

	tracker.Push(65534)
	tracker.Push(65535)

	if missing := tracker.Push(0); len(missing) != 0 {
		t.Fatalf("expected no missing packets, got %v", missing)
	}

	if missing := tracker.Push(1); len(missing) != 0 {
		t.Fatalf("expected no missing packets, got %v", missing)
	}

	if tracker.HighestExtendedSequence() != 65537 {
		t.Fatalf("expected extended sequence 65537, got %d",
			tracker.HighestExtendedSequence())
	}
}

func TestSequenceTrackerWrapAroundLatePacket(t *testing.T) {
	tracker := NewSequenceTracker()

	tracker.Push(65534)
	tracker.Push(65535)

	missing := tracker.Push(1)

	if len(missing) != 1 || missing[0] != 0 {
		t.Fatalf("expected missing packet 0, got %v", missing)
	}

	tracker.Push(0)

	if tracker.PendingMissingPackets() != 0 {
		t.Fatalf("expected pending missing to be 0, got %d",
			tracker.PendingMissingPackets())
	}

	if tracker.LatePackets() != 1 {
		t.Fatalf("expected 1 late packet, got %d",
			tracker.LatePackets())
	}
}

func TestSequenceTrackerReceivedPackets(t *testing.T) {
	tracker := NewSequenceTracker()

	tracker.Push(100)
	tracker.Push(101)
	tracker.Push(102)

	if tracker.ReceivedPackets() != 3 {
		t.Fatalf("expected 3 received packets, got %d",
			tracker.ReceivedPackets())
	}
}

func TestSequenceTrackerPendingMissing(t *testing.T) {
	tracker := NewSequenceTracker()

	tracker.Push(100)
	tracker.Push(103)

	if tracker.PendingMissingPackets() != 2 {
		t.Fatalf("expected 2 pending packets, got %d",
			tracker.PendingMissingPackets())
	}

	if !tracker.IsPendingMissing(101) {
		t.Fatal("expected 101 to be pending")
	}

	if !tracker.IsPendingMissing(102) {
		t.Fatal("expected 102 to be pending")
	}
}

func TestSequenceTrackerLatePacketRemovesPending(t *testing.T) {
	tracker := NewSequenceTracker()

	tracker.Push(100)
	tracker.Push(103)

	tracker.Push(101)

	if tracker.PendingMissingPackets() != 1 {
		t.Fatalf("expected 1 pending packet, got %d",
			tracker.PendingMissingPackets())
	}

	if tracker.IsPendingMissing(101) {
		t.Fatal("expected 101 to be removed from pending")
	}

	if !tracker.IsPendingMissing(102) {
		t.Fatal("expected 102 to remain pending")
	}
}

func TestSequenceTrackerAllMissingRecovered(t *testing.T) {
	tracker := NewSequenceTracker()

	tracker.Push(100)
	tracker.Push(104)

	tracker.Push(101)
	tracker.Push(102)
	tracker.Push(103)

	if tracker.PendingMissingPackets() != 0 {
		t.Fatalf("expected no pending packets, got %d",
			tracker.PendingMissingPackets())
	}

	if tracker.ReceivedPackets() != 5 {
		t.Fatalf("expected 5 received packets, got %d",
			tracker.ReceivedPackets())
	}
}

func TestSequenceTrackerSlidingWindow(t *testing.T) {
	windowSize := uint32(4)

	tracker := NewSequenceTrackerWithWindow(windowSize)

	tracker.Push(1)
	tracker.Push(2)
	tracker.Push(3)
	tracker.Push(4)

	if tracker.ReceivedHistorySize() != 4 {
		t.Fatalf("expected history size 4, got %d",
			tracker.ReceivedHistorySize())
	}

	tracker.Push(5)

	if tracker.ReceivedHistorySize() != 4 {
		t.Fatalf("expected history size to remain 4, got %d",
			tracker.ReceivedHistorySize())
	}

	// Sequence 1 is outside the window now.
	if tracker.ReceivedHistorySize() > int(windowSize) {
		t.Fatalf("history exceeded window size")
	}
}

func TestSequenceTrackerOldPacketOutsideWindow(t *testing.T) {
	tracker := NewSequenceTrackerWithWindow(4)

	tracker.Push(1)
	tracker.Push(2)
	tracker.Push(3)
	tracker.Push(4)
	tracker.Push(5)
	tracker.Push(6)

	before := tracker.ReceivedPackets()

	// 1 is now outside the tracking window.
	tracker.Push(1)

	if tracker.ReceivedPackets() != before+1 {
		t.Fatalf("expected old packet to be accepted as late/outside history")
	}

	if tracker.LatePackets() == 0 {
		t.Fatal("expected old packet to be counted as late")
	}
}

func TestSequenceTrackerLargeJumpResetsWindow(t *testing.T) {
	tracker := NewSequenceTrackerWithWindow(4)

	tracker.Push(100)
	tracker.Push(101)
	tracker.Push(102)

	missing := tracker.Push(200)

	if len(missing) != 0 {
		t.Fatalf("expected no NACK candidates for large jump, got %v",
			missing)
	}

	if tracker.HighestSequence() != 200 {
		t.Fatalf("expected highest sequence 200, got %d",
			tracker.HighestSequence())
	}

	if tracker.ReceivedHistorySize() != 1 {
		t.Fatalf("expected history to reset to 1 packet, got %d",
			tracker.ReceivedHistorySize())
	}
}

func TestSequenceTrackerExtendedSequenceProgression(t *testing.T) {
	tracker := NewSequenceTracker()

	tracker.Push(65534)

	if tracker.HighestExtendedSequence() != 65534 {
		t.Fatalf("expected 65534")
	}

	tracker.Push(65535)

	if tracker.HighestExtendedSequence() != 65535 {
		t.Fatalf("expected 65535")
	}

	tracker.Push(0)

	if tracker.HighestExtendedSequence() != 65536 {
		t.Fatalf("expected 65536 after wrap")
	}

	tracker.Push(1)

	if tracker.HighestExtendedSequence() != 65537 {
		t.Fatalf("expected 65537, got %d",
			tracker.HighestExtendedSequence())
	}
}

func TestSequenceTrackerBackwardWrapIsLate(t *testing.T) {
	tracker := NewSequenceTracker()

	tracker.Push(1)
	tracker.Push(2)
	tracker.Push(3)

	tracker.Push(65535)

	if tracker.LatePackets() != 1 {
		t.Fatalf("expected 65535 to be late, got %d",
			tracker.LatePackets())
	}

	if tracker.HighestExtendedSequence() != 3 {
		t.Fatalf("highest extended sequence changed unexpectedly")
	}
}