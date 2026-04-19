package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
)

// Intervyu qoidalari:
// - Har kuni admin maks 10 ta nomzodni chaqira oladi
// - Slotlar 9:00 dan boshlab har 15 minut: 9:00, 9:15, ..., 11:15 (10 ta slot)
// - Har bir rezumeni maks 3 marta chaqirish mumkin
const (
	interviewMaxPerDay     = 10
	interviewMaxPerRezume  = 4
	interviewStartHour     = 9
	interviewSlotMinutes   = 15
)

// computeNextInterviewSlot — berilgan admin va sanaga bo'sh slotni topadi.
// Agar barcha 10 slot band bo'lsa, xato qaytaradi.
func computeNextInterviewSlot(invitedByID int64, date string) (string, error) {
	rows, err := db.Query(
		"SELECT interview_time FROM interviews WHERE invited_by_id = ? AND interview_date = ?",
		invitedByID, date,
	)
	if err != nil {
		return "", err
	}
	used := map[string]bool{}
	for rows.Next() {
		var t string
		if err := rows.Scan(&t); err == nil {
			used[t] = true
		}
	}
	rows.Close()

	if len(used) >= interviewMaxPerDay {
		return "", fmt.Errorf("bu kunga 10 tadan ortiq intervyu olib bo'lmaydi")
	}

	for k := 0; k < interviewMaxPerDay; k++ {
		totalMin := interviewStartHour*60 + k*interviewSlotMinutes
		candidate := fmt.Sprintf("%02d:%02d", totalMin/60, totalMin%60)
		if !used[candidate] {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("bu kunga 10 tadan ortiq intervyu olib bo'lmaydi")
}

// POST /api/interviews — intervyuga chaqirish
func handleCreateInterview(w http.ResponseWriter, r *http.Request) {
	user := getUserFromCtx(r)
	if !user.CanInterview && user.Role != "super_admin" {
		jsonError(w, "Intervyuga chaqirish huquqingiz yo'q", http.StatusForbidden)
		return
	}

	var body struct {
		RezumeID      int64  `json:"rezume_id"`
		InterviewDate string `json:"interview_date"`
		InterviewTime string `json:"interview_time"` // ixtiyoriy — serverda avtomatik hisoblanadi
		BranchID      int64  `json:"branch_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonError(w, "JSON xato", http.StatusBadRequest)
		return
	}
	if body.RezumeID == 0 || body.InterviewDate == "" || body.InterviewTime == "" {
		jsonError(w, "rezume_id, interview_date va interview_time kerak", http.StatusBadRequest)
		return
	}

	// Rezumeni tekshirish
	rezume, err := getRezumeByID(body.RezumeID)
	if err != nil {
		jsonError(w, "Rezume topilmadi", http.StatusNotFound)
		return
	}

	// Avvalgi bahosiz (rating=0) chaqiriq bo'lsa — o'chiramiz (ertaga qayta chaqirish uchun)
	db.Exec("DELETE FROM interviews WHERE rezume_id = ? AND rating = 0", body.RezumeID)

	// Rezume uchun maks 4 marta chaqirish qoidasi
	var rezumeInterviewCount int
	if err := db.QueryRow("SELECT COUNT(*) FROM interviews WHERE rezume_id = ?", body.RezumeID).Scan(&rezumeInterviewCount); err != nil {
		jsonError(w, "DB xato: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if rezumeInterviewCount >= interviewMaxPerRezume {
		jsonError(w, "Ushbu nomzodni 4 martadan ortiq intervyuga chaqirib bo'lmaydi", http.StatusBadRequest)
		return
	}

	// Branch ma'lumotlarini olish
	var branchName string
	var branchLat, branchLng float64
	if body.BranchID > 0 {
		branch, err := dbGetBranchByID(body.BranchID)
		if err != nil {
			jsonError(w, "Filial topilmadi", http.StatusBadRequest)
			return
		}
		branchName = branch.Name
		branchLat = branch.Latitude
		branchLng = branch.Longitude
	}

	// Rezume statusini interviewing ga o'tkazish
	updateRezumeStatus(body.RezumeID, "interviewing")

	// Interview yaratish
	id, err := dbCreateInterview(body.RezumeID, user.ID, body.InterviewDate, body.InterviewTime, body.BranchID)
	if err != nil {
		jsonError(w, "Interview yaratishda xato: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Foydalanuvchiga TG xabar yuborish
	if rezume.TgUserID != 0 {
		fio := rezume.Familiya + " " + rezume.Ism
		msg := fmt.Sprintf(
			"Assalomu alaykum, %s!\n\n"+
				"Siz intervyuga taklif qilinyapsiz.\n\n"+
				"Sana: %s\n"+
				"Vaqt: %s\n",
			fio, body.InterviewDate, body.InterviewTime,
		)
		if branchName != "" {
			msg += fmt.Sprintf("Filial: %s\n", branchName)
		}
		msg += fmt.Sprintf(
			"\nIltimos, o'z vaqtida keling.\n"+
				"Chaqirgan: %s",
			user.FullName,
		)
		sendTgMessage(rezume.TgUserID, msg)

		// Lokatsiyani yuborish
		if branchLat != 0 && branchLng != 0 {
			sendTgLocation(rezume.TgUserID, branchLat, branchLng)
		}
	}

	interview, _ := dbGetInterviewByID(id)
	jsonResponse(w, interview)

	// WebSocket orqali real-time yangilanish
	if interview != nil {
		broadcastInterviewCreated(interview)
		// rezume statusi ham "interviewing" ga o'tkazildi — ro'yxatdagi kartochka ham yangilansin
		broadcastRezumeStatusUpdate(body.RezumeID, "interviewing", user.FullName)
	}
}

// GET /api/interviews?rezume_id=1&rating=0&date=01.01.2026&invited_by_id=1&page=1&limit=20
func handleGetInterviews(w http.ResponseWriter, r *http.Request) {
	rezumeID, _ := strconv.ParseInt(r.URL.Query().Get("rezume_id"), 10, 64)
	invitedByID, _ := strconv.ParseInt(r.URL.Query().Get("invited_by_id"), 10, 64)
	date := r.URL.Query().Get("date")
	rating := -1 // -1 = barchasi
	if r.URL.Query().Get("rating") != "" {
		rating, _ = strconv.Atoi(r.URL.Query().Get("rating"))
	}
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))

	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}

	interviews, total, err := dbGetInterviews(rezumeID, rating, invitedByID, date, page, limit)
	if err != nil {
		jsonError(w, "DB xato: "+err.Error(), http.StatusInternalServerError)
		return
	}

	pages := 0
	if total > 0 {
		pages = (total + limit - 1) / limit
	}

	jsonResponse(w, map[string]interface{}{
		"data":  interviews,
		"total": total,
		"page":  page,
		"limit": limit,
		"pages": pages,
	})
}

// GET /api/interviews/{id}
func handleGetInterview(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		jsonError(w, "Noto'g'ri ID", http.StatusBadRequest)
		return
	}

	interview, err := dbGetInterviewByID(id)
	if err != nil {
		jsonError(w, "Interview topilmadi", http.StatusNotFound)
		return
	}

	jsonResponse(w, interview)
}

// PATCH /api/interviews/{id} — intervyu natijasini qo'yish (rating + comment + voice)
func handleUpdateInterview(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		jsonError(w, "Noto'g'ri ID", http.StatusBadRequest)
		return
	}

	var body struct {
		Rating    int    `json:"rating"`
		Comment   string `json:"comment"`
		VoiceData string `json:"voice_data"` // base64, "data:audio/xxx;base64,..." format
		VoiceExt  string `json:"voice_ext"`  // "m4a", "aac", "mp3" (optional)
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonError(w, "JSON xato", http.StatusBadRequest)
		return
	}

	if body.Rating < 1 || body.Rating > 4 {
		jsonError(w, "Rating 1-4 bo'lishi kerak. 1=Zo'r, 2=Yaxshi, 3=Qabul qilinmadi, 4=Ishga qabul qilindi", http.StatusBadRequest)
		return
	}

	// Eski yozuvni olib, mavjud voice_url ni saqlaymiz
	existing, _ := dbGetInterviewByID(id)
	voiceUrl := ""
	if existing != nil {
		voiceUrl = existing.VoiceUrl
	}

	// Agar yangi ovoz yuborilgan bo'lsa — saqlaymiz
	if body.VoiceData != "" {
		ext := body.VoiceExt
		if ext == "" {
			ext = "m4a"
		}
		savedUrl, err := saveVoice(body.VoiceData, id, ext)
		if err != nil {
			jsonError(w, "Ovozni saqlashda xato: "+err.Error(), http.StatusInternalServerError)
			return
		}
		voiceUrl = savedUrl
	}

	if err := dbUpdateInterview(id, body.Rating, body.Comment, voiceUrl); err != nil {
		jsonError(w, "Yangilashda xato", http.StatusInternalServerError)
		return
	}

	interview, _ := dbGetInterviewByID(id)
	jsonResponse(w, interview)

	// WebSocket orqali real-time yangilanish
	if interview != nil {
		broadcastInterviewUpdated(interview)

		// Rating=4 (Ishga qabul qilindi) — rezume statusini accepted ga o'tkazish
		if body.Rating == 4 {
			updateRezumeStatus(interview.RezumeID, "accepted")
			broadcastRezumeStatusUpdate(interview.RezumeID, "accepted", "")
		}

		// Rating=3 (Qabul qilinmadi) — 2 ta rejected bo'lsa auto-rejected
		if body.Rating == 3 {
			rejectedCount := countRejectedInterviews(interview.RezumeID)
			if rejectedCount >= 2 {
				updateRezumeStatus(interview.RezumeID, "rejected")
				broadcastRezumeStatusUpdate(interview.RezumeID, "rejected", "")
			}
		}
	}
}

// POST /api/interviews/{id}/reschedule — intervyu vaqtini/sanasini o'zgartirish
func handleRescheduleInterview(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		jsonError(w, "Noto'g'ri ID", http.StatusBadRequest)
		return
	}

	existing, err := dbGetInterviewByID(id)
	if err != nil {
		jsonError(w, "Interview topilmadi", http.StatusNotFound)
		return
	}

	var body struct {
		InterviewDate string `json:"interview_date"`
		InterviewTime string `json:"interview_time"`
		BranchID      int64  `json:"branch_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonError(w, "JSON xato", http.StatusBadRequest)
		return
	}
	if body.InterviewDate == "" || body.InterviewTime == "" {
		jsonError(w, "Sana va vaqt kerak", http.StatusBadRequest)
		return
	}

	branchID := body.BranchID
	if branchID == 0 {
		branchID = existing.BranchID
	}

	if err := dbRescheduleInterview(id, body.InterviewDate, body.InterviewTime, branchID); err != nil {
		jsonError(w, "Yangilashda xato", http.StatusInternalServerError)
		return
	}

	interview, _ := dbGetInterviewByID(id)
	if interview != nil {
		broadcastInterviewUpdated(interview)
	}
	jsonResponse(w, interview)
}

// DELETE /api/interviews/{id} — intervyuni o'chirish
func handleDeleteInterview(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		jsonError(w, "Noto'g'ri ID", http.StatusBadRequest)
		return
	}

	existing, err := dbGetInterviewByID(id)
	if err != nil {
		jsonError(w, "Interview topilmadi", http.StatusNotFound)
		return
	}

	if err := dbDeleteInterview(id); err != nil {
		jsonError(w, "O'chirishda xato", http.StatusInternalServerError)
		return
	}

	// WebSocket orqali xabar
	broadcastInterviewDeleted(existing.ID, existing.RezumeID)

	// Agar rezumening boshqa aktiv intervyulari bo'lmasa — statusni pending ga qaytarish
	remaining := countRemainingActiveInterviews(existing.RezumeID)
	if remaining == 0 {
		updateRezumeStatus(existing.RezumeID, "pending")
		broadcastRezumeStatusUpdate(existing.RezumeID, "pending", "")
	}

	jsonResponse(w, map[string]string{"status": "deleted"})
}

// POST /api/interviews/{id}/send-location — intervyu lokatsiyasini telegramga yuborish
func handleSendInterviewLocation(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		jsonError(w, "Noto'g'ri ID", http.StatusBadRequest)
		return
	}

	interview, err := dbGetInterviewByID(id)
	if err != nil {
		jsonError(w, "Interview topilmadi", http.StatusNotFound)
		return
	}

	// Rezumeni olish (tg_user_id kerak)
	rezume, err := getRezumeByID(interview.RezumeID)
	if err != nil {
		jsonError(w, "Rezume topilmadi", http.StatusNotFound)
		return
	}

	if rezume.TgUserID == 0 {
		jsonError(w, "Foydalanuvchining Telegram ID si yo'q", http.StatusBadRequest)
		return
	}

	if interview.BranchLat == 0 && interview.BranchLng == 0 {
		jsonError(w, "Filialning lokatsiyasi belgilanmagan", http.StatusBadRequest)
		return
	}

	// Xabar yuborish
	msg := fmt.Sprintf(
		"📍 Uchrashuv joyi: %s\n\nSana: %s\nVaqt: %s",
		interview.BranchName, interview.InterviewDate, interview.InterviewTime,
	)
	sendTgMessage(rezume.TgUserID, msg)
	sendTgLocation(rezume.TgUserID, interview.BranchLat, interview.BranchLng)

	jsonResponse(w, map[string]string{"status": "sent"})
}
