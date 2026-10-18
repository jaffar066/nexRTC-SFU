package rtcp

import (
	"encoding/binary"
	"errors"
	"fmt"
	"sort"
)

const (
	RTPFeedbackType uint8 = 205
	NACKFormat      uint8 = 1
)

type NACKPair struct {
	PID uint16
	BLP uint16
}

type NACK struct {
	SenderSSRC uint32
	MediaSSRC  uint32

	Pairs []NACKPair
}

// BuildNACKPairs groups missing sequence numbers into PID/BLP pairs.
// Input is ordered relative to its first sequence number; the values should
// span less than half the uint16 sequence space.
func BuildNACKPairs(missing []uint16) []NACKPair {
	if len(missing) == 0 {
		return nil
	}

	base := missing[0]
	sequences := append([]uint16(nil), missing...)
	sort.Slice(sequences, func(i, j int) bool {
		return uint16(sequences[i]-base) < uint16(sequences[j]-base)
	})

	pairs := make([]NACKPair, 0, len(sequences))
	pid := sequences[0]
	var blp uint16
	lastSequence := pid

	for _, sequence := range sequences[1:] {
		if sequence == lastSequence {
			continue
		}
		distance := uint16(sequence - pid)

		if distance >= 1 && distance <= 16 {
			blp |= 1 << (distance - 1)
			lastSequence = sequence
			continue
		}

		pairs = append(pairs, NACKPair{
			PID: pid,
			BLP: blp,
		})

		pid = sequence
		blp = 0
		lastSequence = sequence
	}

	pairs = append(pairs, NACKPair{
		PID: pid,
		BLP: blp,
	})

	return pairs
}

// Marshal serializes the NACK packet into a byte slice.

func (n *NACK) Marshal() ([]byte, error) {
	if n == nil {
		return nil, errors.New("NACK is nil")
	}
	if len(n.Pairs) == 0 {
		return nil, errors.New("NACK must contain at least one pair")
	}
	const maxNACKPairs = 65533
	if len(n.Pairs) > maxNACKPairs {
		return nil, fmt.Errorf("NACK has too many feedback pairs: %d", len(n.Pairs))
	}

	payload := make([]byte, 8+len(n.Pairs)*4)
	binary.BigEndian.PutUint32(payload[0:4], n.SenderSSRC)
	binary.BigEndian.PutUint32(payload[4:8], n.MediaSSRC)
	offset := 8
	for _, pair := range n.Pairs {
		binary.BigEndian.PutUint16(payload[offset:offset+2], pair.PID)
		binary.BigEndian.PutUint16(payload[offset+2:offset+4], pair.BLP)
		offset += 4
	}
	return (&Packet{
		Version: 2,
		Count:   NACKFormat,
		Type:    RTPFeedbackType,
		Payload: payload,
	}).Marshal()
}

// UnmarshalNACK parses one standalone RTCP NACK packet.
func UnmarshalNACK(data []byte) (*NACK, error) {
	packet, err := Unmarshal(data)
	if err != nil {
		return nil, fmt.Errorf("parse RTCP NACK packet: %w", err)
	}
	if packet.Type != RTPFeedbackType {
		return nil, errors.New("not an RTP feedback packet")
	}
	if packet.Count != NACKFormat {
		return nil, errors.New("not an RTCP NACK packet")
	}
	if len(packet.Payload) < 12 {
		return nil, errors.New("NACK packet must contain at least one pair")
	}
	pairBytes := len(packet.Payload) - 8
	if pairBytes%4 != 0 {
		return nil, errors.New("invalid NACK pair length")
	}
	nack := &NACK{
		SenderSSRC: binary.BigEndian.Uint32(packet.Payload[0:4]),
		MediaSSRC:  binary.BigEndian.Uint32(packet.Payload[4:8]),
		Pairs:      make([]NACKPair, 0, pairBytes/4),
	}

	for offset := 8; offset < len(packet.Payload); offset += 4 {
		pair := NACKPair{
			PID: binary.BigEndian.Uint16(packet.Payload[offset : offset+2]),
			BLP: binary.BigEndian.Uint16(packet.Payload[offset+2 : offset+4]),
		}
		nack.Pairs = append(nack.Pairs, pair)
	}

	return nack, nil
}
