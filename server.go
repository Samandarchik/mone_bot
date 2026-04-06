package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
)

const botToken = "8550220546:AAFEII8AzNdMapEqT_VFtqiqv6h0obbLgzQ"
const baseURL = "https://hr.monebakeryuz.uz"
const adminTgID int64 = 1066137436

// --- Telegram types ---

type TgUpdate struct {
	UpdateID      int64       `json:"update_id"`
	Message       *TgMessage  `json:"message"`
	CallbackQuery *TgCallback `json:"callback_query"`
}

type TgMessage struct {
	MessageID int64   `json:"message_id"`
	Chat      TgChat  `json:"chat"`
	From      *TgUser `json:"from"`
	Text      string  `json:"text"`
}

type TgCallback struct {
	ID      string     `json:"id"`
	From    TgUser     `json:"from"`
	Message *TgMessage `json:"message"`
	Data    string     `json:"data"`
}

type TgChat struct {
	ID int64 `json:"id"`
}

type TgUser struct {
	ID        int64  `json:"id"`
	Username  string `json:"username"`
	FirstName string `json:"first_name"`
}

// --- Data types ---

type LangInfo struct {
	Til    string `json:"til"`
	Daraja string `json:"daraja"`
}

type Anketa struct {
	Lavozim         string     `json:"lavozim"`
	Familiya        string     `json:"familiya"`
	Ism             string     `json:"ism"`
	Sharif          string     `json:"sharif"`
	TugilganSana    string     `json:"tugilgan_sana"`
	BoySm           int        `json:"boy_sm"`
	VaznKg          int        `json:"vazn_kg"`
	YashashManzili  string     `json:"yashash_manzili"`
	Moljal          string     `json:"moljal"`
	UmumiyTajriba   string     `json:"umumiy_tajriba"`
	ChetElTajribasi string     `json:"chet_el_tajribasi"`
	Malumot         string     `json:"malumot"`
	OilaviyHolat    string     `json:"oilaviy_holat"`
	Tillar          []LangInfo `json:"tillar"`
	Telefon         string     `json:"telefon"`
	Qoshimcha       string     `json:"qoshimcha"`
	Rasm            string     `json:"rasm"`
	TgUserID        int64      `json:"tg_user_id"`
	TgUsername      string     `json:"tg_username"`
	TgUsername2     string     `json:"tg_username2"`
}

type RezumeRow struct {
	ID              int64      `json:"id"`
	Lavozim         string     `json:"lavozim"`
	Familiya        string     `json:"familiya"`
	Ism             string     `json:"ism"`
	Sharif          string     `json:"sharif"`
	TugilganSana    string     `json:"tugilgan_sana"`
	BoySm           int        `json:"boy_sm"`
	VaznKg          int        `json:"vazn_kg"`
	YashashManzili  string     `json:"yashash_manzili"`
	Moljal          string     `json:"moljal"`
	UmumiyTajriba   string     `json:"umumiy_tajriba"`
	ChetElTajribasi string     `json:"chet_el_tajribasi"`
	Malumot         string     `json:"malumot"`
	OilaviyHolat    string     `json:"oilaviy_holat"`
	Tillar          []LangInfo `json:"tillar"`
	Telefon         string     `json:"telefon"`
	Qoshimcha       string     `json:"qoshimcha"`
	RasmUrl         string     `json:"rasm_url"`
	TgUserID        int64      `json:"tg_user_id"`
	TgUsername      string     `json:"tg_username"`
	TgUsername2     string     `json:"tg_username2"`
	Status          string         `json:"status"`
	StatusBy        int64          `json:"status_by"`
	StatusByName    string         `json:"status_by_name"`
	CreatedAt       string         `json:"created_at"`
	Interviews      []InterviewRow `json:"interviews"`
}

type Category struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	TgGroupID int64  `json:"tg_group_id"`
	IsActive  bool   `json:"is_active"`
	CreatedAt string `json:"created_at"`
}

type Branch struct {
	ID        int64   `json:"id"`
	Name      string  `json:"name"`
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	IsActive  bool    `json:"is_active"`
	CreatedAt string  `json:"created_at"`
}

type UserRow struct {
	ID           int64  `json:"id"`
	Username     string `json:"username"`
	FullName     string `json:"full_name"`
	Role         string `json:"role"`
	CanInterview bool   `json:"can_interview"`
	IsActive     bool   `json:"is_active"`
	BranchID     int64  `json:"branch_id"`
	CreatedAt    string `json:"created_at"`
}

type UserResponse struct {
	UserRow
	Categories []Category `json:"categories"`
	Branch     *Branch    `json:"branch"`
}

// --- Ishchi anketa types ---

