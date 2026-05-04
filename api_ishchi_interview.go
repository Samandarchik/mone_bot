package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"time"
)

// computeNextIshchiInterviewSlot — admin va sanaga bo'sh slotni topadi (rezume va ishchi alohida).
func computeNextIshchiInterviewSlot(invitedByID int64, date string) (string, error) {
	rows, err := db.Query(
		"SELECT interview_time FROM ishchi_interviews WHERE invited_by_id = ? AND interview_date = ?",
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
		return "", fmt.Errorf("bu kunga 49 tadan ortiq intervyu olib bo'lmaydi")
	}

	for k := 0; k < interviewMaxPerDay; k++ {
		totalMin := interviewStartHour*60 + k*interviewSlotMinutes
		candidate := fmt.Sprintf("%02d:%02d", totalMin/60, totalMin%60)
		if !used[candidate] {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("bu kunga 49 tadan ortiq intervyu olib bo'lmaydi")
}

// POST /api/ishchi-interviews — ishchi anketani intervyuga chaqirish
func handleCreateIshchiInterview(w http.ResponseWriter, r *http.Request) {
	user := getUserFromCtx(r)
	if user.Role != "ishchi_admin" && user.Role != "super_admin" {
		jsonError(w, "Ruxsat yo'q", http.StatusForbidden)
		return
	}
	if !user.CanInterview && user.Role != "super_admin" {
		jsonError(w, "Intervyuga chaqirish huquqingiz yo'q", http.StatusForbidden)
		return
	}

	var body struct {
		IshchiID      int64  `json:"ishchi_id"`
		InterviewDate string `json:"interview_date"`
		InterviewTime string `json:"interview_time"`
		BranchID      int64  `json:"branch_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonError(w, "JSON xato", http.StatusBadRequest)
		return
	}
	if body.IshchiID == 0 || body.InterviewDate == "" || body.InterviewTime == "" {
		jsonError(w, "ishchi_id, interview_date va interview_time kerak", http.StatusBadRequest)
		return
	}

	ishchi, err := getIshchiAnketaByID(body.IshchiID)
	if err != nil {
		jsonError(w, "Ishchi anketa topilmadi", http.StatusNotFound)
		return
	}

	// Avvalgi bahosiz (rating=0) chaqiriqlarni tozalash
	db.Exec("DELETE FROM ishchi_interviews WHERE ishchi_id = ? AND rating = 0", body.IshchiID)

	// Maks 4 marta chaqirish
	var cnt int
	if err := db.QueryRow("SELECT COUNT(*) FROM ishchi_interviews WHERE ishchi_id = ?", body.IshchiID).Scan(&cnt); err != nil {
		jsonError(w, "DB xato: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if cnt >= interviewMaxPerRezume {
		jsonError(w, "Ushbu nomzodni 4 martadan ortiq intervyuga chaqirib bo'lmaydi", http.StatusBadRequest)
		return
	}

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

	updateIshchiStatus(body.IshchiID, "interviewing")

	id, err := dbCreateIshchiInterview(body.IshchiID, user.ID, body.InterviewDate, body.InterviewTime, body.BranchID)
	if err != nil {
		jsonError(w, "Interview yaratishda xato: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if ishchi.TgUserID != 0 {
		msg := fmt.Sprintf(
			"Assalomu alaykum, %s!\n\nSiz intervyuga taklif qilinyapsiz.\n\nSana: %s\nVaqt: %s\n",
			ishchi.FIO, body.InterviewDate, body.InterviewTime,
		)
		if branchName != "" {
			msg += fmt.Sprintf("Filial: %s\n", branchName)
		}
		msg += fmt.Sprintf("\nIltimos, o'z vaqtida keling.\nChaqirgan: %s", user.FullName)
		sendIshchiTgMessage(ishchi.TgUserID, msg)

		if branchLat != 0 && branchLng != 0 {
			sendIshchiTgLocation(ishchi.TgUserID, branchLat, branchLng)
		}
	}

	interview, _ := dbGetIshchiInterviewByID(id)
	jsonResponse(w, interview)

	if interview != nil {
		broadcastIshchiInterviewCreated(interview)
		broadcastIshchiStatusUpdate(body.IshchiID, "interviewing", user.FullName)
	}
}

// GET /api/ishchi-interviews
func handleGetIshchiInterviews(w http.ResponseWriter, r *http.Request) {
	user := getUserFromCtx(r)
	if user.Role == "admin" {
		jsonError(w, "Ruxsat yo'q", http.StatusForbidden)
		return
	}

	ishchiID, _ := strconv.ParseInt(r.URL.Query().Get("ishchi_id"), 10, 64)
	invitedByID, _ := strconv.ParseInt(r.URL.Query().Get("invited_by_id"), 10, 64)
	if user.Role != "super_admin" {
		invitedByID = user.ID
	}
	date := r.URL.Query().Get("date")
	rating := -1
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

	interviews, total, err := dbGetIshchiInterviews(ishchiID, rating, invitedByID, date, page, limit)
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

// GET /api/ishchi-interviews/overdue
func handleGetIshchiOverdueInterviews(w http.ResponseWriter, r *http.Request) {
	user := getUserFromCtx(r)
	if user.Role == "admin" {
		jsonError(w, "Ruxsat yo'q", http.StatusForbidden)
		return
	}

	invitedByID := int64(0)
	if user.Role != "super_admin" {
		invitedByID = user.ID
	}

	interviews, _, err := dbGetIshchiInterviews(0, 0, invitedByID, "", 1, 1000)
	if err != nil {
		jsonError(w, "DB xato: "+err.Error(), http.StatusInternalServerError)
		return
	}

	now := time.Now()
	overdue := make([]IshchiInterviewRow, 0, len(interviews))
	for _, iv := range interviews {
		dt, err := parseInterviewDateTime(iv.InterviewDate, iv.InterviewTime)
		if err != nil {
			continue
		}
		if dt.Before(now) {
			overdue = append(overdue, iv)
		}
	}

	sort.Slice(overdue, func(i, j int) bool {
		di, _ := parseInterviewDateTime(overdue[i].InterviewDate, overdue[i].InterviewTime)
		dj, _ := parseInterviewDateTime(overdue[j].InterviewDate, overdue[j].InterviewTime)
		return di.Before(dj)
	})

	jsonResponse(w, overdue)
}

// GET /api/ishchi-interviews/{id}
func handleGetIshchiInterview(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		jsonError(w, "Noto'g'ri ID", http.StatusBadRequest)
		return
	}
	interview, err := dbGetIshchiInterviewByID(id)
	if err != nil {
		jsonError(w, "Interview topilmadi", http.StatusNotFound)
		return
	}
	jsonResponse(w, interview)
}

// PATCH /api/ishchi-interviews/{id}
func handleUpdateIshchiInterview(w http.ResponseWriter, r *http.Request) {
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
		Rating    int    `json:"rating"`
		Comment   string `json:"comment"`
		VoiceData string `json:"voice_data"`
		VoiceExt  string `json:"voice_ext"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonError(w, "JSON xato", http.StatusBadRequest)
		return
	}

	if body.Rating < 1 || body.Rating > 5 {
		jsonError(w, "Rating 1-5 bo'lishi kerak", http.StatusBadRequest)
		return
	}
	if body.VoiceData == "" {
		jsonError(w, "Ovozli izoh yuborilishi majburiy", http.StatusBadRequest)
		return
	}

	ext := body.VoiceExt
	if ext == "" {
		ext = "m4a"
	}
	voiceUrl, err := saveVoice(body.VoiceData, id, ext)
	if err != nil {
		jsonError(w, "Ovozni saqlashda xato: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if err := dbUpdateIshchiInterview(id, body.Rating, body.Comment, voiceUrl); err != nil {
		jsonError(w, "Yangilashda xato", http.StatusInternalServerError)
		return
	}

	interview, _ := dbGetIshchiInterviewByID(id)
	jsonResponse(w, interview)

	if interview != nil {
		broadcastIshchiInterviewUpdated(interview)

		newStatus := ratingToStatus(body.Rating)
		if newStatus != "" {
			adminName := user.FullName
			if adminName == "" {
				adminName = user.Username
			}
			updateIshchiStatusWithAdmin(interview.IshchiID, newStatus, user.ID, adminName)
			broadcastIshchiStatusUpdate(interview.IshchiID, newStatus, adminName)
		}
	}
}

// POST /api/ishchi-interviews/{id}/reschedule
func handleRescheduleIshchiInterview(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		jsonError(w, "Noto'g'ri ID", http.StatusBadRequest)
		return
	}

	existing, err := dbGetIshchiInterviewByID(id)
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

	if err := dbRescheduleIshchiInterview(id, body.InterviewDate, body.InterviewTime, branchID); err != nil {
		jsonError(w, "Yangilashda xato", http.StatusInternalServerError)
		return
	}

	interview, _ := dbGetIshchiInterviewByID(id)
	if interview != nil {
		broadcastIshchiInterviewUpdated(interview)
	}
	jsonResponse(w, interview)
}

// DELETE /api/ishchi-interviews/{id}
func handleDeleteIshchiInterview(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		jsonError(w, "Noto'g'ri ID", http.StatusBadRequest)
		return
	}

	existing, err := dbGetIshchiInterviewByID(id)
	if err != nil {
		jsonError(w, "Interview topilmadi", http.StatusNotFound)
		return
	}

	if err := dbDeleteIshchiInterview(id); err != nil {
		jsonError(w, "O'chirishda xato", http.StatusInternalServerError)
		return
	}

	broadcastIshchiInterviewDeleted(existing.ID, existing.IshchiID)

	remaining := countRemainingActiveIshchiInterviews(existing.IshchiID)
	if remaining == 0 {
		updateIshchiStatus(existing.IshchiID, "pending")
		broadcastIshchiStatusUpdate(existing.IshchiID, "pending", "")
	}

	jsonResponse(w, map[string]string{"status": "deleted"})
}

// POST /api/ishchi-interviews/{id}/send-location
func handleSendIshchiInterviewLocation(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		jsonError(w, "Noto'g'ri ID", http.StatusBadRequest)
		return
	}

	interview, err := dbGetIshchiInterviewByID(id)
	if err != nil {
		jsonError(w, "Interview topilmadi", http.StatusNotFound)
		return
	}

	ishchi, err := getIshchiAnketaByID(interview.IshchiID)
	if err != nil {
		jsonError(w, "Ishchi topilmadi", http.StatusNotFound)
		return
	}

	if ishchi.TgUserID == 0 {
		jsonError(w, "Foydalanuvchining Telegram ID si yo'q", http.StatusBadRequest)
		return
	}

	if interview.BranchLat == 0 && interview.BranchLng == 0 {
		jsonError(w, "Filialning lokatsiyasi belgilanmagan", http.StatusBadRequest)
		return
	}

	msg := fmt.Sprintf(
		"📍 Uchrashuv joyi: %s\n\nSana: %s\nVaqt: %s",
		interview.BranchName, interview.InterviewDate, interview.InterviewTime,
	)
	sendIshchiTgMessage(ishchi.TgUserID, msg)
	sendIshchiTgLocation(ishchi.TgUserID, interview.BranchLat, interview.BranchLng)

	jsonResponse(w, map[string]string{"status": "sent"})
}
