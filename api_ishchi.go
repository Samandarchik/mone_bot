package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
)

// POST /ishchi-rezume — public, ishchi anketa yuborish
func handleIshchiRezume(w http.ResponseWriter, r *http.Request) {
	var anketa IshchiAnketa
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
			log.Printf("Ishchi rasm saqlashda xato: %v", err)
		}
	}

	// Dublikat tekshiruv
	deleteDuplicateIshchi(anketa.Vakansiya, anketa.TugilganSana, anketa.Telefon)

	// Bazaga saqlash
	id, err := saveIshchiAnketa(&anketa, rasmURL)
	if err != nil {
		log.Printf("Ishchi anketa DB saqlashda xato: %v", err)
	} else {
		log.Printf("Ishchi anketa DB ga saqlandi: id=%d", id)
	}

	// Foydalanuvchiga auto-xabar yuborish
	if anketa.TgUserID != 0 {
		userMsg := fmt.Sprintf(
			"Assalomu alaykum, %s!\n\n"+
				"Anketangiz qabul qilindi. Tez orada ko'rib chiqib, aloqaga chiqamiz.\n\n"+
				"Sizning ma'lumotlaringiz:\n"+
				"━━━━━━━━━━━━━━━━━━━━\n"+
				"Vakansiya: %s\n"+
				"FIO: %s\n"+
				"Tug'ilgan sana: %s\n"+
				"Bo'y: %d sm\n"+
				"Vazn: %d kg\n"+
				"Manzil: %s\n"+
				"Oilaviy holat: %s\n"+
				"Bolalar: %s\n"+
				"Tillar: %s\n"+
				"Ma'lumot: %s\n"+
				"Grafik: %s\n"+
				"Sudimlik: %s\n"+
				"Haydovchilik guvohnomasi: %s\n"+
				"Telefon: %s\n"+
				"━━━━━━━━━━━━━━━━━━━━\n\n"+
				"Rahmat!",
			anketa.FIO,
			anketa.Vakansiya, anketa.FIO, anketa.TugilganSana,
			anketa.BoySm, anketa.VaznKg,
			anketa.Manzil, anketa.OilaviyHolat, anketa.Bolalar,
			anketa.Tillar, anketa.Malumot, anketa.Grafik,
			anketa.Sudimlik, anketa.Haydovchilik, anketa.Telefon,
		)
		if anketa.Rasm != "" && strings.Contains(anketa.Rasm, ",") {
			sendIshchiPhotoToTelegram(anketa.TgUserID, anketa.Rasm, userMsg)
		} else {
			sendIshchiTgMessage(anketa.TgUserID, userMsg)
		}
	}

	jsonResponse(w, map[string]interface{}{"status": "ok", "id": id})
	log.Printf("Ishchi anketa yuborildi: %s (tg_user: %d)", anketa.FIO, anketa.TgUserID)

	// WS broadcast
	if id > 0 {
		if row, err := getIshchiAnketaByID(id); err == nil {
			broadcastNewIshchi(row)
		}
	}
}

// GET /api/ishchi-anketalar — ishchi_admin uchun user_ishchi_categories bo'yicha filter
func handleGetIshchiAnketalar(w http.ResponseWriter, r *http.Request) {
	user := getUserFromCtx(r)

	vakansiya := r.URL.Query().Get("vakansiya")
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

	// Super admin hamma narsani ko'radi, ishchi_admin faqat o'z kategoriyalarini.
	// admin role esa ishchi tomoniga kirishi kerak emas.
	var allowedCategories []string
	switch user.Role {
	case "super_admin":
		// hammasi
	case "ishchi_admin":
		allowedCategories = getUserIshchiCategoryNames(user.ID)
		if len(allowedCategories) == 0 {
			jsonResponse(w, map[string]interface{}{
				"data": []IshchiRow{}, "total": 0, "page": page, "limit": limit, "pages": 0,
			})
			return
		}
	default:
		jsonError(w, "Ruxsat yo'q", http.StatusForbidden)
		return
	}

	anketalar, total, err := getIshchiAnketalar(vakansiya, status, search, allowedCategories, page, limit)
	if err != nil {
		jsonError(w, "DB xato: "+err.Error(), http.StatusInternalServerError)
		return
	}

	attachIshchiInterviews(anketalar)

	pages := 0
	if total > 0 {
		pages = (total + limit - 1) / limit
	}

	jsonResponse(w, map[string]interface{}{
		"data":  anketalar,
		"total": total,
		"page":  page,
		"limit": limit,
		"pages": pages,
	})
}

