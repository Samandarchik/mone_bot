package main

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
)

// POST /api/users — yangi foydalanuvchi yaratish (super_admin)
func handleCreateUser(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Username          string   `json:"username"`
		Password          string   `json:"password"`
		FullName          string   `json:"full_name"`
		Role              string   `json:"role"`
		Roles             []string `json:"roles"`
		CanInterview      bool     `json:"can_interview"`
		CategoryIDs       []int64  `json:"category_ids"`
		IshchiCategoryIDs []int64  `json:"ishchi_category_ids"`
		BranchID          int64    `json:"branch_id"`
		RasmUrl           string   `json:"rasm_url"`
		Telefon           string   `json:"telefon"`
		RezumeID          int64    `json:"rezume_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonError(w, "JSON xato", http.StatusBadRequest)
		return
	}

	if body.Username == "" || body.Password == "" {
		jsonError(w, "Username va password kerak", http.StatusBadRequest)
		return
	}

	// Rollar: yangi `roles` ro'yxati afzal; bo'lmasa eski bitta `role`.
	// Bitta user bir vaqtda ham admin, ham ishchi_admin bo'lishi mumkin.
	incoming := body.Roles
	if len(incoming) == 0 && body.Role != "" {
		incoming = []string{body.Role}
	}
	for _, rr := range incoming {
		if rr != "" && !validRole(rr) {
			jsonError(w, "Noto'g'ri role. Mumkin: admin, ishchi_admin, super_admin", http.StatusBadRequest)
			return
		}
	}
	rolesList := normalizeRoles(incoming, "admin")
	role := primaryRole(rolesList)
	rolesCSV := strings.Join(rolesList, ",")

	id, err := dbCreateUser(body.Username, body.Password, body.FullName, role, rolesCSV, body.CanInterview, body.BranchID, body.RasmUrl, body.Telefon, body.RezumeID)
	if err != nil {
		jsonError(w, "Foydalanuvchi yaratishda xato: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Rollarga qarab kategoriya saqlash. Bitta userda ikkala rol bo'lsa
	// ikkala kategoriya to'plami ham saqlanadi.
	if contains(rolesList, "admin") && len(body.CategoryIDs) > 0 {
		dbSetUserCategories(id, body.CategoryIDs)
	}
	if contains(rolesList, "ishchi_admin") && len(body.IshchiCategoryIDs) > 0 {
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

// PATCH /api/users/reorder — ro'yxatdagi userlarni qo'lda surib (drag)
// o'rnatilgan tartibni saqlash (super_admin). Body: {"ids": [3, 1, 5, ...]}
func handleReorderUsers(w http.ResponseWriter, r *http.Request) {
	var body struct {
		IDs []int64 `json:"ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonError(w, "Noto'g'ri so'rov", http.StatusBadRequest)
		return
	}
	if len(body.IDs) == 0 {
		jsonError(w, "ids bo'sh", http.StatusBadRequest)
		return
	}
	if err := dbReorderUsers(body.IDs); err != nil {
		jsonError(w, "DB xato: "+err.Error(), http.StatusInternalServerError)
		return
	}
	jsonResponse(w, map[string]bool{"ok": true})
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
		FullName          *string  `json:"full_name"`
		Role              *string  `json:"role"`
		Roles             []string `json:"roles"`
		CanInterview      *bool    `json:"can_interview"`
		IsActive          *bool    `json:"is_active"`
		Password          *string  `json:"password"`
		CategoryIDs       []int64  `json:"category_ids"`
		IshchiCategoryIDs []int64  `json:"ishchi_category_ids"`
		BranchID          *int64   `json:"branch_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonError(w, "JSON xato", http.StatusBadRequest)
		return
	}

	fullName := existing.FullName
	canInterview := existing.CanInterview
	isActive := existing.IsActive
	branchID := existing.BranchID

	if body.FullName != nil {
		fullName = *body.FullName
	}

	// Rollar: `roles` ro'yxati yoki eski `role` berilgan bo'lsa yangilaymiz,
	// aks holda mavjud rollar saqlanadi (backward compatible).
	rolesProvided := body.Roles != nil || body.Role != nil
	rolesList := existing.effectiveRoles()
	if rolesProvided {
		incoming := body.Roles
		if len(incoming) == 0 && body.Role != nil {
			incoming = []string{*body.Role}
		}
		for _, rr := range incoming {
			if rr != "" && !validRole(rr) {
				jsonError(w, "Noto'g'ri role", http.StatusBadRequest)
				return
			}
		}
		rolesList = normalizeRoles(incoming, "admin")
	}
	role := primaryRole(rolesList)
	rolesCSV := strings.Join(rolesList, ",")

	if body.CanInterview != nil {
		canInterview = *body.CanInterview
	}
	if body.IsActive != nil {
		isActive = *body.IsActive
	}
	if body.BranchID != nil {
		branchID = *body.BranchID
	}

	if err := dbUpdateUser(id, fullName, role, rolesCSV, canInterview, isActive, branchID); err != nil {
		jsonError(w, "Yangilashda xato", http.StatusInternalServerError)
		return
	}

	if body.Password != nil && *body.Password != "" {
		dbUpdateUserPassword(id, *body.Password)
	}

	hasAdmin := contains(rolesList, "admin")
	hasIshchi := contains(rolesList, "ishchi_admin")

	// Rollar yangilanganda, endi tegishli bo'lmagan rol kategoriyalarini tozalaymiz.
	if rolesProvided {
		if !hasAdmin {
			dbSetUserCategories(id, []int64{})
		}
		if !hasIshchi {
			dbSetUserIshchiCategories(id, []int64{})
		}
	}

	if body.CategoryIDs != nil && hasAdmin {
		dbSetUserCategories(id, body.CategoryIDs)
	}
	if body.IshchiCategoryIDs != nil && hasIshchi {
		dbSetUserIshchiCategories(id, body.IshchiCategoryIDs)
	}

	resp, _ := dbGetUserByID(id)
	jsonResponse(w, resp)
}

// POST /api/users/{id}/link-rezume — foydalanuvchini telefon raqami orqali
// rezume bilan permanent bog'laydi. Bu user avval rezume yo'q paytda yaratilgan
// bo'lsa va keyinroq o'sha telefon bilan rezume kelgan bo'lsa, uni o'chirib
// qayta yaratmasdan bog'lash uchun ishlatiladi (intervyu va boshqa tarixlar
// saqlanib qoladi).
//
// Body ixtiyoriy: {"phone": "+998..."}. Bo'sh bo'lsa user.telefon yoki
// user.username asos qilib olinadi.
func handleLinkUserRezume(w http.ResponseWriter, r *http.Request) {
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

	var body struct {
		Phone string `json:"phone"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)

	phone := body.Phone
	if phone == "" {
		phone = user.Telefon
	}
	if phone == "" {
		phone = user.Username
	}
	phone = normalizePhone(phone)
	if phone == "" {
		jsonError(w, "Telefon raqami yo'q", http.StatusBadRequest)
		return
	}

	rezume, err := getRezumeByPhone(phone)
	if err != nil || rezume == nil {
		jsonError(w, "Bu telefon bo'yicha rezume topilmadi", http.StatusNotFound)
		return
	}

	if err := dbLinkUserToRezume(id, rezume.ID, rezume.RasmUrl, rezume.Telefon); err != nil {
		jsonError(w, "Bog'lashda xato", http.StatusInternalServerError)
		return
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
