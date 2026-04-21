package notify

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
)

func TestHub_SubscribeNotifyReceive(t *testing.T) {
	hub := NewHub()
	userID := uuid.New()

	ch := hub.Subscribe(userID)
	defer hub.Unsubscribe(userID, ch)

	evt := Event{Type: "request_completed", RequestID: uuid.New(), Status: "done"}
	hub.Notify(userID, evt)

	select {
	case data := <-ch:
		var received Event
		if err := json.Unmarshal(data, &received); err != nil {
			t.Fatalf("Failed to unmarshal event: %v", err)
		}
		if received.Type != evt.Type || received.Status != evt.Status {
			t.Fatalf("Event mismatch: got %+v, want %+v", received, evt)
		}
	default:
		t.Fatal("expected to receive event on channel")
	}
}

func TestHub_UnknownUser_Noop(t *testing.T) {
	hub := NewHub()
	userID := uuid.New()

	// Notify a user with no subscribers — should not panic
	evt := Event{Type: "test", RequestID: uuid.New(), Status: "done"}
	hub.Notify(userID, evt)
	// Success: no panic
}

func TestHub_SlowClient_DropEvent(t *testing.T) {
	hub := NewHub()
	userID := uuid.New()

	ch := hub.Subscribe(userID)
	defer hub.Unsubscribe(userID, ch)

	evt := Event{Type: "test", RequestID: uuid.New(), Status: "done"}

	// Fill the channel buffer (capacity 16)
	for i := 0; i < 16; i++ {
		hub.Notify(userID, evt)
	}

	// Next notify should not block (event is dropped silently)
	hub.Notify(userID, evt)
	// Success: no deadlock
}

func TestHub_Unsubscribe_ClosesChannel(t *testing.T) {
	hub := NewHub()
	userID := uuid.New()

	ch := hub.Subscribe(userID)
	hub.Unsubscribe(userID, ch)

	// Channel should be closed
	select {
	case _, ok := <-ch:
		if ok {
			t.Fatal("expected channel to be closed")
		}
	default:
		t.Fatal("expected channel to be closed or return false on receive")
	}
}

func TestHub_MultipleSubscribers(t *testing.T) {
	hub := NewHub()
	userID := uuid.New()

	ch1 := hub.Subscribe(userID)
	ch2 := hub.Subscribe(userID)
	defer hub.Unsubscribe(userID, ch1)
	defer hub.Unsubscribe(userID, ch2)

	evt := Event{Type: "test", RequestID: uuid.New(), Status: "done"}
	hub.Notify(userID, evt)

	// Both channels should receive the event
	select {
	case <-ch1:
	default:
		t.Fatal("ch1 should receive event")
	}
	select {
	case <-ch2:
	default:
		t.Fatal("ch2 should receive event")
	}
}