// GET /api/ishchi-anketalar/{id}
func handleGetIshchiAnketa(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		jsonError(w, "Noto'g'ri ID", http.StatusBadRequest)
		return
	}
	anketa, err := getIshchiAnketaByID(id)
	if err != nil {
		jsonError(w, "Anketa topilmadi", http.StatusNotFound)
		return
	}
	jsonResponse(w, anketa)
}

// PUT /api/ishchi-anketalar/{id}
func handleUpdateIshchiAnketa(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		jsonError(w, "Noto'g'ri ID", http.StatusBadRequest)
		return
	}
	var anketa IshchiAnketa
	if err := json.NewDecoder(r.Body).Decode(&anketa); err != nil {
		jsonError(w, "JSON xato: "+err.Error(), http.StatusBadRequest)
		return
	}
	if err := updateIshchiAnketa(id, &anketa); err != nil {
		jsonError(w, "Yangilashda xato: "+err.Error(), http.StatusInternalServerError)
		return
	}
	jsonResponse(w, map[string]string{"status": "updated"})
	if row, err := getIshchiAnketaByID(id); err == nil {
		broadcastIshchiUpdate(row)
	}
}

// DELETE /api/ishchi-anketalar/{id}
func handleDeleteIshchiAnketa(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		jsonError(w, "Noto'g'ri ID", http.StatusBadRequest)
		return
	}
	if err := deleteIshchiAnketa(id); err != nil {
		jsonError(w, "O'chirishda xato", http.StatusInternalServerError)
		return
	}
	jsonResponse(w, map[string]string{"status": "deleted"})
	broadcastIshchiDelete(id)
}

// PATCH /api/ishchi-anketalar/{id}/status — qabul/rad qilish (rezume bilan parallel)
// Super admin o'zgartira olmaydi — faqat ko'radi.
func handleUpdateIshchiStatus(w http.ResponseWriter, r *http.Request) {
	user := getUserFromCtx(r)
	if user.Role == "super_admin" {
		jsonError(w, "Super admin statusni o'zgartira olmaydi", http.StatusForbidden)
		return
	}
	if user.Role != "ishchi_admin" {
		jsonError(w, "Ruxsat yo'q", http.StatusForbidden)
		return
	}

	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		jsonError(w, "Noto'g'ri ID", http.StatusBadRequest)
		return
	}

	var body struct {
		Status    string `json:"status"`
		VoiceData string `json:"voice_data"`
		VoiceExt  string `json:"voice_ext"`
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
		jsonError(w, "Noto'g'ri status", http.StatusBadRequest)
		return
	}

	if body.Status == "rejected" && body.VoiceData == "" {
		jsonError(w, "Rad qilish uchun ovozli izoh majburiy", http.StatusBadRequest)
		return
	}

	adminName := user.FullName
	if adminName == "" {
		adminName = user.Username
	}

	voiceUrl := ""
	if body.VoiceData != "" {
		ext := body.VoiceExt
		if ext == "" {
			ext = "m4a"
		}
		url, err := saveVoice(body.VoiceData, id, ext)
		if err != nil {
			jsonError(w, "Ovozni saqlashda xato: "+err.Error(), http.StatusInternalServerError)
			return
		}
		voiceUrl = url
	}

	if voiceUrl != "" {
		if err := updateIshchiStatusWithVoice(id, body.Status, user.ID, adminName, voiceUrl); err != nil {
			jsonError(w, "Statusni yangilashda xato", http.StatusInternalServerError)
			return
		}
	} else {
		if err := updateIshchiStatusWithAdmin(id, body.Status, user.ID, adminName); err != nil {
			jsonError(w, "Statusni yangilashda xato", http.StatusInternalServerError)
			return
		}
	}

	if body.Status == "accepted" {
		ishchi, err := getIshchiAnketaByID(id)
		if err == nil && ishchi.TgUserID != 0 {
			sendIshchiTgMessage(ishchi.TgUserID, "Tabriklaymiz! Siz ishga qabul qilindingiz!")
		}
	}
	if body.Status == "rejected" {
		ishchi, err := getIshchiAnketaByID(id)
		if err == nil && ishchi.TgUserID != 0 {
			sendIshchiTgMessage(ishchi.TgUserID, "Afsuski, sizning arizangiz rad etildi.")
		}
	}

	jsonResponse(w, map[string]string{"status": "updated"})
	broadcastIshchiStatusUpdate(id, body.Status, adminName)
}
