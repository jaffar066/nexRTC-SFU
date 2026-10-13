package rtp

import (
	"fmt"
	"net"
)

// UDPSender is a simple RTP packet sender over UDP.
type UDPSender struct {
	Conn *net.UDPConn
}

func NewUDPSender(address string) (*UDPSender, error) {
	addr, err := net.ResolveUDPAddr("udp", address)
	if err != nil {
		return nil, err
	}

	conn, err := net.DialUDP("udp", nil, addr)
	if err != nil {
		return nil, err
	}

	return &UDPSender{Conn: conn}, nil
}

func (s *UDPSender) Send(packet *Packet) error {
	data, err := packet.Marshal()
	if err != nil {
		return err
	}

	_, err = s.Conn.Write(data)
	return err
}

func (s *UDPSender) Close() error {
	return s.Conn.Close()
}

type UDPReceiver struct {
	Conn *net.UDPConn
}

func NewUDPReceiver(address string) (*UDPReceiver, error) {
	addr, err := net.ResolveUDPAddr("udp", address)
	if err != nil {
		return nil, err
	}

	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return nil, err
	}

	return &UDPReceiver{Conn: conn}, nil
}

func (r *UDPReceiver) Receive() (*Packet, *net.UDPAddr, error) {
	buffer := make([]byte, 1500)

	n, remoteAddr, err := r.Conn.ReadFromUDP(buffer)
	if err != nil {
		return nil, nil, err
	}

	packet, err := Unmarshal(buffer[:n])
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse RTP packet: %w", err)
	}

	return packet, remoteAddr, nil
}

func (r *UDPReceiver) Close() error {
	return r.Conn.Close()
}