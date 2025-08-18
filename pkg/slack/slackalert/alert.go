package slackalert

type SlackAlert struct {
	Channel  string   `json:"channel"`
	Mentions []string `json:"mentions"`
	Pin      bool     `json:"pin"`
}
