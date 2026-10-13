package rtp

import (
	"fmt"

	"github.com/jaffar066/nexrtc/internal/rtcp"
)

// BuildRTCPNACK converts a scheduler NACKRequest
// into an RTCP NACK packet.
func BuildRTCPNACK(
	senderSSRC uint32,
	request NACKRequest,
) (*rtcp.NACK, error) {

	if len(request.Sequences) == 0 {
		return nil, fmt.Errorf("NACK request has no sequences")
	}

	pairs := rtcp.BuildNACKPairs(request.Sequences)

	if len(pairs) == 0 {
		return nil, fmt.Errorf("no NACK pairs generated")
	}

	return &rtcp.NACK{
		SenderSSRC: senderSSRC,
		MediaSSRC:  request.SSRC,
		Pairs:      pairs,
	}, nil
}
