package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"

	_ "modernc.org/sqlite"
)

var db *sql.DB

func initDB() {
	var err error
	db, err = sql.Open("sqlite", "./rezumeler.db")
	if err != nil {
		log.Fatal("DB ochishda xato: ", err)
	}

	db.Exec("PRAGMA journal_mode=WAL")
	db.Exec("PRAGMA foreign_keys=ON")

	db.Exec("PRAGMA busy_timeout=5000")

	tables := []string{
		`CREATE TABLE IF NOT EXISTS branches (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL UNIQUE,
			is_active INTEGER NOT NULL DEFAULT 1,
			created_at TEXT NOT NULL DEFAULT (datetime('now','localtime'))
		)`,
		`CREATE TABLE IF NOT EXISTS categories (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL UNIQUE,
			tg_group_id INTEGER NOT NULL DEFAULT 0,
			is_active INTEGER NOT NULL DEFAULT 1,
			created_at TEXT NOT NULL DEFAULT (datetime('now','localtime'))
		)`,
		`CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			username TEXT NOT NULL UNIQUE,
			password_hash TEXT NOT NULL,
			full_name TEXT NOT NULL DEFAULT '',
			role TEXT NOT NULL DEFAULT 'admin',
			can_interview INTEGER NOT NULL DEFAULT 0,
			is_active INTEGER NOT NULL DEFAULT 1,
			created_at TEXT NOT NULL DEFAULT (datetime('now','localtime'))
		)`,
		`CREATE TABLE IF NOT EXISTS user_categories (
			user_id INTEGER NOT NULL,
			category_id INTEGER NOT NULL,
			PRIMARY KEY (user_id, category_id),
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
			FOREIGN KEY (category_id) REFERENCES categories(id) ON DELETE CASCADE
		)`,
		`CREATE TABLE IF NOT EXISTS sessions (
			token TEXT PRIMARY KEY,
			user_id INTEGER NOT NULL,
			created_at TEXT NOT NULL DEFAULT (datetime('now','localtime')),
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
		)`,
		`CREATE TABLE IF NOT EXISTS rezumeler (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			lavozim TEXT NOT NULL DEFAULT '',
			familiya TEXT NOT NULL DEFAULT '',
			ism TEXT NOT NULL DEFAULT '',
			sharif TEXT NOT NULL DEFAULT '',
			tugilgan_sana TEXT NOT NULL DEFAULT '',
			boy_sm INTEGER NOT NULL DEFAULT 0,
			vazn_kg INTEGER NOT NULL DEFAULT 0,
			yashash_manzili TEXT NOT NULL DEFAULT '',
			moljal TEXT NOT NULL DEFAULT '',
			umumiy_tajriba TEXT NOT NULL DEFAULT '',
			chet_el_tajribasi TEXT NOT NULL DEFAULT '',
			malumot TEXT NOT NULL DEFAULT '',
			oilaviy_holat TEXT NOT NULL DEFAULT '',
			tillar TEXT NOT NULL DEFAULT '[]',
			telefon TEXT NOT NULL DEFAULT '',
			qoshimcha TEXT NOT NULL DEFAULT '',
			rasm_url TEXT NOT NULL DEFAULT '',
			tg_user_id INTEGER NOT NULL DEFAULT 0,
			tg_username TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL DEFAULT 'yangi',
			created_at TEXT NOT NULL DEFAULT (datetime('now','localtime'))
		)`,
		`CREATE TABLE IF NOT EXISTS ishchi_anketalar (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			vakansiya TEXT NOT NULL DEFAULT '',
			fio TEXT NOT NULL DEFAULT '',
			tugilgan_sana TEXT NOT NULL DEFAULT '',
			manzil TEXT NOT NULL DEFAULT '',
			oilaviy_holat TEXT NOT NULL DEFAULT '',
			bolalar TEXT NOT NULL DEFAULT '',
			tillar TEXT NOT NULL DEFAULT '',
			malumot TEXT NOT NULL DEFAULT '',
			grafik TEXT NOT NULL DEFAULT '',
			sudimlik TEXT NOT NULL DEFAULT '',
			haydovchilik TEXT NOT NULL DEFAULT '',
			telefon TEXT NOT NULL DEFAULT '',
			rasm_url TEXT NOT NULL DEFAULT '',
			tg_user_id INTEGER NOT NULL DEFAULT 0,
			tg_username TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL DEFAULT 'yangi',
			created_at TEXT NOT NULL DEFAULT (datetime('now','localtime'))
		)`,
		`CREATE TABLE IF NOT EXISTS interviews (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			rezume_id INTEGER NOT NULL,
			invited_by_id INTEGER NOT NULL,
			interview_date TEXT NOT NULL,
			interview_time TEXT NOT NULL,
			rating INTEGER NOT NULL DEFAULT 0,
			comment TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL DEFAULT (datetime('now','localtime')),
			FOREIGN KEY (rezume_id) REFERENCES rezumeler(id) ON DELETE CASCADE,
			FOREIGN KEY (invited_by_id) REFERENCES users(id)
		)`,
	}

	for _, t := range tables {
		if _, err := db.Exec(t); err != nil {
			log.Fatalf("Table yaratishda xato: %v", err)
		}
	}

	// Migration: users jadvaliga branch_id qo'shish (agar yo'q bo'lsa)
	db.Exec("ALTER TABLE users ADD COLUMN branch_id INTEGER NOT NULL DEFAULT 0")

	// Migration: branches jadvaliga latitude/longitude qo'shish
	db.Exec("ALTER TABLE branches ADD COLUMN latitude REAL NOT NULL DEFAULT 0")
	db.Exec("ALTER TABLE branches ADD COLUMN longitude REAL NOT NULL DEFAULT 0")

	// Migration: interviews jadvaliga branch_id qo'shish
	db.Exec("ALTER TABLE interviews ADD COLUMN branch_id INTEGER NOT NULL DEFAULT 0")

	// Migration: interviews jadvaliga voice_url qo'shish (ovozli izoh)
	db.Exec("ALTER TABLE interviews ADD COLUMN voice_url TEXT NOT NULL DEFAULT ''")

	// Migration: rezumeler jadvaliga status_by va status_by_name qo'shish
	db.Exec("ALTER TABLE rezumeler ADD COLUMN status_by INTEGER NOT NULL DEFAULT 0")
	db.Exec("ALTER TABLE rezumeler ADD COLUMN status_by_name TEXT NOT NULL DEFAULT ''")

	// Migration: ishchi_anketalar jadvaliga boy_sm, vazn_kg, lang qo'shish
	db.Exec("ALTER TABLE ishchi_anketalar ADD COLUMN boy_sm INTEGER NOT NULL DEFAULT 0")
	db.Exec("ALTER TABLE ishchi_anketalar ADD COLUMN vazn_kg INTEGER NOT NULL DEFAULT 0")
	db.Exec("ALTER TABLE ishchi_anketalar ADD COLUMN lang TEXT NOT NULL DEFAULT ''")

	// Migration: tg_username2 qo'shish
	db.Exec("ALTER TABLE ishchi_anketalar ADD COLUMN tg_username2 TEXT NOT NULL DEFAULT ''")
	db.Exec("ALTER TABLE rezumeler ADD COLUMN tg_username2 TEXT NOT NULL DEFAULT ''")

	// ishchi_categories jadvalini yaratish
	db.Exec(`CREATE TABLE IF NOT EXISTS ishchi_categories (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL UNIQUE,
		tg_group_id INTEGER NOT NULL DEFAULT 0,
		is_active INTEGER NOT NULL DEFAULT 1,
		created_at TEXT NOT NULL DEFAULT (datetime('now','localtime'))
	)`)

	// Migration: eski statuslarni yangi status tizimiga o'tkazish
	db.Exec("UPDATE rezumeler SET status = 'pending' WHERE status = 'yangi'")
	db.Exec("UPDATE rezumeler SET status = 'interviewing' WHERE status = 'qabul'")
	db.Exec("UPDATE rezumeler SET status = 'rejected' WHERE status = 'rad'")

	seedDB()
	log.Println("SQLite baza tayyor")
}

