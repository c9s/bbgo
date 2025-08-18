package slacknotifier

import (
	"bytes"
	"os"
	"testing"

	"github.com/slack-go/slack"
	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/types"
)

func Test_objectName(t *testing.T) {
	t.Run("deposit", func(t *testing.T) {
		var deposit = &types.Deposit{}
		assert.Equal(t, "Deposit", objectName(deposit))
	})

	t.Run("local type", func(t *testing.T) {
		type A struct{}
		var obj = &A{}
		assert.Equal(t, "A", objectName(obj))
	})
}

func TestSlack(t *testing.T) {
	apiToken := os.Getenv("SLACK_API_TOKEN")
	channel := os.Getenv("SLACK_CHANNEL")
	if len(apiToken) == 0 || len(channel) == 0 {
		t.Skip("SLACK_API_TOKEN and SLACK_CHANNEL must be set")
	}

	client := slack.New(apiToken, slack.OptionDebug(true))

	t.Run("upload file", func(t *testing.T) {
		notifier := New(client, channel)

		data := []byte("test data")
		file := &types.UploadFile{
			FileType: types.FileTypeText,
			FileName: "text.txt",
			Data:     bytes.NewBuffer(data),
		}

		notifier.Upload(file)

		t.Logf("file: %+v", file)
	})
}
