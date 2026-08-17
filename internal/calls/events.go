package calls

import (
	"log"
	"net/http"
	"sync"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

type EventHub struct {
	// Map sessionID -> map of connections
	sessions map[uuid.UUID]map[*websocket.Conn]bool
	mu       sync.RWMutex
}

func NewEventHub() *EventHub {
	return &EventHub{
		sessions: make(map[uuid.UUID]map[*websocket.Conn]bool),
	}
}

func (h *EventHub) HandleWebSocket(w http.ResponseWriter, r *http.Request, sessionID uuid.UUID) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WS: Upgrade failed: %v", err)
		return
	}
	defer conn.Close()

	h.register(sessionID, conn)
	defer h.unregister(sessionID, conn)

	// Keep alive
	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			break
		}
	}
}

func (h *EventHub) register(sessionID uuid.UUID, conn *websocket.Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, ok := h.sessions[sessionID]; !ok {
		h.sessions[sessionID] = make(map[*websocket.Conn]bool)
	}
	h.sessions[sessionID][conn] = true
}

func (h *EventHub) unregister(sessionID uuid.UUID, conn *websocket.Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if conns, ok := h.sessions[sessionID]; ok {
		delete(conns, conn)
		if len(conns) == 0 {
			delete(h.sessions, sessionID)
		}
	}
}

func (h *EventHub) Broadcast(sessionID uuid.UUID, event interface{}) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	conns, ok := h.sessions[sessionID]
	if !ok {
		return
	}

	for conn := range conns {
		_ = conn.WriteJSON(event)
	}
}
