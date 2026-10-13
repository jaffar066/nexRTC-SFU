package rtp

import (
	"fmt"

	"github.com/jaffar066/nexrtc/internal/rtcp"
)

type PacketSender interface {
	Send(packet *Packet) error
}

type RetransmissionManager struct {
	Buffer *RetransmissionBuffer
	Sender PacketSender
}

func NewRetransmissionManager(
	buffer *RetransmissionBuffer,
	sender PacketSender,
) *RetransmissionManager {
	return &RetransmissionManager{
		Buffer: buffer,
		Sender: sender,
	}
}

// ExpandNACKPair converts one NACK pair into
// the RTP sequence numbers that need retransmission.
//
// PID = first missing packet
//
// BLP:
// bit 0 -> PID + 1
// bit 1 -> PID + 2
// ...
// bit 15 -> PID + 16
func ExpandNACKPair(pair rtcp.NACKPair) []uint16 {
	sequences := []uint16{
		pair.PID,
	}

	for i := 0; i < 16; i++ {
		if pair.BLP&(1<<i) != 0 {
			sequence := pair.PID + uint16(i+1)
			sequences = append(sequences, sequence)
		}
	}

	return sequences
}

// HandleNACK finds requested packets in the
// retransmission buffer and sends them again.
//
// It returns the number of packets successfully retransmitted.
func (m *RetransmissionManager) HandleNACK(
	mediaSSRC uint32,
	nack *rtcp.NACK,
) (int, error) {

	if nack == nil {
		return 0, fmt.Errorf("NACK is nil")
	}

	if m.Buffer == nil {
		return 0, fmt.Errorf("retransmission buffer is nil")
	}

	if m.Sender == nil {
		return 0, fmt.Errorf("packet sender is nil")
	}

	if nack.MediaSSRC != mediaSSRC {
		return 0, fmt.Errorf(
			"SSRC mismatch: expected %d, got %d",
			mediaSSRC,
			nack.MediaSSRC,
		)
	}

	retransmitted := 0

	for _, pair := range nack.Pairs {
		sequences := ExpandNACKPair(pair)

		for _, sequenceNumber := range sequences {

			packet, exists := m.Buffer.Get(
				mediaSSRC,
				sequenceNumber,
			)

			if !exists {
				// Packet is no longer available.
				// Continue with the remaining packets.
				continue
			}

			if err := m.Sender.Send(packet); err != nil {
				return retransmitted, fmt.Errorf(
					"failed to retransmit SSRC=%d sequence=%d: %w",
					mediaSSRC,
					sequenceNumber,
					err,
				)
			}

			retransmitted++
		}
	}

	return retransmitted, nil
}