package main

import "strings"

// Bitta foydalanuvchi bir vaqtda bir nechta rolga ega bo'lishi mumkin
// (masalan ham `admin`, ham `ishchi_admin`). Rollar `users.roles` ustunida
// vergul bilan ajratilgan holda saqlanadi (masalan "admin,ishchi_admin").
// Eski yozuvlarda bu ustun bo'sh — bunda eski bitta `role` ustuni asos qilib
// olinadi (backward compatible). super_admin eksklyuziv: u boshqa rollar
// bilan birlashtirilmaydi.

// admin va ishchi_admin — super_admin tomonidan tayinlanadigan rollar.
var validAssignableRoles = map[string]bool{"admin": true, "ishchi_admin": true}

// validRole — UI/API'dan kelishi mumkin bo'lgan barcha rollar.
func validRole(r string) bool {
	return r == "admin" || r == "ishchi_admin" || r == "super_admin"
}

// contains — ro'yxatda qiymat bor-yo'qligi.
func contains(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}

// splitRoles — CSV ("admin,ishchi_admin") → toza, takrorlanmaydigan ro'yxat.
func splitRoles(csv string) []string {
	parts := strings.Split(csv, ",")
	out := make([]string, 0, len(parts))
	seen := map[string]bool{}
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" || seen[p] {
			continue
		}
		seen[p] = true
		out = append(out, p)
	}
	return out
}

// normalizeRoles — kiruvchi rollar ro'yxati (va eski bitta role'dan) yakuniy
// effektiv rollar ro'yxatini hosil qiladi. super_admin bo'lsa faqat
// ["super_admin"] qaytadi (eksklyuziv). Hech narsa qolmasa — ["admin"].
func normalizeRoles(roles []string, fallback string) []string {
	src := roles
	if len(src) == 0 && fallback != "" {
		src = []string{fallback}
	}
	seen := map[string]bool{}
	out := []string{}
	for _, r := range src {
		r = strings.TrimSpace(r)
		if r == "" || seen[r] {
			continue
		}
		seen[r] = true
		out = append(out, r)
	}
	// super_admin eksklyuziv — boshqa rollar bilan aralashmaydi.
	for _, r := range out {
		if r == "super_admin" {
			return []string{"super_admin"}
		}
	}
	filtered := []string{}
	for _, r := range out {
		if validAssignableRoles[r] {
			filtered = append(filtered, r)
		}
	}
	if len(filtered) == 0 {
		filtered = []string{"admin"}
	}
	return filtered
}

// primaryRole — eski `role` ustuni uchun asosiy rolni tanlaydi.
func primaryRole(roles []string) string {
	for _, want := range []string{"super_admin", "admin", "ishchi_admin"} {
		for _, r := range roles {
			if r == want {
				return want
			}
		}
	}
	if len(roles) > 0 {
		return roles[0]
	}
	return "admin"
}

// fillRoles — DB'dan o'qilgan CSV'ni UserRow.Roles ga joylaydi. CSV bo'sh
// (eski yozuv) bo'lsa eski bitta `role` ustuniga qaytadi — shu sababli eski
// foydalanuvchilar muammosiz ishlaydi.
func fillRoles(u *UserRow, csv string) {
	u.Roles = splitRoles(csv)
	if len(u.Roles) == 0 && u.Role != "" {
		u.Roles = []string{u.Role}
	}
}

// effectiveRoles — foydalanuvchining haqiqiy rollari (bo'sh bo'lsa role'dan).
func (u *UserRow) effectiveRoles() []string {
	if u == nil {
		return nil
	}
	if len(u.Roles) > 0 {
		return u.Roles
	}
	if u.Role != "" {
		return []string{u.Role}
	}
	return nil
}

// hasRole — foydalanuvchi berilgan rolga ega bo'lsa true.
func (u *UserRow) hasRole(role string) bool {
	for _, r := range u.effectiveRoles() {
		if r == role {
			return true
		}
	}
	return false
}
