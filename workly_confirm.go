// workly_confirm.go — Workly grafik tasdiqlash tugmalari ([Ha]/[Yo'q]).
//
// isup-gateway (davomat tizimi) xodimlarga xabarni SHU botning tokeni bilan
// yuboradi (token 2026-07-19 dan beri u bilan bo'lishilgan), lekin getUpdates
// faqat shu processda ishlaydi — shuning uchun tugma bosilganda callback BU
// YERGA keladi va javob gateway'ga (shu serverning o'zida, 127.0.0.1:8092)
// uzatiladi. Gateway holatni roster_confirms ga yozadi va ilovaga SSE orqali
// tarqatadi (grafikdagi katak sariq → yashil/qizil bo'ladi).
//
// callback_data formati: "wkc:<token>:y|n" — token gateway bergan tasodifiy
// qiymat (roster_confirms.token), ruxsat tekshiruvi shu tokenning o'zi.
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

// worklyGatewayURL — isup-gateway shu serverning o'zida ishlaydi.
const worklyGatewayURL = "http://127.0.0.1:8092"

var worklyHTTP = &http.Client{Timeout: 10 * time.Second}

// handleWorklyConfirm — [Ha]/[Yo'q] bosilganda gateway'ga uzatadi.
// Xodim fikrini o'zgartirsa ikkinchi tugmani bosaveradi — tugmalar qoladi,
// xabar oxiridagi javob qatori yangilanadi.
func handleWorklyConfirm(cb *TgCallback, token, answer string) {
	ans, txt := "no", "❌ Javobingiz qayd etildi: KELMAYMAN"
	if answer == "y" {
		ans, txt = "yes", "✅ Javobingiz qayd etildi: KELAMAN"
	}
	resp, err := worklyHTTP.Get(fmt.Sprintf("%s/confirm/%s/%s", worklyGatewayURL, token, ans))
	if err != nil {
		log.Printf("workly confirm xato: %v", err)
		answerCallback(cb.ID, "Xatolik — birozdan keyin qayta urinib ko'ring")
		return
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		answerCallback(cb.ID, "Bu grafik yozuvi topilmadi (o'zgargan bo'lishi mumkin)")
		return
	}
	answerCallback(cb.ID, txt)
	if cb.Message != nil {
		// Eski javob qatorini olib tashlab, yangisini yozamiz.
		base := cb.Message.Text
		for _, sep := range []string{"\n\n✅ Javobingiz", "\n\n❌ Javobingiz"} {
			if i := strings.Index(base, sep); i >= 0 {
				base = base[:i]
			}
		}
		worklyEditWithButtons(cb.Message.Chat.ID, cb.Message.MessageID,
			base+"\n\n"+txt, token)
	}
}

// worklyEditWithButtons — xabar matnini yangilab, [Ha]/[Yo'q] tugmalarini
// QOLDIRADI (mavjud editMessageText tugmalarni olib tashlab qo'yardi).
func worklyEditWithButtons(chatID, messageID int64, text, token string) {
	payload := map[string]interface{}{
		"chat_id":    chatID,
		"message_id": messageID,
		"text":       text,
		"reply_markup": map[string]interface{}{
			"inline_keyboard": [][]map[string]string{{
				{"text": "✅ Ha, kelaman", "callback_data": "wkc:" + token + ":y"},
				{"text": "❌ Yo'q, kelmayman", "callback_data": "wkc:" + token + ":n"},
			}},
		},
	}
	jsonData, _ := json.Marshal(payload)
	url := fmt.Sprintf("https://api.telegram.org/bot%s/editMessageText", botToken)
	resp, err := http.Post(url, "application/json", bytes.NewReader(jsonData))
	if err != nil {
		log.Printf("workly edit xato: %v", err)
		return
	}
	resp.Body.Close()
}
