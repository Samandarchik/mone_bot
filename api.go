package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// GET /api/public/categories — public, faol kategoriyalar ro'yxati
func handlePublicCategories(w http.ResponseWriter, r *http.Request) {
	cats, err := dbGetCategories()
	if err != nil {
		jsonError(w, "DB xato", http.StatusInternalServerError)
		return
	}
	active := []map[string]interface{}{}
	for _, c := range cats {
		if c.IsActive {
			active = append(active, map[string]interface{}{"id": c.ID, "name": c.Name})
		}
	}
	jsonResponse(w, active)
}

// POST /api/report-error — frontenddan xatolik xabari (admin TG ga yuboriladi)
func handleReportError(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Error      string `json:"error"`
		TgUserID   int64  `json:"tg_user_id"`
		TgUsername string `json:"tg_username"`
		FIO        string `json:"fio"`
		Telefon    string `json:"telefon"`
		Lavozim    string `json:"lavozim"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonError(w, "JSON xato", http.StatusBadRequest)
		return
	}

	msg := fmt.Sprintf(
		"🚨 XATOLIK — Rezume yuborishda\n\n"+
			"❌ Xatolik: %s\n\n"+
			"👤 FIO: %s\n"+
			"📱 Telefon: %s\n"+
			"💼 Lavozim: %s\n"+
			"🆔 TG ID: %d\n"+
			"📎 TG Username: @%s",
		body.Error, body.FIO, body.Telefon, body.Lavozim, body.TgUserID, body.TgUsername,
	)
	sendTgMessage(adminTgID, msg)

	jsonResponse(w, map[string]string{"status": "reported"})
}

// POST /rezume — public, rezume yuborish (DB + Telegram)
func handleRezume(w http.ResponseWriter, r *http.Request) {
	var anketa Anketa
	if err := json.NewDecoder(r.Body).Decode(&anketa); err != nil {
		fio := strings.TrimSpace(anketa.Familiya + " " + anketa.Ism)
		notifyAdmin("JSON parse xato", err.Error(), fio, anketa.Telefon, anketa.TgUserID, anketa.TgUsername)
		jsonError(w, "JSON xato: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Rasmni faylga saqlash
	rasmURL := ""
	if anketa.Rasm != "" && strings.Contains(anketa.Rasm, ",") {
		var err error
		rasmURL, err = saveImage(anketa.Rasm, anketa.TgUserID)
		if err != nil {
			log.Printf("Rasm saqlashda xato: %v", err)
			fio := strings.TrimSpace(anketa.Familiya + " " + anketa.Ism)
			notifyAdmin("Rasm saqlashda xato", err.Error(), fio, anketa.Telefon, anketa.TgUserID, anketa.TgUsername)
		}
	}

	// Dublikat tekshiruv: tugilgan_sana + lavozim + telefon bir xil bo'lsa eski rezumeni o'chirish
	deleteDuplicateRezume(anketa.Lavozim, anketa.TugilganSana, anketa.Telefon)

	// Bazaga saqlash
	id, err := saveRezume(&anketa, rasmURL)
	if err != nil {
		log.Printf("DB saqlashda xato: %v", err)
		fio := strings.TrimSpace(anketa.Familiya + " " + anketa.Ism)
		notifyAdmin("DB saqlashda xato", err.Error(), fio, anketa.Telefon, anketa.TgUserID, anketa.TgUsername)
	} else {
		log.Printf("Rezume DB ga saqlandi: id=%d", id)
	}

	// Telegram caption
	var tillarStr string
	for _, t := range anketa.Tillar {
		if t.Daraja != "" {
			tillarStr += fmt.Sprintf("  %s: %s\n", t.Til, t.Daraja)
		}
	}
	if tillarStr == "" {
		tillarStr = "  —\n"
	}

	fio := strings.TrimSpace(anketa.Familiya + " " + anketa.Ism + " " + anketa.Sharif)

	// Foydalanuvchiga auto-xabar yuborish: to'liq ma'lumotlari bilan
	if anketa.TgUserID != 0 {
		userMsg := fmt.Sprintf(
			"Assalomu alaykum, %s!\n\n"+
				"Rezumeyingiz qabul qilindi. Tez orada ko'rib chiqib, aloqaga chiqamiz.\n\n"+
				"Sizning ma'lumotlaringiz:\n"+
				"━━━━━━━━━━━━━━━━━━━━\n"+
				"Lavozim: %s\n"+
				"FIO: %s\n"+
				"Tug'ilgan sana: %s\n"+
				"Bo'y: %d sm\n"+
				"Vazn: %d kg\n"+
				"Manzil: %s\n"+
				"Mo'ljal: %s\n"+
				"Umumiy tajriba: %s\n"+
				"Chet el tajribasi: %s\n"+
				"Ma'lumot: %s\n"+
				"Oilaviy holat: %s\n"+
				"Tillar:\n%s"+
				"Telefon: %s\n"+
				"Qo'shimcha: %s\n"+
				"━━━━━━━━━━━━━━━━━━━━\n\n"+
				"Rahmat!",
			fio,
			anketa.Lavozim, fio, anketa.TugilganSana,
			anketa.BoySm, anketa.VaznKg,
			anketa.YashashManzili, anketa.Moljal,
			anketa.UmumiyTajriba, anketa.ChetElTajribasi,
			anketa.Malumot, anketa.OilaviyHolat, tillarStr,
			anketa.Telefon, anketa.Qoshimcha,
		)
		// Rasm bilan yuborish
		if anketa.Rasm != "" && strings.Contains(anketa.Rasm, ",") {
			sendPhotoToTelegram(anketa.TgUserID, anketa.Rasm, userMsg, nil)
		} else {
			sendTgMessage(anketa.TgUserID, userMsg)
		}
	}

	jsonResponse(w, map[string]interface{}{"status": "ok", "id": id})
	log.Printf("Anketa yuborildi: %s (tg_user: %d)", fio, anketa.TgUserID)

	// WebSocket orqali yangi rezumeni broadcast qilish
	if id > 0 {
		if rez, err := getRezumeByID(id); err == nil {
			broadcastNewRezume(rez)
		}
	}
}

// GET /api/rezumeler — foydalanuvchi kategoriyalari bo'yicha filtrlangan
func handleGetRezumeler(w http.ResponseWriter, r *http.Request) {
	user := getUserFromCtx(r)

	lavozim := r.URL.Query().Get("lavozim")
	status := r.URL.Query().Get("status")
	search := r.URL.Query().Get("search")
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))

	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}

	// Super admin hamma narsani ko'radi, oddiy admin faqat o'z kategoriyalarini
	var allowedCategories []string
	if user.Role != "super_admin" {
		allowedCategories = getUserCategoryNames(user.ID)
		if len(allowedCategories) == 0 {
			jsonResponse(w, map[string]interface{}{
				"data": []RezumeRow{}, "total": 0, "page": page, "limit": limit, "pages": 0,
			})
			return
		}
	}

	rezumeler, total, err := getRezumeler(lavozim, status, search, allowedCategories, page, limit)
	if err != nil {
		jsonError(w, "DB xato: "+err.Error(), http.StatusInternalServerError)
		return
	}

	attachInterviews(rezumeler)

	pages := 0
	if total > 0 {
		pages = (total + limit - 1) / limit
	}

	jsonResponse(w, map[string]interface{}{
		"data":  rezumeler,
		"total": total,
		"page":  page,
		"limit": limit,
		"pages": pages,
	})
}

// GET /api/rezumeler/{id}
func handleGetRezume(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		jsonError(w, "Noto'g'ri ID", http.StatusBadRequest)
		return
	}

	rezume, err := getRezumeByID(id)
	if err != nil {
		jsonError(w, "Rezume topilmadi", http.StatusNotFound)
		return
	}

	// Interviews ni qo'shish
	single := []RezumeRow{*rezume}
	attachInterviews(single)

	jsonResponse(w, single[0])
}

// DELETE /api/rezumeler/{id}
func handleDeleteRezume(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		jsonError(w, "Noto'g'ri ID", http.StatusBadRequest)
		return
	}

	if err := deleteRezume(id); err != nil {
		jsonError(w, "O'chirishda xato", http.StatusInternalServerError)
		return
	}

	jsonResponse(w, map[string]string{"status": "deleted"})
	broadcastRezumeDelete(id)
}

// PATCH /api/rezumeler/{id}/status — qabul/rad qilish
// Super admin o'zgartira olmaydi — faqat ko'radi.
func handleUpdateStatus(w http.ResponseWriter, r *http.Request) {
	user := getUserFromCtx(r)
	if user.Role == "super_admin" {
		jsonError(w, "Super admin statusni o'zgartira olmaydi", http.StatusForbidden)
		return
	}

	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		jsonError(w, "Noto'g'ri ID", http.StatusBadRequest)
		return
	}

	var body struct {
		Status string `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonError(w, "JSON xato", http.StatusBadRequest)
		return
	}

	validStatuses := map[string]bool{
		"pending": true, "interviewing": true, "trial": true,
		"rejected": true, "accepted": true, "reserve": true, "noshow": true,
	}
	if !validStatuses[body.Status] {
		jsonError(w, "Noto'g'ri status. Mumkin: pending, interviewing, trial, rejected, accepted, reserve, noshow", http.StatusBadRequest)
		return
	}

	// Admin ma'lumotlarini olish
	adminName := user.FullName
	if adminName == "" {
		adminName = user.Username
	}

	if err := updateRezumeStatusWithAdmin(id, body.Status, user.ID, adminName); err != nil {
		jsonError(w, "Statusni yangilashda xato", http.StatusInternalServerError)
		return
	}

	// Accepted bo'lsa — foydalanuvchiga auto-xabar
	if body.Status == "accepted" {
		rezume, err := getRezumeByID(id)
		if err == nil && rezume.TgUserID != 0 {
			sendTgMessage(rezume.TgUserID, "Tabriklaymiz! Siz ishga qabul qilindingiz!")
		}
	}

	// Rejected bo'lsa — foydalanuvchiga xabar
	if body.Status == "rejected" {
		rezume, err := getRezumeByID(id)
		if err == nil && rezume.TgUserID != 0 {
			sendTgMessage(rezume.TgUserID, "Afsuski, sizning arizangiz rad etildi.")
		}
	}

	jsonResponse(w, map[string]string{"status": "updated"})
	broadcastRezumeStatusUpdate(id, body.Status, adminName)
}

