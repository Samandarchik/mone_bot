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
	"fmt"
	"log"
	"net/http"
	"time"
)

// worklyGatewayURL — isup-gateway shu serverning o'zida ishlaydi.
const worklyGatewayURL = "http://127.0.0.1:8092"

var worklyHTTP = &http.Client{Timeout: 10 * time.Second}

// handleWorklyConfirm — [Ha]/[Yo'q] bosilganda gateway'ga uzatadi.
// Javobdan keyin tugmalar OLIB TASHLANADI (bitta javob) — fikr o'zgarsa
// xodim menejerga aytadi, menejer grafikni tahrirlaydi.
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
	// editMessageText reply_markup yubormaydi — tugmalar o'zi yo'qoladi.
	if cb.Message != nil {
		editMessageText(cb.Message.Chat.ID, cb.Message.MessageID,
			cb.Message.Text+"\n\n"+txt)
	}
}
