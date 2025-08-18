package util

import (
	"strings"
	"unicode/utf8"
)

func MaskKey(key string) string {
	if len(key) == 0 {
		return "{empty}"
	}

	h := min(len(key)/3, 5)

	maskKey := key[0:h]
	maskKey += strings.Repeat("*", len(key)-h*2)
	maskKey += key[len(key)-h:]
	return maskKey
}

func StringSplitByLength(s string, length int) (result []string) {
	var left, right int
	for left, right = 0, length; right < len(s); left, right = right, right+length {
		for !utf8.RuneStart(s[right]) {
			right--
		}
		result = append(result, s[left:right])
	}
	if len(s)-left > 0 {
		result = append(result, s[left:])
	}
	return result
}
