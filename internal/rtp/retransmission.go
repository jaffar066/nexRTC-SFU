package rtp

import (
	"fmt"
	"sync"
)

type RetransmissionKey struct {
	SSRC           uint32
	SequenceNumber uint16
}

type RetransmissionBuffer struct {
	mu sync.RWMutex

	maxPackets int

	packets map[RetransmissionKey]*Packet
	order   []RetransmissionKey
}

func NewRetransmissionBuffer(maxPackets int) *RetransmissionBuffer {
	if maxPackets <= 0 {
		maxPackets = 1000
	}

	return &RetransmissionBuffer{
		maxPackets: maxPackets,
		packets:    make(map[RetransmissionKey]*Packet),
		order:      make([]RetransmissionKey, 0, maxPackets),
	}
}

func (b *RetransmissionBuffer) Store(packet *Packet) {
	if packet == nil {
		return
	}

	key := RetransmissionKey{
		SSRC:           packet.SSRC,
		SequenceNumber: packet.SequenceNumber,
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	// If packet already exists, replace it.
	if _, exists := b.packets[key]; exists {
		b.packets[key] = packet
		return
	}

	b.packets[key] = packet
	b.order = append(b.order, key)

	// Remove oldest packets when buffer is full.
	for len(b.order) > b.maxPackets {
		oldest := b.order[0]

		b.order = b.order[1:]

		delete(b.packets, oldest)
	}
}

func (b *RetransmissionBuffer) Get(
	ssrc uint32,
	sequenceNumber uint16,
) (*Packet, bool) {

	key := RetransmissionKey{
		SSRC:           ssrc,
		SequenceNumber: sequenceNumber,
	}

	b.mu.RLock()
	defer b.mu.RUnlock()

	packet, exists := b.packets[key]

	if !exists {
		return nil, false
	}

	return packet, true
}

func (b *RetransmissionBuffer) Size() int {
	b.mu.RLock()
	defer b.mu.RUnlock()

	return len(b.packets)
}

func (b *RetransmissionBuffer) GetAndSend(
	ssrc uint32,
	sequenceNumber uint16,
	sender *UDPSender,
) error {
	packet, exists := b.Get(ssrc, sequenceNumber)

	if !exists {
		return fmt.Errorf(
			"packet not found: SSRC=%d Sequence=%d",
			ssrc,
			sequenceNumber,
		)
	}

	return sender.Send(packet)
}