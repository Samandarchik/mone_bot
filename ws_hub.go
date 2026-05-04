package main

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	wsWriteWait  = 10 * time.Second
	wsPongWait   = 60 * time.Second
	wsPingPeriod = 30 * time.Second
)

// WSEvent — WebSocket orqali yuboriladigan event
type WSEvent struct {
	Type string      `json:"type"` // "new_rezume", "status_update", "delete"
	Data interface{} `json:"data"`
}

// Client — bitta WebSocket ulanish
type wsClient struct {
	conn                    *websocket.Conn
	send                    chan []byte
	allowedCategories       []string // rezume kategoriyalari (admin uchun)
	allowedIshchiCategories []string // ishchi kategoriyalari (ishchi_admin uchun)
	role                    string
	isSuperAdmin            bool
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
	user, err := dbGetUserByToken(token)
	if err != nil || !user.IsActive {
		http.Error(w, "noto'g'ri token", http.StatusUnauthorized)
		return
	}

	// Super admin — hamma narsani ko'radi. admin → rezume kategoriyalari, ishchi_admin → ishchi kategoriyalari.
	isSuperAdmin := user.Role == "super_admin"
	var allowedCategories []string
	var allowedIshchiCategories []string
	if user.Role == "admin" {
		cats := getUserCategories(user.ID)
		for _, c := range cats {
			allowedCategories = append(allowedCategories, c.Name)
		}
	} else if user.Role == "ishchi_admin" {
		cats := getUserIshchiCategories(user.ID)
		for _, c := range cats {
			allowedIshchiCategories = append(allowedIshchiCategories, c.Name)
		}
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WS upgrade xato: %v", err)
		return
	}

	client := &wsClient{
		conn:                    conn,
		send:                    make(chan []byte, 256),
		allowedCategories:       allowedCategories,
		allowedIshchiCategories: allowedIshchiCategories,
		role:                    user.Role,
		isSuperAdmin:            isSuperAdmin,
	}
	hub.register(client)

	// Ulanganida mavjud ma'lumotlarni yuborish (foydalanuvchi kategoriyalari bilan filtrlangan)
	go func() {
		// Rezume init: faqat super_admin yoki admin uchun
		if user.Role != "ishchi_admin" {
			rezumeler, _, err := getRezumeler("", "", "", allowedCategories, 1, 100)
			if err == nil {
				data, _ := json.Marshal(WSEvent{Type: "init", Data: rezumeler})
				client.send <- data
			}
		}
		// Ishchi init: faqat super_admin yoki ishchi_admin uchun
		if user.Role != "admin" {
			ishchilar, _, err := getIshchiAnketalar("", "", "", allowedIshchiCategories, 1, 100)
			if err == nil {
				attachIshchiInterviews(ishchilar)
				data, _ := json.Marshal(WSEvent{Type: "ishchi_init", Data: ishchilar})
				client.send <- data
			}
		}
	}()

	// Yozish goroutine (xabar + ping)
	go func() {
		ticker := time.NewTicker(wsPingPeriod)
		defer func() {
			ticker.Stop()
			conn.Close()
		}()
		for {
			select {
			case msg, ok := <-client.send:
				if !ok {
					_ = conn.WriteMessage(websocket.CloseMessage, []byte{})
					return
				}
				conn.SetWriteDeadline(time.Now().Add(wsWriteWait))
				if err := conn.WriteMessage(websocket.TextMessage, msg); err != nil {
					return
				}
			case <-ticker.C:
				conn.SetWriteDeadline(time.Now().Add(wsWriteWait))
				if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
					return
				}
			}
		}
	}()

	// O'qish goroutine (ping/pong uchun)
	go func() {
		defer hub.unregister(client)
		defer conn.Close()
		conn.SetReadDeadline(time.Now().Add(wsPongWait))
		conn.SetPongHandler(func(string) error {
			conn.SetReadDeadline(time.Now().Add(wsPongWait))
			return nil
		})
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				break
			}
		}
	}()
}

// sendToClients — event-ni filtr funksiyasi orqali clientlarga yuboradi.
// shouldSend(c) true qaytarsa, client xabar oladi.
func sendToClients(event WSEvent, shouldSend func(c *wsClient) bool) {
	data, err := json.Marshal(event)
	if err != nil {
		log.Printf("WS marshal xato: %v", err)
		return
	}
	hub.mu.RLock()
	defer hub.mu.RUnlock()
	for c := range hub.clients {
		if !shouldSend(c) {
			continue
		}
		select {
		case c.send <- data:
		default:
			go hub.unregister(c)
		}
	}
}

