package main

import (
	"encoding/json"
	"net/http"
	"strconv"
)

// POST /api/users — yangi foydalanuvchi yaratish (super_admin)
func handleCreateUser(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Username          string  `json:"username"`
		Password          string  `json:"password"`
		FullName          string  `json:"full_name"`
		Role              string  `json:"role"`
		CanInterview      bool    `json:"can_interview"`
		CategoryIDs       []int64 `json:"category_ids"`
		IshchiCategoryIDs []int64 `json:"ishchi_category_ids"`
		BranchID          int64   `json:"branch_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonError(w, "JSON xato", http.StatusBadRequest)
		return
	}

	if body.Username == "" || body.Password == "" {
		jsonError(w, "Username va password kerak", http.StatusBadRequest)
		return
	}
	if body.Role == "" {
		body.Role = "admin"
	}
	validRoles := map[string]bool{"admin": true, "super_admin": true, "ishchi_admin": true}
	if !validRoles[body.Role] {
		jsonError(w, "Noto'g'ri role. Mumkin: admin, super_admin, ishchi_admin", http.StatusBadRequest)
		return
	}

	hash, err := hashPassword(body.Password)
	if err != nil {
		jsonError(w, "Parol xatolik", http.StatusInternalServerError)
		return
	}

	id, err := dbCreateUser(body.Username, hash, body.FullName, body.Role, body.CanInterview, body.BranchID)
	if err != nil {
		jsonError(w, "Foydalanuvchi yaratishda xato: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Role-ga qarab kategoriya saqlash:
	// admin → user_categories (rezume kategoriyalari)
	// ishchi_admin → user_ishchi_categories (ishchi kategoriyalari)
	if body.Role == "admin" && len(body.CategoryIDs) > 0 {
		dbSetUserCategories(id, body.CategoryIDs)
	}
	if body.Role == "ishchi_admin" && len(body.IshchiCategoryIDs) > 0 {
		dbSetUserIshchiCategories(id, body.IshchiCategoryIDs)
	}

	resp, _ := dbGetUserByID(id)
	jsonResponse(w, resp)
}

// GET /api/users — barcha foydalanuvchilar (super_admin)
func handleGetUsers(w http.ResponseWriter, r *http.Request) {
	users, err := dbGetUsers()
	if err != nil {
		jsonError(w, "DB xato: "+err.Error(), http.StatusInternalServerError)
		return
	}
	jsonResponse(w, users)
}

// GET /api/users/{id} — bitta foydalanuvchi (super_admin)
func handleGetUserAPI(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		jsonError(w, "Noto'g'ri ID", http.StatusBadRequest)
		return
	}

	user, err := dbGetUserByID(id)
	if err != nil {
		jsonError(w, "Foydalanuvchi topilmadi", http.StatusNotFound)
		return
	}

	jsonResponse(w, user)
}

// PATCH /api/users/{id} — foydalanuvchini yangilash (super_admin)
func handleUpdateUser(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		jsonError(w, "Noto'g'ri ID", http.StatusBadRequest)
		return
	}

	existing, err := dbGetUserByID(id)
	if err != nil {
		jsonError(w, "Foydalanuvchi topilmadi", http.StatusNotFound)
		return
	}

	var body struct {
		FullName          *string `json:"full_name"`
		Role              *string `json:"role"`
		CanInterview      *bool   `json:"can_interview"`
		IsActive          *bool   `json:"is_active"`
		Password          *string `json:"password"`
		CategoryIDs       []int64 `json:"category_ids"`
		IshchiCategoryIDs []int64 `json:"ishchi_category_ids"`
		BranchID          *int64  `json:"branch_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonError(w, "JSON xato", http.StatusBadRequest)
		return
	}

	fullName := existing.FullName
	role := existing.Role
	canInterview := existing.CanInterview
	isActive := existing.IsActive
	branchID := existing.BranchID

	if body.FullName != nil {
		fullName = *body.FullName
	}
	if body.Role != nil {
		validRoles := map[string]bool{"admin": true, "super_admin": true, "ishchi_admin": true}
		if !validRoles[*body.Role] {
			jsonError(w, "Noto'g'ri role", http.StatusBadRequest)
			return
		}
		role = *body.Role
	}
	if body.CanInterview != nil {
		canInterview = *body.CanInterview
	}
	if body.IsActive != nil {
		isActive = *body.IsActive
	}
	if body.BranchID != nil {
		branchID = *body.BranchID
	}

	if err := dbUpdateUser(id, fullName, role, canInterview, isActive, branchID); err != nil {
		jsonError(w, "Yangilashda xato", http.StatusInternalServerError)
		return
	}

	if body.Password != nil && *body.Password != "" {
		hash, _ := hashPassword(*body.Password)
		dbUpdateUserPassword(id, hash)
	}

	// Role o'zgartirilganda, eski rol kategoriyalarini tozalaymiz
	if body.Role != nil {
		if role != "admin" {
			dbSetUserCategories(id, []int64{})
		}
		if role != "ishchi_admin" {
			dbSetUserIshchiCategories(id, []int64{})
		}
	}

	if body.CategoryIDs != nil && role == "admin" {
		dbSetUserCategories(id, body.CategoryIDs)
	}
	if body.IshchiCategoryIDs != nil && role == "ishchi_admin" {
		dbSetUserIshchiCategories(id, body.IshchiCategoryIDs)
	}

	resp, _ := dbGetUserByID(id)
	jsonResponse(w, resp)
}

// DELETE /api/users/{id} — foydalanuvchini o'chirish (super_admin)
func handleDeleteUser(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		jsonError(w, "Noto'g'ri ID", http.StatusBadRequest)
		return
	}

	if err := dbDeleteUser(id); err != nil {
		jsonError(w, "O'chirishda xato", http.StatusInternalServerError)
		return
	}

	jsonResponse(w, map[string]string{"status": "deleted"})
}
