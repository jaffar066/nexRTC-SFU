package rtp

import (
	"fmt"

	"github.com/jaffar066/nexrtc/internal/rtcp"
)

type RTCPHandler struct {
	RetransmissionManager *RetransmissionManager
}

func NewRTCPHandler(
	manager *RetransmissionManager,
) *RTCPHandler {
	return &RTCPHandler{
		RetransmissionManager: manager,
	}
}

// HandlePacket receives a raw RTCP packet,
// identifies the RTCP type and handles it.
func (h *RTCPHandler) HandlePacket(data []byte) (int, error) {
	if len(data) < rtcp.HeaderSize {
		return 0, fmt.Errorf("RTCP packet too short")
	}

	packetType := data[1]

	switch packetType {

	case rtcp.RTPFeedbackType:
		return h.handleRTPFeedback(data)

	default:
		return 0, fmt.Errorf(
			"unsupported RTCP packet type: %d",
			packetType,
		)
	}
}

func (h *RTCPHandler) handleRTPFeedback(
	data []byte,
) (int, error) {

	if len(data) < 4 {
		return 0, fmt.Errorf("RTCP feedback packet too short")
	}

	// NACK has FMT = 1.
	fmtValue := data[0] & 0x1F

	if fmtValue != rtcp.NACKFormat {
		return 0, fmt.Errorf(
			"unsupported RTP feedback format: %d",
			fmtValue,
		)
	}

	nack, err := rtcp.UnmarshalNACK(data)

	if err != nil {
		return 0, fmt.Errorf(
			"failed to parse NACK: %w",
			err,
		)
	}

	if h.RetransmissionManager == nil {
		return 0, fmt.Errorf(
			"retransmission manager is nil",
		)
	}

	return h.RetransmissionManager.HandleNACK(
		nack.MediaSSRC,
		nack,
	)
}