// Rezume broadcastlar — ishchi_admin ga yuborilmaydi
func broadcastNewRezume(rezume *RezumeRow) {
	sendToClients(WSEvent{Type: "new_rezume", Data: rezume}, func(c *wsClient) bool {
		if c.role == "ishchi_admin" {
			return false
		}
		if c.isSuperAdmin {
			return true
		}
		for _, cat := range c.allowedCategories {
			if cat == rezume.Lavozim {
				return true
			}
		}
		return false
	})
}

func broadcastRezumeStatusUpdate(id int64, status, statusByName string) {
	sendToClients(WSEvent{Type: "status_update", Data: map[string]interface{}{
		"id": id, "status": status, "status_by_name": statusByName,
	}}, func(c *wsClient) bool { return c.role != "ishchi_admin" })
}

func broadcastRezumeDelete(id int64) {
	sendToClients(WSEvent{Type: "delete", Data: map[string]interface{}{"id": id}},
		func(c *wsClient) bool { return c.role != "ishchi_admin" })
}

// Interview (rezume) broadcast — ishchi_admin uchun emas
func broadcastInterviewCreated(interview *InterviewRow) {
	sendToClients(WSEvent{Type: "interview_created", Data: interview},
		func(c *wsClient) bool { return c.role != "ishchi_admin" })
}

func broadcastInterviewUpdated(interview *InterviewRow) {
	sendToClients(WSEvent{Type: "interview_updated", Data: interview},
		func(c *wsClient) bool { return c.role != "ishchi_admin" })
}

func broadcastInterviewDeleted(id, rezumeID int64) {
	sendToClients(WSEvent{Type: "interview_deleted", Data: map[string]interface{}{"id": id, "rezume_id": rezumeID}},
		func(c *wsClient) bool { return c.role != "ishchi_admin" })
}

// Ishchi broadcastlar — admin (rezume) ga yuborilmaydi
func broadcastNewIshchi(ishchi *IshchiRow) {
	sendToClients(WSEvent{Type: "new_ishchi", Data: ishchi}, func(c *wsClient) bool {
		if c.role == "admin" {
			return false
		}
		if c.isSuperAdmin {
			return true
		}
		// ishchi_admin: kategoriya filtri
		for _, cat := range c.allowedIshchiCategories {
			if cat == ishchi.Vakansiya {
				return true
			}
		}
		return false
	})
}

func broadcastIshchiUpdate(ishchi *IshchiRow) {
	sendToClients(WSEvent{Type: "ishchi_update", Data: ishchi}, func(c *wsClient) bool {
		if c.role == "admin" {
			return false
		}
		if c.isSuperAdmin {
			return true
		}
		for _, cat := range c.allowedIshchiCategories {
			if cat == ishchi.Vakansiya {
				return true
			}
		}
		return false
	})
}

func broadcastIshchiDelete(id int64) {
	sendToClients(WSEvent{Type: "ishchi_delete", Data: map[string]interface{}{"id": id}},
		func(c *wsClient) bool { return c.role != "admin" })
}

func broadcastIshchiStatusUpdate(id int64, status, statusByName string) {
	sendToClients(WSEvent{Type: "ishchi_status_update", Data: map[string]interface{}{
		"id": id, "status": status, "status_by_name": statusByName,
	}}, func(c *wsClient) bool { return c.role != "admin" })
}

func broadcastIshchiInterviewCreated(interview *IshchiInterviewRow) {
	sendToClients(WSEvent{Type: "ishchi_interview_created", Data: interview},
		func(c *wsClient) bool { return c.role != "admin" })
}

func broadcastIshchiInterviewUpdated(interview *IshchiInterviewRow) {
	sendToClients(WSEvent{Type: "ishchi_interview_updated", Data: interview},
		func(c *wsClient) bool { return c.role != "admin" })
}

func broadcastIshchiInterviewDeleted(id, ishchiID int64) {
	sendToClients(WSEvent{Type: "ishchi_interview_deleted", Data: map[string]interface{}{"id": id, "ishchi_id": ishchiID}},
		func(c *wsClient) bool { return c.role != "admin" })
}
