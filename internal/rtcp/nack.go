package rtcp

import (
	"encoding/binary"
	"errors"
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

// BuildNACKPairs takes a slice of missing sequence numbers and returns a slice of NACKPair structs.

func BuildNACKPairs(missing []uint16) []NACKPair {
	if len(missing) == 0 {
		return nil
	}

	pairs := make([]NACKPair, 0)

	pid := missing[0]
	var blp uint16

	for i := 1; i < len(missing); i++ {
		distance := uint16(missing[i] - pid)

		if distance >= 1 && distance <= 16 {
			blp |= 1 << (distance - 1)
			continue
		}

		pairs = append(pairs, NACKPair{
			PID: pid,
			BLP: blp,
		})

		pid = missing[i]
		blp = 0
	}

	pairs = append(pairs, NACKPair{
		PID: pid,
		BLP: blp,
	})

	return pairs
}

// Marshal serializes the NACK packet into a byte slice.

func (n *NACK) Marshal() ([]byte, error) {
	if len(n.Pairs) == 0 {
		return nil, errors.New("NACK must contain at least one pair")
	}

	// Header + Sender SSRC + Media SSRC + NACK pairs
	size := 12 + len(n.Pairs)*4

	data := make([]byte, size)

	// Version = 2
	data[0] = 2 << 6

	// FMT = 1
	data[0] |= NACKFormat

	// PT = 205
	data[1] = RTPFeedbackType

	// RTCP length
	length := uint16(size/4 - 1)

	binary.BigEndian.PutUint16(data[2:4], length)

	// Sender SSRC
	binary.BigEndian.PutUint32(
		data[4:8],
		n.SenderSSRC,
	)

	// Media SSRC
	binary.BigEndian.PutUint32(
		data[8:12],
		n.MediaSSRC,
	)

	offset := 12

	for _, pair := range n.Pairs {
		binary.BigEndian.PutUint16(
			data[offset:offset+2],
			pair.PID,
		)

		binary.BigEndian.PutUint16(
			data[offset+2:offset+4],
			pair.BLP,
		)

		offset += 4
	}

	return data, nil
}

// UnmarshalNACK deserializes a byte slice into a NACK packet.

func UnmarshalNACK(data []byte) (*NACK, error) {
	if len(data) < 12 {
		return nil, errors.New("NACK packet too short")
	}

	if data[0]>>6 != 2 {
		return nil, errors.New("unsupported RTCP version")
	}

	if data[0]&0x1F != NACKFormat {
		return nil, errors.New("not a NACK packet")
	}

	if data[1] != RTPFeedbackType {
		return nil, errors.New("not an RTP feedback packet")
	}

	pairBytes := len(data) - 12

	if pairBytes%4 != 0 {
		return nil, errors.New("invalid NACK pair length")
	}

	nack := &NACK{
		SenderSSRC: binary.BigEndian.Uint32(data[4:8]),
		MediaSSRC:  binary.BigEndian.Uint32(data[8:12]),
		Pairs:      make([]NACKPair, 0, pairBytes/4),
	}

	offset := 12

	for offset < len(data) {
		pair := NACKPair{
			PID: binary.BigEndian.Uint16(
				data[offset : offset+2],
			),

			BLP: binary.BigEndian.Uint16(
				data[offset+2 : offset+4],
			),
		}

		nack.Pairs = append(nack.Pairs, pair)

		offset += 4
	}

	return nack, nil
}