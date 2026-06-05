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
	userID                  int64    // ulangan foydalanuvchi ID'si
	deviceID                string   // shu ulanishning qurilma kaliti (sessiyadan)
	allowedCategories       []string // rezume kategoriyalari (admin uchun)
	allowedIshchiCategories []string // ishchi kategoriyalari (ishchi_admin uchun)
	role                    string
	isSuperAdmin            bool
	// canRezume — rezume eventlarini oladi (super_admin yoki admin roli).
	// canIshchi — ishchi eventlarini oladi (super_admin yoki ishchi_admin roli).
	// Bitta user ikkala rolga ega bo'lsa, ikkalasi ham true bo'ladi.
	canRezume bool
	canIshchi bool
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

	// Super admin — hamma narsani ko'radi. admin → rezume kategoriyalari,
	// ishchi_admin → ishchi kategoriyalari. Bitta user ikkala rolga ega bo'lsa,
	// ikkala kategoriya to'plamini ham oladi.
	isSuperAdmin := user.hasRole("super_admin")
	canRezume := isSuperAdmin || user.hasRole("admin")
	canIshchi := isSuperAdmin || user.hasRole("ishchi_admin")
	var allowedCategories []string
	var allowedIshchiCategories []string
	if user.hasRole("admin") {
		cats := getUserCategories(user.ID)
		for _, c := range cats {
			allowedCategories = append(allowedCategories, c.Name)
		}
	}
	if user.hasRole("ishchi_admin") {
		cats := getUserIshchiCategories(user.ID)
		for _, c := range cats {
			allowedIshchiCategories = append(allowedIshchiCategories, c.Name)
		}
	}

	// Shu ulanishning qurilma kaliti — super_admin qurilmani o'chirganda aynan
	// shu ulanishga "force_logout" yuborish uchun ishlatiladi.
	deviceID, _ := dbGetSessionDeviceID(token)

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WS upgrade xato: %v", err)
		return
	}

	client := &wsClient{
		conn:                    conn,
		send:                    make(chan []byte, 256),
		userID:                  user.ID,
		deviceID:                deviceID,
		allowedCategories:       allowedCategories,
		allowedIshchiCategories: allowedIshchiCategories,
		role:                    user.Role,
		isSuperAdmin:            isSuperAdmin,
		canRezume:               canRezume,
		canIshchi:               canIshchi,
	}
	hub.register(client)

	// Ulanganida mavjud ma'lumotlarni yuborish (foydalanuvchi kategoriyalari bilan filtrlangan)
	go func() {
		// Rezume init: super_admin yoki admin roli bo'lsa
		if canRezume {
			rezumeler, _, err := getRezumeler("", "", "", "", allowedCategories, 1, 100)
			if err == nil {
				data, _ := json.Marshal(WSEvent{Type: "init", Data: rezumeler})
				client.send <- data
			}
		}
		// Ishchi init: super_admin yoki ishchi_admin roli bo'lsa
		if canIshchi {
			ishchilar, _, err := getIshchiAnketalar("", "", "", "", allowedIshchiCategories, 1, 100)
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

// broadcastForceLogout — super_admin bir qurilmani o'chirganda, aynan o'sha
// foydalanuvchining o'sha qurilmasidagi ochiq WS ulanish(lar)iga "force_logout"
// yuboradi. Mijoz buni olib darhol login ekraniga qaytadi (HTTP 401 ni kutmasdan).
func broadcastForceLogout(userID int64, deviceID string) {
	if deviceID == "" {
		return
	}
	sendToClients(WSEvent{Type: "force_logout", Data: map[string]interface{}{"device_id": deviceID}},
		func(c *wsClient) bool { return c.userID == userID && c.deviceID == deviceID })
}

// Rezume broadcastlar — ishchi_admin ga yuborilmaydi
func broadcastNewRezume(rezume *RezumeRow) {
	sendToClients(WSEvent{Type: "new_rezume", Data: rezume}, func(c *wsClient) bool {
		if !c.canRezume {
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

func broadcastRezumeStatusUpdate(id int64, status, statusByName, statusVoiceUrl string) {
	sendToClients(WSEvent{Type: "status_update", Data: map[string]interface{}{
		"id": id, "status": status, "status_by_name": statusByName, "status_voice_url": statusVoiceUrl,
	}}, func(c *wsClient) bool { return c.canRezume })
}

func broadcastRezumeDelete(id int64) {
	sendToClients(WSEvent{Type: "delete", Data: map[string]interface{}{"id": id}},
		func(c *wsClient) bool { return c.canRezume })
}

// Interview (rezume) broadcast — ishchi_admin uchun emas
func broadcastInterviewCreated(interview *InterviewRow) {
	sendToClients(WSEvent{Type: "interview_created", Data: interview},
		func(c *wsClient) bool { return c.canRezume })
}

func broadcastInterviewUpdated(interview *InterviewRow) {
	sendToClients(WSEvent{Type: "interview_updated", Data: interview},
		func(c *wsClient) bool { return c.canRezume })
}

func broadcastInterviewDeleted(id, rezumeID int64) {
	sendToClients(WSEvent{Type: "interview_deleted", Data: map[string]interface{}{"id": id, "rezume_id": rezumeID}},
		func(c *wsClient) bool { return c.canRezume })
}

// Ishchi broadcastlar — admin (rezume) ga yuborilmaydi
func broadcastNewIshchi(ishchi *IshchiRow) {
	sendToClients(WSEvent{Type: "new_ishchi", Data: ishchi}, func(c *wsClient) bool {
		if !c.canIshchi {
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
		if !c.canIshchi {
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
		func(c *wsClient) bool { return c.canIshchi })
}

func broadcastIshchiStatusUpdate(id int64, status, statusByName string) {
	sendToClients(WSEvent{Type: "ishchi_status_update", Data: map[string]interface{}{
		"id": id, "status": status, "status_by_name": statusByName,
	}}, func(c *wsClient) bool { return c.canIshchi })
}

func broadcastIshchiInterviewCreated(interview *IshchiInterviewRow) {
	sendToClients(WSEvent{Type: "ishchi_interview_created", Data: interview},
		func(c *wsClient) bool { return c.canIshchi })
}

func broadcastIshchiInterviewUpdated(interview *IshchiInterviewRow) {
	sendToClients(WSEvent{Type: "ishchi_interview_updated", Data: interview},
		func(c *wsClient) bool { return c.canIshchi })
}

func broadcastIshchiInterviewDeleted(id, ishchiID int64) {
	sendToClients(WSEvent{Type: "ishchi_interview_deleted", Data: map[string]interface{}{"id": id, "ishchi_id": ishchiID}},
		func(c *wsClient) bool { return c.canIshchi })
}
