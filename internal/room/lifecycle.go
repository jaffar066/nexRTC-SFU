package room

import (
	"fmt"
	"sync"
)

type RoomState string

const (
	RoomStateNew     RoomState = "NEW"
	RoomStateActive  RoomState = "ACTIVE"
	RoomStateClosing RoomState = "CLOSING"
	RoomStateClosed  RoomState = "CLOSED"
)

type ParticipantState string

const (
	ParticipantStateJoining      ParticipantState = "JOINING"
	ParticipantStateActive       ParticipantState = "ACTIVE"
	ParticipantStateDisconnected ParticipantState = "DISCONNECTED"
	ParticipantStateLeft         ParticipantState = "LEFT"
)

type Room struct {
	mu           sync.RWMutex
	id           string
	state        RoomState
	participants map[string]*Participant
}

func NewRoom(id string) (*Room, error) {
	if id == "" {
		return nil, fmt.Errorf("room ID is empty")
	}
	return &Room{
		id:           id,
		state:        RoomStateNew,
		participants: make(map[string]*Participant),
	}, nil
}

func (r *Room) ID() string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.id
}

func (r *Room) State() RoomState {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.state
}

func (r *Room) Activate() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.state == RoomStateActive {
		return nil
	}
	if r.state != RoomStateNew {
		return fmt.Errorf("cannot activate room in state %s", r.state)
	}
	r.state = RoomStateActive
	return nil
}

func (r *Room) BeginClose() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.state == RoomStateClosing {
		return nil
	}
	if r.state != RoomStateNew && r.state != RoomStateActive {
		return fmt.Errorf("cannot begin closing room in state %s", r.state)
	}
	r.state = RoomStateClosing
	return nil
}

func (r *Room) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.state == RoomStateClosed {
		return nil
	}
	if r.state != RoomStateNew && r.state != RoomStateActive && r.state != RoomStateClosing {
		return fmt.Errorf("cannot close room in state %s", r.state)
	}
	r.state = RoomStateClosing
	for id, participant := range r.participants {
		participant.leave()
		delete(r.participants, id)
	}
	r.state = RoomStateClosed
	return nil
}

func (r *Room) AddParticipant(participant *Participant) error {
	if participant == nil {
		return fmt.Errorf("participant is nil")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.state != RoomStateActive {
		return fmt.Errorf("cannot add participant to room in state %s", r.state)
	}
	if _, exists := r.participants[participant.ID()]; exists {
		return fmt.Errorf("participant %q is already in room %q", participant.ID(), r.id)
	}
	if err := participant.join(r.id); err != nil {
		return err
	}
	r.participants[participant.ID()] = participant
	return nil
}

func (r *Room) RemoveParticipant(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	participant, exists := r.participants[id]
	if !exists {
		return fmt.Errorf("participant %q is not in room %q", id, r.id)
	}
	participant.leave()
	delete(r.participants, id)
	return nil
}

func (r *Room) Participant(id string) (*Participant, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	participant, exists := r.participants[id]
	return participant, exists
}

func (r *Room) ParticipantCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.participants)
}

type Participant struct {
	mu     sync.RWMutex
	id     string
	roomID string
	state  ParticipantState
}

func NewParticipant(id string) (*Participant, error) {
	if id == "" {
		return nil, fmt.Errorf("participant ID is empty")
	}
	return &Participant{id: id, state: ParticipantStateJoining}, nil
}

func (p *Participant) ID() string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.id
}

func (p *Participant) RoomID() string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.roomID
}

func (p *Participant) State() ParticipantState {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.state
}

func (p *Participant) Disconnect() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.state == ParticipantStateDisconnected {
		return nil
	}
	if p.state != ParticipantStateActive {
		return fmt.Errorf("cannot disconnect participant in state %s", p.state)
	}
	p.state = ParticipantStateDisconnected
	return nil
}

func (p *Participant) join(roomID string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.state != ParticipantStateJoining {
		return fmt.Errorf("cannot add participant in state %s", p.state)
	}
	p.state = ParticipantStateActive
	p.roomID = roomID
	return nil
}

func (p *Participant) leave() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.state = ParticipantStateLeft
	p.roomID = ""
}
