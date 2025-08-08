package logic

import "strconv"

// isReservedKeyword checks if the keyword is a number
func IsReservedKeyword(keyword string) bool {
	if _, err := strconv.Atoi(keyword); err == nil {
		return true
	}
	return false
}
