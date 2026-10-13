package rtp

const DefaultSequenceWindowSize uint32 = 1024

// SequenceResult contains the result of processing one RTP sequence number.
type SequenceResult struct {
	Missing     []uint16
	Recovered   bool
	Duplicate   bool
	ExtendedSeq uint32
}

type SequenceTracker struct {
	lastSequence    uint16
	initialized     bool
	highestSequence uint16
	highestExtended uint32

	received       map[uint32]bool
	pendingMissing map[uint32]bool

	windowSize uint32

	receivedPackets  uint64
	duplicatePackets uint64
	latePackets      uint64
}

func NewSequenceTracker() *SequenceTracker {
	return NewSequenceTrackerWithWindow(DefaultSequenceWindowSize)
}

func NewSequenceTrackerWithWindow(windowSize uint32) *SequenceTracker {
	if windowSize == 0 {
		windowSize = DefaultSequenceWindowSize
	}

	return &SequenceTracker{
		received:       make(map[uint32]bool),
		pendingMissing: make(map[uint32]bool),
		windowSize:     windowSize,
	}
}

// Push keeps the original API.
//
// It returns only the newly detected missing sequence numbers.
// Existing code and tests can continue using Push().
func (t *SequenceTracker) Push(sequence uint16) []uint16 {
	result := t.PushResult(sequence)
	return result.Missing
}

// PushResult processes an RTP sequence number and provides detailed
// information about what happened to that packet.
func (t *SequenceTracker) PushResult(sequence uint16) SequenceResult {
	if !t.initialized {
		t.initialized = true
		t.lastSequence = sequence
		t.highestSequence = sequence
		t.highestExtended = uint32(sequence)

		t.received[t.highestExtended] = true
		t.receivedPackets++

		return SequenceResult{
			ExtendedSeq: t.highestExtended,
		}
	}

	extended, forward := t.extendSequence(sequence)

	// Packet was already received.
	if t.received[extended] {
		t.duplicatePackets++

		return SequenceResult{
			Duplicate:   true,
			ExtendedSeq: extended,
		}
	}

	// New packet is ahead of the current highest sequence.
	if forward {
		distance := extended - t.highestExtended

		// Very large jump.
		//
		// We do not create thousands of fake missing packets.
		// Instead, start a new tracking window from this packet.
		if distance > t.windowSize {
			t.highestExtended = extended
			t.highestSequence = sequence
			t.lastSequence = sequence

			t.received = make(map[uint32]bool)
			t.pendingMissing = make(map[uint32]bool)

			t.received[extended] = true
			t.receivedPackets++

			return SequenceResult{
				ExtendedSeq: extended,
			}
		}

		var missing []uint16

		for seq := t.highestExtended + 1; seq < extended; seq++ {
			if !t.received[seq] {
				t.pendingMissing[seq] = true
				missing = append(missing, uint16(seq))
			}
		}

		t.highestExtended = extended
		t.highestSequence = sequence
		t.lastSequence = sequence

		t.received[extended] = true
		t.receivedPackets++

		t.cleanupWindow()

		return SequenceResult{
			Missing:     missing,
			ExtendedSeq: extended,
		}
	}

	// Packet arrived late and was previously marked as missing.
	if t.pendingMissing[extended] {
		delete(t.pendingMissing, extended)

		t.received[extended] = true
		t.receivedPackets++
		t.latePackets++

		t.cleanupWindow()

		return SequenceResult{
			Recovered:   true,
			ExtendedSeq: extended,
		}
	}

	// Old packet that was not previously marked as missing.
	t.received[extended] = true
	t.receivedPackets++
	t.latePackets++

	t.cleanupWindow()

	return SequenceResult{
		ExtendedSeq: extended,
	}
}

func (t *SequenceTracker) extendSequence(
	sequence uint16,
) (uint32, bool) {

	current := t.highestSequence

	// Signed 16-bit difference gives us the direction
	// while correctly handling RTP sequence wrap-around.
	delta := int16(sequence - current)

	extended := t.highestExtended + uint32(int32(delta))

	return extended, delta > 0
}

func (t *SequenceTracker) cleanupWindow() {
	minSequence := uint32(0)

	if t.highestExtended+1 > t.windowSize {
		minSequence =
			t.highestExtended -
				t.windowSize +
				1
	}

	for sequence := range t.received {
		if sequence < minSequence {
			delete(t.received, sequence)
		}
	}

	for sequence := range t.pendingMissing {
		if sequence < minSequence {
			delete(t.pendingMissing, sequence)
		}
	}
}

func (t *SequenceTracker) ReceivedPackets() uint64 {
	return t.receivedPackets
}

func (t *SequenceTracker) DuplicatePackets() uint64 {
	return t.duplicatePackets
}

func (t *SequenceTracker) LatePackets() uint64 {
	return t.latePackets
}

func (t *SequenceTracker) HighestSequence() uint16 {
	return t.highestSequence
}

func (t *SequenceTracker) HighestExtendedSequence() uint32 {
	return t.highestExtended
}

func (t *SequenceTracker) PendingMissingPackets() int {
	return len(t.pendingMissing)
}

func (t *SequenceTracker) IsPendingMissing(
	sequence uint16,
) bool {
	extended, _ := t.extendSequence(sequence)

	return t.pendingMissing[extended]
}

func (t *SequenceTracker) WindowSize() uint32 {
	return t.windowSize
}

func (t *SequenceTracker) ReceivedHistorySize() int {
	return len(t.received)
}
