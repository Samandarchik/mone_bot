package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"strings"
	"time"
)

// ===================== BOT POLLING =====================

func startBotPolling() {
	offset := int64(0)
	log.Println("Bot polling boshlandi...")
	for {
		updates, err := getUpdates(offset)
		if err != nil {
			log.Printf("getUpdates xato: %v", err)
			time.Sleep(3 * time.Second)
			continue
		}
		for _, upd := range updates {
			offset = upd.UpdateID + 1
			if upd.Message != nil {
				handleBotMessage(upd.Message)
			}
			if upd.CallbackQuery != nil {
				handleCallbackQuery(upd.CallbackQuery)
			}
		}
	}
}

func getUpdates(offset int64) ([]TgUpdate, error) {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/getUpdates?offset=%d&timeout=30", botToken, offset)
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		OK     bool       `json:"ok"`
		Result []TgUpdate `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return result.Result, nil
}

// ===================== BOT MESSAGE HANDLER =====================

func handleBotMessage(msg *TgMessage) {
	chatID := msg.Chat.ID
	text := strings.TrimSpace(msg.Text)

	if text == "/start" {
		stateMu.Lock()
		userStates[chatID] = "waiting_phone"
		stateMu.Unlock()

		sendTgMessage(chatID, "Assalomu alaykum!\n\nIltimos, telefon raqamingizni yuboring.\nMisol: 998901234567")
		return
	}

	stateMu.RLock()
	state := userStates[chatID]
	stateMu.RUnlock()

	if state == "waiting_phone" {
		phone := strings.ReplaceAll(text, " ", "")
		phone = strings.ReplaceAll(phone, "-", "")
		phone = strings.ReplaceAll(phone, "(", "")
		phone = strings.ReplaceAll(phone, ")", "")
		if strings.HasPrefix(phone, "+") {
			phone = phone[1:]
		}
		if !strings.HasPrefix(phone, "998") || len(phone) != 12 {
			sendTgMessage(chatID, "Telefon raqam noto'g'ri formatda.\nIltimos, to'g'ri raqam yuboring.\nMisol: 998901234567")
			return
		}

		username := ""
		if msg.From != nil {
			username = msg.From.Username
		}

		link := fmt.Sprintf("%s/+%s/%d/%s", baseURL, phone, chatID, username)
		sendTgMessage(chatID, fmt.Sprintf("Rahmat!\n\nAnketa to'ldirish uchun quyidagi havolani bosing:\n\n%s", link))

		stateMu.Lock()
		delete(userStates, chatID)
		stateMu.Unlock()
	}
}

// ===================== CALLBACK QUERY HANDLER =====================

func handleCallbackQuery(cb *TgCallback) {
	data := cb.Data
	parts := strings.SplitN(data, ":", 3)
	if len(parts) < 2 {
		answerCallback(cb.ID, "Xato")
		return
	}

	action := parts[0]

	switch action {
	case "day":
		if len(parts) < 3 {
			return
		}
		dayCode := parts[1]
		tgUserID := cb.From.ID
		assignSlot(tgUserID, dayCode, cb)
	default:
		answerCallback(cb.ID, "Bu tugma endi ishlamaydi. Iltimos, dastur orqali qabul/rad qiling.")
	}
}

func sendDaySelection(tgUserID int64) {
	keyboard := map[string]interface{}{
		"inline_keyboard": [][]map[string]string{
			{
				{"text": "Dushanba", "callback_data": fmt.Sprintf("day:mon:%d", tgUserID)},
				{"text": "Chorshanba", "callback_data": fmt.Sprintf("day:wed:%d", tgUserID)},
				{"text": "Juma", "callback_data": fmt.Sprintf("day:fri:%d", tgUserID)},
			},
		},
	}

	payload := map[string]interface{}{
		"chat_id":      tgUserID,
		"text":         "Tabriklaymiz! Siz qabul qilindingiz!\n\nQaysi kunga uchrashuvga kelasiz?",
		"reply_markup": keyboard,
	}

	jsonData, _ := json.Marshal(payload)
	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", botToken)
	resp, err := http.Post(url, "application/json", bytes.NewReader(jsonData))
	if err != nil {
		log.Printf("sendDaySelection xato: %v", err)
		return
	}
	defer resp.Body.Close()
}

func assignSlot(tgUserID int64, dayCode string, cb *TgCallback) {
	var targetWeekday time.Weekday
	var dayName string
	switch dayCode {
	case "mon":
		targetWeekday = time.Monday
		dayName = "Dushanba"
	case "wed":
		targetWeekday = time.Wednesday
		dayName = "Chorshanba"
	case "fri":
		targetWeekday = time.Friday
		dayName = "Juma"
	default:
		return
	}

	now := time.Now()
	daysUntil := int(targetWeekday - now.Weekday())
	if daysUntil <= 0 {
		daysUntil += 7
	}
	nextDate := now.AddDate(0, 0, daysUntil)
	dateKey := nextDate.Format("2006-01-02")

	scheduleMu.Lock()
	count := schedule[dateKey]
	if count >= 10 {
		scheduleMu.Unlock()
		answerCallback(cb.ID, "Bu kunga joy qolmagan!")
		sendTgMessage(tgUserID, "Afsuski, "+dayName+" kuniga joy qolmagan. Boshqa kunni tanlang.")
		return
	}
	schedule[dateKey] = count + 1
	slotIndex := count
	scheduleMu.Unlock()

	totalMinutes := slotIndex * 15
	hour := 9 + totalMinutes/60
	minute := totalMinutes % 60
	timeStr := fmt.Sprintf("%02d:%02d", hour, minute)
	dateStr := nextDate.Format("02.01.2006")

	if cb.Message != nil {
		newText := fmt.Sprintf("✅ Siz %s kuni (%s) soat %s da uchrashuvga yozildingiz!\n\nIltimos, o'z vaqtida keling.", dayName, dateStr, timeStr)
		editMessageText(cb.Message.Chat.ID, cb.Message.MessageID, newText)
	}

	answerCallback(cb.ID, fmt.Sprintf("%s %s", dayName, timeStr))
}

// ===================== TELEGRAM API =====================

func sendPhotoToTelegram(chatID int64, base64Data, caption string, replyMarkup map[string]interface{}) error {
	parts := strings.SplitN(base64Data, ",", 2)
	if len(parts) != 2 {
		return fmt.Errorf("noto'g'ri base64 format")
	}

	imgBytes, err := base64.StdEncoding.DecodeString(parts[1])
	if err != nil {
		return fmt.Errorf("base64 decode xato: %w", err)
	}

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	writer.WriteField("chat_id", fmt.Sprintf("%d", chatID))
	writer.WriteField("caption", caption)

	if replyMarkup != nil {
		markupJSON, _ := json.Marshal(replyMarkup)
		writer.WriteField("reply_markup", string(markupJSON))
	}

	part, err := writer.CreateFormFile("photo", "photo.jpg")
	if err != nil {
		return fmt.Errorf("form file xato: %w", err)
	}
	part.Write(imgBytes)
	writer.Close()

	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendPhoto", botToken)
	resp, err := http.Post(url, writer.FormDataContentType(), body)
	if err != nil {
		return fmt.Errorf("HTTP xato: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("telegram xato %d: %s", resp.StatusCode, string(respBody))
	}
	return nil
}

func sendMessageToTelegramWithKeyboard(chatID int64, text string, replyMarkup map[string]interface{}) error {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", botToken)
	payload := map[string]interface{}{
		"chat_id": chatID,
		"text":    text,
	}
	if replyMarkup != nil {
		payload["reply_markup"] = replyMarkup
	}
	jsonData, _ := json.Marshal(payload)

	resp, err := http.Post(url, "application/json", bytes.NewReader(jsonData))
	if err != nil {
		return fmt.Errorf("HTTP xato: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("telegram xato %d: %s", resp.StatusCode, string(respBody))
	}
	return nil
}

func sendTgMessage(chatID int64, text string) {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", botToken)
	payload := map[string]interface{}{
		"chat_id": chatID,
		"text":    text,
	}
	jsonData, _ := json.Marshal(payload)
	resp, err := http.Post(url, "application/json", bytes.NewReader(jsonData))
	if err != nil {
		log.Printf("sendTgMessage xato: %v", err)
		return
	}
	resp.Body.Close()
}

func editMessageText(chatID int64, messageID int64, text string) {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/editMessageText", botToken)
	payload := map[string]interface{}{
		"chat_id":    chatID,
		"message_id": messageID,
		"text":       text,
	}
	jsonData, _ := json.Marshal(payload)
	resp, err := http.Post(url, "application/json", bytes.NewReader(jsonData))
	if err != nil {
		log.Printf("editMessageText xato: %v", err)
		return
	}
	resp.Body.Close()
}

func editMessageCaption(chatID int64, messageID int64, caption string) {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/editMessageCaption", botToken)
	payload := map[string]interface{}{
		"chat_id":    chatID,
		"message_id": messageID,
		"caption":    caption,
	}
	jsonData, _ := json.Marshal(payload)
	resp, err := http.Post(url, "application/json", bytes.NewReader(jsonData))
	if err != nil {
		log.Printf("editMessageCaption xato: %v", err)
		return
	}
	resp.Body.Close()
}

func sendTgLocation(chatID int64, latitude, longitude float64) {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendLocation", botToken)
	payload := map[string]interface{}{
		"chat_id":   chatID,
		"latitude":  latitude,
		"longitude": longitude,
	}
	jsonData, _ := json.Marshal(payload)
	resp, err := http.Post(url, "application/json", bytes.NewReader(jsonData))
	if err != nil {
		log.Printf("sendTgLocation xato: %v", err)
		return
	}
	resp.Body.Close()
}

func answerCallback(callbackID string, text string) {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/answerCallbackQuery", botToken)
	payload := map[string]interface{}{
		"callback_query_id": callbackID,
		"text":              text,
	}
	jsonData, _ := json.Marshal(payload)
	resp, err := http.Post(url, "application/json", bytes.NewReader(jsonData))
	if err != nil {
		log.Printf("answerCallback xato: %v", err)
		return
	}
	resp.Body.Close()
}
