package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"strings"

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
			password_hash TEXT NOT NULL DEFAULT '',
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

	// Migration: users jadvaliga rezume-dan kelgan rasm va telefon qo'shish
	db.Exec("ALTER TABLE users ADD COLUMN rasm_url TEXT NOT NULL DEFAULT ''")
	db.Exec("ALTER TABLE users ADD COLUMN telefon TEXT NOT NULL DEFAULT ''")
	db.Exec("ALTER TABLE users ADD COLUMN rezume_id INTEGER NOT NULL DEFAULT 0")

	// Migration: users jadvaliga roles (ko'p rol) ustunini qo'shish. Vergul bilan
	// ajratilgan (masalan "admin,ishchi_admin"). Bo'sh bo'lsa eski `role` ishlatiladi.
	db.Exec("ALTER TABLE users ADD COLUMN roles TEXT NOT NULL DEFAULT ''")

	// Migration: users jadvaliga sort_order qo'shish — qo'lda surib (drag)
	// o'rnatilgan ro'yxat tartibini saqlash uchun. Mavjud yozuvlarni dastlab
	// rol guruhi + id bo'yicha tartiblab qo'yamiz, shunda eski ko'rinish saqlanadi.
	db.Exec("ALTER TABLE users ADD COLUMN sort_order INTEGER NOT NULL DEFAULT 0")
	db.Exec(`UPDATE users SET sort_order =
		(CASE role
			WHEN 'super_admin' THEN 0
			WHEN 'admin' THEN 1
			WHEN 'ishchi_admin' THEN 2
			ELSE 3
		END) * 100000 + id
		WHERE sort_order = 0`)

	// Migration: branches jadvaliga latitude/longitude qo'shish
	db.Exec("ALTER TABLE branches ADD COLUMN latitude REAL NOT NULL DEFAULT 0")
	db.Exec("ALTER TABLE branches ADD COLUMN longitude REAL NOT NULL DEFAULT 0")
	db.Exec("ALTER TABLE branches ADD COLUMN address TEXT NOT NULL DEFAULT ''")

	// Migration: interviews jadvaliga branch_id qo'shish
	db.Exec("ALTER TABLE interviews ADD COLUMN branch_id INTEGER NOT NULL DEFAULT 0")

	// Migration: interviews jadvaliga voice_url qo'shish (ovozli izoh)
	db.Exec("ALTER TABLE interviews ADD COLUMN voice_url TEXT NOT NULL DEFAULT ''")

	// Migration: rezumeler jadvaliga status_by va status_by_name qo'shish
	db.Exec("ALTER TABLE rezumeler ADD COLUMN status_by INTEGER NOT NULL DEFAULT 0")
	db.Exec("ALTER TABLE rezumeler ADD COLUMN status_by_name TEXT NOT NULL DEFAULT ''")

	// Migration: rezumeler jadvaliga status_voice_url qo'shish (rad qilishda majburiy ovozli izoh)
	db.Exec("ALTER TABLE rezumeler ADD COLUMN status_voice_url TEXT NOT NULL DEFAULT ''")

	// Migration: ishchi_anketalar jadvaliga boy_sm, vazn_kg, lang qo'shish
	db.Exec("ALTER TABLE ishchi_anketalar ADD COLUMN boy_sm INTEGER NOT NULL DEFAULT 0")
	db.Exec("ALTER TABLE ishchi_anketalar ADD COLUMN vazn_kg INTEGER NOT NULL DEFAULT 0")
	db.Exec("ALTER TABLE ishchi_anketalar ADD COLUMN lang TEXT NOT NULL DEFAULT ''")

	// Migration: tg_username2 qo'shish
	db.Exec("ALTER TABLE ishchi_anketalar ADD COLUMN tg_username2 TEXT NOT NULL DEFAULT ''")
	db.Exec("ALTER TABLE rezumeler ADD COLUMN tg_username2 TEXT NOT NULL DEFAULT ''")

	// Migration: source — reklama manbasi (Telegram /start deep-link parametri)
	// Misol: /start uzaidev yoki /start samarqandvakansiya. tg_username bilan adashmasligi uchun alohida ustun.
	db.Exec("ALTER TABLE rezumeler ADD COLUMN source TEXT NOT NULL DEFAULT ''")
	db.Exec("ALTER TABLE ishchi_anketalar ADD COLUMN source TEXT NOT NULL DEFAULT ''")

	// Migration: face_rasm_url — Face ID apparati uchun yaqindan olingan yuz rasmi.
	// Faqat ?start=oldWorker (eski ishchi) anketalarida to'ldiriladi.
	db.Exec("ALTER TABLE rezumeler ADD COLUMN face_rasm_url TEXT NOT NULL DEFAULT ''")

	// Migration: lavozimlar — anketada tanlangan bir nechta vakansiya.
	// "|" bilan ajratilgan ro'yxat sifatida saqlanadi (kategoriya nomlarida "|" uchramaydi).
	// `lavozim` ustuni birinchi tanlangan lavozimni saqlab qoladi — eski
	// so'rovlar va intervyu joinlari shu ustunga tayanadi.
	db.Exec("ALTER TABLE rezumeler ADD COLUMN lavozimlar TEXT NOT NULL DEFAULT ''")
	db.Exec("UPDATE rezumeler SET lavozimlar = lavozim WHERE TRIM(COALESCE(lavozimlar,'')) = '' AND TRIM(lavozim) != ''")

	// ishchi_categories jadvalini yaratish
	db.Exec(`CREATE TABLE IF NOT EXISTS ishchi_categories (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL UNIQUE,
		tg_group_id INTEGER NOT NULL DEFAULT 0,
		is_active INTEGER NOT NULL DEFAULT 1,
		created_at TEXT NOT NULL DEFAULT (datetime('now','localtime'))
	)`)

	// user_ishchi_categories — ishchi_admin role uchun kategoriya filtri
	db.Exec(`CREATE TABLE IF NOT EXISTS user_ishchi_categories (
		user_id INTEGER NOT NULL,
		category_id INTEGER NOT NULL,
		PRIMARY KEY (user_id, category_id),
		FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
		FOREIGN KEY (category_id) REFERENCES ishchi_categories(id) ON DELETE CASCADE
	)`)

	// ishchi_interviews — rezume interviews bilan parallel
	db.Exec(`CREATE TABLE IF NOT EXISTS ishchi_interviews (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		ishchi_id INTEGER NOT NULL,
		invited_by_id INTEGER NOT NULL,
		interview_date TEXT NOT NULL,
		interview_time TEXT NOT NULL,
		branch_id INTEGER NOT NULL DEFAULT 0,
		rating INTEGER NOT NULL DEFAULT 0,
		comment TEXT NOT NULL DEFAULT '',
		voice_url TEXT NOT NULL DEFAULT '',
		created_at TEXT NOT NULL DEFAULT (datetime('now','localtime')),
		FOREIGN KEY (ishchi_id) REFERENCES ishchi_anketalar(id) ON DELETE CASCADE,
		FOREIGN KEY (invited_by_id) REFERENCES users(id)
	)`)

	// Migration: ishchi_anketalar ga status_by, status_by_name, status_voice_url
	db.Exec("ALTER TABLE ishchi_anketalar ADD COLUMN status_by INTEGER NOT NULL DEFAULT 0")
	db.Exec("ALTER TABLE ishchi_anketalar ADD COLUMN status_by_name TEXT NOT NULL DEFAULT ''")
	db.Exec("ALTER TABLE ishchi_anketalar ADD COLUMN status_voice_url TEXT NOT NULL DEFAULT ''")

	// Migration: eski statuslarni yangi status tizimiga o'tkazish
	db.Exec("UPDATE rezumeler SET status = 'pending' WHERE status = 'yangi'")
	db.Exec("UPDATE rezumeler SET status = 'interviewing' WHERE status = 'qabul'")
	db.Exec("UPDATE rezumeler SET status = 'rejected' WHERE status = 'rad'")
	db.Exec("UPDATE ishchi_anketalar SET status = 'pending' WHERE status = 'yangi' OR status = ''")

	// Migration: ?start=oldWorker havolasidan kelgan, lekin eski binary 'pending'
	// qilib saqlab qo'ygan rezumelarni 'old_worker' statusiga o'tkazish.
	db.Exec("UPDATE rezumeler SET status = 'old_worker' WHERE status = 'pending' AND LOWER(source) = 'oldworker'")

	// Migration: password ustunini qo'shish (plain text)
	db.Exec("ALTER TABLE users ADD COLUMN password TEXT NOT NULL DEFAULT ''")

	// user_devices — har bir foydalanuvchi login qilgan qurilmalar tarixi.
	// Dedup (user_id, device_id) bo'yicha: bir xil fizik qurilmadan qayta login
	// faqat last_login_at'ni yangilaydi, yangi qator qo'shmaydi. Shu sababli
	// "nechta qurilma kirgan" = noyob device_id'lar soni; "qachon" = created_at
	// (birinchi login) va last_login_at (oxirgi login). super_admin login
	// qilganda yozilmaydi (uning qurilmasi hech kimning ro'yxatiga tushmaydi).
	db.Exec(`CREATE TABLE IF NOT EXISTS user_devices (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id INTEGER NOT NULL,
		device_id TEXT NOT NULL,
		device_name TEXT NOT NULL DEFAULT '',
		platform TEXT NOT NULL DEFAULT '',
		last_login_at TEXT NOT NULL DEFAULT (datetime('now','localtime')),
		created_at TEXT NOT NULL DEFAULT (datetime('now','localtime')),
		UNIQUE(user_id, device_id),
		FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
	)`)
	db.Exec("CREATE INDEX IF NOT EXISTS idx_user_devices_user ON user_devices(user_id)")

	// Migration: sessions jadvaliga device_id qo'shish. Login'da qaysi qurilmadan
	// kelganini sessiyaga bog'laymiz, shunda super_admin bitta qurilmani
	// o'chirganda faqat o'sha qurilmaning tokeni yaroqsiz bo'ladi. Eski (migratsiyadan
	// oldingi) sessiyalarda device_id bo'sh bo'ladi — ular qurilmaga bog'lanmagan.
	db.Exec("ALTER TABLE sessions ADD COLUMN device_id TEXT NOT NULL DEFAULT ''")

	// Migration: bir nomzodga bittadan ortiq bahosiz (rating=0) intervyu bo'lmasligi
	// kerak. Chaqirish logikasi yangi chaqiriqdan oldin eski bahosizlarni o'chiradi,
	// shuning uchun bu invariant. Lekin eski/buzilgan ma'lumotda (yoki dublikat rezume
	// yozuvlarida) bir odamga ikkita bahosiz intervyu qolib ketgan bo'lishi mumkin —
	// kalendar har slotga faqat bittasini ko'rsatgani uchun ikkinchisi yashirin turadi
	// va vaqtni o'zgartirilganda "ikkita bo'lib" oshkor bo'ladi. Bu yerda har bir
	// nomzod (telefon, bo'lmasa rezume_id) bo'yicha eng oxirgi (MAX(id)) bahosizni
	// qoldirib, qolganlarini o'chiramiz. Idempotent — har startupda xavfsiz ishlaydi.
	db.Exec(`DELETE FROM interviews
		WHERE rating = 0 AND id NOT IN (
			SELECT MAX(i.id) FROM interviews i
			LEFT JOIN rezumeler r ON r.id = i.rezume_id
			WHERE i.rating = 0
			GROUP BY CASE WHEN COALESCE(r.telefon,'') != '' THEN 'p:' || r.telefon ELSE 'r:' || i.rezume_id END
		)`)

	seedDB()
	log.Println("SQLite baza tayyor")
}

