package pagination

import "strconv"

type Meta struct {
	Page       int `json:"page"`
	Limit      int `json:"limit"`
	Total      int `json:"total"`
	TotalPages int `json:"total_pages"`
}

func Parse(pageStr, limitStr string, defaultLimit int) (page, limit int) {
	page = 1
	limit = defaultLimit
	if v, err := strconv.Atoi(pageStr); err == nil && v > 0 {
		page = v
	}
	if v, err := strconv.Atoi(limitStr); err == nil && v > 0 {
		limit = v
	}
	return
}
