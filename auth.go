package main

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"
)

type contextKey string

const userCtxKey contextKey = "user"

func generateToken() string {
	b := make([]byte, 32)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func getUserFromCtx(r *http.Request) *UserRow {
	if u, ok := r.Context().Value(userCtxKey).(*UserRow); ok {
		return u
	}
	return nil
}

// authRequired — faqat avtorizatsiya qilingan foydalanuvchilar uchun
func authRequired(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := r.Header.Get("Authorization")
		token = strings.TrimPrefix(token, "Bearer ")
		if token == "" {
			jsonError(w, "Avtorizatsiya talab qilinadi", http.StatusUnauthorized)
			return
		}
		user, err := dbGetUserByToken(token)
		if err != nil || !user.IsActive {
			jsonError(w, "Token yaroqsiz", http.StatusUnauthorized)
			return
		}
		ctx := context.WithValue(r.Context(), userCtxKey, user)
		next(w, r.WithContext(ctx))
	}
}

// superAdminRequired — faqat super_admin uchun
func superAdminRequired(next http.HandlerFunc) http.HandlerFunc {
	return authRequired(func(w http.ResponseWriter, r *http.Request) {
		user := getUserFromCtx(r)
		if user.Role != "super_admin" {
			jsonError(w, "Super admin ruxsati kerak", http.StatusForbidden)
			return
		}
		next(w, r)
	})
}

// POST /api/auth/login
func handleLogin(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Password string `json:"password"`
		// Qurilma ma'lumotlari — mijoz har login'da yuboradi (ikkilamchi:
		// bo'sh bo'lsa ham login ishlaydi). DeviceID — fizik qurilmaning
		// barqaror noyob kaliti (mijozda bir marta yaratilib saqlanadi), shu
		// sababli bitta qurilma ikki marta sanalmaydi.
		DeviceID   string `json:"deviceId"`
		DeviceName string `json:"deviceName"`
		Platform   string `json:"platform"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonError(w, "JSON xato", http.StatusBadRequest)
		return
	}
	if body.Password == "" {
		jsonError(w, "Parol kerak", http.StatusBadRequest)
		return
	}

	user, err := dbGetUserByPassword(body.Password)
	if err != nil {
		jsonError(w, "Parol xato", http.StatusUnauthorized)
		return
	}
	if !user.IsActive {
		jsonError(w, "Foydalanuvchi bloklangan", http.StatusForbidden)
		return
	}

	// Sessiyani qurilmaga bog'laymiz (device_id), shunda super_admin o'sha
	// qurilmani o'chirganda faqat shu qurilmaning tokeni yaroqsiz bo'ladi.
	deviceID := strings.TrimSpace(body.DeviceID)
	token := generateToken()
	dbCreateSession(token, user.ID, deviceID)

	// Qurilmani qayd etish — faqat super_admin BO'LMAGAN akauntlar uchun va
	// mijoz device_id yuborgan bo'lsa. Dedup (user_id, device_id): bir xil
	// qurilmadan qayta login faqat last_login_at'ni yangilaydi. super_admin
	// login qilganda yozilmaydi — uning qurilmasi hech kimning ro'yxatiga
	// tushmaydi. Device yozuvi xatosi login'ni buzmasin.
	if user.Role != "super_admin" && deviceID != "" {
		if derr := dbUpsertUserDevice(user.ID, deviceID, strings.TrimSpace(body.DeviceName), strings.TrimSpace(body.Platform)); derr != nil {
			log.Printf("user_device yozishda xato (user %d): %v", user.ID, derr)
		}
	}

	cats := getUserCategories(user.ID)
	ishchiCats := getUserIshchiCategories(user.ID)
	branch := getBranchPtr(user.BranchID)
	jsonResponse(w, map[string]interface{}{
		"token": token,
		"user": UserResponse{
			UserRow:          *user,
			Categories:       cats,
			IshchiCategories: ishchiCats,
			Branch:           branch,
			DeviceCount:      dbCountUserDevices(user.ID),
		},
	})
}

// GET /api/users/{id}/devices — bitta foydalanuvchi login qilgan qurilmalar
// ro'yxati (super_admin). Eng oxirgi login birinchi keladi.
func handleGetUserDevices(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		jsonError(w, "Noto'g'ri ID", http.StatusBadRequest)
		return
	}
	devices, err := dbGetUserDevices(id)
	if err != nil {
		jsonError(w, "DB xato: "+err.Error(), http.StatusInternalServerError)
		return
	}
	jsonResponse(w, map[string]interface{}{"devices": devices})
}

// DELETE /api/users/{id}/devices/{deviceId} — super_admin foydalanuvchining
// bitta qurilmasini ro'yxatdan o'chiradi va o'sha qurilmadagi tokenni yaroqsiz
// qiladi (qurilma qaytadan login qilishi shart bo'ladi). {deviceId} —
// user_devices.id (qurilma yozuvining ID'si).
func handleDeleteUserDevice(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		jsonError(w, "Noto'g'ri ID", http.StatusBadRequest)
		return
	}
	deviceRowID, err := strconv.ParseInt(r.PathValue("deviceId"), 10, 64)
	if err != nil {
		jsonError(w, "Noto'g'ri qurilma ID", http.StatusBadRequest)
		return
	}
	deviceID, err := dbDeleteUserDevice(id, deviceRowID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			jsonError(w, "Qurilma topilmadi", http.StatusNotFound)
			return
		}
		jsonError(w, "DB xato: "+err.Error(), http.StatusInternalServerError)
		return
	}
	// Real-time: o'sha qurilmaning ochiq WS ulanishiga darhol "force_logout"
	// yuboramiz — mijoz 401 ni kutmasdan login ekraniga qaytadi.
	broadcastForceLogout(id, deviceID)
	jsonResponse(w, map[string]string{"status": "ok"})
}

// POST /api/auth/logout
func handleLogout(w http.ResponseWriter, r *http.Request) {
	token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	dbDeleteSession(token)
	jsonResponse(w, map[string]string{"status": "ok"})
}

// GET /api/auth/me
func handleMe(w http.ResponseWriter, r *http.Request) {
	user := getUserFromCtx(r)
	resp, err := dbGetUserByID(user.ID)
	if err != nil {
		jsonError(w, "Foydalanuvchi topilmadi", http.StatusNotFound)
		return
	}
	jsonResponse(w, resp)
}
