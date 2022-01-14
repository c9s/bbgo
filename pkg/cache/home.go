package cache

import (
	"os"
	"path"
)

func prepareDir(p string) string {
	_, err := os.Stat(p)
	if err != nil {
		_ = os.Mkdir(p, 0777)
	}

	return p
}

func CacheDir() string {
	home := HomeDir()
	dir := path.Join(home, "cache")
	return prepareDir(dir)
}

func HomeDir() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		panic(err)
	}

	dir := path.Join(homeDir, ".bbgo")
	return prepareDir(dir)
}
