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

	// Bazaga saqlash
	id, err := saveIshchiAnketa(&anketa, rasmURL)
	if err != nil {
		log.Printf("Ishchi anketa DB saqlashda xato: %v", err)
	} else {
		log.Printf("Ishchi anketa DB ga saqlandi: id=%d", id)
	}

	// Telegram caption
	tg2 := ""
	if anketa.TgUsername2 != "" {
		tg2 = "\nTelegram 2: @" + anketa.TgUsername2
	}
	caption := fmt.Sprintf(
		"Вакансия: %s\n"+
			"ФИО: %s\n"+
			"Дата рождения: %s\n"+
			"Рост: %d см\n"+
			"Вес: %d кг\n"+
			"Адрес: %s\n"+
			"Семейный статус: %s\n"+
			"Дети: %s\n"+
			"Знания языка: %s\n"+
			"Образования: %s\n"+
			"График: %s\n"+
			"Судимость: %s\n"+
			"Водительские удостоверения: %s\n"+
			"Телефон: %s%s\n"+
			"━━━━━━━━━━━━━━━━━━━━",
		anketa.Vakansiya, anketa.FIO, anketa.TugilganSana,
		anketa.BoySm, anketa.VaznKg,
		anketa.Manzil, anketa.OilaviyHolat, anketa.Bolalar,
		anketa.Tillar, anketa.Malumot, anketa.Grafik,
		anketa.Sudimlik, anketa.Haydovchilik, anketa.Telefon, tg2,
	)

	// Admin TG ga yuborish (ishchi bot orqali)
	var tgErr error
	if anketa.Rasm != "" && strings.Contains(anketa.Rasm, ",") {
		tgErr = sendIshchiPhotoToTelegram(adminTgID, anketa.Rasm, caption)
	} else {
		sendIshchiTgMessage(adminTgID, caption)
	}

	if tgErr != nil {
		log.Printf("Ishchi Telegram xato: %v", tgErr)
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
}

// GET /api/ishchi-anketalar
func handleGetIshchiAnketalar(w http.ResponseWriter, r *http.Request) {
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

	anketalar, total, err := getIshchiAnketalar(vakansiya, status, search, page, limit)
	if err != nil {
		jsonError(w, "DB xato: "+err.Error(), http.StatusInternalServerError)
		return
	}

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
}
