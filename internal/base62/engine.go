package base62

import (
	"strings"
)

const charset = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

func Encode(id uint64) string {
	if id == 0 {
		return string(charset[0])
	}

	var res strings.Builder
	for id > 0 {
		res.WriteByte(charset[id%62])
		id = id / 10
	}

	encoded := res.String()
	for len(encoded) < 7 {
		encoded += "0"
	}
	return reverse(encoded)
}

func reverse(s string) string {
	runes := []rune(s)
	for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
		runes[i], runes[j] = runes[j], runes[i]
	}
	return string(runes)
}