func seedDB() {
	// Demo/default super adminlar (admin/admin123 va admin0055/0055) o'chirildi.
	// Endi seed orqali hech qanday super admin avtomatik yaratilmaydi.
}

// ===================== REZUME CRUD =====================

// lavozimSep — `lavozimlar` ustunidagi ro'yxat ajratuvchisi. Kategoriya nomlari
// "Повар / Oshpaz" ko'rinishida bo'lgani uchun "|" xavfsiz ajratuvchi.
const lavozimSep = "|"

// lavozimlarCol — eski yozuvlar uchun fallback: `lavozimlar` bo'sh bo'lsa,
// bitta elementli ro'yxat sifatida `lavozim` ishlatiladi.
const lavozimlarCol = "COALESCE(NULLIF(TRIM(lavozimlar),''), lavozim)"

// joinLavozimlar — tanlangan lavozimlar ro'yxatini ustun qiymatiga aylantiradi.
// Bo'shlar va takrorlar tashlab yuboriladi; ro'yxat bo'sh bo'lsa `fallback`
// (bitta `lavozim` maydoni) ishlatiladi.
func joinLavozimlar(list []string, fallback string) string {
	seen := map[string]bool{}
	out := []string{}
	for _, v := range list {
		v = strings.TrimSpace(v)
		if v == "" || seen[v] {
			continue
		}
		seen[v] = true
		out = append(out, strings.ReplaceAll(v, lavozimSep, "/"))
	}
	if len(out) == 0 {
		fallback = strings.TrimSpace(fallback)
		if fallback == "" {
			return ""
		}
		return strings.ReplaceAll(fallback, lavozimSep, "/")
	}
	return strings.Join(out, lavozimSep)
}

// splitLavozimlar — ustun qiymatini ro'yxatga ajratadi.
func splitLavozimlar(s, fallback string) []string {
	out := []string{}
	for _, v := range strings.Split(s, lavozimSep) {
		if v = strings.TrimSpace(v); v != "" {
			out = append(out, v)
		}
	}
	if len(out) == 0 {
		if fallback = strings.TrimSpace(fallback); fallback != "" {
			out = append(out, fallback)
		}
	}
	return out
}

// anketaLavozimlar — anketada tanlangan lavozimlarni normallashtirilgan
// (bo'shsiz, takrorsiz) ro'yxat qilib qaytaradi.
func anketaLavozimlar(a *Anketa) []string {
	return splitLavozimlar(joinLavozimlar(a.Lavozimlar, a.Lavozim), a.Lavozim)
}

// firstLavozim — ro'yxatdagi birinchi lavozim (`lavozim` ustuni uchun).
func firstLavozim(list []string, fallback string) string {
	for _, v := range list {
		if v = strings.TrimSpace(v); v != "" {
			return v
		}
	}
	return strings.TrimSpace(fallback)
}

// lavozimMatchSQL — rezume tanlangan lavozimlaridan biri `cats` ichida bormi?
// Har bir kategoriya uchun instr('|'||lavozimlar||'|', '|'||cat||'|') tekshiriladi.
func lavozimMatchSQL(cats []string) (string, []interface{}) {
	parts := []string{}
	args := []interface{}{}
	for _, c := range cats {
		parts = append(parts, "instr(LOWER('|'||"+lavozimlarCol+"||'|'), LOWER('|'||?||'|')) > 0")
		args = append(args, c)
	}
	return "(" + strings.Join(parts, " OR ") + ")", args
}

