package peer

import (
	"fmt"
	"sync"
)

type PeerSessionState string

const (
	PeerSessionStateNew        PeerSessionState = "NEW"
	PeerSessionStateConnecting PeerSessionState = "CONNECTING"
	PeerSessionStateActive     PeerSessionState = "ACTIVE"
	PeerSessionStateFailed     PeerSessionState = "FAILED"
	PeerSessionStateClosing    PeerSessionState = "CLOSING"
	PeerSessionStateClosed     PeerSessionState = "CLOSED"
)

type TransportState string

const (
	TransportStateNew        TransportState = "NEW"
	TransportStateConnecting TransportState = "CONNECTING"
	TransportStateConnected  TransportState = "CONNECTED"
	TransportStateFailed     TransportState = "FAILED"
	TransportStateClosing    TransportState = "CLOSING"
	TransportStateClosed     TransportState = "CLOSED"
)

type PeerSession struct {
	mu            sync.RWMutex
	id            string
	participantID string
	state         PeerSessionState
}

func NewPeerSession(id, participantID string) (*PeerSession, error) {
	if id == "" {
		return nil, fmt.Errorf("peer session ID is empty")
	}
	if participantID == "" {
		return nil, fmt.Errorf("peer session participant ID is empty")
	}
	return &PeerSession{
		id:            id,
		participantID: participantID,
		state:         PeerSessionStateNew,
	}, nil
}

func (s *PeerSession) ID() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.id
}

func (s *PeerSession) ParticipantID() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.participantID
}

func (s *PeerSession) State() PeerSessionState {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.state
}

func (s *PeerSession) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state == PeerSessionStateConnecting {
		return nil
	}
	if s.state != PeerSessionStateNew {
		return fmt.Errorf("cannot start peer session in state %s", s.state)
	}
	s.state = PeerSessionStateConnecting
	return nil
}

func (s *PeerSession) Activate() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state == PeerSessionStateActive {
		return nil
	}
	if s.state != PeerSessionStateConnecting {
		return fmt.Errorf("cannot activate peer session in state %s", s.state)
	}
	s.state = PeerSessionStateActive
	return nil
}

func (s *PeerSession) Fail() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state == PeerSessionStateFailed {
		return nil
	}
	if s.state != PeerSessionStateConnecting && s.state != PeerSessionStateActive {
		return fmt.Errorf("cannot fail peer session in state %s", s.state)
	}
	s.state = PeerSessionStateFailed
	return nil
}

func (s *PeerSession) BeginClose() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state == PeerSessionStateClosing {
		return nil
	}
	if s.state == PeerSessionStateClosed {
		return fmt.Errorf("cannot begin closing peer session in state %s", s.state)
	}
	s.state = PeerSessionStateClosing
	return nil
}

func (s *PeerSession) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state == PeerSessionStateClosed {
		return nil
	}
	if s.state != PeerSessionStateClosing {
		return fmt.Errorf("cannot close peer session in state %s", s.state)
	}
	s.state = PeerSessionStateClosed
	return nil
}

type Transport struct {
	mu    sync.RWMutex
	id    string
	state TransportState
}

func NewTransport(id string) (*Transport, error) {
	if id == "" {
		return nil, fmt.Errorf("transport ID is empty")
	}
	return &Transport{id: id, state: TransportStateNew}, nil
}

func (t *Transport) ID() string {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.id
}

func (t *Transport) State() TransportState {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.state
}

func (t *Transport) Start() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.state == TransportStateConnecting {
		return nil
	}
	if t.state != TransportStateNew {
		return fmt.Errorf("cannot connect transport in state %s", t.state)
	}
	t.state = TransportStateConnecting
	return nil
}

func (t *Transport) Connect() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.state == TransportStateConnected {
		return nil
	}
	if t.state != TransportStateConnecting {
		return fmt.Errorf("cannot connect transport in state %s", t.state)
	}
	t.state = TransportStateConnected
	return nil
}

func (t *Transport) Fail() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.state == TransportStateFailed {
		return nil
	}
	if t.state != TransportStateConnecting && t.state != TransportStateConnected {
		return fmt.Errorf("cannot fail transport in state %s", t.state)
	}
	t.state = TransportStateFailed
	return nil
}

func (t *Transport) BeginClose() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.state == TransportStateClosing {
		return nil
	}
	if t.state == TransportStateClosed {
		return fmt.Errorf("cannot begin closing transport in state %s", t.state)
	}
	t.state = TransportStateClosing
	return nil
}

func (t *Transport) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.state == TransportStateClosed {
		return nil
	}
	if t.state != TransportStateClosing {
		return fmt.Errorf("cannot close transport in state %s", t.state)
	}
	t.state = TransportStateClosed
	return nil
}
