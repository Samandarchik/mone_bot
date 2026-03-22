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
)

const botToken = "8550220546:AAFEII8AzNdMapEqT_VFtqiqv6h0obbLgzQ"

var positionGroupMap = map[string]int64{
	"Горничная уборщица / Xonadon tozalovchi": -5014841679,
	"Магазинщик / Do'konchi":                  -5170258928,
	"Кухня Салатница / Kuxnya Salatnisa":      -5126056788,
	"Официант / Ofitsiant":                    -5170258928,
	"Повар / Oshpaz":                          -5126056788,
	"Водитель / Haydovchi":                    -5132239156,
}

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
}

const htmlPage = `<!DOCTYPE html>
<html lang="uz">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>MONE - Ishga qabul anketa</title>
<style>
@import url('https://fonts.googleapis.com/css2?family=Inter:wght@400;500;600;700&display=swap');
*{box-sizing:border-box;margin:0;padding:0}
body{font-family:'Inter',system-ui,-apple-system,sans-serif;background:linear-gradient(135deg,#667eea 0%,#764ba2 100%);min-height:100vh}

.header{background:rgba(255,255,255,.15);backdrop-filter:blur(20px);padding:16px 20px;position:sticky;top:0;z-index:100;border-bottom:1px solid rgba(255,255,255,.2)}
.header-inner{max-width:640px;margin:0 auto;display:flex;justify-content:space-between;align-items:center}
.logo{color:#fff;font-size:22px;font-weight:700;letter-spacing:-0.5px}
.logo span{opacity:.7;font-weight:400;font-size:14px;margin-left:8px}
.lang-sw{display:flex;gap:4px;background:rgba(255,255,255,.15);border-radius:10px;padding:3px}
.lang-sw button{padding:6px 16px;border:none;border-radius:8px;background:transparent;color:rgba(255,255,255,.8);font-size:13px;cursor:pointer;font-weight:600;transition:all .2s;font-family:inherit}
.lang-sw button.active{background:#fff;color:#764ba2;box-shadow:0 2px 8px rgba(0,0,0,.15)}

.wrap{max-width:640px;margin:0 auto;padding:24px 16px 40px}

.progress-bar{background:rgba(255,255,255,.2);border-radius:20px;height:6px;margin-bottom:24px;overflow:hidden}
.progress-fill{height:100%;background:#fff;border-radius:20px;transition:width .4s ease;width:0%}

.card{background:#fff;border-radius:16px;padding:20px 24px;margin-bottom:12px;box-shadow:0 2px 12px rgba(0,0,0,.08);transition:transform .2s;animation:fadeUp .4s ease both}
.card:hover{transform:translateY(-1px)}
@keyframes fadeUp{from{opacity:0;transform:translateY(12px)}to{opacity:1;transform:translateY(0)}}

.step-num{display:inline-flex;align-items:center;justify-content:center;width:28px;height:28px;background:linear-gradient(135deg,#667eea,#764ba2);color:#fff;border-radius:50%;font-size:12px;font-weight:700;margin-right:10px;flex-shrink:0}
.lbl{font-size:15px;color:#1a1a2e;margin-bottom:14px;display:flex;align-items:center;font-weight:600}
.bg{display:flex;flex-wrap:wrap;gap:8px}

.btn{
  padding:10px 18px;
  border:2px solid #e8e8ef;
  border-radius:12px;
  background:#fafaff;
  color:#4a4a6a;
  font-size:13px;
  cursor:pointer;
  transition:all .2s;
  font-weight:500;
  font-family:inherit;
}
.btn:hover{border-color:#667eea;background:#f0f0ff;color:#667eea}
.btn.on{
  background:linear-gradient(135deg,#667eea,#764ba2);
  border-color:transparent;
  color:#fff;
  font-weight:600;
  box-shadow:0 4px 12px rgba(102,126,234,.4);
  transform:scale(1.02);
}

input[type=text],input[type=tel],input[type=date],textarea{
  width:100%;
  padding:12px 16px;
  border:2px solid #e8e8ef;
  border-radius:12px;
  font-size:14px;
  background:#fafaff;
  color:#1a1a2e;
  outline:none;
  font-family:inherit;
  transition:all .2s;
}
input:focus,textarea:focus{border-color:#667eea;background:#fff;box-shadow:0 0 0 4px rgba(102,126,234,.12)}
textarea{resize:vertical}

.rrow{display:flex;align-items:center;gap:14px}
.rrow input[type=range]{flex:1;-webkit-appearance:none;height:6px;background:linear-gradient(90deg,#667eea,#764ba2);border-radius:10px;outline:none}
.rrow input[type=range]::-webkit-slider-thumb{-webkit-appearance:none;width:22px;height:22px;background:#fff;border:3px solid #667eea;border-radius:50%;cursor:pointer;box-shadow:0 2px 8px rgba(0,0,0,.15)}
.rval{font-size:22px;font-weight:700;background:linear-gradient(135deg,#667eea,#764ba2);-webkit-background-clip:text;-webkit-text-fill-color:transparent;min-width:55px;text-align:right}
.g2{display:grid;grid-template-columns:1fr 1fr;gap:12px}

.lang-row{display:flex;align-items:center;gap:10px;margin-bottom:12px;flex-wrap:wrap;padding:10px 14px;background:#fafaff;border-radius:12px;border:1px solid #e8e8ef}
.lang-label{font-size:14px;color:#1a1a2e;font-weight:600;min-width:110px}
.lang-btns{display:flex;gap:6px;flex-wrap:wrap}

.lang-btn{
  padding:6px 14px;
  border:2px solid #e8e8ef;
  border-radius:10px;
  background:#fff;
  color:#4a4a6a;
  font-size:12px;
  cursor:pointer;
  transition:all .2s;
  font-weight:500;
  font-family:inherit;
}
.lang-btn:hover{border-color:#667eea;color:#667eea}
.lang-btn.on{
  background:linear-gradient(135deg,#667eea,#764ba2);
  border-color:transparent;
  color:#fff;
  font-weight:600;
}

.photo-box{border:2px dashed #d0d0e0;border-radius:16px;padding:2rem;text-align:center;cursor:pointer;background:#fafaff;transition:all .2s}
.photo-box:hover{border-color:#667eea;background:#f0f0ff}

.save-btn{width:100%;padding:16px;background:linear-gradient(135deg,#22c55e,#16a34a);border:none;border-radius:14px;color:#fff;font-size:16px;font-weight:700;cursor:pointer;margin-top:8px;transition:all .2s;font-family:inherit;box-shadow:0 4px 16px rgba(34,197,94,.4)}
.save-btn:hover{transform:translateY(-2px);box-shadow:0 6px 20px rgba(34,197,94,.5)}
.save-btn:active{transform:translateY(0)}
.save-btn:disabled{opacity:.5;cursor:not-allowed;transform:none;box-shadow:none}

.msg{border-radius:14px;padding:16px 20px;font-size:14px;margin-top:12px;display:none;font-weight:600;animation:fadeUp .3s ease}
.msg.ok{background:#dcfce7;color:#166534;border:2px solid #86efac}
.msg.err{background:#fee2e2;color:#991b1b;border:2px solid #fca5a5}

.add-lang{display:flex;gap:8px;margin-top:12px}
.add-lang input{flex:1}
.add-lang button{padding:10px 18px;border:2px solid #e8e8ef;border-radius:12px;background:#fafaff;color:#4a4a6a;font-size:13px;cursor:pointer;white-space:nowrap;font-weight:600;transition:all .2s;font-family:inherit}
.add-lang button:hover{background:#f0f0ff;border-color:#667eea;color:#667eea}

.added-lang{display:flex;flex-wrap:wrap;gap:8px;margin-top:10px}
.lang-tag{display:flex;align-items:center;gap:8px;background:#eff6ff;border:2px solid #bfdbfe;border-radius:10px;padding:6px 14px;font-size:13px;color:#1e40af;font-weight:500}
.lang-tag select{font-size:12px;border:1px solid #bfdbfe;border-radius:6px;background:#fff;color:#1e40af;padding:3px 6px;font-family:inherit}
.lang-tag button{background:none;border:none;cursor:pointer;color:#94a3b8;font-size:16px;padding:0;line-height:1;transition:color .2s}
.lang-tag button:hover{color:#ef4444}

.sec-title{font-size:11px;color:#94a3b8;text-transform:uppercase;letter-spacing:1px;margin-bottom:8px;margin-top:4px;font-weight:700}

.overlay{display:none;position:fixed;top:0;left:0;width:100%;height:100%;background:rgba(0,0,0,.92);z-index:9999;justify-content:center;align-items:center;cursor:pointer;backdrop-filter:blur(10px);animation:fadeIn .2s ease}
.overlay.show{display:flex}
.overlay img{max-width:92%;max-height:92%;object-fit:contain;border-radius:12px;box-shadow:0 20px 60px rgba(0,0,0,.5)}
@keyframes fadeIn{from{opacity:0}to{opacity:1}}

.sample-img{cursor:pointer;transition:all .25s;border-radius:12px !important}
.sample-img:hover{transform:scale(1.08);box-shadow:0 8px 24px rgba(0,0,0,.2)}

.photo-samples{display:flex;gap:14px;justify-content:center;flex-wrap:wrap;margin-bottom:16px}

.date-display{display:flex;align-items:center;gap:10px;padding:12px 16px;border:2px solid #e8e8ef;border-radius:12px;background:#fafaff;cursor:pointer;transition:all .2s;font-size:14px;color:#1a1a2e;font-weight:500}
.date-display:hover{border-color:#667eea}
.date-display .placeholder{color:#94a3b8;font-weight:400}

.picker-overlay{display:none;position:fixed;top:0;left:0;width:100%;height:100%;background:rgba(0,0,0,.5);z-index:10000;justify-content:center;align-items:flex-end;backdrop-filter:blur(4px)}
.picker-overlay.show{display:flex}

.picker-sheet{background:#f2f2f7;border-radius:20px 20px 0 0;width:100%;max-width:640px;padding:0;animation:slideUp .3s ease}
@keyframes slideUp{from{transform:translateY(100%)}to{transform:translateY(0)}}

.picker-header{display:flex;justify-content:space-between;align-items:center;padding:14px 20px;border-bottom:1px solid #d1d1d6}
.picker-header button{background:none;border:none;font-size:16px;font-weight:600;cursor:pointer;font-family:inherit;padding:4px 8px}
.picker-cancel{color:#8e8e93}
.picker-done{color:#007aff}

.picker-body{display:flex;height:220px;overflow:hidden;position:relative;background:#fff;border-radius:0 0 20px 20px}
.picker-body::before{content:'';position:absolute;top:0;left:0;right:0;height:88px;background:linear-gradient(to bottom,rgba(255,255,255,.95),rgba(255,255,255,.6));z-index:2;pointer-events:none}
.picker-body::after{content:'';position:absolute;bottom:0;left:0;right:0;height:88px;background:linear-gradient(to top,rgba(255,255,255,.95),rgba(255,255,255,.6));z-index:2;pointer-events:none}
.picker-highlight{position:absolute;top:50%;left:0;right:0;height:44px;transform:translateY(-50%);background:rgba(120,120,128,.12);border-top:0.5px solid rgba(60,60,67,.15);border-bottom:0.5px solid rgba(60,60,67,.15);z-index:1;pointer-events:none}

.picker-col{flex:1;overflow-y:scroll;-webkit-overflow-scrolling:touch;scroll-snap-type:y mandatory;position:relative;z-index:3}
.picker-col::-webkit-scrollbar{display:none}
.picker-col{-ms-overflow-style:none;scrollbar-width:none}
.picker-item{height:44px;display:flex;align-items:center;justify-content:center;font-size:20px;color:#1c1c1e;scroll-snap-align:center;font-weight:400;opacity:.3;transition:opacity .15s}
.picker-item.active{opacity:1;font-weight:600}
.picker-spacer{height:88px}

.picker-col-sep{width:1px;background:rgba(60,60,67,.08);align-self:stretch;margin:44px 0}

@media(max-width:480px){
  .g2{grid-template-columns:1fr}
  .card{padding:16px 18px}
  .lang-row{flex-direction:column;align-items:flex-start}
  .btn{padding:9px 14px;font-size:12px}
}
</style>
</head>
<body>

<div class="overlay" id="img-overlay" onclick="this.classList.remove('show')">
  <img id="overlay-img" src="">
</div>

<div class="header">
  <div class="header-inner">
    <div class="logo">MONE <span data-uz="Anketa" data-ru="Анкета">Anketa</span></div>
    <div class="lang-sw">
      <button class="active" onclick="switchLang('uz')">UZ</button>
      <button onclick="switchLang('ru')">RU</button>
    </div>
  </div>
</div>

<div class="wrap">

<div class="progress-bar"><div class="progress-fill" id="progress"></div></div>

<div class="card" style="animation-delay:.05s">
  <span class="lbl"><span class="step-num">1</span><span data-uz="Lavozim tanlang" data-ru="Выберите должность">Lavozim tanlang</span></span>
  <div class="bg" id="g-lavozim">
    <button class="btn" onclick="sel(this,'lavozim')">Горничная уборщица / Xonadon tozalovchi</button>
    <button class="btn" onclick="sel(this,'lavozim')">Магазинщик / Do'konchi</button>
    <button class="btn" onclick="sel(this,'lavozim')">Кухня Салатница / Kuxnya Salatnisa</button>
    <button class="btn" onclick="sel(this,'lavozim')">Официант / Ofitsiant</button>
    <button class="btn" onclick="sel(this,'lavozim')">Повар / Oshpaz</button>
    <button class="btn" onclick="sel(this,'lavozim')">Водитель / Haydovchi</button>
  </div>
</div>

<div class="card" style="animation-delay:.1s">
  <span class="lbl"><span class="step-num">2</span><span data-uz="F.I.Sh" data-ru="ФИО">F.I.Sh</span></span>
  <div style="display:flex;flex-direction:column;gap:10px">
    <input type="text" id="familiya" data-uz-ph="Familiya" data-ru-ph="Фамилия" placeholder="Familiya">
    <input type="text" id="ism" data-uz-ph="Ism" data-ru-ph="Имя" placeholder="Ism">
    <input type="text" id="sharif" data-uz-ph="Sharif" data-ru-ph="Отчество" placeholder="Sharif">
  </div>
</div>

<div class="card" style="animation-delay:.15s">
  <span class="lbl"><span class="step-num">3</span><span data-uz="Tug'ilgan sana" data-ru="Дата рождения">Tug'ilgan sana</span></span>
  <div class="date-display" id="date-display" onclick="openDatePicker()">
    <span id="date-text" class="placeholder" data-uz="Sanani tanlang" data-ru="Выберите дату">Sanani tanlang</span>
  </div>
  <input type="hidden" id="tugilgan">
</div>

<div class="picker-overlay" id="picker-overlay" onclick="if(event.target===this)closeDatePicker()">
  <div class="picker-sheet">
    <div class="picker-header">
      <button class="picker-cancel" onclick="closeDatePicker()" data-uz="Bekor" data-ru="Отмена">Bekor</button>
      <span style="font-weight:600;font-size:16px;color:#1c1c1e" data-uz="Tug'ilgan sana" data-ru="Дата рождения">Tug'ilgan sana</span>
      <button class="picker-done" onclick="confirmDate()" data-uz="Tayyor" data-ru="Готово">Tayyor</button>
    </div>
    <div class="picker-body">
      <div class="picker-col" id="pick-day"></div>
      <div class="picker-col-sep"></div>
      <div class="picker-col" id="pick-month"></div>
      <div class="picker-col-sep"></div>
      <div class="picker-col" id="pick-year"></div>
      <div class="picker-highlight"></div>
    </div>
  </div>
</div>

<div class="card g2" style="animation-delay:.2s">
  <div>
    <span class="lbl"><span class="step-num">4</span><span data-uz="Bo'y (sm)" data-ru="Рост (см)">Bo'y (sm)</span></span>
    <div class="rrow">
      <input type="range" min="150" max="210" step="1" value="170" id="boy" oninput="document.getElementById('bv').textContent=this.value;updateProgress()">
      <span class="rval"><span id="bv">170</span></span>
    </div>
  </div>
  <div>
    <span class="lbl"><span class="step-num">5</span><span data-uz="Vazn (kg)" data-ru="Вес (кг)">Vazn (kg)</span></span>
    <div class="rrow">
      <input type="range" min="50" max="120" step="1" value="70" id="vazn" oninput="document.getElementById('vv').textContent=this.value;updateProgress()">
      <span class="rval"><span id="vv">70</span></span>
    </div>
  </div>
</div>

<div class="card" style="animation-delay:.25s">
  <span class="lbl"><span class="step-num">6</span><span data-uz="Yashash manzili" data-ru="Адрес проживания">Yashash manzili</span></span>
  <input type="text" id="adres" data-uz-ph="Shahar, ko'cha, uy raqami" data-ru-ph="Город, улица, дом" placeholder="Shahar, ko'cha, uy raqami">
</div>

<div class="card" style="animation-delay:.3s">
  <span class="lbl"><span class="step-num">7</span><span data-uz="Mo'ljal" data-ru="Ориентир">Mo'ljal</span></span>
  <input type="text" id="moljal" data-uz-ph="Yaqin joy, bino nomi" data-ru-ph="Ближайшее место, здание" placeholder="Yaqin joy, bino nomi">
</div>

<div class="card" style="animation-delay:.35s">
  <span class="lbl"><span class="step-num">8</span><span data-uz="Umumiy tajriba" data-ru="Общий стаж">Umumiy tajriba</span></span>
  <input type="text" id="tajriba" data-uz-ph="Masalan: 3 yil 6 oy" data-ru-ph="Например: 3 года 6 месяцев" placeholder="Masalan: 3 yil 6 oy">
</div>

<div class="card" style="animation-delay:.4s">
  <span class="lbl"><span class="step-num">9</span><span data-uz="Chet el tajribasi" data-ru="Работа за рубежом">Chet el tajribasi</span></span>
  <div class="bg" id="g-chet">
    <button class="btn" data-uz="Bor" data-ru="Есть" onclick="sel(this,'chet')">Bor</button>
    <button class="btn" data-uz="Yo'q" data-ru="Нет" onclick="sel(this,'chet')">Yo'q</button>
  </div>
</div>

<div class="card" style="animation-delay:.45s">
  <span class="lbl"><span class="step-num">10</span><span data-uz="Ma'lumot darajasi" data-ru="Уровень образования">Ma'lumot darajasi</span></span>
  <div class="bg" id="g-malumot">
    <button class="btn" data-uz="O'rta" data-ru="Среднее" onclick="sel(this,'malumot')">O'rta</button>
    <button class="btn" data-uz="O'rta maxsus" data-ru="Среднее специальное" onclick="sel(this,'malumot')">O'rta maxsus</button>
    <button class="btn" data-uz="Oliy" data-ru="Высшее" onclick="sel(this,'malumot')">Oliy</button>
  </div>
</div>

<div class="card" style="animation-delay:.5s">
  <span class="lbl"><span class="step-num">11</span><span data-uz="Oilaviy holat" data-ru="Семейное положение">Oilaviy holat</span></span>
  <div class="bg" id="g-oila">
    <button class="btn" data-uz="Oilaviy" data-ru="Женат/Замужем" onclick="sel(this,'oila')">Oilaviy</button>
    <button class="btn" data-uz="Oilasiz" data-ru="Не женат/Не замужем" onclick="sel(this,'oila')">Oilasiz</button>
  </div>
</div>

<div class="card" style="animation-delay:.55s">
  <span class="lbl"><span class="step-num">12</span><span data-uz="Tillar" data-ru="Языки">Tillar</span></span>
  <div class="lang-row">
    <span class="lang-label" data-uz="Ingliz tili" data-ru="Английский">Ingliz tili</span>
    <div class="lang-btns" id="lg-en">
      <button class="lang-btn" data-uz="Bilmayman" data-ru="Не знаю" onclick="setLang(this,'en','bilmayman')">Bilmayman</button>
      <button class="lang-btn" data-uz="Bilaman" data-ru="Знаю" onclick="setLang(this,'en','bilaman')">Bilaman</button>
      <button class="lang-btn" data-uz="Zo'r bilaman" data-ru="Отлично" onclick="setLang(this,'en','zor bilaman')">Zo'r bilaman</button>
    </div>
  </div>
  <div class="lang-row">
    <span class="lang-label" data-uz="Rus tili" data-ru="Русский">Rus tili</span>
    <div class="lang-btns" id="lg-ru">
      <button class="lang-btn" data-uz="Bilmayman" data-ru="Не знаю" onclick="setLang(this,'ru','bilmayman')">Bilmayman</button>
      <button class="lang-btn" data-uz="Bilaman" data-ru="Знаю" onclick="setLang(this,'ru','bilaman')">Bilaman</button>
      <button class="lang-btn" data-uz="Zo'r bilaman" data-ru="Отлично" onclick="setLang(this,'ru','zor bilaman')">Zo'r bilaman</button>
    </div>
  </div>
  <div class="lang-row">
    <span class="lang-label" data-uz="O'zbek tili" data-ru="Узбекский">O'zbek tili</span>
    <div class="lang-btns" id="lg-uz">
      <button class="lang-btn" data-uz="Bilmayman" data-ru="Не знаю" onclick="setLang(this,'uz','bilmayman')">Bilmayman</button>
      <button class="lang-btn" data-uz="Bilaman" data-ru="Знаю" onclick="setLang(this,'uz','bilaman')">Bilaman</button>
      <button class="lang-btn" data-uz="Zo'r bilaman" data-ru="Отлично" onclick="setLang(this,'uz','zor bilaman')">Zo'r bilaman</button>
    </div>
  </div>
  <div style="margin-top:14px">
    <span class="sec-title" data-uz="Qo'shimcha til" data-ru="Дополнительный язык">Qo'shimcha til</span>
    <div class="add-lang">
      <input type="text" id="extra-lang-name" data-uz-ph="Til nomi (masalan: Turk tili)" data-ru-ph="Название языка (например: Турецкий)" placeholder="Til nomi (masalan: Turk tili)">
      <button onclick="addExtraLang()" data-uz="+ Qo'shish" data-ru="+ Добавить">+ Qo'shish</button>
    </div>
    <div id="extra-langs" class="added-lang"></div>
  </div>
</div>

<div class="card" style="animation-delay:.6s">
  <span class="lbl"><span class="step-num">13</span><span data-uz="Telefon" data-ru="Телефон">Telefon</span></span>
  <div id="tel-wrap" style="display:flex;align-items:center;border:2px solid #e8e8ef;border-radius:12px;background:#fafaff;transition:all .2s;cursor:text" onclick="document.getElementById('telefon').focus()">
    <span style="padding:12px 4px 12px 16px;font-size:14px;color:#1a1a2e;white-space:nowrap">+998</span>
    <input type="tel" id="telefon" placeholder="901234567" maxlength="9" style="border:none;background:transparent;box-shadow:none;padding:12px 16px 12px 4px;outline:none" oninput="this.value=this.value.replace(/[^0-9]/g,'')" onfocus="document.getElementById('tel-wrap').style.borderColor='#667eea';document.getElementById('tel-wrap').style.background='#fff';document.getElementById('tel-wrap').style.boxShadow='0 0 0 4px rgba(102,126,234,.12)'" onblur="document.getElementById('tel-wrap').style.borderColor='#e8e8ef';document.getElementById('tel-wrap').style.background='#fafaff';document.getElementById('tel-wrap').style.boxShadow='none'">
  </div>
</div>

<div class="card" style="animation-delay:.65s">
  <span class="lbl"><span class="step-num">14</span><span data-uz="Qo'shimcha ma'lumot" data-ru="Дополнительная информация">Qo'shimcha ma'lumot</span></span>
  <textarea id="extra" rows="3" data-uz-ph="Boshqa ko'nikmalar, izohlar..." data-ru-ph="Другие навыки, комментарии..." placeholder="Boshqa ko'nikmalar, izohlar..."></textarea>
</div>

<div class="card" style="animation-delay:.7s">
  <span class="lbl"><span class="step-num">15</span><span data-uz="Rasm yuklash" data-ru="Загрузить фото">Rasm yuklash</span></span>
  <div style="margin-bottom:16px">
    <p style="font-size:13px;color:#64748b;margin-bottom:10px;font-weight:600" data-uz="Namuna — rasmingiz shunday bo'lishi kerak:" data-ru="Образец — ваше фото должно быть таким:">Namuna — rasmingiz shunday bo'lishi kerak:</p>
    <div class="photo-samples">
      <img src="https://moneapp.monebakeryuz.uz/static/1_resized.jpg" class="sample-img" style="width:120px;height:160px;object-fit:cover;border:3px solid #e8e8ef" onclick="showImg(this.src)">
      <img src="https://moneapp.monebakeryuz.uz/static/2_resized.jpg" class="sample-img" style="width:120px;height:160px;object-fit:cover;border:3px solid #e8e8ef" onclick="showImg(this.src)">
    </div>
  </div>
  <div class="photo-box" onclick="document.getElementById('photo-inp').click()" id="photo-box">
    <div id="photo-content">
      <div style="font-size:36px;color:#c0c0d0">+</div>
      <p style="font-size:14px;color:#64748b;margin-top:8px;font-weight:500" data-uz="Rasmingizni yuklang" data-ru="Загрузите ваше фото">Rasmingizni yuklang</p>
      <p style="font-size:12px;color:#94a3b8;margin-top:4px">JPG, PNG — max 5MB</p>
    </div>
  </div>
  <input type="file" id="photo-inp" accept="image/*" style="display:none" onchange="prevPhoto(this)">
</div>

<button class="save-btn" id="save-btn" onclick="submitForm()" data-uz="Saqlash va yuborish" data-ru="Сохранить и отправить">Saqlash va yuborish</button>
<div class="msg ok" id="msg-ok" data-uz="Anketa muvaffaqiyatli yuborildi!" data-ru="Анкета успешно отправлена!">Anketa muvaffaqiyatli yuborildi!</div>
<div class="msg err" id="msg-err">Xatolik yuz berdi.</div>
</div>

<script>
let curLang='uz';

function switchLang(lang){
  curLang=lang;
  document.querySelectorAll('.lang-sw button').forEach(b=>b.classList.remove('active'));
  document.querySelector('.lang-sw button[onclick="switchLang(\''+lang+'\')"]').classList.add('active');
  document.querySelectorAll('[data-uz]').forEach(el=>{
    const text=el.getAttribute('data-'+lang);
    if(text) el.textContent=text;
  });
  document.querySelectorAll('[data-uz-ph]').forEach(el=>{
    const ph=el.getAttribute('data-'+lang+'-ph');
    if(ph) el.placeholder=ph;
  });
}

const months=['Yanvar','Fevral','Mart','Aprel','May','Iyun','Iyul','Avgust','Sentyabr','Oktyabr','Noyabr','Dekabr'];
const monthsRu=['Январь','Февраль','Март','Апрель','Май','Июнь','Июль','Август','Сентябрь','Октябрь','Ноябрь','Декабрь'];
let pickDay=15,pickMonth=5,pickYear=2000;

function buildPickerCol(col,items,selected,onScroll){
  col.innerHTML='<div class="picker-spacer"></div>'+items.map((item,i)=>'<div class="picker-item" data-idx="'+i+'">'+item+'</div>').join('')+'<div class="picker-spacer"></div>';
  const target=col.querySelector('[data-idx="'+selected+'"]');
  if(target) col.scrollTop=target.offsetTop-col.offsetTop-88;
  updateActive(col);
  col.onscroll=()=>{updateActive(col);if(onScroll)onScroll()};
}

function updateActive(col){
  const items=col.querySelectorAll('.picker-item');
  const center=col.scrollTop+col.offsetHeight/2;
  items.forEach(item=>{
    const itemCenter=item.offsetTop+22;
    const dist=Math.abs(center-itemCenter);
    item.classList.toggle('active',dist<22);
  });
}

function getSelectedIdx(col){
  const items=col.querySelectorAll('.picker-item');
  const center=col.scrollTop+col.offsetHeight/2;
  let closest=0,minDist=Infinity;
  items.forEach((item,i)=>{
    const d=Math.abs(item.offsetTop+22-center);
    if(d<minDist){minDist=d;closest=i}
  });
  return closest;
}

function snapTo(col,idx){
  const item=col.querySelector('[data-idx="'+idx+'"]');
  if(item) col.scrollTo({top:item.offsetTop-col.offsetTop-88,behavior:'smooth'});
}

function openDatePicker(){
  const dayCol=document.getElementById('pick-day');
  const monthCol=document.getElementById('pick-month');
  const yearCol=document.getElementById('pick-year');
  const days=Array.from({length:31},(_,i)=>String(i+1).padStart(2,'0'));
  const mList=curLang==='ru'?monthsRu:months;
  const years=[];for(let y=1960;y<=2010;y++)years.push(String(y));
  buildPickerCol(dayCol,days,pickDay-1);
  buildPickerCol(monthCol,mList,pickMonth);
  buildPickerCol(yearCol,years,pickYear-1960);
  document.getElementById('picker-overlay').classList.add('show');
  document.body.style.overflow='hidden';
  setTimeout(()=>{snapTo(dayCol,pickDay-1);snapTo(monthCol,pickMonth);snapTo(yearCol,pickYear-1960)},50);
}

function closeDatePicker(){
  document.getElementById('picker-overlay').classList.remove('show');
  document.body.style.overflow='';
}

function confirmDate(){
  const dayCol=document.getElementById('pick-day');
  const monthCol=document.getElementById('pick-month');
  const yearCol=document.getElementById('pick-year');
  const d=getSelectedIdx(dayCol)+1;
  const m=getSelectedIdx(monthCol);
  const y=getSelectedIdx(yearCol)+1960;
  pickDay=d;pickMonth=m;pickYear=y;
  const mList=curLang==='ru'?monthsRu:months;
  const dateStr=String(d).padStart(2,'0')+'.'+String(m+1).padStart(2,'0')+'.'+y;
  const displayStr=d+' '+mList[m]+' '+y;
  document.getElementById('tugilgan').value=dateStr;
  const txt=document.getElementById('date-text');
  txt.textContent=displayStr;
  txt.classList.remove('placeholder');
  closeDatePicker();
  updateProgress();
}

function showImg(src){
  document.getElementById('overlay-img').src=src;
  document.getElementById('img-overlay').classList.add('show');
}

const state={lavozim:'',chet:'',malumot:'',oila:''};
const langs={en:'',ru:'',uz:''};
const extraLangs=[];
let photoBase64=null;

function updateProgress(){
  let filled=0;const total=15;
  if(state.lavozim)filled++;
  if(document.getElementById('familiya').value)filled++;
  if(document.getElementById('ism').value)filled++;
  if(document.getElementById('tugilgan').value)filled++;
  filled++;filled++;
  if(document.getElementById('adres').value)filled++;
  if(document.getElementById('moljal').value)filled++;
  if(document.getElementById('tajriba').value)filled++;
  if(state.chet)filled++;
  if(state.malumot)filled++;
  if(state.oila)filled++;
  if(langs.en||langs.ru||langs.uz)filled++;
  if(document.getElementById('telefon').value)filled++;
  if(photoBase64)filled++;
  document.getElementById('progress').style.width=Math.round(filled/total*100)+'%';
}

document.addEventListener('input',updateProgress);

function sel(btn,group){
  document.getElementById('g-'+group).querySelectorAll('.btn').forEach(b=>b.classList.remove('on'));
  btn.classList.add('on');
  state[group]=btn.textContent.trim();
  updateProgress();
}

function setLang(btn,lang,val){
  document.getElementById('lg-'+lang).querySelectorAll('.lang-btn').forEach(b=>b.classList.remove('on'));
  btn.classList.add('on');
  langs[lang]=val;
  updateProgress();
}

function addExtraLang(){
  const inp=document.getElementById('extra-lang-name');
  const name=inp.value.trim();
  if(!name)return;
  const idx=extraLangs.length;
  extraLangs.push({til:name,daraja:''});
  const container=document.getElementById('extra-langs');
  const tag=document.createElement('div');
  tag.className='lang-tag';
  tag.id='etag-'+idx;
  tag.innerHTML='<span>'+name+'</span>'
    +'<select onchange="extraLangs['+idx+'].daraja=this.value"><option value="">Daraja</option><option>Bilmayman</option><option>Bilaman</option><option>Zo\'r bilaman</option></select>'
    +'<button onclick="removeExtraLang('+idx+')" title="O\'chirish">&#x2715;</button>';
  container.appendChild(tag);
  inp.value='';
}

function removeExtraLang(idx){
  extraLangs.splice(idx,1);
  rebuildExtra();
}

function rebuildExtra(){
  const c=document.getElementById('extra-langs');
  c.innerHTML='';
  extraLangs.forEach((l,i)=>{
    const tag=document.createElement('div');
    tag.className='lang-tag';
    tag.innerHTML='<span>'+l.til+'</span>'
      +'<select onchange="extraLangs['+i+'].daraja=this.value"><option value="">Daraja</option><option>Bilmayman</option><option>Bilaman</option><option>Zo\'r bilaman</option></select>'
      +'<button onclick="removeExtraLang('+i+')" title="O\'chirish">&#x2715;</button>';
    c.appendChild(tag);
  });
}

function prevPhoto(input){
  if(!input.files[0])return;
  const reader=new FileReader();
  reader.onload=e=>{
    photoBase64=e.target.result;
    document.getElementById('photo-content').innerHTML='<img src="'+e.target.result+'" style="width:100px;height:100px;border-radius:50%;object-fit:cover;border:3px solid #667eea;box-shadow:0 4px 16px rgba(102,126,234,.3)"><p style="font-size:13px;margin-top:10px;color:#64748b;font-weight:500">'+input.files[0].name+'</p>';
    updateProgress();
  };
  reader.readAsDataURL(input.files[0]);
}

async function submitForm(){
  const required=[
    {id:'familiya',uz:'Familiya',ru:'Фамилия'},
    {id:'ism',uz:'Ism',ru:'Имя'},
    {id:'sharif',uz:'Sharif',ru:'Отчество'},
    {id:'tugilgan',uz:'Tug\'ilgan sana',ru:'Дата рождения'},
    {id:'boy',uz:'Bo\'y',ru:'Рост'},
    {id:'vazn',uz:'Vazn',ru:'Вес'},
    {id:'adres',uz:'Yashash manzili',ru:'Адрес проживания'},
    {id:'moljal',uz:'Mo\'ljal',ru:'Ориентир'},
    {id:'tajriba',uz:'Umumiy tajriba',ru:'Общий опыт'},
    {id:'telefon',uz:'Telefon',ru:'Телефон'}
  ];
  const stateRequired=[
    {key:'lavozim',uz:'Lavozim',ru:'Должность'},
    {key:'chet',uz:'Chet el tajribasi',ru:'Опыт за рубежом'},
    {key:'malumot',uz:'Ma\'lumot',ru:'Образование'},
    {key:'oila',uz:'Oilaviy holat',ru:'Семейное положение'}
  ];
  const missing=[];
  for(const f of required){
    if(!document.getElementById(f.id).value.trim()) missing.push(curLang==='uz'?f.uz:f.ru);
  }
  for(const s of stateRequired){
    if(!state[s.key]) missing.push(curLang==='uz'?s.uz:s.ru);
  }
  if(!photoBase64) missing.push(curLang==='uz'?'Rasm':'Фото');
  if(missing.length>0){
    document.getElementById('msg-err').textContent=(curLang==='uz'?'Iltimos, to\'ldiring: ':'Пожалуйста, заполните: ')+missing.join(', ');
    document.getElementById('msg-err').style.display='block';
    return;
  }
  const telVal=document.getElementById('telefon').value.replace(/[^0-9]/g,'');
  if(telVal.length!==9){
    document.getElementById('msg-err').textContent=curLang==='uz'?'Telefon raqam 9 ta raqamdan iborat bo\'lishi kerak':'Номер телефона должен содержать 9 цифр';
    document.getElementById('msg-err').style.display='block';
    return;
  }
  const btn=document.getElementById('save-btn');
  btn.disabled=true;
  btn.textContent=curLang==='uz'?'Yuborilmoqda...':'Отправляется...';
  document.getElementById('msg-ok').style.display='none';
  document.getElementById('msg-err').style.display='none';

  const allLangs=[
    {til:'Ingliz tili',daraja:langs.en},
    {til:'Rus tili',daraja:langs.ru},
    {til:"O'zbek tili",daraja:langs.uz},
    ...extraLangs
  ];

  const payload={
    lavozim:state.lavozim,
    familiya:document.getElementById('familiya').value,
    ism:document.getElementById('ism').value,
    sharif:document.getElementById('sharif').value,
    tugilgan_sana:document.getElementById('tugilgan').value,
    boy_sm:parseInt(document.getElementById('boy').value),
    vazn_kg:parseInt(document.getElementById('vazn').value),
    yashash_manzili:document.getElementById('adres').value,
    moljal:document.getElementById('moljal').value,
    umumiy_tajriba:document.getElementById('tajriba').value,
    chet_el_tajribasi:state.chet,
    malumot:state.malumot,
    oilaviy_holat:state.oila,
    tillar:allLangs,
    telefon:'+998'+document.getElementById('telefon').value,
    qoshimcha:document.getElementById('extra').value,
    rasm:photoBase64
  };

  try{
    const res=await fetch('/rezume',{
      method:'POST',
      headers:{'Content-Type':'application/json'},
      body:JSON.stringify(payload)
    });
    if(res.ok){
      document.getElementById('msg-ok').style.display='block';
      btn.textContent=curLang==='uz'?'Yuborildi!':'Отправлено!';
      document.getElementById('progress').style.width='100%';
    }else{
      const errText=await res.text();
      throw new Error(errText);
    }
  }catch(e){
    document.getElementById('msg-err').textContent=(curLang==='uz'?'Xatolik: ':'Ошибка: ')+e.message;
    document.getElementById('msg-err').style.display='block';
    btn.disabled=false;
    btn.textContent=curLang==='uz'?'Saqlash va yuborish':'Сохранить и отправить';
  }
}
</script>
</body>
</html>`

