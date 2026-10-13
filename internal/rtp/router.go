package rtp

import (
	"errors"
	"fmt"
	"sort"
	"sync"
)

type RTPPacketForwarder interface {
	Forward(packet *Packet) error
}

type RTPRouter struct {
	mu     sync.RWMutex
	routes map[uint32]map[string]PacketSender
}

func NewRTPRouter() *RTPRouter {
	return &RTPRouter{
		routes: make(map[uint32]map[string]PacketSender),
	}
}

func (r *RTPRouter) Subscribe(
	ssrc uint32,
	subscriberID string,
	sender PacketSender,
) error {
	if r == nil {
		return fmt.Errorf("RTP router is nil")
	}
	if subscriberID == "" {
		return fmt.Errorf("subscriber ID is empty")
	}
	if sender == nil {
		return fmt.Errorf("packet sender is nil")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if r.routes == nil {
		r.routes = make(map[uint32]map[string]PacketSender)
	}

	subscribers := r.routes[ssrc]
	if subscribers == nil {
		subscribers = make(map[string]PacketSender)
		r.routes[ssrc] = subscribers
	}
	if _, exists := subscribers[subscriberID]; exists {
		return fmt.Errorf("subscriber %q is already subscribed to SSRC %d", subscriberID, ssrc)
	}

	subscribers[subscriberID] = sender
	return nil
}

func (r *RTPRouter) Unsubscribe(ssrc uint32, subscriberID string) {
	if r == nil {
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	subscribers := r.routes[ssrc]
	delete(subscribers, subscriberID)
	if len(subscribers) == 0 {
		delete(r.routes, ssrc)
	}
}

func (r *RTPRouter) Forward(packet *Packet) error {
	if r == nil {
		return fmt.Errorf("RTP router is nil")
	}
	if packet == nil {
		return fmt.Errorf("RTP packet is nil")
	}

	r.mu.RLock()
	subscribers := r.routes[packet.SSRC]
	ids := make([]string, 0, len(subscribers))
	for id := range subscribers {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	destinations := make([]struct {
		id     string
		sender PacketSender
	}, 0, len(ids))
	for _, id := range ids {
		destinations = append(destinations, struct {
			id     string
			sender PacketSender
		}{id: id, sender: subscribers[id]})
	}
	r.mu.RUnlock()

	var forwardErrors []error
	for _, destination := range destinations {
		if err := destination.sender.Send(cloneRTPPacket(packet)); err != nil {
			forwardErrors = append(forwardErrors, fmt.Errorf(
				"forward RTP packet SSRC=%d to subscriber %q: %w",
				packet.SSRC,
				destination.id,
				err,
			))
		}
	}

	return errors.Join(forwardErrors...)
}

func (r *RTPRouter) SubscriberCount(ssrc uint32) int {
	if r == nil {
		return 0
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	return len(r.routes[ssrc])
}

func cloneRTPPacket(packet *Packet) *Packet {
	clone := *packet
	clone.Payload = append([]byte(nil), packet.Payload...)
	return &clone
}