type IshchiAnketa struct {
	Vakansiya    string `json:"vakansiya"`
	FIO          string `json:"fio"`
	TugilganSana string `json:"tugilgan_sana"`
	BoySm        int    `json:"boy_sm"`
	VaznKg       int    `json:"vazn_kg"`
	Manzil       string `json:"manzil"`
	Lang         string `json:"lang"`
	OilaviyHolat string `json:"oilaviy_holat"`
	Bolalar      string `json:"bolalar"`
	Tillar       string `json:"tillar"`
	Malumot      string `json:"malumot"`
	Grafik       string `json:"grafik"`
	Sudimlik     string `json:"sudimlik"`
	Haydovchilik string `json:"haydovchilik"`
	Telefon      string `json:"telefon"`
	Rasm         string `json:"rasm"`
	TgUserID     int64  `json:"tg_user_id"`
	TgUsername   string `json:"tg_username"`
	TgUsername2  string `json:"tg_username2"`
}

type IshchiRow struct {
	ID           int64  `json:"id"`
	Vakansiya    string `json:"vakansiya"`
	FIO          string `json:"fio"`
	TugilganSana string `json:"tugilgan_sana"`
	BoySm        int    `json:"boy_sm"`
	VaznKg       int    `json:"vazn_kg"`
	Manzil       string `json:"manzil"`
	Lang         string `json:"lang"`
	OilaviyHolat string `json:"oilaviy_holat"`
	Bolalar      string `json:"bolalar"`
	Tillar       string `json:"tillar"`
	Malumot      string `json:"malumot"`
	Grafik       string `json:"grafik"`
	Sudimlik     string `json:"sudimlik"`
	Haydovchilik string `json:"haydovchilik"`
	Telefon      string `json:"telefon"`
	RasmUrl      string `json:"rasm_url"`
	TgUserID     int64  `json:"tg_user_id"`
	TgUsername   string `json:"tg_username"`
	TgUsername2  string `json:"tg_username2"`
	Status       string `json:"status"`
	CreatedAt    string `json:"created_at"`
}

type InterviewRow struct {
	ID            int64   `json:"id"`
	RezumeID      int64   `json:"rezume_id"`
	InvitedByID   int64   `json:"invited_by_id"`
	InvitedByName string  `json:"invited_by_name"`
	InterviewDate string  `json:"interview_date"`
	InterviewTime string  `json:"interview_time"`
	BranchID      int64   `json:"branch_id"`
	BranchName    string  `json:"branch_name"`
	BranchLat     float64 `json:"branch_lat"`
	BranchLng     float64 `json:"branch_lng"`
	Rating        int     `json:"rating"`
	RatingText    string  `json:"rating_text"`
	Comment       string  `json:"comment"`
	CreatedAt     string  `json:"created_at"`
	// Rezume info
	RezumeFIO     string `json:"rezume_fio"`
	RezumeLavozim string `json:"rezume_lavozim"`
	RezumeTelefon string `json:"rezume_telefon"`
}

// --- State ---

var (
	userStates = make(map[int64]string)
	stateMu    sync.RWMutex

	schedule   = make(map[string]int)
	scheduleMu sync.Mutex
)

var htmlPage string
var adminPage string
var demoPage string
var privacyPage string
var ishchiPage string