func seedDB() {
	// Kategoriyalarni seed qilish
	positions := map[string]int64{
		"Горничная уборщица / Xonadon tozalovchi": -5014841679,
		"Магазинщик / Do'konchi":                  -5170258928,
		"Кухня Салатница / Kuxnya Salatnisa":      -5126056788,
		"Официант / Ofitsiant":                    -5170258928,
		"Повар / Oshpaz":                          -5126056788,
		"Водитель / Haydovchi":                    -5132239156,
	}
	for name, groupID := range positions {
		db.Exec("INSERT OR IGNORE INTO categories (name, tg_group_id) VALUES (?, ?)", name, groupID)
	}

	// Branchlarni seed qilish
	branches := []string{"Gelion", "Fresco", "Sibirski", "Marxabo"}
	for _, name := range branches {
		db.Exec("INSERT OR IGNORE INTO branches (name) VALUES (?)", name)
	}

	// Default super admin yaratish
	var count int
	db.QueryRow("SELECT COUNT(*) FROM users WHERE role = 'super_admin'").Scan(&count)
	if count == 0 {
		hash, _ := hashPassword("admin123")
		db.Exec(
			"INSERT INTO users (username, password_hash, full_name, role, can_interview) VALUES (?, ?, ?, ?, ?)",
			"admin", hash, "Super Admin", "super_admin", 1,
		)
		log.Println("Default super admin yaratildi: admin / admin123")
	}
}

// ===================== REZUME CRUD =====================

