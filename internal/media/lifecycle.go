package media

import (
	"fmt"
	"sync"
)

type State string

const (
	StateNew     State = "NEW"
	StateActive  State = "ACTIVE"
	StateMuted   State = "MUTED"
	StatePaused  State = "PAUSED"
	StateClosing State = "CLOSING"
	StateClosed  State = "CLOSED"
)

type Kind string

const (
	KindAudio Kind = "audio"
	KindVideo Kind = "video"
)

type Producer struct {
	mu            sync.RWMutex
	id            string
	participantID string
	kind          Kind
	state         State
}

func NewProducer(id, participantID string, kind Kind) (*Producer, error) {
	if id == "" {
		return nil, fmt.Errorf("producer ID is empty")
	}
	if participantID == "" {
		return nil, fmt.Errorf("producer participant ID is empty")
	}
	if kind != KindAudio && kind != KindVideo {
		return nil, fmt.Errorf("unsupported producer kind %q", kind)
	}
	return &Producer{
		id:            id,
		participantID: participantID,
		kind:          kind,
		state:         StateNew,
	}, nil
}

func (p *Producer) ID() string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.id
}

func (p *Producer) ParticipantID() string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.participantID
}

func (p *Producer) Kind() Kind {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.kind
}

func (p *Producer) State() State {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.state
}

func (p *Producer) Activate() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.state == StateActive {
		return nil
	}
	if p.state != StateNew {
		return fmt.Errorf("cannot activate producer in state %s", p.state)
	}
	p.state = StateActive
	return nil
}

func (p *Producer) SetMuted(muted bool) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if muted && p.state == StateMuted {
		return nil
	}
	if !muted && p.state == StateActive {
		return nil
	}
	if muted && p.state == StateActive {
		p.state = StateMuted
		return nil
	}
	if !muted && p.state == StateMuted {
		p.state = StateActive
		return nil
	}
	return fmt.Errorf("cannot change producer mute state to %t in state %s", muted, p.state)
}

func (p *Producer) Pause() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.state == StatePaused {
		return nil
	}
	if p.state != StateActive {
		return fmt.Errorf("cannot pause producer in state %s", p.state)
	}
	p.state = StatePaused
	return nil
}

func (p *Producer) Resume() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.state == StateActive {
		return nil
	}
	if p.state != StatePaused {
		return fmt.Errorf("cannot resume producer in state %s", p.state)
	}
	p.state = StateActive
	return nil
}

func (p *Producer) BeginClose() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.state == StateClosing {
		return nil
	}
	if p.state == StateClosed {
		return fmt.Errorf("cannot begin closing producer in state %s", p.state)
	}
	p.state = StateClosing
	return nil
}

func (p *Producer) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.state == StateClosed {
		return nil
	}
	if p.state != StateClosing {
		return fmt.Errorf("cannot close producer in state %s", p.state)
	}
	p.state = StateClosed
	return nil
}

type Consumer struct {
	mu         sync.RWMutex
	id         string
	producerID string
	state      State
}

func NewConsumer(id, producerID string) (*Consumer, error) {
	if id == "" {
		return nil, fmt.Errorf("consumer ID is empty")
	}
	if producerID == "" {
		return nil, fmt.Errorf("consumer producer ID is empty")
	}
	return &Consumer{id: id, producerID: producerID, state: StateNew}, nil
}

func (c *Consumer) ID() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.id
}

func (c *Consumer) ProducerID() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.producerID
}

func (c *Consumer) State() State {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.state
}

func (c *Consumer) Activate() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.state == StateActive {
		return nil
	}
	if c.state != StateNew {
		return fmt.Errorf("cannot activate consumer in state %s", c.state)
	}
	c.state = StateActive
	return nil
}

func (c *Consumer) Pause() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.state == StatePaused {
		return nil
	}
	if c.state != StateActive {
		return fmt.Errorf("cannot pause consumer in state %s", c.state)
	}
	c.state = StatePaused
	return nil
}

func (c *Consumer) Resume() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.state == StateActive {
		return nil
	}
	if c.state != StatePaused {
		return fmt.Errorf("cannot resume consumer in state %s", c.state)
	}
	c.state = StateActive
	return nil
}

func (c *Consumer) BeginClose() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.state == StateClosing {
		return nil
	}
	if c.state == StateClosed {
		return fmt.Errorf("cannot begin closing consumer in state %s", c.state)
	}
	c.state = StateClosing
	return nil
}

func (c *Consumer) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.state == StateClosed {
		return nil
	}
	if c.state != StateClosing {
		return fmt.Errorf("cannot close consumer in state %s", c.state)
	}
	c.state = StateClosed
	return nil
}
