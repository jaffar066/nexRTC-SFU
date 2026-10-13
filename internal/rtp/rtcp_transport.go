package rtp

import (
	"fmt"
	"net"

	"github.com/jaffar066/nexrtc/internal/rtcp"
)

type RTCPTransport struct {
	conn *net.UDPConn
}

// BuildRTCPNACK converts a scheduler NACKRequest
func NewRTCPTransport(address string) (*RTCPTransport, error) {
	addr, err := net.ResolveUDPAddr("udp", address)
	if err != nil {
		return nil, fmt.Errorf("resolve RTCP address: %w", err)
	}

	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return nil, fmt.Errorf("listen RTCP UDP: %w", err)
	}

	return &RTCPTransport{
		conn: conn,
	}, nil
}

func (t *RTCPTransport) Send(
	address string,
	packet *rtcp.NACK,
) error {
	if packet == nil {
		return fmt.Errorf("RTCP packet is nil")
	}

	data, err := packet.Marshal()
	if err != nil {
		return fmt.Errorf("marshal RTCP packet: %w", err)
	}

	target, err := net.ResolveUDPAddr("udp", address)
	if err != nil {
		return fmt.Errorf("resolve target address: %w", err)
	}

	_, err = t.conn.WriteToUDP(data, target)
	if err != nil {
		return fmt.Errorf("send RTCP packet: %w", err)
	}

	return nil
}

func (t *RTCPTransport) Receive() ([]byte, *net.UDPAddr, error) {
	buffer := make([]byte, 1500)

	n, address, err := t.conn.ReadFromUDP(buffer)
	if err != nil {
		return nil, nil, fmt.Errorf("receive RTCP packet: %w", err)
	}

	return buffer[:n], address, nil
}

func (t *RTCPTransport) Close() error {
	if t.conn == nil {
		return nil
	}

	return t.conn.Close()
}

func (t *RTCPTransport) LocalAddr() string {
	if t.conn == nil {
		return ""
	}

	return t.conn.LocalAddr().String()
}
