package server

import (
	"context"
	"encoding/json"
	"net/http"
	"os/exec"
	"runtime"
	"time"

	"github.com/sirupsen/logrus"
)

func getJSON(url string, data interface{}) error {
	var client = &http.Client{
		Timeout: 200 * time.Millisecond,
	}
	r, err := client.Get(url)
	if err != nil {
		return err
	}

	defer r.Body.Close()

	return json.NewDecoder(r.Body).Decode(data)
}

func openURL(url string) error {
	cmd := exec.Command("open", url)
	return cmd.Start()
}

func filterStrings(slice []string, needle string) (ns []string) {
	for _, str := range slice {
		if str == needle {
			continue
		}

		ns = append(ns, str)
	}

	return ns
}

func openBrowser(ctx context.Context, bind string) {
	if runtime.GOOS == "darwin" {
		baseURL := "http://" + bind
		go pingAndOpenURL(ctx, baseURL)
	} else {
		logrus.Warnf("%s is not supported for opening browser automatically", runtime.GOOS)
	}
}

func resolveBind(a []string) string {
	switch len(a) {
	case 0:
		return DefaultBindAddress

	case 1:
		return a[0]

	default:
		panic("too many parameters for binding")
	}

	return ""
}
