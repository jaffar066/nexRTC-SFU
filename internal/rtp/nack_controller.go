package rtp

import (
	"fmt"
	"time"
)

type NACKController struct {
	Streams *StreamManager
	Sender  *NACKSender
}

func NewNACKController(
	streams *StreamManager,
	sender *NACKSender,
) *NACKController {
	return &NACKController{
		Streams: streams,
		Sender:  sender,
	}
}

// PollAndSend checks the NACK scheduler and sends
// all NACK requests that are ready.
func (c *NACKController) PollAndSend(
	now time.Time,
) ([]NACKRequest, error) {

	if c.Streams == nil {
		return nil, fmt.Errorf("stream manager is nil")
	}

	if c.Sender == nil {
		return nil, fmt.Errorf("NACK sender is nil")
	}

	requests := c.Streams.PollNACKs(now)

	for _, request := range requests {
		if err := c.Sender.Send(request); err != nil {
			return requests, fmt.Errorf(
				"send NACK for SSRC %d: %w",
				request.SSRC,
				err,
			)
		}
	}

	return requests, nil
}