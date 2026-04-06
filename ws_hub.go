package main

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

// WSEvent — WebSocket orqali yuboriladigan event
type WSEvent struct {
	Type string      `json:"type"` // "new_rezume", "status_update", "delete"
	Data interface{} `json:"data"`
}

// Client — bitta WebSocket ulanish
type wsClient struct {
	conn *websocket.Conn
	send chan []byte
}

// Hub — barcha WebSocket clientlarni boshqaradi
type wsHub struct {
	mu      sync.RWMutex
	clients map[*wsClient]bool
}

var hub = &wsHub{
	clients: make(map[*wsClient]bool),
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

// Yangi client qo'shish
func (h *wsHub) register(c *wsClient) {
	h.mu.Lock()
	h.clients[c] = true
	h.mu.Unlock()
	log.Printf("WS client ulandi. Jami: %d", len(h.clients))
}

// Client o'chirish
func (h *wsHub) unregister(c *wsClient) {
	h.mu.Lock()
	if _, ok := h.clients[c]; ok {
		delete(h.clients, c)
		close(c.send)
	}
	h.mu.Unlock()
	log.Printf("WS client uzildi. Jami: %d", len(h.clients))
}

// Barcha clientlarga event yuborish
func (h *wsHub) broadcast(event WSEvent) {
	data, err := json.Marshal(event)
	if err != nil {
		log.Printf("WS broadcast marshal xato: %v", err)
		return
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	for c := range h.clients {
		select {
		case c.send <- data:
		default:
			// Client sekin, o'chiramiz
			go h.unregister(c)
		}
	}
}

// WebSocket handler
func handleWebSocket(w http.ResponseWriter, r *http.Request) {
	// Token tekshirish
	token := r.URL.Query().Get("token")
	if token == "" {
		http.Error(w, "token kerak", http.StatusUnauthorized)
		return
	}
	_, err := dbGetUserByToken(token)
	if err != nil {
		http.Error(w, "noto'g'ri token", http.StatusUnauthorized)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WS upgrade xato: %v", err)
		return
	}

	client := &wsClient{
		conn: conn,
		send: make(chan []byte, 256),
	}
	hub.register(client)

	// Ulanganida mavjud ma'lumotlarni yuborish
	go func() {
		rezumeler, _, err := getRezumeler("", "", "", nil, 1, 100)
		if err == nil {
			data, _ := json.Marshal(WSEvent{Type: "init", Data: rezumeler})
			client.send <- data
		}
		ishchilar, _, err := getIshchiAnketalar("", "", "", 1, 100)
		if err == nil {
			data, _ := json.Marshal(WSEvent{Type: "ishchi_init", Data: ishchilar})
			client.send <- data
		}
	}()

	// Yozish goroutine
	go func() {
		defer conn.Close()
		for msg := range client.send {
			if err := conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				break
			}
		}
	}()

	// O'qish goroutine (ping/pong uchun)
	go func() {
		defer hub.unregister(client)
		defer conn.Close()
		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				break
			}
		}
	}()
}

// Broadcast helper funksiyalari
func broadcastNewRezume(rezume interface{}) {
	hub.broadcast(WSEvent{Type: "new_rezume", Data: rezume})
}

func broadcastRezumeStatusUpdate(id int64, status, statusByName string) {
	hub.broadcast(WSEvent{Type: "status_update", Data: map[string]interface{}{
		"id":             id,
		"status":         status,
		"status_by_name": statusByName,
	}})
}

func broadcastRezumeDelete(id int64) {
	hub.broadcast(WSEvent{Type: "delete", Data: map[string]interface{}{"id": id}})
}

// Ishchi anketalar broadcast
func broadcastNewIshchi(ishchi interface{}) {
	hub.broadcast(WSEvent{Type: "new_ishchi", Data: ishchi})
}

func broadcastIshchiUpdate(ishchi interface{}) {
	hub.broadcast(WSEvent{Type: "ishchi_update", Data: ishchi})
}

func broadcastIshchiDelete(id int64) {
	hub.broadcast(WSEvent{Type: "ishchi_delete", Data: map[string]interface{}{"id": id}})
}
