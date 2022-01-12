package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/google/uuid"
	tb "gopkg.in/tucnak/telebot.v2"
)

func main() {
	b, err := tb.NewBot(tb.Settings{
		// You can also set custom API URL.
		// If field is empty it equals to "https://api.telegram.org".
		// URL: "http://195.129.111.17:8012",

		Token:  os.Getenv("TELEGRAM_BOT_TOKEN"),
		Poller: &tb.LongPoller{Timeout: 10 * time.Second},
		// Synchronous: false,
		Verbose: true,
		// ParseMode:   "",
		// Reporter:    nil,
		// Client:      nil,
		// Offline:     false,
	})

	if err != nil {
		log.Fatal(err)
		return
	}

	b.Handle("/hello", func(m *tb.Message) {
		fmt.Printf("message: %#v\n", m)
		// b.Send(m.Sender, "Hello World!")
	})

	b.Handle(tb.OnText, func(m *tb.Message) {
		fmt.Printf("text: %#v\n", m)
		// all the text messages that weren't
		// captured by existing handlers
	})

	b.Handle(tb.OnQuery, func(q *tb.Query) {
		fmt.Printf("query: %#v\n", q)

		// r := &tb.ReplyMarkup{}
		// r.URL("test", "https://media.tenor.com/images/f176705ae1bb3c457e19d8cd71718ac0/tenor.gif")
		urls := []string{
			// "https://media.tenor.com/images/aae0cdf3c5a291cd7b96432180f6eee3/tenor.png",
			// "https://media.tenor.com/images/905c1a9b1f56ae3c458b1ef58fd46357/tenor.png",

			"https://media.tenor.com/images/2e69768f9537957ed3015a80ebc3f0f1/tenor.gif",
			"https://media.tenor.com/images/6fcd72b29127a55e5c35db86d06d665c/tenor.gif",
			"https://media.tenor.com/images/05dbf5bf3a3b88275bb045691541dc53/tenor.gif",
			"https://media.tenor.com/images/0e1a52cfe5616c1509090d6ec2312db0/tenor.gif",
			"https://media.tenor.com/images/1ca04a449b26e1f7d45682a79d2c8697/tenor.gif",
			"https://media.tenor.com/images/a2844b186fb71c376226b56c4ea7730a/tenor.gif",
			"https://media.tenor.com/images/ec636a1ebce1a3fc1c795b851c125b31/tenor.gif",
			"https://media.tenor.com/images/ae103819cb05a0cf7497900b77b87d80/tenor.gif",
		}

		results := make(tb.Results, len(urls)) // []tb.Result
		for i, url := range urls {
			// result := &tb.PhotoResult{
			result := &tb.GifResult{
				ResultBase: tb.ResultBase{
					// Type:        "photo",
					// Content:     nil,
					// ReplyMarkup: nil,
					ID:          uuid.New().String(),
				},

				URL: url,

				// required for photos
				ThumbURL: url,
			}

			results[i] = result
			// needed to set a unique string ID for each result
			// results[i].SetResultID(strconv.Itoa(i))
		}

		err := b.Answer(q, &tb.QueryResponse{
			QueryID: q.ID,
			Results: results,
			// CacheTime:  60, // a minute
			IsPersonal: true,
		})

		if err != nil {
			log.Println(err)
		}
	})

	b.Start()
}