func saveRezume(a *Anketa, rasmURL string) (int64, error) {
	tillarJSON, _ := json.Marshal(a.Tillar)
	result, err := db.Exec(`INSERT INTO rezumeler
		(lavozim, familiya, ism, sharif, tugilgan_sana, boy_sm, vazn_kg,
		 yashash_manzili, moljal, umumiy_tajriba, chet_el_tajribasi,
		 malumot, oilaviy_holat, tillar, telefon, qoshimcha, rasm_url,
		 tg_user_id, tg_username, tg_username2)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		a.Lavozim, a.Familiya, a.Ism, a.Sharif, a.TugilganSana,
		a.BoySm, a.VaznKg, a.YashashManzili, a.Moljal,
		a.UmumiyTajriba, a.ChetElTajribasi, a.Malumot, a.OilaviyHolat,
		string(tillarJSON), a.Telefon, a.Qoshimcha, rasmURL,
		a.TgUserID, a.TgUsername, a.TgUsername2,
	)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

func getRezumeler(lavozim, status, search string, allowedCategories []string, page, limit int) ([]RezumeRow, int, error) {
	where := "1=1"
	args := []interface{}{}

	if lavozim != "" {
		where += " AND lavozim = ?"
		args = append(args, lavozim)
	}
	if status != "" {
		where += " AND status = ?"
		args = append(args, status)
	} else {
		// Default (Barchasi): rejected rezumelarni ko'rsatmaslik + 4 marta chaqirilganlar asosiy listdan chiqsin
		where += " AND status != 'rejected'"
		where += " AND id NOT IN (SELECT rezume_id FROM interviews GROUP BY rezume_id HAVING COUNT(*) >= 4)"
	}
	if search != "" {
		where += " AND (familiya LIKE ? OR ism LIKE ? OR telefon LIKE ?)"
		s := "%" + search + "%"
		args = append(args, s, s, s)
	}
	if len(allowedCategories) > 0 {
		ph := ""
		for i, cat := range allowedCategories {
			if i > 0 {
				ph += ","
			}
			ph += "?"
			args = append(args, cat)
		}
		where += " AND lavozim IN (" + ph + ")"
	}

	var total int
	err := db.QueryRow("SELECT COUNT(*) FROM rezumeler WHERE "+where, args...).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * limit
	query := fmt.Sprintf(
		`SELECT id, lavozim, familiya, ism, sharif, tugilgan_sana, boy_sm, vazn_kg,
		 yashash_manzili, moljal, umumiy_tajriba, chet_el_tajribasi, malumot, oilaviy_holat,
		 tillar, telefon, qoshimcha, rasm_url, tg_user_id, tg_username, COALESCE(tg_username2,''), status, status_by, status_by_name, created_at
		 FROM rezumeler WHERE %s ORDER BY id DESC LIMIT ? OFFSET ?`, where)
	args = append(args, limit, offset)

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	results := []RezumeRow{}
	for rows.Next() {
		var r RezumeRow
		var tillarStr string
		err := rows.Scan(
			&r.ID, &r.Lavozim, &r.Familiya, &r.Ism, &r.Sharif, &r.TugilganSana,
			&r.BoySm, &r.VaznKg, &r.YashashManzili, &r.Moljal, &r.UmumiyTajriba,
			&r.ChetElTajribasi, &r.Malumot, &r.OilaviyHolat, &tillarStr, &r.Telefon,
			&r.Qoshimcha, &r.RasmUrl, &r.TgUserID, &r.TgUsername, &r.TgUsername2, &r.Status, &r.StatusBy, &r.StatusByName, &r.CreatedAt,
		)
		if err != nil {
			return nil, 0, err
		}
		json.Unmarshal([]byte(tillarStr), &r.Tillar)
		if r.Tillar == nil {
			r.Tillar = []LangInfo{}
		}
		results = append(results, r)
	}
	return results, total, nil
}

func getRezumeByID(id int64) (*RezumeRow, error) {
	var r RezumeRow
	var tillarStr string
	err := db.QueryRow(
		`SELECT id, lavozim, familiya, ism, sharif, tugilgan_sana, boy_sm, vazn_kg,
		 yashash_manzili, moljal, umumiy_tajriba, chet_el_tajribasi, malumot, oilaviy_holat,
		 tillar, telefon, qoshimcha, rasm_url, tg_user_id, tg_username, COALESCE(tg_username2,''), status, status_by, status_by_name, created_at
		 FROM rezumeler WHERE id = ?`, id).Scan(
		&r.ID, &r.Lavozim, &r.Familiya, &r.Ism, &r.Sharif, &r.TugilganSana,
		&r.BoySm, &r.VaznKg, &r.YashashManzili, &r.Moljal, &r.UmumiyTajriba,
		&r.ChetElTajribasi, &r.Malumot, &r.OilaviyHolat, &tillarStr, &r.Telefon,
		&r.Qoshimcha, &r.RasmUrl, &r.TgUserID, &r.TgUsername, &r.TgUsername2, &r.Status, &r.StatusBy, &r.StatusByName, &r.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	json.Unmarshal([]byte(tillarStr), &r.Tillar)
	if r.Tillar == nil {
		r.Tillar = []LangInfo{}
	}
	return &r, nil
}

func updateRezumeStatus(id int64, status string) error {
	_, err := db.Exec("UPDATE rezumeler SET status = ? WHERE id = ?", status, id)
	return err
}

func updateRezumeStatusWithAdmin(id int64, status string, adminID int64, adminName string) error {
	_, err := db.Exec("UPDATE rezumeler SET status = ?, status_by = ?, status_by_name = ? WHERE id = ?", status, adminID, adminName, id)
	return err
}

func deleteRezume(id int64) error {
	_, err := db.Exec("DELETE FROM rezumeler WHERE id = ?", id)
	return err
}

// ===================== USER CRUD =====================

func dbCreateUser(username, passwordHash, fullName, role string, canInterview bool, branchID int64) (int64, error) {
	ci := 0
	if canInterview {
		ci = 1
	}
	result, err := db.Exec(
		"INSERT INTO users (username, password_hash, full_name, role, can_interview, branch_id) VALUES (?, ?, ?, ?, ?, ?)",
		username, passwordHash, fullName, role, ci, branchID,
	)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

func dbSetUserCategories(userID int64, categoryIDs []int64) error {
	db.Exec("DELETE FROM user_categories WHERE user_id = ?", userID)
	for _, catID := range categoryIDs {
		db.Exec("INSERT INTO user_categories (user_id, category_id) VALUES (?, ?)", userID, catID)
	}
	return nil
}

func dbGetUsers() ([]UserResponse, error) {
	rows, err := db.Query(
		"SELECT id, username, full_name, role, can_interview, is_active, branch_id, created_at FROM users ORDER BY id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	users := []UserResponse{}
	for rows.Next() {
		var u UserRow
		var ci, ia int
		if err := rows.Scan(&u.ID, &u.Username, &u.FullName, &u.Role, &ci, &ia, &u.BranchID, &u.CreatedAt); err != nil {
			return nil, err
		}
		u.CanInterview = ci == 1
		u.IsActive = ia == 1
		cats := getUserCategories(u.ID)
		branch := getBranchPtr(u.BranchID)
		users = append(users, UserResponse{UserRow: u, Categories: cats, Branch: branch})
	}
	return users, nil
}

func dbGetUserByID(id int64) (*UserResponse, error) {
	var u UserRow
	var ci, ia int
	err := db.QueryRow(
		"SELECT id, username, full_name, role, can_interview, is_active, branch_id, created_at FROM users WHERE id = ?", id).
		Scan(&u.ID, &u.Username, &u.FullName, &u.Role, &ci, &ia, &u.BranchID, &u.CreatedAt)
	if err != nil {
		return nil, err
	}
	u.CanInterview = ci == 1
	u.IsActive = ia == 1
	cats := getUserCategories(u.ID)
	branch := getBranchPtr(u.BranchID)
	return &UserResponse{UserRow: u, Categories: cats, Branch: branch}, nil
}

func dbGetUserByUsername(username string) (*UserRow, string, error) {
	var u UserRow
	var passwordHash string
	var ci, ia int
	err := db.QueryRow(
		"SELECT id, username, password_hash, full_name, role, can_interview, is_active, branch_id, created_at FROM users WHERE username = ?",
		username).Scan(&u.ID, &u.Username, &passwordHash, &u.FullName, &u.Role, &ci, &ia, &u.BranchID, &u.CreatedAt)
	if err != nil {
		return nil, "", err
	}
	u.CanInterview = ci == 1
	u.IsActive = ia == 1
	return &u, passwordHash, nil
}

func dbUpdateUser(id int64, fullName, role string, canInterview, isActive bool, branchID int64) error {
	ci, ia := 0, 0
	if canInterview {
		ci = 1
	}
	if isActive {
		ia = 1
	}
	_, err := db.Exec(
		"UPDATE users SET full_name=?, role=?, can_interview=?, is_active=?, branch_id=? WHERE id=?",
		fullName, role, ci, ia, branchID, id)
	return err
}

func dbUpdateUserPassword(id int64, passwordHash string) error {
	_, err := db.Exec("UPDATE users SET password_hash = ? WHERE id = ?", passwordHash, id)
	return err
}

func dbDeleteUser(id int64) error {
	_, err := db.Exec("DELETE FROM users WHERE id = ?", id)
	return err
}

func getUserCategories(userID int64) []Category {
	rows, err := db.Query(
		`SELECT c.id, c.name, c.tg_group_id, c.is_active, c.created_at
		 FROM categories c JOIN user_categories uc ON c.id = uc.category_id
		 WHERE uc.user_id = ?`, userID)
	if err != nil {
		return []Category{}
	}
	defer rows.Close()

	cats := []Category{}
	for rows.Next() {
		var c Category
		var ia int
		rows.Scan(&c.ID, &c.Name, &c.TgGroupID, &ia, &c.CreatedAt)
		c.IsActive = ia == 1
		cats = append(cats, c)
	}
	return cats
}

func getUserCategoryNames(userID int64) []string {
	cats := getUserCategories(userID)
	names := make([]string, len(cats))
	for i, c := range cats {
		names[i] = c.Name
	}
	return names
}

// ===================== CATEGORY CRUD =====================

func dbCreateCategory(name string, tgGroupID int64) (int64, error) {
	result, err := db.Exec("INSERT INTO categories (name, tg_group_id) VALUES (?, ?)", name, tgGroupID)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

func dbGetCategories() ([]Category, error) {
	rows, err := db.Query("SELECT id, name, tg_group_id, is_active, created_at FROM categories ORDER BY id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	cats := []Category{}
	for rows.Next() {
		var c Category
		var ia int
		rows.Scan(&c.ID, &c.Name, &c.TgGroupID, &ia, &c.CreatedAt)
		c.IsActive = ia == 1
		cats = append(cats, c)
	}
	return cats, nil
}

func dbGetCategoryByID(id int64) (*Category, error) {
	var c Category
	var ia int
	err := db.QueryRow("SELECT id, name, tg_group_id, is_active, created_at FROM categories WHERE id = ?", id).
		Scan(&c.ID, &c.Name, &c.TgGroupID, &ia, &c.CreatedAt)
	if err != nil {
		return nil, err
	}
	c.IsActive = ia == 1
	return &c, nil
}

func dbUpdateCategory(id int64, name string, tgGroupID int64, isActive bool) error {
	ia := 0
	if isActive {
		ia = 1
	}

	var oldName string
	if err := db.QueryRow("SELECT name FROM categories WHERE id = ?", id).Scan(&oldName); err != nil {
		return err
	}

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec("UPDATE categories SET name=?, tg_group_id=?, is_active=? WHERE id=?",
		name, tgGroupID, ia, id); err != nil {
		return err
	}

	if oldName != name {
		if _, err := tx.Exec("UPDATE rezumeler SET lavozim = ? WHERE lavozim = ?", name, oldName); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func dbDeleteCategory(id int64) error {
	_, err := db.Exec("DELETE FROM categories WHERE id = ?", id)
	return err
}

func getCategoryByName(name string) (*Category, error) {
	var c Category
	var ia int
	err := db.QueryRow("SELECT id, name, tg_group_id, is_active, created_at FROM categories WHERE name = ?", name).
		Scan(&c.ID, &c.Name, &c.TgGroupID, &ia, &c.CreatedAt)
	if err != nil {
		return nil, err
	}
	c.IsActive = ia == 1
	return &c, nil
}

// ===================== SESSION CRUD =====================

func dbCreateSession(token string, userID int64) error {
	_, err := db.Exec("INSERT INTO sessions (token, user_id) VALUES (?, ?)", token, userID)
	return err
}

func dbGetUserByToken(token string) (*UserRow, error) {
	var u UserRow
	var ci, ia int
	err := db.QueryRow(
		`SELECT u.id, u.username, u.full_name, u.role, u.can_interview, u.is_active, u.branch_id, u.created_at
		 FROM users u JOIN sessions s ON u.id = s.user_id WHERE s.token = ?`, token).
		Scan(&u.ID, &u.Username, &u.FullName, &u.Role, &ci, &ia, &u.BranchID, &u.CreatedAt)
	if err != nil {
		return nil, err
	}
	u.CanInterview = ci == 1
	u.IsActive = ia == 1
	return &u, nil
}

func dbDeleteSession(token string) error {
	_, err := db.Exec("DELETE FROM sessions WHERE token = ?", token)
	return err
}

// ===================== INTERVIEW CRUD =====================

func ratingText(rating int) string {
	switch rating {
	case 1:
		return "Zo'r"
	case 2:
		return "Yaxshi"
	case 3:
		return "Qabul qilinmadi"
	case 4:
		return "Ishga qabul qilindi"
	default:
		return "Kutilmoqda"
	}
}

// Rezume dublikatini tekshirish va eski dublikatni o'chirish
func deleteDuplicateRezume(lavozim, tugilganSana, telefon string) {
	if lavozim == "" || tugilganSana == "" || telefon == "" {
		return
	}
	db.Exec("DELETE FROM rezumeler WHERE lavozim = ? AND tugilgan_sana = ? AND telefon = ?",
		lavozim, tugilganSana, telefon)
}

// Rezume uchun rejected interviewlar sonini tekshirish
func countRejectedInterviews(rezumeID int64) int {
	var count int
	db.QueryRow("SELECT COUNT(*) FROM interviews WHERE rezume_id = ? AND rating = 3", rezumeID).Scan(&count)
	return count
}

// Rezumening qolgan faol intervyulari soni (o'chirilmaganlar)
func countRemainingActiveInterviews(rezumeID int64) int {
	var count int
	db.QueryRow("SELECT COUNT(*) FROM interviews WHERE rezume_id = ?", rezumeID).Scan(&count)
	return count
}

func dbCreateInterview(rezumeID, invitedByID int64, date, time string, branchID int64) (int64, error) {
	result, err := db.Exec(
		"INSERT INTO interviews (rezume_id, invited_by_id, interview_date, interview_time, branch_id) VALUES (?, ?, ?, ?, ?)",
		rezumeID, invitedByID, date, time, branchID,
	)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

func dbGetInterviews(rezumeID int64, rating int, invitedByID int64, date string, page, limit int) ([]InterviewRow, int, error) {
	where := "1=1"
	args := []interface{}{}

	if rezumeID > 0 {
		where += " AND i.rezume_id = ?"
		args = append(args, rezumeID)
	}
	if rating >= 0 {
		where += " AND i.rating = ?"
		args = append(args, rating)
	}
	if invitedByID > 0 {
		where += " AND i.invited_by_id = ?"
		args = append(args, invitedByID)
	}
	if date != "" {
		where += " AND i.interview_date = ?"
		args = append(args, date)
	}

	var total int
	err := db.QueryRow("SELECT COUNT(*) FROM interviews i WHERE "+where, args...).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * limit
	query := fmt.Sprintf(
		`SELECT i.id, i.rezume_id, i.invited_by_id, COALESCE(u.full_name, u.username),
		 i.interview_date, i.interview_time, i.branch_id,
		 COALESCE(b.name, ''), COALESCE(b.latitude, 0), COALESCE(b.longitude, 0),
		 i.rating, i.comment, COALESCE(i.voice_url, ''), i.created_at,
		 COALESCE(r.familiya || ' ' || r.ism, ''), COALESCE(r.lavozim, ''), COALESCE(r.telefon, ''),
		 COALESCE(r.rasm_url, ''), COALESCE(r.tg_username, ''), COALESCE(r.tg_user_id, 0)
		 FROM interviews i
		 LEFT JOIN users u ON u.id = i.invited_by_id
		 LEFT JOIN rezumeler r ON r.id = i.rezume_id
		 LEFT JOIN branches b ON b.id = i.branch_id
		 WHERE %s ORDER BY i.id DESC LIMIT ? OFFSET ?`, where)
	args = append(args, limit, offset)

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	results := []InterviewRow{}
	for rows.Next() {
		var row InterviewRow
		err := rows.Scan(
			&row.ID, &row.RezumeID, &row.InvitedByID, &row.InvitedByName,
			&row.InterviewDate, &row.InterviewTime, &row.BranchID,
			&row.BranchName, &row.BranchLat, &row.BranchLng,
			&row.Rating, &row.Comment, &row.VoiceUrl, &row.CreatedAt,
			&row.RezumeFIO, &row.RezumeLavozim, &row.RezumeTelefon,
			&row.RezumeRasmUrl, &row.RezumeTgUsername, &row.RezumeTgUserID,
		)
		if err != nil {
			return nil, 0, err
		}
		row.RatingText = ratingText(row.Rating)
		results = append(results, row)
	}
	return results, total, nil
}

func dbGetInterviewByID(id int64) (*InterviewRow, error) {
	var row InterviewRow
	err := db.QueryRow(
		`SELECT i.id, i.rezume_id, i.invited_by_id, COALESCE(u.full_name, u.username),
		 i.interview_date, i.interview_time, i.branch_id,
		 COALESCE(b.name, ''), COALESCE(b.latitude, 0), COALESCE(b.longitude, 0),
		 i.rating, i.comment, COALESCE(i.voice_url, ''), i.created_at,
		 COALESCE(r.familiya || ' ' || r.ism, ''), COALESCE(r.lavozim, ''), COALESCE(r.telefon, ''),
		 COALESCE(r.rasm_url, ''), COALESCE(r.tg_username, ''), COALESCE(r.tg_user_id, 0)
		 FROM interviews i
		 LEFT JOIN users u ON u.id = i.invited_by_id
		 LEFT JOIN rezumeler r ON r.id = i.rezume_id
		 LEFT JOIN branches b ON b.id = i.branch_id
		 WHERE i.id = ?`, id).Scan(
		&row.ID, &row.RezumeID, &row.InvitedByID, &row.InvitedByName,
		&row.InterviewDate, &row.InterviewTime, &row.BranchID,
		&row.BranchName, &row.BranchLat, &row.BranchLng,
		&row.Rating, &row.Comment, &row.VoiceUrl, &row.CreatedAt,
		&row.RezumeFIO, &row.RezumeLavozim, &row.RezumeTelefon,
		&row.RezumeRasmUrl, &row.RezumeTgUsername, &row.RezumeTgUserID,
	)
	if err != nil {
		return nil, err
	}
	row.RatingText = ratingText(row.Rating)
	return &row, nil
}

func dbUpdateInterview(id int64, rating int, comment, voiceUrl string) error {
	_, err := db.Exec("UPDATE interviews SET rating = ?, comment = ?, voice_url = ? WHERE id = ?", rating, comment, voiceUrl, id)
	return err
}

func dbRescheduleInterview(id int64, date, time string, branchID int64) error {
	_, err := db.Exec(
		"UPDATE interviews SET interview_date = ?, interview_time = ?, branch_id = ? WHERE id = ?",
		date, time, branchID, id,
	)
	return err
}

func dbDeleteInterview(id int64) error {
	_, err := db.Exec("DELETE FROM interviews WHERE id = ?", id)
	return err
}

// ===================== BRANCH CRUD =====================

func getBranchPtr(branchID int64) *Branch {
	if branchID == 0 {
		return nil
	}
	b, err := dbGetBranchByID(branchID)
	if err != nil {
		return nil
	}
	return b
}

func dbCreateBranch(name string, latitude, longitude float64) (int64, error) {
	result, err := db.Exec("INSERT INTO branches (name, latitude, longitude) VALUES (?, ?, ?)", name, latitude, longitude)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

func dbGetBranches() ([]Branch, error) {
	rows, err := db.Query("SELECT id, name, latitude, longitude, is_active, created_at FROM branches ORDER BY id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	results := []Branch{}
	for rows.Next() {
		var b Branch
		var ia int
		rows.Scan(&b.ID, &b.Name, &b.Latitude, &b.Longitude, &ia, &b.CreatedAt)
		b.IsActive = ia == 1
		results = append(results, b)
	}
	return results, nil
}

func dbGetBranchByID(id int64) (*Branch, error) {
	var b Branch
	var ia int
	err := db.QueryRow("SELECT id, name, latitude, longitude, is_active, created_at FROM branches WHERE id = ?", id).
		Scan(&b.ID, &b.Name, &b.Latitude, &b.Longitude, &ia, &b.CreatedAt)
	if err != nil {
		return nil, err
	}
	b.IsActive = ia == 1
	return &b, nil
}

func dbUpdateBranch(id int64, name string, latitude, longitude float64, isActive bool) error {
	ia := 0
	if isActive {
		ia = 1
	}
	_, err := db.Exec("UPDATE branches SET name=?, latitude=?, longitude=?, is_active=? WHERE id=?", name, latitude, longitude, ia, id)
	return err
}

func dbDeleteBranch(id int64) error {
	_, err := db.Exec("DELETE FROM branches WHERE id = ?", id)
	return err
}

// ===================== INTERVIEWS ATTACH =====================

func attachInterviews(rezumeler []RezumeRow) {
	if len(rezumeler) == 0 {
		return
	}
	ph := ""
	args := []interface{}{}
	for i, r := range rezumeler {
		if i > 0 {
			ph += ","
		}
		ph += "?"
		args = append(args, r.ID)
	}

	rows, err := db.Query(fmt.Sprintf(
		`SELECT i.id, i.rezume_id, i.invited_by_id, COALESCE(u.full_name, u.username, ''),
		 i.interview_date, i.interview_time, i.branch_id,
		 COALESCE(b.name, ''), COALESCE(b.latitude, 0), COALESCE(b.longitude, 0),
		 i.rating, i.comment, COALESCE(i.voice_url, ''), i.created_at
		 FROM interviews i
		 LEFT JOIN users u ON u.id = i.invited_by_id
		 LEFT JOIN branches b ON b.id = i.branch_id
		 WHERE i.rezume_id IN (%s) ORDER BY i.id DESC`, ph), args...)
	if err != nil {
		return
	}
	defer rows.Close()

	imap := map[int64][]InterviewRow{}
	for rows.Next() {
		var row InterviewRow
		rows.Scan(&row.ID, &row.RezumeID, &row.InvitedByID, &row.InvitedByName,
			&row.InterviewDate, &row.InterviewTime, &row.BranchID,
			&row.BranchName, &row.BranchLat, &row.BranchLng,
			&row.Rating, &row.Comment, &row.VoiceUrl, &row.CreatedAt)
		row.RatingText = ratingText(row.Rating)
		imap[row.RezumeID] = append(imap[row.RezumeID], row)
	}

	for i := range rezumeler {
		if interviews, ok := imap[rezumeler[i].ID]; ok {
			rezumeler[i].Interviews = interviews
		}
		// nil qoladi = JSON da null chiqadi
	}
}

// ===================== ISHCHI ANKETA CRUD =====================

func getIshchiAnketalar(vakansiya, status, search string, page, limit int) ([]IshchiRow, int, error) {
	where := "1=1"
	args := []interface{}{}

	if vakansiya != "" {
		where += " AND vakansiya = ?"
		args = append(args, vakansiya)
	}
	if status != "" {
		where += " AND status = ?"
		args = append(args, status)
	}
	if search != "" {
		where += " AND (fio LIKE ? OR telefon LIKE ?)"
		s := "%" + search + "%"
		args = append(args, s, s)
	}

	var total int
	err := db.QueryRow("SELECT COUNT(*) FROM ishchi_anketalar WHERE "+where, args...).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * limit
	query := fmt.Sprintf(
		`SELECT id, vakansiya, fio, tugilgan_sana, boy_sm, vazn_kg, manzil, lang, oilaviy_holat, bolalar,
		 tillar, malumot, grafik, sudimlik, haydovchilik, telefon,
		 rasm_url, tg_user_id, tg_username, COALESCE(tg_username2,''), status, created_at
		 FROM ishchi_anketalar WHERE %s ORDER BY id DESC LIMIT ? OFFSET ?`, where)
	args = append(args, limit, offset)

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	results := []IshchiRow{}
	for rows.Next() {
		var r IshchiRow
		err := rows.Scan(
			&r.ID, &r.Vakansiya, &r.FIO, &r.TugilganSana, &r.BoySm, &r.VaznKg, &r.Manzil, &r.Lang,
			&r.OilaviyHolat, &r.Bolalar, &r.Tillar, &r.Malumot,
			&r.Grafik, &r.Sudimlik, &r.Haydovchilik, &r.Telefon,
			&r.RasmUrl, &r.TgUserID, &r.TgUsername, &r.TgUsername2, &r.Status, &r.CreatedAt,
		)
		if err != nil {
			return nil, 0, err
		}
		results = append(results, r)
	}
	return results, total, nil
}

func getIshchiAnketaByID(id int64) (*IshchiRow, error) {
	var r IshchiRow
	err := db.QueryRow(
		`SELECT id, vakansiya, fio, tugilgan_sana, boy_sm, vazn_kg, manzil, lang, oilaviy_holat, bolalar,
		 tillar, malumot, grafik, sudimlik, haydovchilik, telefon,
		 rasm_url, tg_user_id, tg_username, COALESCE(tg_username2,''), status, created_at
		 FROM ishchi_anketalar WHERE id = ?`, id).Scan(
		&r.ID, &r.Vakansiya, &r.FIO, &r.TugilganSana, &r.BoySm, &r.VaznKg, &r.Manzil, &r.Lang,
		&r.OilaviyHolat, &r.Bolalar, &r.Tillar, &r.Malumot,
		&r.Grafik, &r.Sudimlik, &r.Haydovchilik, &r.Telefon,
		&r.RasmUrl, &r.TgUserID, &r.TgUsername, &r.TgUsername2, &r.Status, &r.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &r, nil
}

func deleteIshchiAnketa(id int64) error {
	_, err := db.Exec("DELETE FROM ishchi_anketalar WHERE id = ?", id)
	return err
}

func saveIshchiAnketa(a *IshchiAnketa, rasmURL string) (int64, error) {
	result, err := db.Exec(`INSERT INTO ishchi_anketalar
		(vakansiya, fio, tugilgan_sana, boy_sm, vazn_kg, manzil, lang, oilaviy_holat, bolalar,
		 tillar, malumot, grafik, sudimlik, haydovchilik, telefon,
		 rasm_url, tg_user_id, tg_username, tg_username2)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		a.Vakansiya, a.FIO, a.TugilganSana, a.BoySm, a.VaznKg, a.Manzil, a.Lang,
		a.OilaviyHolat, a.Bolalar, a.Tillar, a.Malumot,
		a.Grafik, a.Sudimlik, a.Haydovchilik, a.Telefon,
		rasmURL, a.TgUserID, a.TgUsername, a.TgUsername2,
	)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

func updateIshchiAnketa(id int64, a *IshchiAnketa) error {
	_, err := db.Exec(`UPDATE ishchi_anketalar SET
		vakansiya=?, fio=?, tugilgan_sana=?, boy_sm=?, vazn_kg=?, manzil=?,
		oilaviy_holat=?, bolalar=?, tillar=?, malumot=?, grafik=?,
		sudimlik=?, haydovchilik=?, telefon=?, tg_username=?, tg_username2=?
		WHERE id=?`,
		a.Vakansiya, a.FIO, a.TugilganSana, a.BoySm, a.VaznKg, a.Manzil,
		a.OilaviyHolat, a.Bolalar, a.Tillar, a.Malumot, a.Grafik,
		a.Sudimlik, a.Haydovchilik, a.Telefon, a.TgUsername, a.TgUsername2,
		id,
	)
	return err
}

// ===================== ISHCHI CATEGORY CRUD =====================

func dbCreateIshchiCategory(name string, tgGroupID int64) (int64, error) {
	result, err := db.Exec("INSERT INTO ishchi_categories (name, tg_group_id) VALUES (?, ?)", name, tgGroupID)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

func dbGetIshchiCategories() ([]Category, error) {
	rows, err := db.Query("SELECT id, name, tg_group_id, is_active, created_at FROM ishchi_categories ORDER BY id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	cats := []Category{}
	for rows.Next() {
		var c Category
		var ia int
		rows.Scan(&c.ID, &c.Name, &c.TgGroupID, &ia, &c.CreatedAt)
		c.IsActive = ia == 1
		cats = append(cats, c)
	}
	return cats, nil
}

func dbGetIshchiCategoryByID(id int64) (*Category, error) {
	var c Category
	var ia int
	err := db.QueryRow("SELECT id, name, tg_group_id, is_active, created_at FROM ishchi_categories WHERE id = ?", id).
		Scan(&c.ID, &c.Name, &c.TgGroupID, &ia, &c.CreatedAt)
	if err != nil {
		return nil, err
	}
	c.IsActive = ia == 1
	return &c, nil
}

func dbUpdateIshchiCategory(id int64, name string, tgGroupID int64, isActive bool) error {
	ia := 0
	if isActive {
		ia = 1
	}
	_, err := db.Exec("UPDATE ishchi_categories SET name=?, tg_group_id=?, is_active=? WHERE id=?",
		name, tgGroupID, ia, id)
	return err
}

func dbDeleteIshchiCategory(id int64) error {
	_, err := db.Exec("DELETE FROM ishchi_categories WHERE id = ?", id)
	return err
}
