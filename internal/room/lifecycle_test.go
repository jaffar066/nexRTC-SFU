package room

import "testing"

func TestRoomLifecycleAndParticipantCleanup(t *testing.T) {
	r, err := NewRoom("room-a")
	if err != nil {
		t.Fatal(err)
	}
	if err := r.Activate(); err != nil {
		t.Fatal(err)
	}
	if err := r.Activate(); err != nil {
		t.Fatalf("repeated activation should be idempotent: %v", err)
	}

	p, err := NewParticipant("participant-a")
	if err != nil {
		t.Fatal(err)
	}
	if err := r.AddParticipant(p); err != nil {
		t.Fatal(err)
	}
	if p.State() != ParticipantStateActive || p.RoomID() != "room-a" {
		t.Fatalf("participant after joining: state=%s room=%q", p.State(), p.RoomID())
	}
	if r.ParticipantCount() != 1 {
		t.Fatalf("participant count = %d, want 1", r.ParticipantCount())
	}

	if err := r.BeginClose(); err != nil {
		t.Fatal(err)
	}
	if err := r.AddParticipant(mustParticipant(t, "late")); err == nil {
		t.Fatal("closing room accepted a new participant")
	}
	if err := r.Close(); err != nil {
		t.Fatal(err)
	}
	if r.State() != RoomStateClosed || r.ParticipantCount() != 0 {
		t.Fatalf("room after close: state=%s participants=%d", r.State(), r.ParticipantCount())
	}
	if p.State() != ParticipantStateLeft || p.RoomID() != "" {
		t.Fatalf("participant was not cleaned up: state=%s room=%q", p.State(), p.RoomID())
	}
	if err := r.Close(); err != nil {
		t.Fatalf("repeated close should be idempotent: %v", err)
	}
}

func TestRoomMembershipAndStateValidation(t *testing.T) {
	r := mustRoom(t, "room")
	p := mustParticipant(t, "p")
	if err := r.AddParticipant(p); err == nil {
		t.Fatal("new room accepted participant before activation")
	}
	if err := r.Activate(); err != nil {
		t.Fatal(err)
	}
	if err := r.AddParticipant(p); err != nil {
		t.Fatal(err)
	}
	if err := r.AddParticipant(p); err == nil {
		t.Fatal("room accepted duplicate participant")
	}
	if err := p.Disconnect(); err != nil {
		t.Fatal(err)
	}
	if err := p.Disconnect(); err != nil {
		t.Fatalf("repeated disconnect should be idempotent: %v", err)
	}
	if err := r.RemoveParticipant("p"); err != nil {
		t.Fatal(err)
	}
	if p.State() != ParticipantStateLeft || r.ParticipantCount() != 0 {
		t.Fatalf("participant removal failed: state=%s count=%d", p.State(), r.ParticipantCount())
	}
	if err := p.Disconnect(); err == nil {
		t.Fatal("left participant accepted disconnect")
	}
}

func TestRoomRejectsInvalidTransitions(t *testing.T) {
	r := mustRoom(t, "room")
	if err := r.BeginClose(); err != nil {
		t.Fatal(err)
	}
	if err := r.Activate(); err == nil {
		t.Fatal("closing room transitioned back to active")
	}
	if err := r.BeginClose(); err != nil {
		t.Fatalf("repeated BeginClose should be idempotent: %v", err)
	}
	if err := r.Close(); err != nil {
		t.Fatal(err)
	}
	if err := r.BeginClose(); err == nil {
		t.Fatal("closed room began closing again")
	}
}

func TestConstructorsRejectEmptyIDs(t *testing.T) {
	if _, err := NewRoom(""); err == nil {
		t.Fatal("NewRoom accepted an empty ID")
	}
	if _, err := NewParticipant(""); err == nil {
		t.Fatal("NewParticipant accepted an empty ID")
	}
}

func mustRoom(t *testing.T, id string) *Room {
	t.Helper()
	r, err := NewRoom(id)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func mustParticipant(t *testing.T, id string) *Participant {
	t.Helper()
	p, err := NewParticipant(id)
	if err != nil {
		t.Fatal(err)
	}
	return p
}
