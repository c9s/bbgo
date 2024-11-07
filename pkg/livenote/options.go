package livenote

type Option interface{}

type Mention struct {
	User string
}

type Comment struct {
	Text  string
	Users []string
}
