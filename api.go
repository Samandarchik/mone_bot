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

// POST /rezume — public, rezume yuborish (DB + Telegram)
func handleRezume(w http.ResponseWriter, r *http.Request) {
	var anketa Anketa
	if err := json.NewDecoder(r.Body).Decode(&anketa); err != nil {
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
		}
	}

	// Bazaga saqlash
	id, err := saveRezume(&anketa, rasmURL)
	if err != nil {
		log.Printf("DB saqlashda xato: %v", err)
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

	caption := fmt.Sprintf(
		"Должность: %s\n"+
			"ФИО: %s\n"+
			"Дата рождения: %s\n"+
			"Рост: %d см\n"+
			"Вес: %d кг\n"+
			"Адрес: %s\n"+
			"Ориентир: %s\n"+
			"Общий стаж: %s\n"+
			"Работа за рубежом: %s\n"+
			"Образование: %s\n"+
			"Семейное положение: %s\n"+
			"Tillar:\n%s"+
			"Телефон: %s\n"+
			"Qo'shimcha: %s\n"+
			"━━━━━━━━━━━━━━━━━━━━",
		anketa.Lavozim, fio, anketa.TugilganSana,
		anketa.BoySm, anketa.VaznKg,
		anketa.YashashManzili, anketa.Moljal,
		anketa.UmumiyTajriba, anketa.ChetElTajribasi,
		anketa.Malumot, anketa.OilaviyHolat, tillarStr,
		anketa.Telefon, anketa.Qoshimcha,
	)

	// Guruh ID ni kategoriyadan olish
	cat, catErr := getCategoryByName(anketa.Lavozim)
	var groupID int64
	if catErr != nil || cat.TgGroupID == 0 {
		groupID = -1003862297561
	} else {
		groupID = cat.TgGroupID
	}

	// Callback data: rezume ID + tg_user_id
	replyMarkup := map[string]interface{}{
		"inline_keyboard": [][]map[string]string{
			{
				{"text": "✅ Qabul qilish", "callback_data": fmt.Sprintf("accept:%d:%d", id, anketa.TgUserID)},
				{"text": "❌ Qabul qilmaslik", "callback_data": fmt.Sprintf("reject:%d:%d", id, anketa.TgUserID)},
			},
		},
	}

	var tgErr error
	if anketa.Rasm != "" && strings.Contains(anketa.Rasm, ",") {
		tgErr = sendPhotoToTelegram(groupID, anketa.Rasm, caption, replyMarkup)
	} else {
		tgErr = sendMessageToTelegramWithKeyboard(groupID, caption, replyMarkup)
	}

	if tgErr != nil {
		log.Printf("Telegram xato: %v", tgErr)
	}

	// Foydalanuvchiga auto-xabar yuborish: "rezumeyingizni ko'rib chiqamiz"
	if anketa.TgUserID != 0 {
		userMsg := fmt.Sprintf(
			"Assalomu alaykum, %s!\n\nRezumeyingiz qabul qilindi. Tez orada ko'rib chiqib, aloqaga chiqamiz.\n\nRahmat!",
			fio,
		)
		// Rasm bilan yuborish
		if anketa.Rasm != "" && strings.Contains(anketa.Rasm, ",") {
			sendPhotoToTelegram(anketa.TgUserID, anketa.Rasm, userMsg, nil)
		} else {
			sendTgMessage(anketa.TgUserID, userMsg)
		}
	}

	jsonResponse(w, map[string]interface{}{"status": "ok", "id": id})
	log.Printf("Anketa yuborildi: %s -> guruh %d (tg_user: %d)", fio, groupID, anketa.TgUserID)
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
}

// PATCH /api/rezumeler/{id}/status — qabul/rad qilish
func handleUpdateStatus(w http.ResponseWriter, r *http.Request) {
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

	validStatuses := map[string]bool{"yangi": true, "qabul": true, "rad": true}
	if !validStatuses[body.Status] {
		jsonError(w, "Noto'g'ri status. Mumkin: yangi, qabul, rad", http.StatusBadRequest)
		return
	}

	if err := updateRezumeStatus(id, body.Status); err != nil {
		jsonError(w, "Statusni yangilashda xato", http.StatusInternalServerError)
		return
	}

	// Qabul qilinsa — foydalanuvchiga auto-xabar
	if body.Status == "qabul" {
		rezume, err := getRezumeByID(id)
		if err == nil && rezume.TgUserID != 0 {
			sendTgMessage(rezume.TgUserID, "Tabriklaymiz! Sizning rezumeyingiz qabul qilindi. Tez orada siz bilan bog'lanamiz.")
		}
	}
	// Rad qilinsa — xabar yuborilmaydi

	jsonResponse(w, map[string]string{"status": "updated"})
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

func jsonResponse(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

func jsonError(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
