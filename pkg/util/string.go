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
	maskKey := key[0:5]
	return maskKey + strings.Repeat("*", len(key)-1-5)
}
