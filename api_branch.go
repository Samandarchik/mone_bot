package main

import (
	"encoding/json"
	"net/http"
	"strconv"
)

// POST /api/branches — yangi filial yaratish (super_admin)
func handleCreateBranch(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonError(w, "JSON xato", http.StatusBadRequest)
		return
	}
	if body.Name == "" {
		jsonError(w, "Filial nomi kerak", http.StatusBadRequest)
		return
	}

	id, err := dbCreateBranch(body.Name)
	if err != nil {
		jsonError(w, "Filial yaratishda xato: "+err.Error(), http.StatusBadRequest)
		return
	}

	branch, _ := dbGetBranchByID(id)
	jsonResponse(w, branch)
}

// GET /api/branches — barcha filiallar
func handleGetBranches(w http.ResponseWriter, r *http.Request) {
	branches, err := dbGetBranches()
	if err != nil {
		jsonError(w, "DB xato: "+err.Error(), http.StatusInternalServerError)
		return
	}
	jsonResponse(w, branches)
}

// PATCH /api/branches/{id} — filialni yangilash (super_admin)
func handleUpdateBranch(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		jsonError(w, "Noto'g'ri ID", http.StatusBadRequest)
		return
	}

	existing, err := dbGetBranchByID(id)
	if err != nil {
		jsonError(w, "Filial topilmadi", http.StatusNotFound)
		return
	}

	var body struct {
		Name     *string `json:"name"`
		IsActive *bool   `json:"is_active"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonError(w, "JSON xato", http.StatusBadRequest)
		return
	}

	name := existing.Name
	isActive := existing.IsActive

	if body.Name != nil {
		name = *body.Name
	}
	if body.IsActive != nil {
		isActive = *body.IsActive
	}

	if err := dbUpdateBranch(id, name, isActive); err != nil {
		jsonError(w, "Yangilashda xato: "+err.Error(), http.StatusInternalServerError)
		return
	}

	branch, _ := dbGetBranchByID(id)
	jsonResponse(w, branch)
}

// DELETE /api/branches/{id} — filialni o'chirish (super_admin)
func handleDeleteBranch(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		jsonError(w, "Noto'g'ri ID", http.StatusBadRequest)
		return
	}

	if err := dbDeleteBranch(id); err != nil {
		jsonError(w, "O'chirishda xato", http.StatusInternalServerError)
		return
	}

	jsonResponse(w, map[string]string{"status": "deleted"})
}
