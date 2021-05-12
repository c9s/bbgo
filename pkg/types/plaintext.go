package types

type PlainText interface {
	PlainText() string
}

type Stringer interface {
	String() string
}
