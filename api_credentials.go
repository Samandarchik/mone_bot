package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"
)

// versionLinks — version-check endpointidan olingan ilova yuklab olish linklari.
type versionLinks struct {
	IOS     string // App Store linki (appstoreUrl)
	Android string // Play Store / yuklab olish linki (playstoreUrl)
}

// fetchVersionLinks — versionCheckURL dan ilova linklarini oladi.
// Javob ko'rinishi:
//
//	{"status":"success","data":{"appstoreUrl":"...","playstoreUrl":"..."}}
func fetchVersionLinks() (versionLinks, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(versionCheckURL)
	if err != nil {
		return versionLinks{}, fmt.Errorf("versiya endpointiga ulanib bo'lmadi: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return versionLinks{}, fmt.Errorf("versiya endpointi %d qaytardi", resp.StatusCode)
	}
	var body struct {
		Data struct {
			AppstoreURL  string `json:"appstoreUrl"`
			PlaystoreURL string `json:"playstoreUrl"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return versionLinks{}, fmt.Errorf("versiya javobini o'qib bo'lmadi: %w", err)
	}
	return versionLinks{
		IOS:     strings.TrimSpace(body.Data.AppstoreURL),
		Android: strings.TrimSpace(body.Data.PlaystoreURL),
	}, nil
}

// buildCredentialsMessage — bitta foydalanuvchi uchun login xabari matnini
// quradi: ism (salomlashish), username, parol va ilova (iOS/Android) yuklab
// olish havolalari. Havola bo'sh bo'lsa "—" ko'rsatiladi.
func buildCredentialsMessage(fullName, username, password string, links versionLinks) string {
	name := strings.TrimSpace(fullName)
	if name == "" {
		name = strings.TrimSpace(username)
	}
	iosLink := links.IOS
	if iosLink == "" {
		iosLink = "—"
	}
	androidLink := links.Android
	if androidLink == "" {
		androidLink = "—"
	}
	return fmt.Sprintf(
		"Assalomu alaykum, %s!\n\nTizimga kirish ma'lumotlaringiz:\nLogin: %s\nParol: %s\n\niOS linki: %s\nAndroid linki: %s",
		name, strings.TrimSpace(username), password, iosLink, androidLink,
	)
}

// sendCredsTgMessage — credentials xabarini berilgan chat ID'ga yuboradi va
// Telegram rad etsa xato qaytaradi (sendTgMessage'dan farqli, bu funksiya
// natijani tekshiradi — shuning uchun "yuborildi/xato" hisobi aniq bo'ladi).
func sendCredsTgMessage(chatID int64, text string) error {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", botToken)
	payload, _ := json.Marshal(map[string]interface{}{
		"chat_id":                  chatID,
		"text":                     text,
		"disable_web_page_preview": true,
	})
	resp, err := http.Post(url, "application/json", bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("Telegram'ga ulanib bo'lmadi: %w", err)
	}
	defer resp.Body.Close()
	var tgResp struct {
		OK          bool   `json:"ok"`
		Description string `json:"description"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&tgResp)
	if !tgResp.OK {
		desc := tgResp.Description
		if desc == "" {
			desc = "noma'lum xato"
		}
		return fmt.Errorf("Telegram rad etdi: %s", desc)
	}
	return nil
}

// handleSendAllCredentials — POST /api/users/send-all-credentials
// (faqat super_admin). super_admin'dan boshqa har bir aktiv foydalanuvchiga
// login ma'lumotlarini (ism + username + parol + ilova iOS/Android linklari)
// Telegram orqali yuboradi. Foydalanuvchining Telegram chat ID'si bog'langan
// rezume orqali (users.rezume_id -> rezumeler.tg_user_id) olinadi. Linklar
// versionCheckURL'dan bir marta olinadi (xato bo'lsa linksiz yuboriladi).
// Javob: yuborilganlar soni, Telegram'i yo'q (skipped) va xatolik bo'lganlar
// (failed) ro'yxati.
func handleSendAllCredentials(w http.ResponseWriter, r *http.Request) {
	// Ilova linklarini bir marta olamiz. Xato bo'lsa, jarayonni to'xtatmaymiz —
	// linksiz (—) bo'lsa ham login ma'lumotlari yuboriladi.
	links, err := fetchVersionLinks()
	if err != nil {
		log.Printf("send-all-credentials: versiya linklari olinmadi: %v", err)
	}

	rows, err := db.Query(`
		SELECT u.full_name, u.username, COALESCE(u.password,''), COALESCE(rz.tg_user_id, 0)
		FROM users u
		LEFT JOIN rezumeler rz ON rz.id = u.rezume_id
		WHERE u.role != 'super_admin' AND u.is_active = 1
	`)
	if err != nil {
		jsonError(w, "DB xato: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	type credUser struct {
		fullName string
		username string
		password string
		tgUserID int64
	}
	var all []credUser
	for rows.Next() {
		var u credUser
		if err := rows.Scan(&u.fullName, &u.username, &u.password, &u.tgUserID); err != nil {
			continue
		}
		all = append(all, u)
	}

	sent := 0
	failed := []map[string]string{}
	skipped := []string{}
	for _, u := range all {
		label := strings.TrimSpace(u.fullName)
		if label == "" {
			label = strings.TrimSpace(u.username)
		}
		if u.tgUserID == 0 {
			skipped = append(skipped, label)
			continue
		}
		msg := buildCredentialsMessage(u.fullName, u.username, u.password, links)
		if err := sendCredsTgMessage(u.tgUserID, msg); err != nil {
			failed = append(failed, map[string]string{"name": label, "error": err.Error()})
			continue
		}
		sent++
	}

	jsonResponse(w, map[string]interface{}{
		"message": fmt.Sprintf("%d ta foydalanuvchiga yuborildi", sent),
		"sent":    sent,
		"skipped": skipped,
		"failed":  failed,
	})
}
