package notify

import (
	"encoding/json"
	"sync"

	"github.com/google/uuid"
)

// Event is a notification sent to connected clients.
type Event struct {
	Type      string    `json:"type"`       // "request_completed", "request_failed"
	RequestID uuid.UUID `json:"request_id"`
	Status    string    `json:"status"`
}

// Hub manages per-user SSE client channels.
type Hub struct {
	mu      sync.RWMutex
	clients map[uuid.UUID]map[chan []byte]struct{} // userID → set of channels
}

// NewHub creates a new notification hub.
func NewHub() *Hub {
	return &Hub{
		clients: make(map[uuid.UUID]map[chan []byte]struct{}),
	}
}

// Subscribe creates a channel for the given user. Caller must call Unsubscribe when done.
func (h *Hub) Subscribe(userID uuid.UUID) chan []byte {
	ch := make(chan []byte, 16)
	h.mu.Lock()
	if h.clients[userID] == nil {
		h.clients[userID] = make(map[chan []byte]struct{})
	}
	h.clients[userID][ch] = struct{}{}
	h.mu.Unlock()
	return ch
}

// Unsubscribe removes a client channel.
func (h *Hub) Unsubscribe(userID uuid.UUID, ch chan []byte) {
	h.mu.Lock()
	if clients, ok := h.clients[userID]; ok {
		delete(clients, ch)
		if len(clients) == 0 {
			delete(h.clients, userID)
		}
	}
	h.mu.Unlock()
	close(ch)
}

// Notify sends an event to all connected clients for the given user.
func (h *Hub) Notify(userID uuid.UUID, evt Event) {
	data, err := json.Marshal(evt)
	if err != nil {
		return
	}

	h.mu.RLock()
	clients := h.clients[userID]
	h.mu.RUnlock()

	for ch := range clients {
		select {
		case ch <- data:
		default:
			// Client too slow, skip
		}
	}
}