// --- Yordamchi funksiyalar ---

func saveImage(base64Data string, tgUserID int64) (string, error) {
	parts := strings.SplitN(base64Data, ",", 2)
	if len(parts) != 2 {
		return "", fmt.Errorf("noto'g'ri base64 format")
	}
	imgBytes, err := base64.StdEncoding.DecodeString(parts[1])
	if err != nil {
		return "", err
	}
	filename := fmt.Sprintf("%d_%d.jpg", time.Now().UnixMilli(), tgUserID)
	path := filepath.Join("uploads", filename)
	if err := os.WriteFile(path, imgBytes, 0644); err != nil {
		return "", err
	}
	return "/uploads/" + filename, nil
}

// saveVoice — base64 ovoz (shu jumladan "data:audio/...;base64,..." prefix bilan ham) ni faylga saqlaydi.
func saveVoice(base64Data string, interviewID int64, ext string) (string, error) {
	data := base64Data
	if strings.Contains(data, ",") {
		parts := strings.SplitN(data, ",", 2)
		data = parts[1]
	}
	audioBytes, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		return "", err
	}
	ext = strings.TrimPrefix(ext, ".")
	if ext == "" {
		ext = "m4a"
	}
	filename := fmt.Sprintf("voice_%d_%d.%s", interviewID, time.Now().UnixMilli(), ext)
	path := filepath.Join("uploads", filename)
	if err := os.WriteFile(path, audioBytes, 0644); err != nil {
		return "", err
	}
	return "/uploads/" + filename, nil
}

func jsonResponse(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

func jsonError(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

func notifyAdmin(title, errMsg, fio, telefon string, tgUserID int64, tgUsername string) {
	msg := fmt.Sprintf(
		"🚨 XATOLIK — %s\n\n"+
			"❌ Sabab: %s\n\n"+
			"👤 FIO: %s\n"+
			"📱 Telefon: %s\n"+
			"🆔 TG ID: %d\n"+
			"📎 TG Username: @%s",
		title, errMsg, fio, telefon, tgUserID, tgUsername,
	)
	go sendTgMessage(adminTgID, msg)
}
