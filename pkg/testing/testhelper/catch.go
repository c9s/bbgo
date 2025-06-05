package testhelper

type CatchMatcher struct {
	f func(x any)
}

func Catch(f func(x any)) *CatchMatcher {
	return &CatchMatcher{
		f: f,
	}
}

func (m *CatchMatcher) Matches(x interface{}) bool {
	m.f(x)
	return true
}

func (m *CatchMatcher) String() string {
	return "CatchMatcher"
}
