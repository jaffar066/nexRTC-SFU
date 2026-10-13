package rtp

import "testing"

func TestRetransmissionBuffer(t *testing.T) {
	buffer := NewRetransmissionBuffer(10)

	packet := &Packet{
		Version:        2,
		PayloadType:    96,
		SequenceNumber: 100,
		Timestamp:      90000,
		SSRC:           12345,
		Payload:        []byte("video packet"),
	}

	buffer.Store(packet)

	if buffer.Size() != 1 {
		t.Fatalf(
			"expected buffer size 1, got %d",
			buffer.Size(),
		)
	}

	found, exists := buffer.Get(12345, 100)

	if !exists {
		t.Fatal("packet not found")
	}

	if found.SequenceNumber != 100 {
		t.Fatalf(
			"expected sequence 100, got %d",
			found.SequenceNumber,
		)
	}

	if found.SSRC != 12345 {
		t.Fatalf(
			"expected SSRC 12345, got %d",
			found.SSRC,
		)
	}
}

func TestRetransmissionBufferSSRCIsolation(t *testing.T) {
	buffer := NewRetransmissionBuffer(10)

	video := &Packet{
		Version:        2,
		SequenceNumber: 100,
		SSRC:           1111,
		Payload:        []byte("video"),
	}

	audio := &Packet{
		Version:        2,
		SequenceNumber: 100,
		SSRC:           2222,
		Payload:        []byte("audio"),
	}

	buffer.Store(video)
	buffer.Store(audio)

	_, videoExists := buffer.Get(1111, 100)
	_, audioExists := buffer.Get(2222, 100)

	if !videoExists {
		t.Fatal("video packet not found")
	}

	if !audioExists {
		t.Fatal("audio packet not found")
	}
}

func TestRetransmissionBufferEviction(t *testing.T) {
	buffer := NewRetransmissionBuffer(3)

	for seq := uint16(100); seq <= 103; seq++ {
		packet := &Packet{
			Version:        2,
			SequenceNumber: seq,
			SSRC:           12345,
		}

		buffer.Store(packet)
	}

	if buffer.Size() != 3 {
		t.Fatalf(
			"expected size 3, got %d",
			buffer.Size(),
		)
	}

	_, exists := buffer.Get(12345, 100)

	if exists {
		t.Fatal("oldest packet should have been evicted")
	}

	_, exists = buffer.Get(12345, 103)

	if !exists {
		t.Fatal("latest packet should exist")
	}
}

func TestRetransmissionBufferGetAndSend(t *testing.T) {
	receiver, err := NewUDPReceiver("127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer receiver.Close()

	address := receiver.Conn.LocalAddr().String()

	sender, err := NewUDPSender(address)
	if err != nil {
		t.Fatal(err)
	}
	defer sender.Close()

	buffer := NewRetransmissionBuffer(10)

	packet := &Packet{
		Version:        2,
		PayloadType:    96,
		SequenceNumber: 100,
		Timestamp:      90000,
		SSRC:           12345,
		Payload:        []byte("retransmitted packet"),
	}

	buffer.Store(packet)

	err = buffer.GetAndSend(
		12345,
		100,
		sender,
	)

	if err != nil {
		t.Fatal(err)
	}

	received, _, err := receiver.Receive()
	if err != nil {
		t.Fatal(err)
	}

	if received.SSRC != 12345 {
		t.Fatalf("expected SSRC 12345, got %d", received.SSRC)
	}

	if received.SequenceNumber != 100 {
		t.Fatalf(
			"expected sequence 100, got %d",
			received.SequenceNumber,
		)
	}

	if string(received.Payload) != "retransmitted packet" {
		t.Fatalf("payload mismatch")
	}
}