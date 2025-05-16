package apikey

type Source struct {
	Entries []Entry
}

func (s *Source) Len() int {
	return len(s.Entries)
}

func (s *Source) Get(index int) *Entry {
	if index < 0 || index >= len(s.Entries) {
		return nil
	}

	return &s.Entries[index]
}
