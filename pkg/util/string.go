package util

import "strings"

func StringSliceContains(slice []string, needle string) bool {
	for _, s := range slice {
		if s == needle {
			return true
		}
	}

	return false
}

func MaskKey(key string) string {
	if len(key) == 0 {
		return "{empty}"
	}

	h := len(key) / 3
	if h > 5 {
		h = 5
	}

	maskKey := key[0:h]
	maskKey += strings.Repeat("*", len(key)-h*2)
	maskKey += key[len(key)-h:]
	return maskKey
}
