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

const ishchiBotToken = "8759082054:AAHNHn_l6Nqg4S6GGXj88veIKqrgtuJ1a5U"

var ishchiUserStates = make(map[int64]string)

func startIshchiBotPolling() {
	offset := int64(0)
	log.Println("Ishchi bot polling boshlandi...")
	for {
		updates, err := ishchiGetUpdates(offset)
		if err != nil {
			log.Printf("ishchi getUpdates xato: %v", err)
			time.Sleep(3 * time.Second)
			continue
		}
		for _, upd := range updates {
			offset = upd.UpdateID + 1
			if upd.Message != nil {
				handleIshchiBotMessage(upd.Message)
			}
		}
	}
}

func ishchiGetUpdates(offset int64) ([]TgUpdate, error) {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/getUpdates?offset=%d&timeout=30", ishchiBotToken, offset)
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

func handleIshchiBotMessage(msg *TgMessage) {
	chatID := msg.Chat.ID
	text := strings.TrimSpace(msg.Text)

	if text == "/start" {
		stateMu.Lock()
		ishchiUserStates[chatID] = "waiting_phone"
		stateMu.Unlock()

		sendIshchiTgMessage(chatID, "Assalomu alaykum!\n\nIltimos, telefon raqamingizni yuboring.\nMisol: 998901234567")
		return
	}

	stateMu.RLock()
	state := ishchiUserStates[chatID]
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
			sendIshchiTgMessage(chatID, "Telefon raqam noto'g'ri formatda.\nIltimos, to'g'ri raqam yuboring.\nMisol: 998901234567")
			return
		}

		username := ""
		if msg.From != nil {
			username = msg.From.Username
		}

		link := fmt.Sprintf("%s/ishchi/+%s/%d/%s", baseURL, phone, chatID, username)
		sendIshchiTgMessage(chatID, fmt.Sprintf("Rahmat!\n\nAnketa to'ldirish uchun quyidagi havolani bosing:\n\n%s", link))

		stateMu.Lock()
		delete(ishchiUserStates, chatID)
		stateMu.Unlock()
	}
}

// ===================== TELEGRAM API (ishchi bot) =====================

func sendIshchiTgMessage(chatID int64, text string) {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", ishchiBotToken)
	payload := map[string]interface{}{
		"chat_id": chatID,
		"text":    text,
	}
	jsonData, _ := json.Marshal(payload)
	resp, err := http.Post(url, "application/json", bytes.NewReader(jsonData))
	if err != nil {
		log.Printf("sendIshchiTgMessage xato: %v", err)
		return
	}
	resp.Body.Close()
}

func sendIshchiTgLocation(chatID int64, latitude, longitude float64) {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendLocation", ishchiBotToken)
	payload := map[string]interface{}{
		"chat_id":   chatID,
		"latitude":  latitude,
		"longitude": longitude,
	}
	jsonData, _ := json.Marshal(payload)
	resp, err := http.Post(url, "application/json", bytes.NewReader(jsonData))
	if err != nil {
		log.Printf("sendIshchiTgLocation xato: %v", err)
		return
	}
	resp.Body.Close()
}

func sendIshchiPhotoToTelegram(chatID int64, base64Data, caption string) error {
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

	part, err := writer.CreateFormFile("photo", "photo.jpg")
	if err != nil {
		return fmt.Errorf("form file xato: %w", err)
	}
	part.Write(imgBytes)
	writer.Close()

	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendPhoto", ishchiBotToken)
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
