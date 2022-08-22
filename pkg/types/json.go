package types

type JsonStruct struct {
	Key   string
	Json  string
	Type  string
	Value interface{}
}

type JsonArr []JsonStruct

func (a JsonArr) Len() int           { return len(a) }
func (a JsonArr) Less(i, j int) bool { return a[i].Key < a[j].Key }
func (a JsonArr) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
