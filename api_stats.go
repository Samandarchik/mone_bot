package main

import (
	"net/http"
)

// SourceStat — bitta reklama manbasi bo'yicha aggregatsiya.
// Source bo'sh bo'lsa "" — "manba ko'rsatilmagan" deb hisoblanadi.
type SourceStat struct {
	Source       string `json:"source"`
	Total        int    `json:"total"`
	Pending      int    `json:"pending"`
	Interviewing int    `json:"interviewing"`
	Trial        int    `json:"trial"`
	Accepted     int    `json:"accepted"`
	Rejected     int    `json:"rejected"`
	Reserve      int    `json:"reserve"`
	Noshow       int    `json:"noshow"`
}

// GET /api/stats/sources — rezumeler bo'yicha manba statistikasi (super_admin).
func handleStatsRezumeSources(w http.ResponseWriter, r *http.Request) {
	stats, err := dbSourceStats("rezumeler")
	if err != nil {
		jsonError(w, "DB xato: "+err.Error(), http.StatusInternalServerError)
		return
	}
	jsonResponse(w, stats)
}

// GET /api/stats/ishchi-sources — ishchi anketalar bo'yicha manba statistikasi (super_admin).
func handleStatsIshchiSources(w http.ResponseWriter, r *http.Request) {
	stats, err := dbSourceStats("ishchi_anketalar")
	if err != nil {
		jsonError(w, "DB xato: "+err.Error(), http.StatusInternalServerError)
		return
	}
	jsonResponse(w, stats)
}

// dbSourceStats — table nomi bo'yicha source aggregatsiyasini hisoblaydi.
// Faqat ichki funksiyadan chaqiriladi, table nomi konstant string sifatida beriladi (SQL injection xavfi yo'q).
func dbSourceStats(table string) ([]SourceStat, error) {
	if table != "rezumeler" && table != "ishchi_anketalar" {
		return nil, nil
	}
	query := `SELECT COALESCE(source,'') as src, status, COUNT(*) FROM ` + table + ` GROUP BY src, status ORDER BY src`
	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	bySource := map[string]*SourceStat{}
	for rows.Next() {
		var src, status string
		var n int
		if err := rows.Scan(&src, &status, &n); err != nil {
			return nil, err
		}
		s, ok := bySource[src]
		if !ok {
			s = &SourceStat{Source: src}
			bySource[src] = s
		}
		s.Total += n
		switch status {
		case "pending", "yangi":
			s.Pending += n
		case "interviewing", "qabul":
			s.Interviewing += n
		case "trial":
			s.Trial += n
		case "accepted":
			s.Accepted += n
		case "rejected", "rad":
			s.Rejected += n
		case "reserve":
			s.Reserve += n
		case "noshow":
			s.Noshow += n
		}
	}

	out := make([]SourceStat, 0, len(bySource))
	for _, s := range bySource {
		out = append(out, *s)
	}
	// Total bo'yicha kamayish tartibida
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j].Total > out[j-1].Total; j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out, nil
}
