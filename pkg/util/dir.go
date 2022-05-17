package util

import (
	"fmt"
	"os"
)

func SafeMkdirAll(p string) error {
	st, err := os.Stat(p)
	if err == nil {
		if !st.IsDir() {
			return fmt.Errorf("path %s is not a directory", p)
		}

		return nil
	}

	if os.IsNotExist(err) {
		return os.MkdirAll(p, 0755)
	}

	return nil
}