func saveRezume(a *Anketa, rasmURL, faceRasmURL string) (int64, error) {
	tillarJSON, _ := json.Marshal(a.Tillar)
	source := sanitizeSource(a.Source)
	// Bot havolasi ?start=oldWorker bilan ochilgan bo'lsa — bu eski ishchi:
	// rezume "kutilmoqda" emas, darhol "old_worker" statusi bilan tushadi.
	status := "pending"
	if strings.EqualFold(source, "oldWorker") {
		status = "old_worker"
	}
	// Bir nechta lavozim tanlangan bo'lsa — hammasi `lavozimlar` ustuniga,
	// birinchisi esa eski `lavozim` ustuniga yoziladi.
	lavozimlar := joinLavozimlar(a.Lavozimlar, a.Lavozim)
	lavozim := firstLavozim(a.Lavozimlar, a.Lavozim)
	result, err := db.Exec(`INSERT INTO rezumeler
		(lavozim, lavozimlar, familiya, ism, sharif, tugilgan_sana, boy_sm, vazn_kg,
		 yashash_manzili, moljal, umumiy_tajriba, chet_el_tajribasi,
		 malumot, oilaviy_holat, tillar, telefon, qoshimcha, rasm_url, face_rasm_url,
		 tg_user_id, tg_username, tg_username2, source, status)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		lavozim, lavozimlar, a.Familiya, a.Ism, a.Sharif, a.TugilganSana,
		a.BoySm, a.VaznKg, a.YashashManzili, a.Moljal,
		a.UmumiyTajriba, a.ChetElTajribasi, a.Malumot, a.OilaviyHolat,
		string(tillarJSON), a.Telefon, a.Qoshimcha, rasmURL, faceRasmURL,
		a.TgUserID, a.TgUsername, a.TgUsername2, source, status,
	)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

// updateRezume — rezume ma'lumotlarini (statusdan tashqari) tahrirlaydi.
// Status, source, tg_user_id, rasm o'zgartirilmaydi — ular alohida boshqariladi.
func updateRezume(id int64, a *Anketa) error {
	tillarJSON, _ := json.Marshal(a.Tillar)
	lavozimlar := joinLavozimlar(a.Lavozimlar, a.Lavozim)
	lavozim := firstLavozim(a.Lavozimlar, a.Lavozim)
	_, err := db.Exec(`UPDATE rezumeler SET
		lavozim=?, lavozimlar=?, familiya=?, ism=?, sharif=?, tugilgan_sana=?, boy_sm=?, vazn_kg=?,
		yashash_manzili=?, moljal=?, umumiy_tajriba=?, chet_el_tajribasi=?,
		malumot=?, oilaviy_holat=?, tillar=?, telefon=?, qoshimcha=?,
		tg_username=?, tg_username2=?
		WHERE id=?`,
		lavozim, lavozimlar, a.Familiya, a.Ism, a.Sharif, a.TugilganSana, a.BoySm, a.VaznKg,
		a.YashashManzili, a.Moljal, a.UmumiyTajriba, a.ChetElTajribasi,
		a.Malumot, a.OilaviyHolat, string(tillarJSON), a.Telefon, a.Qoshimcha,
		a.TgUsername, a.TgUsername2,
		id,
	)
	return err
}

func getRezumeler(lavozim, status, search, source string, allowedCategories []string, page, limit int) ([]RezumeRow, int, error) {
	where := "1=1"
	args := []interface{}{}

	if lavozim != "" {
		// Rezume bir nechta lavozimga yuborilgan bo'lishi mumkin — tanlangan
		// kategoriya shu ro'yxat ichida bo'lsa ham ko'rinadi.
		cond, cargs := lavozimMatchSQL([]string{lavozim})
		where += " AND " + cond
		args = append(args, cargs...)
	}
	if source != "" {
		where += " AND source = ?"
		args = append(args, source)
	}
	if status != "" {
		where += " AND status = ?"
		args = append(args, status)
	} else if source == "" {
		// Default (Barchasi): rejected rezumelarni ko'rsatmaslik + 4 marta chaqirilganlar asosiy listdan chiqsin.
		// Manba bo'yicha filter berilganda esa hammasi (status filtersiz) ko'rinishi kerak.
		where += " AND status != 'rejected'"
		where += " AND id NOT IN (SELECT rezume_id FROM interviews GROUP BY rezume_id HAVING COUNT(*) >= 4)"
	}
	if search != "" {
		where += " AND (familiya LIKE ? OR ism LIKE ? OR telefon LIKE ?)"
		s := "%" + search + "%"
		args = append(args, s, s, s)
	}
	if len(allowedCategories) > 0 {
		cond, cargs := lavozimMatchSQL(allowedCategories)
		where += " AND " + cond
		args = append(args, cargs...)
	}

	var total int
	err := db.QueryRow("SELECT COUNT(*) FROM rezumeler WHERE "+where, args...).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * limit
	query := fmt.Sprintf(
		`SELECT id, lavozim, COALESCE(NULLIF(TRIM(lavozimlar),''), lavozim), familiya, ism, sharif, tugilgan_sana, boy_sm, vazn_kg,
		 yashash_manzili, moljal, umumiy_tajriba, chet_el_tajribasi, malumot, oilaviy_holat,
		 tillar, telefon, qoshimcha, rasm_url, COALESCE(face_rasm_url,''), tg_user_id, tg_username, COALESCE(tg_username2,''), status, status_by, status_by_name, COALESCE(status_voice_url,''), COALESCE(source,''), created_at
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
		var tillarStr, lavozimlarStr string
		err := rows.Scan(
			&r.ID, &r.Lavozim, &lavozimlarStr, &r.Familiya, &r.Ism, &r.Sharif, &r.TugilganSana,
			&r.BoySm, &r.VaznKg, &r.YashashManzili, &r.Moljal, &r.UmumiyTajriba,
			&r.ChetElTajribasi, &r.Malumot, &r.OilaviyHolat, &tillarStr, &r.Telefon,
			&r.Qoshimcha, &r.RasmUrl, &r.FaceRasmUrl, &r.TgUserID, &r.TgUsername, &r.TgUsername2, &r.Status, &r.StatusBy, &r.StatusByName, &r.StatusVoiceUrl, &r.Source, &r.CreatedAt,
		)
		if err != nil {
			return nil, 0, err
		}
		r.Lavozimlar = splitLavozimlar(lavozimlarStr, r.Lavozim)
		json.Unmarshal([]byte(tillarStr), &r.Tillar)
		if r.Tillar == nil {
			r.Tillar = []LangInfo{}
		}
		results = append(results, r)
	}
	return results, total, nil
}

