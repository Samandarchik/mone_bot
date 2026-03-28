package main

import (
	"encoding/json"
	"net/http"
	"strconv"
)

// POST /api/categories — yangi kategoriya yaratish (super_admin)
func handleCreateCategory(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name      string `json:"name"`
		TgGroupID int64  `json:"tg_group_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonError(w, "JSON xato", http.StatusBadRequest)
		return
	}
	if body.Name == "" {
		jsonError(w, "Kategoriya nomi kerak", http.StatusBadRequest)
		return
	}

	id, err := dbCreateCategory(body.Name, body.TgGroupID)
	if err != nil {
		jsonError(w, "Kategoriya yaratishda xato: "+err.Error(), http.StatusBadRequest)
		return
	}

	cat, _ := dbGetCategoryByID(id)
	jsonResponse(w, cat)
}

// GET /api/categories — barcha kategoriyalar
func handleGetCategories(w http.ResponseWriter, r *http.Request) {
	cats, err := dbGetCategories()
	if err != nil {
		jsonError(w, "DB xato: "+err.Error(), http.StatusInternalServerError)
		return
	}
	jsonResponse(w, cats)
}

// PATCH /api/categories/{id} — kategoriyani yangilash (super_admin)
func handleUpdateCategory(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		jsonError(w, "Noto'g'ri ID", http.StatusBadRequest)
		return
	}

	existing, err := dbGetCategoryByID(id)
	if err != nil {
		jsonError(w, "Kategoriya topilmadi", http.StatusNotFound)
		return
	}

	var body struct {
		Name      *string `json:"name"`
		TgGroupID *int64  `json:"tg_group_id"`
		IsActive  *bool   `json:"is_active"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonError(w, "JSON xato", http.StatusBadRequest)
		return
	}

	name := existing.Name
	tgGroupID := existing.TgGroupID
	isActive := existing.IsActive

	if body.Name != nil {
		name = *body.Name
	}
	if body.TgGroupID != nil {
		tgGroupID = *body.TgGroupID
	}
	if body.IsActive != nil {
		isActive = *body.IsActive
	}

	if err := dbUpdateCategory(id, name, tgGroupID, isActive); err != nil {
		jsonError(w, "Yangilashda xato: "+err.Error(), http.StatusInternalServerError)
		return
	}

	cat, _ := dbGetCategoryByID(id)
	jsonResponse(w, cat)
}

// DELETE /api/categories/{id} — kategoriyani o'chirish (super_admin)
func handleDeleteCategory(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		jsonError(w, "Noto'g'ri ID", http.StatusBadRequest)
		return
	}

	if err := dbDeleteCategory(id); err != nil {
		jsonError(w, "O'chirishda xato", http.StatusInternalServerError)
		return
	}

	jsonResponse(w, map[string]string{"status": "deleted"})
}