func main() {
	http.HandleFunc("/rezume", handleRezume)
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, htmlPage)
	})

	log.Println("Server ishga tushdi: http://localhost:3333")
	log.Fatal(http.ListenAndServe(":3333", nil))
}

func handleRezume(w http.ResponseWriter, r *http.Request) {
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}
	if r.Method != "POST" {
		http.Error(w, "Faqat POST", http.StatusMethodNotAllowed)
		return
	}

	var anketa Anketa
	if err := json.NewDecoder(r.Body).Decode(&anketa); err != nil {
		http.Error(w, "JSON xato: "+err.Error(), http.StatusBadRequest)
		return
	}

	var tillarStr string
	for _, t := range anketa.Tillar {
		if t.Daraja != "" {
			tillarStr += fmt.Sprintf("  %s: %s\n", t.Til, t.Daraja)
		}
	}
	if tillarStr == "" {
		tillarStr = "  —\n"
	}

	fio := strings.TrimSpace(anketa.Familiya + " " + anketa.Ism + " " + anketa.Sharif)

	caption := fmt.Sprintf(
		"Должность: %s\n"+
			"ФИО: %s\n"+
			"Дата рождения: %s\n"+
			"Рост: %d см\n"+
			"Вес: %d кг\n"+
			"Адрес: %s\n"+
			"Ориентир: %s\n"+
			"Общий стаж: %s\n"+
			"Работа за рубежом: %s\n"+
			"Образование: %s\n"+
			"Семейное положение: %s\n"+
			"Tillar:\n%s"+
			"Телефон: %s\n"+
			"Qo'shimcha: %s\n"+
			"━━━━━━━━━━━━━━━━━━━━",
		anketa.Lavozim, fio, anketa.TugilganSana,
		anketa.BoySm, anketa.VaznKg,
		anketa.YashashManzili, anketa.Moljal,
		anketa.UmumiyTajriba, anketa.ChetElTajribasi,
		anketa.Malumot, anketa.OilaviyHolat, tillarStr,
		anketa.Telefon, anketa.Qoshimcha,
	)

	groupID, ok := positionGroupMap[anketa.Lavozim]
	if !ok {
		groupID = -1003862297561
	}

	var err error
	if anketa.Rasm != "" && strings.Contains(anketa.Rasm, ",") {
		err = sendPhotoToTelegram(groupID, anketa.Rasm, caption)
	} else {
		err = sendMessageToTelegram(groupID, caption)
	}

	if err != nil {
		log.Printf("Telegram xato: %v", err)
		http.Error(w, "Telegram yuborishda xato: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	log.Printf("Anketa yuborildi: %s -> guruh %d", fio, groupID)
}

func sendPhotoToTelegram(chatID int64, base64Data, caption string) error {
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

func sendMessageToTelegram(chatID int64, text string) error {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", botToken)
	payload := map[string]interface{}{
		"chat_id": chatID,
		"text":    text,
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
