package livenote

type Option interface{}

type OptionCompare struct {
	Value bool
}

func CompareObject(value bool) *OptionCompare {
	return &OptionCompare{Value: value}
}

type OptionOneTimeMention struct {
	Users []string
}

func OneTimeMention(users ...string) *OptionOneTimeMention {
	return &OptionOneTimeMention{Users: users}
}

type OptionComment struct {
	Text  string
	Users []string
}

func Comment(text string, users ...string) *OptionComment {
	return &OptionComment{
		Text:  text,
		Users: users,
	}
}