func main() {
	data, err := os.ReadFile("index.html")
	if err != nil {
		log.Fatal("index.html faylni o'qishda xato: ", err)
	}
	htmlPage = string(data)

	adminData, err := os.ReadFile("admin.html")
	if err != nil {
		log.Fatal("admin.html faylni o'qishda xato: ", err)
	}
	adminPage = string(adminData)

	demoData, err := os.ReadFile("demo.html")
	if err != nil {
		log.Fatal("demo.html faylni o'qishda xato: ", err)
	}
	demoPage = string(demoData)

	privacyData, err := os.ReadFile("privacy.html")
	if err != nil {
		log.Fatal("privacy.html faylni o'qishda xato: ", err)
	}
	privacyPage = string(privacyData)

	ishchiData, err := os.ReadFile("ishchi.html")
	if err != nil {
		log.Fatal("ishchi.html faylni o'qishda xato: ", err)
	}
	ishchiPage = string(ishchiData)

	os.MkdirAll("uploads", 0755)
	initDB()

	go startBotPolling()
	go startIshchiBotPolling()

	mux := http.NewServeMux()

	// Public — rezume yuborish
	mux.HandleFunc("POST /rezume", handleRezume)
	mux.HandleFunc("POST /ishchi-rezume", handleIshchiRezume)
	mux.HandleFunc("POST /api/report-error", handleReportError)
	mux.HandleFunc("GET /api/public/categories", handlePublicCategories)

	// Auth
	mux.HandleFunc("POST /api/auth/login", handleLogin)
	mux.HandleFunc("POST /api/auth/logout", authRequired(handleLogout))
	mux.HandleFunc("GET /api/auth/me", authRequired(handleMe))

	// Rezume API (auth kerak)
	mux.HandleFunc("GET /api/rezumeler", authRequired(handleGetRezumeler))
	mux.HandleFunc("GET /api/rezumeler/{id}", authRequired(handleGetRezume))
	mux.HandleFunc("DELETE /api/rezumeler/{id}", authRequired(handleDeleteRezume))
	mux.HandleFunc("PATCH /api/rezumeler/{id}/status", authRequired(handleUpdateStatus))

	// Ishchi Anketa API (auth kerak)
	mux.HandleFunc("GET /api/ishchi-anketalar", authRequired(handleGetIshchiAnketalar))
	mux.HandleFunc("GET /api/ishchi-anketalar/{id}", authRequired(handleGetIshchiAnketa))
	mux.HandleFunc("PUT /api/ishchi-anketalar/{id}", authRequired(handleUpdateIshchiAnketa))
	mux.HandleFunc("DELETE /api/ishchi-anketalar/{id}", authRequired(handleDeleteIshchiAnketa))

	// Interview API
	mux.HandleFunc("POST /api/interviews", authRequired(handleCreateInterview))
	mux.HandleFunc("GET /api/interviews", authRequired(handleGetInterviews))
	mux.HandleFunc("GET /api/interviews/{id}", authRequired(handleGetInterview))
	mux.HandleFunc("PATCH /api/interviews/{id}", authRequired(handleUpdateInterview))
	mux.HandleFunc("POST /api/interviews/{id}/send-location", authRequired(handleSendInterviewLocation))

	// User API (super_admin)
	mux.HandleFunc("POST /api/users", superAdminRequired(handleCreateUser))
	mux.HandleFunc("GET /api/users", superAdminRequired(handleGetUsers))
	mux.HandleFunc("GET /api/users/{id}", superAdminRequired(handleGetUserAPI))
	mux.HandleFunc("PATCH /api/users/{id}", superAdminRequired(handleUpdateUser))
	mux.HandleFunc("DELETE /api/users/{id}", superAdminRequired(handleDeleteUser))

	// Branch API
	mux.HandleFunc("POST /api/branches", superAdminRequired(handleCreateBranch))
	mux.HandleFunc("GET /api/branches", authRequired(handleGetBranches))
	mux.HandleFunc("PATCH /api/branches/{id}", superAdminRequired(handleUpdateBranch))
	mux.HandleFunc("DELETE /api/branches/{id}", superAdminRequired(handleDeleteBranch))

	// Category API
	mux.HandleFunc("POST /api/categories", superAdminRequired(handleCreateCategory))
	mux.HandleFunc("GET /api/categories", authRequired(handleGetCategories))
	mux.HandleFunc("PATCH /api/categories/{id}", superAdminRequired(handleUpdateCategory))
	mux.HandleFunc("DELETE /api/categories/{id}", superAdminRequired(handleDeleteCategory))

	// Ishchi Category API
	mux.HandleFunc("POST /api/ishchi-categories", superAdminRequired(handleCreateIshchiCategory))
	mux.HandleFunc("GET /api/ishchi-categories", authRequired(handleGetIshchiCategories))
	mux.HandleFunc("PATCH /api/ishchi-categories/{id}", superAdminRequired(handleUpdateIshchiCategory))
	mux.HandleFunc("DELETE /api/ishchi-categories/{id}", superAdminRequired(handleDeleteIshchiCategory))

	// Swagger
	mux.HandleFunc("GET /swagger", handleSwaggerUI)
	mux.HandleFunc("GET /swagger.json", handleSwaggerJSON)

	// Static
	mux.Handle("/uploads/", http.StripPrefix("/uploads/", http.FileServer(http.Dir("uploads"))))

	// Frontend
	mux.HandleFunc("GET /privacy", handlePrivacy)
	mux.HandleFunc("GET /demo", handleDemo)
	mux.HandleFunc("GET /admin", handleAdmin)
	mux.HandleFunc("/ishchi/", handleIshchi)
	mux.HandleFunc("/", handleRoot)

	handler := corsMiddleware(mux)

	log.Println("Server ishga tushdi: http://localhost:3333")
	log.Println("Swagger: http://localhost:3333/swagger")
	log.Fatal(http.ListenAndServe(":3333", handler))
}

func handleRoot(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if strings.HasPrefix(r.URL.Path, "/ishchi/") || r.URL.Path == "/ishchi" {
		fmt.Fprint(w, ishchiPage)
		return
	}
	fmt.Fprint(w, htmlPage)
}

func handleAdmin(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, adminPage)
}

func handlePrivacy(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, privacyPage)
}

func handleDemo(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, demoPage)
}

func handleIshchi(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, ishchiPage)
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func handleSwaggerUI(w http.ResponseWriter, r *http.Request) {
	html := `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <title>HR Bot API - Swagger</title>
    <link rel="stylesheet" type="text/css" href="https://unpkg.com/swagger-ui-dist@5/swagger-ui.css">
    <style>html{box-sizing:border-box;overflow-y:scroll}*,*:before,*:after{box-sizing:inherit}body{margin:0;background:#fafafa}</style>
</head>
<body>
    <div id="swagger-ui"></div>
    <script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-bundle.js"></script>
    <script>
        window.onload = function() {
            SwaggerUIBundle({
                url: '/swagger.json',
                dom_id: '#swagger-ui',
                presets: [SwaggerUIBundle.presets.apis, SwaggerUIBundle.SwaggerUIStandalonePreset],
                layout: "BaseLayout"
            })
        }
    </script>
</body>
</html>`
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, html)
}

func handleSwaggerJSON(w http.ResponseWriter, r *http.Request) {
	data, err := os.ReadFile("swagger.json")
	if err != nil {
		http.Error(w, "swagger.json topilmadi", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Write(data)
}
