package bbgo

import (
	"os"
	"path"
)

func SourceDir() string {
	home := HomeDir()
	return path.Join(home, "source")
}

func HomeDir() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		panic(err)
	}

	dotDir := path.Join(homeDir, ".bbgo")

	_, err = os.Stat(dotDir)
	if err != nil {
		_ = os.Mkdir(dotDir, 0777)
	}

	return dotDir
}
