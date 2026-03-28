package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

type contextKey string

const userCtxKey contextKey = "user"

func hashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(bytes), err
}

func checkPassword(password, hash string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

func generateToken() string {
	b := make([]byte, 32)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func getUserFromCtx(r *http.Request) *UserRow {
	if u, ok := r.Context().Value(userCtxKey).(*UserRow); ok {
		return u
	}
	return nil
}

// authRequired — faqat avtorizatsiya qilingan foydalanuvchilar uchun
func authRequired(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := r.Header.Get("Authorization")
		token = strings.TrimPrefix(token, "Bearer ")
		if token == "" {
			jsonError(w, "Avtorizatsiya talab qilinadi", http.StatusUnauthorized)
			return
		}
		user, err := dbGetUserByToken(token)
		if err != nil || !user.IsActive {
			jsonError(w, "Token yaroqsiz", http.StatusUnauthorized)
			return
		}
		ctx := context.WithValue(r.Context(), userCtxKey, user)
		next(w, r.WithContext(ctx))
	}
}

// superAdminRequired — faqat super_admin uchun
func superAdminRequired(next http.HandlerFunc) http.HandlerFunc {
	return authRequired(func(w http.ResponseWriter, r *http.Request) {
		user := getUserFromCtx(r)
		if user.Role != "super_admin" {
			jsonError(w, "Super admin ruxsati kerak", http.StatusForbidden)
			return
		}
		next(w, r)
	})
}

// POST /api/auth/login
func handleLogin(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonError(w, "JSON xato", http.StatusBadRequest)
		return
	}
	if body.Username == "" || body.Password == "" {
		jsonError(w, "Username va password kerak", http.StatusBadRequest)
		return
	}

	user, passwordHash, err := dbGetUserByUsername(body.Username)
	if err != nil {
		jsonError(w, "Login yoki parol xato", http.StatusUnauthorized)
		return
	}
	if !user.IsActive {
		jsonError(w, "Foydalanuvchi bloklangan", http.StatusForbidden)
		return
	}
	if !checkPassword(body.Password, passwordHash) {
		jsonError(w, "Login yoki parol xato", http.StatusUnauthorized)
		return
	}

	token := generateToken()
	dbCreateSession(token, user.ID)

	cats := getUserCategories(user.ID)
	branch := getBranchPtr(user.BranchID)
	jsonResponse(w, map[string]interface{}{
		"token": token,
		"user": UserResponse{
			UserRow:    *user,
			Categories: cats,
			Branch:     branch,
		},
	})
}

// POST /api/auth/logout
func handleLogout(w http.ResponseWriter, r *http.Request) {
	token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	dbDeleteSession(token)
	jsonResponse(w, map[string]string{"status": "ok"})
}

// GET /api/auth/me
func handleMe(w http.ResponseWriter, r *http.Request) {
	user := getUserFromCtx(r)
	resp, err := dbGetUserByID(user.ID)
	if err != nil {
		jsonError(w, "Foydalanuvchi topilmadi", http.StatusNotFound)
		return
	}
	jsonResponse(w, resp)
}
