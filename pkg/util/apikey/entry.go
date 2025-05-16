package apikey

type Entry struct {
	Index  int
	Key    string
	Secret string
	Fields map[string]string
}

func (e *Entry) Get(field string) (val string, ok bool) {
	val, ok = e.Fields[field]
	return val, ok
}
