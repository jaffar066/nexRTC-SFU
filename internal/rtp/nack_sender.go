package rtp

import (
	"fmt"
)

type NACKSender struct {
	Transport     *RTCPTransport
	SenderSSRC    uint32
	RemoteAddress string
}

func NewNACKSender(
	transport *RTCPTransport,
	senderSSRC uint32,
	remoteAddress string,
) *NACKSender {
	return &NACKSender{
		Transport:     transport,
		SenderSSRC:    senderSSRC,
		RemoteAddress: remoteAddress,
	}
}

func (s *NACKSender) Send(request NACKRequest) error {
	if s.Transport == nil {
		return fmt.Errorf("RTCP transport is nil")
	}

	if request.SSRC == 0 {
		return fmt.Errorf("media SSRC is zero")
	}

	if len(request.Sequences) == 0 {
		return fmt.Errorf("NACK request has no sequences")
	}

	nack, err := BuildRTCPNACK(
		s.SenderSSRC,
		request,
	)
	if err != nil {
		return fmt.Errorf("build RTCP NACK: %w", err)
	}

	if err := s.Transport.Send(
		s.RemoteAddress,
		nack,
	); err != nil {
		return fmt.Errorf("send RTCP NACK: %w", err)
	}

	return nil
}
