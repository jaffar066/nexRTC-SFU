package rtp

import (
	"fmt"
	"net"
	"time"
)

type RTPReceiveLoop struct {
	conn       *net.UDPConn
	streams    *StreamManager
	forwarder  RTPPacketForwarder
	clockRate  uint32
	bufferSize int
}

type rtpReadError struct {
	err error
}

func (e *rtpReadError) Error() string {
	return fmt.Sprintf("read RTP packet: %v", e.err)
}

func (e *rtpReadError) Unwrap() error {
	return e.err
}

func NewRTPReceiveLoop(
	conn *net.UDPConn,
	streams *StreamManager,
	clockRate uint32,
) *RTPReceiveLoop {
	return &RTPReceiveLoop{
		conn:       conn,
		streams:    streams,
		clockRate:  clockRate,
		bufferSize: 1500,
	}
}

func NewRTPReceiveLoopWithForwarder(
	conn *net.UDPConn,
	streams *StreamManager,
	clockRate uint32,
	forwarder RTPPacketForwarder,
) *RTPReceiveLoop {
	loop := NewRTPReceiveLoop(conn, streams, clockRate)
	loop.forwarder = forwarder
	return loop
}

func (r *RTPReceiveLoop) ReceiveOnce() (*Packet, error) {
	if r.conn == nil {
		return nil, fmt.Errorf("RTP connection is nil")
	}

	if r.streams == nil {
		return nil, fmt.Errorf("stream manager is nil")
	}

	buffer := make([]byte, r.bufferSize)

	n, _, err := r.conn.ReadFromUDP(buffer)
	if err != nil {
		return nil, &rtpReadError{err: err}
	}

	packet, err := Unmarshal(buffer[:n])
	if err != nil {
		return nil, fmt.Errorf("unmarshal RTP packet: %w", err)
	}

	now := time.Now()
	arrivalTime := float64(now.UnixNano()) / 1e9

	r.streams.ProcessPacketAt(
		packet,
		arrivalTime,
		r.clockRate,
		now,
	)

	if r.forwarder != nil {
		if err := r.forwarder.Forward(packet); err != nil {
			return packet, fmt.Errorf("forward RTP packet: %w", err)
		}
	}

	return packet, nil
}