// getRezumeCounts — joriy status uchun har bir lavozim (kategoriya) bo'yicha rezume sonini qaytaradi.
// where-mantig'i getRezumeler bilan bir xil: status bo'sh bo'lsa default filtr (rejected'lar va 4+ intervyu
// chaqirilganlar chiqarib tashlanadi), allowedCategories berilsa faqat shu kategoriyalar.
// Natija: lavozim -> son map'i va jami rezume soni. Bitta rezume bir nechta
// lavozimga yozilgan bo'lishi mumkin, shuning uchun jami — qiymatlar yig'indisi
// emas, alohida rezumelar soni.
func getRezumeCounts(status string, allowedCategories []string) (map[string]int, int, error) {
	where := "1=1"
	args := []interface{}{}

	if status != "" {
		where += " AND status = ?"
		args = append(args, status)
	} else {
		where += " AND status != 'rejected'"
		where += " AND id NOT IN (SELECT rezume_id FROM interviews GROUP BY rezume_id HAVING COUNT(*) >= 4)"
	}
	allowed := map[string]bool{}
	if len(allowedCategories) > 0 {
		cond, cargs := lavozimMatchSQL(allowedCategories)
		where += " AND " + cond
		args = append(args, cargs...)
		for _, c := range allowedCategories {
			allowed[c] = true
		}
	}

	// Har bir rezumening lavozimlar ro'yxati o'qiladi va Go tomonida sanaladi —
	// bitta rezume o'zi tanlagan har bir kategoriya tagida ko'rinadi.
	rows, err := db.Query("SELECT "+lavozimlarCol+" FROM rezumeler WHERE "+where, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	counts := map[string]int{}
	total := 0
	for rows.Next() {
		var lavozimlar string
		if err := rows.Scan(&lavozimlar); err != nil {
			return nil, 0, err
		}
		total++
		for _, l := range splitLavozimlar(lavozimlar, "") {
			if len(allowed) > 0 && !allowed[l] {
				continue
			}
			counts[l]++
		}
	}
	return counts, total, rows.Err()
}

func getRezumeByID(id int64) (*RezumeRow, error) {
	var r RezumeRow
	var tillarStr, lavozimlarStr string
	err := db.QueryRow(
		`SELECT id, lavozim, COALESCE(NULLIF(TRIM(lavozimlar),''), lavozim), familiya, ism, sharif, tugilgan_sana, boy_sm, vazn_kg,
		 yashash_manzili, moljal, umumiy_tajriba, chet_el_tajribasi, malumot, oilaviy_holat,
		 tillar, telefon, qoshimcha, rasm_url, COALESCE(face_rasm_url,''), tg_user_id, tg_username, COALESCE(tg_username2,''), status, status_by, status_by_name, COALESCE(status_voice_url,''), COALESCE(source,''), created_at
		 FROM rezumeler WHERE id = ?`, id).Scan(
		&r.ID, &r.Lavozim, &lavozimlarStr, &r.Familiya, &r.Ism, &r.Sharif, &r.TugilganSana,
		&r.BoySm, &r.VaznKg, &r.YashashManzili, &r.Moljal, &r.UmumiyTajriba,
		&r.ChetElTajribasi, &r.Malumot, &r.OilaviyHolat, &tillarStr, &r.Telefon,
		&r.Qoshimcha, &r.RasmUrl, &r.FaceRasmUrl, &r.TgUserID, &r.TgUsername, &r.TgUsername2, &r.Status, &r.StatusBy, &r.StatusByName, &r.StatusVoiceUrl, &r.Source, &r.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	r.Lavozimlar = splitLavozimlar(lavozimlarStr, r.Lavozim)
	json.Unmarshal([]byte(tillarStr), &r.Tillar)
	if r.Tillar == nil {
		r.Tillar = []LangInfo{}
	}
	return &r, nil
}

// getRezumeByPhone — telefon raqami bo'yicha rezume qidiradi (eng so'nggi qaytariladi).
// Telefon raqami normalize qilinadi: bo'sh joy/qavs/tire olib tashlanadi va oxirgi 9 raqami bo'yicha qidirish ham qo'llaniladi.
func getRezumeByPhone(phone string) (*RezumeRow, error) {
	normalized := normalizePhone(phone)
	suffix := normalized
	if len(suffix) > 9 {
		suffix = suffix[len(suffix)-9:]
	}

	var r RezumeRow
	var tillarStr, lavozimlarStr string
	err := db.QueryRow(
		`SELECT id, lavozim, COALESCE(NULLIF(TRIM(lavozimlar),''), lavozim), familiya, ism, sharif, tugilgan_sana, boy_sm, vazn_kg,
		 yashash_manzili, moljal, umumiy_tajriba, chet_el_tajribasi, malumot, oilaviy_holat,
		 tillar, telefon, qoshimcha, rasm_url, COALESCE(face_rasm_url,''), tg_user_id, tg_username, COALESCE(tg_username2,''), status, status_by, status_by_name, COALESCE(status_voice_url,''), COALESCE(source,''), created_at
		 FROM rezumeler
		 WHERE replace(replace(replace(replace(replace(telefon,' ',''),'+',''),'-',''),'(',''),')','') = ?
		    OR replace(replace(replace(replace(replace(telefon,' ',''),'+',''),'-',''),'(',''),')','') LIKE ?
		 ORDER BY id DESC LIMIT 1`,
		normalized, "%"+suffix).Scan(
		&r.ID, &r.Lavozim, &lavozimlarStr, &r.Familiya, &r.Ism, &r.Sharif, &r.TugilganSana,
		&r.BoySm, &r.VaznKg, &r.YashashManzili, &r.Moljal, &r.UmumiyTajriba,
		&r.ChetElTajribasi, &r.Malumot, &r.OilaviyHolat, &tillarStr, &r.Telefon,
		&r.Qoshimcha, &r.RasmUrl, &r.FaceRasmUrl, &r.TgUserID, &r.TgUsername, &r.TgUsername2, &r.Status, &r.StatusBy, &r.StatusByName, &r.StatusVoiceUrl, &r.Source, &r.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	r.Lavozimlar = splitLavozimlar(lavozimlarStr, r.Lavozim)
	json.Unmarshal([]byte(tillarStr), &r.Tillar)
	if r.Tillar == nil {
		r.Tillar = []LangInfo{}
	}
	return &r, nil
}

// sanitizeSource — Telegram /start deep-link parametrini tozalaydi.
// Faqat [a-zA-Z0-9_-] belgilarini qoldiradi, max 64 belgi (Telegram cheklovi).
// Bo'sh string qaytarsa — manba ko'rsatilmagan deb hisoblanadi.
func sanitizeSource(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s) && len(out) < 64; i++ {
		c := s[i]
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_' || c == '-' {
			out = append(out, c)
		}
	}
	return string(out)
}

// normalizePhone — telefon raqamidan bo'sh joy, +, -, (, ) belgilarini olib tashlaydi.
func normalizePhone(phone string) string {
	out := phone
	for _, ch := range []string{" ", "+", "-", "(", ")"} {
		out = strings.ReplaceAll(out, ch, "")
	}
	return out
}

func updateRezumeStatus(id int64, status string) error {
	_, err := db.Exec("UPDATE rezumeler SET status = ? WHERE id = ?", status, id)
	return err
}

func updateRezumeStatusWithAdmin(id int64, status string, adminID int64, adminName string) error {
	_, err := db.Exec("UPDATE rezumeler SET status = ?, status_by = ?, status_by_name = ? WHERE id = ?", status, adminID, adminName, id)
	return err
}

func updateRezumeStatusWithVoice(id int64, status string, adminID int64, adminName, voiceUrl string) error {
	_, err := db.Exec(
		"UPDATE rezumeler SET status = ?, status_by = ?, status_by_name = ?, status_voice_url = ? WHERE id = ?",
		status, adminID, adminName, voiceUrl, id,
	)
	return err
}

func deleteRezume(id int64) error {
	_, err := db.Exec("DELETE FROM rezumeler WHERE id = ?", id)
	return err
}

// ===================== USER CRUD =====================

func dbCreateUser(username, password, fullName, role, roles string, canInterview bool, branchID int64, rasmUrl, telefon string, rezumeID int64) (int64, error) {
	ci := 0
	if canInterview {
		ci = 1
	}
	// Yangi user ro'yxat oxiriga qo'shiladi: mavjud eng katta sort_order + 1.
	var nextOrder int64
	db.QueryRow("SELECT COALESCE(MAX(sort_order), 0) + 1 FROM users").Scan(&nextOrder)
	result, err := db.Exec(
		"INSERT INTO users (username, password, password_hash, full_name, role, roles, can_interview, branch_id, rasm_url, telefon, rezume_id, sort_order) VALUES (?, ?, '', ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		username, password, fullName, role, roles, ci, branchID, rasmUrl, telefon, rezumeID, nextOrder,
	)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

// dbReorderUsers — berilgan id'lar ketma-ketligi bo'yicha sort_order'ni
// qayta o'rnatadi. Ro'yxatdagi tartib (0,1,2,...) saqlanadi.
func dbReorderUsers(orderedIDs []int64) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	for i, id := range orderedIDs {
		if _, err := tx.Exec("UPDATE users SET sort_order = ? WHERE id = ?", i, id); err != nil {
			tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}

func dbSetUserCategories(userID int64, categoryIDs []int64) error {
	db.Exec("DELETE FROM user_categories WHERE user_id = ?", userID)
	for _, catID := range categoryIDs {
		db.Exec("INSERT INTO user_categories (user_id, category_id) VALUES (?, ?)", userID, catID)
	}
	return nil
}

func dbGetUsers() ([]UserResponse, error) {
	// Tartib: qo'lda surib (drag) o'rnatilgan sort_order bo'yicha, keyin id.
	// Shu bilan har bir user ro'yxatda super admin belgilagan joyda turadi
	// va qayta yuklanganda aralashib ketmaydi.
	rows, err := db.Query(
		`SELECT id, username, COALESCE(password,''), full_name, role, COALESCE(roles,''), can_interview, is_active, branch_id, COALESCE(rasm_url,''), COALESCE(telefon,''), COALESCE(rezume_id,0), created_at
		 FROM users
		 ORDER BY sort_order, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	users := []UserResponse{}
	for rows.Next() {
		var u UserRow
		var ci, ia int
		var rolesCSV string
		if err := rows.Scan(&u.ID, &u.Username, &u.Password, &u.FullName, &u.Role, &rolesCSV, &ci, &ia, &u.BranchID, &u.RasmUrl, &u.Telefon, &u.RezumeID, &u.CreatedAt); err != nil {
			return nil, err
		}
		u.CanInterview = ci == 1
		u.IsActive = ia == 1
		fillRoles(&u, rolesCSV)
		cats := getUserCategories(u.ID)
		ishchiCats := getUserIshchiCategories(u.ID)
		branch := getBranchPtr(u.BranchID)
		users = append(users, UserResponse{UserRow: u, Categories: cats, IshchiCategories: ishchiCats, Branch: branch, DeviceCount: dbCountUserDevices(u.ID)})
	}
	return users, nil
}

// dbUpsertUserDevice — login'da qurilmani qayd etadi. Dedup (user_id, device_id):
// bir xil qurilma allaqachon bo'lsa faqat last_login_at + nom/platforma yangilanadi.
func dbUpsertUserDevice(userID int64, deviceID, deviceName, platform string) error {
	_, err := db.Exec(
		`INSERT INTO user_devices (user_id, device_id, device_name, platform, last_login_at)
		 VALUES (?, ?, ?, ?, datetime('now','localtime'))
		 ON CONFLICT(user_id, device_id) DO UPDATE SET
			device_name   = excluded.device_name,
			platform      = excluded.platform,
			last_login_at = datetime('now','localtime')`,
		userID, deviceID, deviceName, platform,
	)
	return err
}

// dbCountUserDevices — foydalanuvchi login qilgan noyob qurilmalar soni.
func dbCountUserDevices(userID int64) int {
	var n int
	db.QueryRow("SELECT COUNT(*) FROM user_devices WHERE user_id = ?", userID).Scan(&n)
	return n
}

// dbGetUserDevices — bitta foydalanuvchining qurilmalari, oxirgi login birinchi.
func dbGetUserDevices(userID int64) ([]UserDevice, error) {
	rows, err := db.Query(
		`SELECT id, device_id, device_name, platform, last_login_at, created_at
		 FROM user_devices WHERE user_id = ? ORDER BY last_login_at DESC`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	devices := []UserDevice{}
	for rows.Next() {
		var d UserDevice
		if err := rows.Scan(&d.ID, &d.DeviceID, &d.DeviceName, &d.Platform, &d.LastLoginAt, &d.CreatedAt); err != nil {
			continue
		}
		devices = append(devices, d)
	}
	return devices, nil
}

func dbGetUserByID(id int64) (*UserResponse, error) {
	var u UserRow
	var ci, ia int
	var rolesCSV string
	err := db.QueryRow(
		"SELECT id, username, COALESCE(password,''), full_name, role, COALESCE(roles,''), can_interview, is_active, branch_id, COALESCE(rasm_url,''), COALESCE(telefon,''), COALESCE(rezume_id,0), created_at FROM users WHERE id = ?", id).
		Scan(&u.ID, &u.Username, &u.Password, &u.FullName, &u.Role, &rolesCSV, &ci, &ia, &u.BranchID, &u.RasmUrl, &u.Telefon, &u.RezumeID, &u.CreatedAt)
	if err != nil {
		return nil, err
	}
	u.CanInterview = ci == 1
	u.IsActive = ia == 1
	fillRoles(&u, rolesCSV)
	cats := getUserCategories(u.ID)
	ishchiCats := getUserIshchiCategories(u.ID)
	branch := getBranchPtr(u.BranchID)
	return &UserResponse{UserRow: u, Categories: cats, IshchiCategories: ishchiCats, Branch: branch, DeviceCount: dbCountUserDevices(u.ID)}, nil
}

func dbGetUserByPassword(password string) (*UserRow, error) {
	var u UserRow
	var ci, ia int
	var rolesCSV string
	err := db.QueryRow(
		"SELECT id, username, COALESCE(password,''), full_name, role, COALESCE(roles,''), can_interview, is_active, branch_id, COALESCE(rasm_url,''), COALESCE(telefon,''), COALESCE(rezume_id,0), created_at FROM users WHERE password = ? LIMIT 1",
		password).Scan(&u.ID, &u.Username, &u.Password, &u.FullName, &u.Role, &rolesCSV, &ci, &ia, &u.BranchID, &u.RasmUrl, &u.Telefon, &u.RezumeID, &u.CreatedAt)
	if err != nil {
		return nil, err
	}
	u.CanInterview = ci == 1
	u.IsActive = ia == 1
	fillRoles(&u, rolesCSV)
	return &u, nil
}

func dbUpdateUser(id int64, fullName, role, roles string, canInterview, isActive bool, branchID int64) error {
	ci, ia := 0, 0
	if canInterview {
		ci = 1
	}
	if isActive {
		ia = 1
	}
	_, err := db.Exec(
		"UPDATE users SET full_name=?, role=?, roles=?, can_interview=?, is_active=?, branch_id=? WHERE id=?",
		fullName, role, roles, ci, ia, branchID, id)
	return err
}

// dbLinkUserToRezume — foydalanuvchi yozuviga rezume ID, rasm va telefonni
// permanent bog'laydi. rasm_url va telefon faqat avval bo'sh bo'lsa
// yangilanadi, shu sababli noto'g'ri qayta yozish bo'lmaydi.
func dbLinkUserToRezume(userID, rezumeID int64, rasmUrl, telefon string) error {
	_, err := db.Exec(
		`UPDATE users
		 SET rezume_id = ?,
		     rasm_url = CASE WHEN COALESCE(rasm_url,'') = '' THEN ? ELSE rasm_url END,
		     telefon  = CASE WHEN COALESCE(telefon,'')  = '' THEN ? ELSE telefon  END
		 WHERE id = ?`,
		rezumeID, rasmUrl, telefon, userID)
	return err
}

// dbReplaceUserRezume — foydalanuvchiga bog'langan rezumeni boshqa rezume bilan
// ALMASHTIRADI. dbLinkUserToRezume'dan farqi: rasm_url va telefon faqat bo'sh
// bo'lganda emas, MAJBURAN qayta yoziladi (super_admin "Rezumeni almashtirish"
// tugmasi bilan boshqa telefon raqamidagi rezumeni bog'laganda). full_name ham
// rezumedagi F.I.Sh bilan yangilanadi (bo'sh bo'lsa tegilmaydi). Login
// (username) va parol o'zgarmaydi — user eski hisobi bilan kirishda davom etadi.
func dbReplaceUserRezume(userID, rezumeID int64, rasmUrl, telefon, fullName string) error {
	if fullName != "" {
		_, err := db.Exec(
			"UPDATE users SET rezume_id = ?, rasm_url = ?, telefon = ?, full_name = ? WHERE id = ?",
			rezumeID, rasmUrl, telefon, fullName, userID)
		return err
	}
	_, err := db.Exec(
		"UPDATE users SET rezume_id = ?, rasm_url = ?, telefon = ? WHERE id = ?",
		rezumeID, rasmUrl, telefon, userID)
	return err
}

func dbUpdateUserPassword(id int64, password string) error {
	_, err := db.Exec("UPDATE users SET password = ? WHERE id = ?", password, id)
	return err
}

func dbDeleteUser(id int64) error {
	_, err := db.Exec("DELETE FROM users WHERE id = ?", id)
	return err
}

func getUserCategories(userID int64) []Category {
	rows, err := db.Query(
		`SELECT c.id, c.name, c.is_active, c.created_at
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
		rows.Scan(&c.ID, &c.Name, &ia, &c.CreatedAt)
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

func dbCreateCategory(name string) (int64, error) {
	result, err := db.Exec("INSERT INTO categories (name) VALUES (?)", name)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

func dbGetCategories() ([]Category, error) {
	rows, err := db.Query("SELECT id, name, is_active, created_at FROM categories ORDER BY id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	cats := []Category{}
	for rows.Next() {
		var c Category
		var ia int
		rows.Scan(&c.ID, &c.Name, &ia, &c.CreatedAt)
		c.IsActive = ia == 1
		cats = append(cats, c)
	}
	return cats, nil
}

func dbGetCategoryByID(id int64) (*Category, error) {
	var c Category
	var ia int
	err := db.QueryRow("SELECT id, name, is_active, created_at FROM categories WHERE id = ?", id).
		Scan(&c.ID, &c.Name, &ia, &c.CreatedAt)
	if err != nil {
		return nil, err
	}
	c.IsActive = ia == 1
	return &c, nil
}

func dbUpdateCategory(id int64, name string, isActive bool) error {
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

	if _, err := tx.Exec("UPDATE categories SET name=?, is_active=? WHERE id=?",
		name, ia, id); err != nil {
		return err
	}

	if oldName != name {
		if _, err := tx.Exec("UPDATE rezumeler SET lavozim = ? WHERE lavozim = ?", name, oldName); err != nil {
			return err
		}
		// Ko'p lavozimli rezumelarda ham eski nomni yangisiga almashtiramiz.
		if _, err := tx.Exec(
			"UPDATE rezumeler SET lavozimlar = TRIM(replace('|'||"+lavozimlarCol+"||'|', '|'||?||'|', '|'||?||'|'), '|') "+
				"WHERE instr('|'||"+lavozimlarCol+"||'|', '|'||?||'|') > 0",
			oldName, name, oldName); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func dbDeleteCategory(id int64) error {
	_, err := db.Exec("UPDATE categories SET is_active = 0 WHERE id = ?", id)
	return err
}

func getCategoryByName(name string) (*Category, error) {
	var c Category
	var ia int
	err := db.QueryRow("SELECT id, name, is_active, created_at FROM categories WHERE name = ?", name).
		Scan(&c.ID, &c.Name, &ia, &c.CreatedAt)
	if err != nil {
		return nil, err
	}
	c.IsActive = ia == 1
	return &c, nil
}

// ===================== SESSION CRUD =====================

func dbCreateSession(token string, userID int64, deviceID string) error {
	_, err := db.Exec("INSERT INTO sessions (token, user_id, device_id) VALUES (?, ?, ?)", token, userID, deviceID)
	return err
}

// dbDeleteUserSessions — foydalanuvchining BARCHA sessiyalarini o'chiradi. Paroli
// almashtirilganda chaqiriladi: har bir qurilmada token yaroqsiz bo'lib, qaytadan
// login talab qilinadi.
func dbDeleteUserSessions(userID int64) error {
	_, err := db.Exec("DELETE FROM sessions WHERE user_id = ?", userID)
	return err
}

// dbDeleteUserDevice — bitta qurilmani (user_devices.id bo'yicha) ro'yxatdan
// o'chiradi va o'sha qurilmaga bog'langan sessiyalarni ham o'chiradi (device_id
// orqali), shunda o'sha qurilmadagi token darhol yaroqsiz bo'ladi. user_id bilan
// cheklangan. Qurilma topilmasa sql.ErrNoRows qaytaradi.
// O'chirilgan qurilmaning device_id'sini ham qaytaradi — chaqiruvchi shu orqali
// o'sha qurilmaning ochiq WS ulanishiga "force_logout" yuboradi.
func dbDeleteUserDevice(userID, deviceRowID int64) (string, error) {
	var deviceID string
	if err := db.QueryRow(
		"SELECT device_id FROM user_devices WHERE id = ? AND user_id = ?",
		deviceRowID, userID,
	).Scan(&deviceID); err != nil {
		return "", err
	}
	if deviceID != "" {
		db.Exec("DELETE FROM sessions WHERE user_id = ? AND device_id = ?", userID, deviceID)
	}
	_, err := db.Exec("DELETE FROM user_devices WHERE id = ? AND user_id = ?", deviceRowID, userID)
	return deviceID, err
}

// dbGetSessionDeviceID — token bo'yicha sessiyaning device_id'sini qaytaradi.
func dbGetSessionDeviceID(token string) (string, error) {
	var deviceID string
	err := db.QueryRow("SELECT COALESCE(device_id,'') FROM sessions WHERE token = ?", token).Scan(&deviceID)
	return deviceID, err
}

func dbGetUserByToken(token string) (*UserRow, error) {
	var u UserRow
	var ci, ia int
	var rolesCSV string
	err := db.QueryRow(
		`SELECT u.id, u.username, COALESCE(u.password,''), u.full_name, u.role, COALESCE(u.roles,''), u.can_interview, u.is_active, u.branch_id, COALESCE(u.rasm_url,''), COALESCE(u.telefon,''), COALESCE(u.rezume_id,0), u.created_at
		 FROM users u JOIN sessions s ON u.id = s.user_id WHERE s.token = ?`, token).
		Scan(&u.ID, &u.Username, &u.Password, &u.FullName, &u.Role, &rolesCSV, &ci, &ia, &u.BranchID, &u.RasmUrl, &u.Telefon, &u.RezumeID, &u.CreatedAt)
	if err != nil {
		return nil, err
	}
	u.CanInterview = ci == 1
	u.IsActive = ia == 1
	fillRoles(&u, rolesCSV)
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
		return "Приглашён, Не пришёл на собеседование"
	case 2:
		return "Отказ"
	case 3:
		return "Используется 2"
	case 4:
		return "Принято"
	case 5:
		return "Резерв"
	default:
		return "Kutilmoqda"
	}
}

// ratingToStatus — rating baholanganda rezume qanday statusga o'tishini aytadi.
// rating=1 (kelmadi) → noshow; 2 → rejected; 3 → trial; 4 → accepted; 5 → reserve.
func ratingToStatus(rating int) string {
	switch rating {
	case 1:
		return "noshow"
	case 2:
		return "rejected"
	case 3:
		return "trial"
	case 4:
		return "accepted"
	case 5:
		return "reserve"
	default:
		return ""
	}
}

// Rezume dublikatini tekshirish va eski dublikatni o'chirish.
// Dublikat kaliti: tug'ilgan sana (yili) + lavozim (kategoriya) + ism + telefon.
// Anketada bir nechta lavozim tanlanishi mumkin — eski rezumening lavozimlaridan
// birortasi yangisi bilan kesishsa, u dublikat hisoblanadi.
// Taqqoslash katta-kichik harflarga (va atrofdagi bo'sh joylarga) bog'liq emas.
func deleteDuplicateRezume(lavozimlar []string, tugilganSana, ism, telefon string) {
	if len(lavozimlar) == 0 || tugilganSana == "" || ism == "" || telefon == "" {
		return
	}
	cond, args := lavozimMatchSQL(lavozimlar)
	args = append(args, tugilganSana, ism, telefon)
	db.Exec(`DELETE FROM rezumeler WHERE `+cond+` AND
		TRIM(tugilgan_sana) = TRIM(?) AND
		LOWER(TRIM(ism)) = LOWER(TRIM(?)) AND
		TRIM(telefon) = TRIM(?)`, args...)
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

func dbCreateBranch(name string, latitude, longitude float64, address string) (int64, error) {
	result, err := db.Exec("INSERT INTO branches (name, latitude, longitude, address) VALUES (?, ?, ?, ?)", name, latitude, longitude, address)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

func dbGetBranches() ([]Branch, error) {
	rows, err := db.Query("SELECT id, name, latitude, longitude, address, is_active, created_at FROM branches ORDER BY id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	results := []Branch{}
	for rows.Next() {
		var b Branch
		var ia int
		rows.Scan(&b.ID, &b.Name, &b.Latitude, &b.Longitude, &b.Address, &ia, &b.CreatedAt)
		b.IsActive = ia == 1
		results = append(results, b)
	}
	return results, nil
}

func dbGetBranchByID(id int64) (*Branch, error) {
	var b Branch
	var ia int
	err := db.QueryRow("SELECT id, name, latitude, longitude, address, is_active, created_at FROM branches WHERE id = ?", id).
		Scan(&b.ID, &b.Name, &b.Latitude, &b.Longitude, &b.Address, &ia, &b.CreatedAt)
	if err != nil {
		return nil, err
	}
	b.IsActive = ia == 1
	return &b, nil
}

func dbUpdateBranch(id int64, name string, latitude, longitude float64, address string, isActive bool) error {
	ia := 0
	if isActive {
		ia = 1
	}
	_, err := db.Exec("UPDATE branches SET name=?, latitude=?, longitude=?, address=?, is_active=? WHERE id=?", name, latitude, longitude, address, ia, id)
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

func getIshchiAnketalar(vakansiya, status, search, source string, allowedCategories []string, page, limit int) ([]IshchiRow, int, error) {
	where := "1=1"
	args := []interface{}{}

	if vakansiya != "" {
		where += " AND vakansiya = ?"
		args = append(args, vakansiya)
	}
	if source != "" {
		where += " AND source = ?"
		args = append(args, source)
	}
	if status != "" {
		where += " AND status = ?"
		args = append(args, status)
	} else if source == "" {
		// Default: rejected ishchilarni ko'rsatmaslik + 4 marta chaqirilganlar asosiy listdan chiqsin.
		// Manba bo'yicha filter berilganda esa hammasi (status filtersiz) ko'rinishi kerak.
		where += " AND status != 'rejected'"
		where += " AND id NOT IN (SELECT ishchi_id FROM ishchi_interviews GROUP BY ishchi_id HAVING COUNT(*) >= 4)"
	}
	if search != "" {
		where += " AND (fio LIKE ? OR telefon LIKE ?)"
		s := "%" + search + "%"
		args = append(args, s, s)
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
		where += " AND vakansiya IN (" + ph + ")"
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
		 rasm_url, tg_user_id, tg_username, COALESCE(tg_username2,''),
		 status, COALESCE(status_by,0), COALESCE(status_by_name,''), COALESCE(status_voice_url,''), COALESCE(source,''), created_at
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
			&r.RasmUrl, &r.TgUserID, &r.TgUsername, &r.TgUsername2,
			&r.Status, &r.StatusBy, &r.StatusByName, &r.StatusVoiceUrl, &r.Source, &r.CreatedAt,
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
		 rasm_url, tg_user_id, tg_username, COALESCE(tg_username2,''),
		 status, COALESCE(status_by,0), COALESCE(status_by_name,''), COALESCE(status_voice_url,''), COALESCE(source,''), created_at
		 FROM ishchi_anketalar WHERE id = ?`, id).Scan(
		&r.ID, &r.Vakansiya, &r.FIO, &r.TugilganSana, &r.BoySm, &r.VaznKg, &r.Manzil, &r.Lang,
		&r.OilaviyHolat, &r.Bolalar, &r.Tillar, &r.Malumot,
		&r.Grafik, &r.Sudimlik, &r.Haydovchilik, &r.Telefon,
		&r.RasmUrl, &r.TgUserID, &r.TgUsername, &r.TgUsername2,
		&r.Status, &r.StatusBy, &r.StatusByName, &r.StatusVoiceUrl, &r.Source, &r.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &r, nil
}

func updateIshchiStatus(id int64, status string) error {
	_, err := db.Exec("UPDATE ishchi_anketalar SET status = ? WHERE id = ?", status, id)
	return err
}

func updateIshchiStatusWithAdmin(id int64, status string, adminID int64, adminName string) error {
	_, err := db.Exec("UPDATE ishchi_anketalar SET status = ?, status_by = ?, status_by_name = ? WHERE id = ?",
		status, adminID, adminName, id)
	return err
}

func updateIshchiStatusWithVoice(id int64, status string, adminID int64, adminName, voiceUrl string) error {
	_, err := db.Exec(
		"UPDATE ishchi_anketalar SET status = ?, status_by = ?, status_by_name = ?, status_voice_url = ? WHERE id = ?",
		status, adminID, adminName, voiceUrl, id,
	)
	return err
}

// Ishchi dublikat
func deleteDuplicateIshchi(vakansiya, tugilganSana, telefon string) {
	if vakansiya == "" || tugilganSana == "" || telefon == "" {
		return
	}
	db.Exec("DELETE FROM ishchi_anketalar WHERE vakansiya = ? AND tugilgan_sana = ? AND telefon = ?",
		vakansiya, tugilganSana, telefon)
}

func countRejectedIshchiInterviews(ishchiID int64) int {
	var count int
	db.QueryRow("SELECT COUNT(*) FROM ishchi_interviews WHERE ishchi_id = ? AND rating = 3", ishchiID).Scan(&count)
	return count
}

func countRemainingActiveIshchiInterviews(ishchiID int64) int {
	var count int
	db.QueryRow("SELECT COUNT(*) FROM ishchi_interviews WHERE ishchi_id = ?", ishchiID).Scan(&count)
	return count
}

func dbCreateIshchiInterview(ishchiID, invitedByID int64, date, time string, branchID int64) (int64, error) {
	result, err := db.Exec(
		"INSERT INTO ishchi_interviews (ishchi_id, invited_by_id, interview_date, interview_time, branch_id) VALUES (?, ?, ?, ?, ?)",
		ishchiID, invitedByID, date, time, branchID,
	)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

func dbGetIshchiInterviews(ishchiID int64, rating int, invitedByID int64, date string, page, limit int) ([]IshchiInterviewRow, int, error) {
	where := "1=1"
	args := []interface{}{}

	if ishchiID > 0 {
		where += " AND i.ishchi_id = ?"
		args = append(args, ishchiID)
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
	err := db.QueryRow("SELECT COUNT(*) FROM ishchi_interviews i WHERE "+where, args...).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * limit
	query := fmt.Sprintf(
		`SELECT i.id, i.ishchi_id, i.invited_by_id, COALESCE(u.full_name, u.username),
		 i.interview_date, i.interview_time, i.branch_id,
		 COALESCE(b.name, ''), COALESCE(b.latitude, 0), COALESCE(b.longitude, 0),
		 i.rating, i.comment, COALESCE(i.voice_url, ''), i.created_at,
		 COALESCE(a.fio, ''), COALESCE(a.vakansiya, ''), COALESCE(a.telefon, ''),
		 COALESCE(a.rasm_url, ''), COALESCE(a.tg_username, ''), COALESCE(a.tg_user_id, 0)
		 FROM ishchi_interviews i
		 LEFT JOIN users u ON u.id = i.invited_by_id
		 LEFT JOIN ishchi_anketalar a ON a.id = i.ishchi_id
		 LEFT JOIN branches b ON b.id = i.branch_id
		 WHERE %s ORDER BY i.id DESC LIMIT ? OFFSET ?`, where)
	args = append(args, limit, offset)

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	results := []IshchiInterviewRow{}
	for rows.Next() {
		var row IshchiInterviewRow
		err := rows.Scan(
			&row.ID, &row.IshchiID, &row.InvitedByID, &row.InvitedByName,
			&row.InterviewDate, &row.InterviewTime, &row.BranchID,
			&row.BranchName, &row.BranchLat, &row.BranchLng,
			&row.Rating, &row.Comment, &row.VoiceUrl, &row.CreatedAt,
			&row.IshchiFIO, &row.IshchiVakansiya, &row.IshchiTelefon,
			&row.IshchiRasmUrl, &row.IshchiTgUsername, &row.IshchiTgUserID,
		)
		if err != nil {
			return nil, 0, err
		}
		row.RatingText = ratingText(row.Rating)
		results = append(results, row)
	}
	return results, total, nil
}

func dbGetIshchiInterviewByID(id int64) (*IshchiInterviewRow, error) {
	var row IshchiInterviewRow
	err := db.QueryRow(
		`SELECT i.id, i.ishchi_id, i.invited_by_id, COALESCE(u.full_name, u.username),
		 i.interview_date, i.interview_time, i.branch_id,
		 COALESCE(b.name, ''), COALESCE(b.latitude, 0), COALESCE(b.longitude, 0),
		 i.rating, i.comment, COALESCE(i.voice_url, ''), i.created_at,
		 COALESCE(a.fio, ''), COALESCE(a.vakansiya, ''), COALESCE(a.telefon, ''),
		 COALESCE(a.rasm_url, ''), COALESCE(a.tg_username, ''), COALESCE(a.tg_user_id, 0)
		 FROM ishchi_interviews i
		 LEFT JOIN users u ON u.id = i.invited_by_id
		 LEFT JOIN ishchi_anketalar a ON a.id = i.ishchi_id
		 LEFT JOIN branches b ON b.id = i.branch_id
		 WHERE i.id = ?`, id).Scan(
		&row.ID, &row.IshchiID, &row.InvitedByID, &row.InvitedByName,
		&row.InterviewDate, &row.InterviewTime, &row.BranchID,
		&row.BranchName, &row.BranchLat, &row.BranchLng,
		&row.Rating, &row.Comment, &row.VoiceUrl, &row.CreatedAt,
		&row.IshchiFIO, &row.IshchiVakansiya, &row.IshchiTelefon,
		&row.IshchiRasmUrl, &row.IshchiTgUsername, &row.IshchiTgUserID,
	)
	if err != nil {
		return nil, err
	}
	row.RatingText = ratingText(row.Rating)
	return &row, nil
}

func dbUpdateIshchiInterview(id int64, rating int, comment, voiceUrl string) error {
	_, err := db.Exec("UPDATE ishchi_interviews SET rating = ?, comment = ?, voice_url = ? WHERE id = ?",
		rating, comment, voiceUrl, id)
	return err
}

func dbRescheduleIshchiInterview(id int64, date, time string, branchID int64) error {
	_, err := db.Exec(
		"UPDATE ishchi_interviews SET interview_date = ?, interview_time = ?, branch_id = ? WHERE id = ?",
		date, time, branchID, id,
	)
	return err
}

func dbDeleteIshchiInterview(id int64) error {
	_, err := db.Exec("DELETE FROM ishchi_interviews WHERE id = ?", id)
	return err
}

func attachIshchiInterviews(rows []IshchiRow) {
	if len(rows) == 0 {
		return
	}
	ph := ""
	args := []interface{}{}
	for i, r := range rows {
		if i > 0 {
			ph += ","
		}
		ph += "?"
		args = append(args, r.ID)
	}

	q, err := db.Query(fmt.Sprintf(
		`SELECT i.id, i.ishchi_id, i.invited_by_id, COALESCE(u.full_name, u.username, ''),
		 i.interview_date, i.interview_time, i.branch_id,
		 COALESCE(b.name, ''), COALESCE(b.latitude, 0), COALESCE(b.longitude, 0),
		 i.rating, i.comment, COALESCE(i.voice_url, ''), i.created_at
		 FROM ishchi_interviews i
		 LEFT JOIN users u ON u.id = i.invited_by_id
		 LEFT JOIN branches b ON b.id = i.branch_id
		 WHERE i.ishchi_id IN (%s) ORDER BY i.id DESC`, ph), args...)
	if err != nil {
		return
	}
	defer q.Close()

	imap := map[int64][]IshchiInterviewRow{}
	for q.Next() {
		var row IshchiInterviewRow
		q.Scan(&row.ID, &row.IshchiID, &row.InvitedByID, &row.InvitedByName,
			&row.InterviewDate, &row.InterviewTime, &row.BranchID,
			&row.BranchName, &row.BranchLat, &row.BranchLng,
			&row.Rating, &row.Comment, &row.VoiceUrl, &row.CreatedAt)
		row.RatingText = ratingText(row.Rating)
		imap[row.IshchiID] = append(imap[row.IshchiID], row)
	}

	for i := range rows {
		if interviews, ok := imap[rows[i].ID]; ok {
			rows[i].Interviews = interviews
		}
	}
}

// User-Ishchi-Categories junction
func dbSetUserIshchiCategories(userID int64, categoryIDs []int64) error {
	db.Exec("DELETE FROM user_ishchi_categories WHERE user_id = ?", userID)
	for _, catID := range categoryIDs {
		db.Exec("INSERT INTO user_ishchi_categories (user_id, category_id) VALUES (?, ?)", userID, catID)
	}
	return nil
}

func getUserIshchiCategories(userID int64) []Category {
	rows, err := db.Query(
		`SELECT c.id, c.name, c.is_active, c.created_at
		 FROM ishchi_categories c JOIN user_ishchi_categories uc ON c.id = uc.category_id
		 WHERE uc.user_id = ?`, userID)
	if err != nil {
		return []Category{}
	}
	defer rows.Close()

	cats := []Category{}
	for rows.Next() {
		var c Category
		var ia int
		rows.Scan(&c.ID, &c.Name, &ia, &c.CreatedAt)
		c.IsActive = ia == 1
		cats = append(cats, c)
	}
	return cats
}

func getUserIshchiCategoryNames(userID int64) []string {
	cats := getUserIshchiCategories(userID)
	names := make([]string, len(cats))
	for i, c := range cats {
		names[i] = c.Name
	}
	return names
}

func deleteIshchiAnketa(id int64) error {
	_, err := db.Exec("DELETE FROM ishchi_anketalar WHERE id = ?", id)
	return err
}

func saveIshchiAnketa(a *IshchiAnketa, rasmURL string) (int64, error) {
	result, err := db.Exec(`INSERT INTO ishchi_anketalar
		(vakansiya, fio, tugilgan_sana, boy_sm, vazn_kg, manzil, lang, oilaviy_holat, bolalar,
		 tillar, malumot, grafik, sudimlik, haydovchilik, telefon,
		 rasm_url, tg_user_id, tg_username, tg_username2, source, status)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'pending')`,
		a.Vakansiya, a.FIO, a.TugilganSana, a.BoySm, a.VaznKg, a.Manzil, a.Lang,
		a.OilaviyHolat, a.Bolalar, a.Tillar, a.Malumot,
		a.Grafik, a.Sudimlik, a.Haydovchilik, a.Telefon,
		rasmURL, a.TgUserID, a.TgUsername, a.TgUsername2, sanitizeSource(a.Source),
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

func dbCreateIshchiCategory(name string) (int64, error) {
	result, err := db.Exec("INSERT INTO ishchi_categories (name) VALUES (?)", name)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

func dbGetIshchiCategories() ([]Category, error) {
	rows, err := db.Query("SELECT id, name, is_active, created_at FROM ishchi_categories ORDER BY id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	cats := []Category{}
	for rows.Next() {
		var c Category
		var ia int
		rows.Scan(&c.ID, &c.Name, &ia, &c.CreatedAt)
		c.IsActive = ia == 1
		cats = append(cats, c)
	}
	return cats, nil
}

func dbGetIshchiCategoryByID(id int64) (*Category, error) {
	var c Category
	var ia int
	err := db.QueryRow("SELECT id, name, is_active, created_at FROM ishchi_categories WHERE id = ?", id).
		Scan(&c.ID, &c.Name, &ia, &c.CreatedAt)
	if err != nil {
		return nil, err
	}
	c.IsActive = ia == 1
	return &c, nil
}

func dbGetIshchiCategoryByName(name string) (*Category, error) {
	var c Category
	var ia int
	err := db.QueryRow("SELECT id, name, is_active, created_at FROM ishchi_categories WHERE name = ?", name).
		Scan(&c.ID, &c.Name, &ia, &c.CreatedAt)
	if err != nil {
		return nil, err
	}
	c.IsActive = ia == 1
	return &c, nil
}

func dbUpdateIshchiCategory(id int64, name string, isActive bool) error {
	ia := 0
	if isActive {
		ia = 1
	}
	_, err := db.Exec("UPDATE ishchi_categories SET name=?, is_active=? WHERE id=?",
		name, ia, id)
	return err
}

func dbDeleteIshchiCategory(id int64) error {
	_, err := db.Exec("DELETE FROM ishchi_categories WHERE id = ?", id)
	return err
}
