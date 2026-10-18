package peer

import "testing"

func TestPeerSessionLifecycle(t *testing.T) {
	session := mustPeerSession(t, "session-a", "participant-a")
	if err := session.Activate(); err == nil {
		t.Fatal("new peer session activated without starting")
	}
	if err := session.Start(); err != nil {
		t.Fatal(err)
	}
	if err := session.Start(); err != nil {
		t.Fatalf("repeated start should be idempotent: %v", err)
	}
	if err := session.Activate(); err != nil {
		t.Fatal(err)
	}
	if err := session.Fail(); err != nil {
		t.Fatal(err)
	}
	if err := session.Activate(); err == nil {
		t.Fatal("failed peer session transitioned back to active")
	}
	if err := session.BeginClose(); err != nil {
		t.Fatal(err)
	}
	if err := session.Close(); err != nil {
		t.Fatal(err)
	}
	if err := session.Close(); err != nil {
		t.Fatalf("repeated close should be idempotent: %v", err)
	}
	if session.State() != PeerSessionStateClosed {
		t.Fatalf("state = %s, want CLOSED", session.State())
	}
}

func TestTransportLifecycle(t *testing.T) {
	transport := mustTransport(t, "transport-a")
	if err := transport.Connect(); err == nil {
		t.Fatal("new transport connected without starting")
	}
	if err := transport.Start(); err != nil {
		t.Fatal(err)
	}
	if err := transport.Connect(); err != nil {
		t.Fatal(err)
	}
	if err := transport.Fail(); err != nil {
		t.Fatal(err)
	}
	if err := transport.Connect(); err == nil {
		t.Fatal("failed transport transitioned back to connected")
	}
	if err := transport.BeginClose(); err != nil {
		t.Fatal(err)
	}
	if err := transport.Close(); err != nil {
		t.Fatal(err)
	}
	if transport.State() != TransportStateClosed {
		t.Fatalf("state = %s, want CLOSED", transport.State())
	}
}

func TestConstructorsRejectEmptyIdentifiers(t *testing.T) {
	if _, err := NewPeerSession("", "participant"); err == nil {
		t.Fatal("NewPeerSession accepted an empty session ID")
	}
	if _, err := NewPeerSession("session", ""); err == nil {
		t.Fatal("NewPeerSession accepted an empty participant ID")
	}
	if _, err := NewTransport(""); err == nil {
		t.Fatal("NewTransport accepted an empty ID")
	}
}

func mustPeerSession(t *testing.T, id, participantID string) *PeerSession {
	t.Helper()
	session, err := NewPeerSession(id, participantID)
	if err != nil {
		t.Fatal(err)
	}
	return session
}

func mustTransport(t *testing.T, id string) *Transport {
	t.Helper()
	transport, err := NewTransport(id)
	if err != nil {
		t.Fatal(err)
	}
	return transport
}
