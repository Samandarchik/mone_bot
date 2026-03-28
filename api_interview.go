package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
)

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
		InterviewTime string `json:"interview_time"`
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

	// Rezume statusini qabul qilish
	updateRezumeStatus(body.RezumeID, "qabul")

	// Interview yaratish
	id, err := dbCreateInterview(body.RezumeID, user.ID, body.InterviewDate, body.InterviewTime)
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
				"Vaqt: %s\n\n"+
				"Iltimos, o'z vaqtida keling.\n"+
				"Chaqirgan: %s",
			fio, body.InterviewDate, body.InterviewTime, user.FullName,
		)
		sendTgMessage(rezume.TgUserID, msg)
	}

	interview, _ := dbGetInterviewByID(id)
	jsonResponse(w, interview)
}

// GET /api/interviews?rezume_id=1&rating=0&page=1&limit=20
func handleGetInterviews(w http.ResponseWriter, r *http.Request) {
	rezumeID, _ := strconv.ParseInt(r.URL.Query().Get("rezume_id"), 10, 64)
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

	interviews, total, err := dbGetInterviews(rezumeID, rating, page, limit)
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

// PATCH /api/interviews/{id} — intervyu natijasini qo'yish (rating + comment)
func handleUpdateInterview(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		jsonError(w, "Noto'g'ri ID", http.StatusBadRequest)
		return
	}

	var body struct {
		Rating  int    `json:"rating"`
		Comment string `json:"comment"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonError(w, "JSON xato", http.StatusBadRequest)
		return
	}

	if body.Rating < 1 || body.Rating > 3 {
		jsonError(w, "Rating 1, 2 yoki 3 bo'lishi kerak. 1=Zo'r, 2=Qoniqarli, 3=Qabul qilinmadi", http.StatusBadRequest)
		return
	}

	if err := dbUpdateInterview(id, body.Rating, body.Comment); err != nil {
		jsonError(w, "Yangilashda xato", http.StatusInternalServerError)
		return
	}

	interview, _ := dbGetInterviewByID(id)
	jsonResponse(w, interview)
}

