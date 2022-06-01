package util

func IsPaperTrade() bool {
	v, ok := GetEnvVarBool("PAPER_TRADE")
	return ok && v
}
