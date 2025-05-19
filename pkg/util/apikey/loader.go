package apikey

import (
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/sirupsen/logrus"
)

type Source struct {
	Entries []Entry
}

func (s *Source) Len() int {
	return len(s.Entries)
}

func (s *Source) Add(entry Entry) {
	s.Entries = append(s.Entries, entry)
}

func (s *Source) Get(index int) *Entry {
	if index < 0 || index >= len(s.Entries) {
		return nil
	}

	return &s.Entries[index]
}

func NewSourceFromArray(data [][]string) *Source {
	var entries []Entry

	for i, item := range data {
		if len(item) < 2 {
			continue
		}

		entry := Entry{
			Index:  i + 1,
			Key:    item[0],
			Secret: item[1],
			Fields: map[string]string{
				"KEY":    item[0],
				"SECRET": item[1],
			},
		}

		entries = append(entries, entry)
	}

	return &Source{Entries: entries}
}

type EnvKeyLoader struct {
	Prefix  string
	Postfix string

	RequiredFields []string
	AllowPartial   bool

	pattern *regexp.Regexp
}

func NewEnvKeyLoader(prefix, postfix string, fields ...string) *EnvKeyLoader {
	pattern := regexp.MustCompile(
		`^` +
			prefix +
			`(?P<field>[A-Z0-9_]+)` + // Match the field name (e.g., KEY)
			postfix +
			`_(?P<index>\d+)` + // Match the index (e.g., 1, 2, etc.)
			`$`,
	)
	return &EnvKeyLoader{
		Prefix:         prefix,
		Postfix:        postfix,
		RequiredFields: fields,
		pattern:        pattern,
	}
}

// Load loads the environment variables and returns a Source object.
// call Load(os.Environ())
func (l *EnvKeyLoader) Load(envVars []string) (*Source, error) {
	entryMap := make(map[int]map[string]string)

	re := l.pattern
	for _, env := range envVars {
		pair := strings.SplitN(env, "=", 2)
		if len(pair) != 2 {
			continue
		}

		if !strings.HasPrefix(pair[0], l.Prefix) {
			continue
		}

		name, value := pair[0], pair[1]
		matches := re.FindStringSubmatch(name)
		if len(matches) < 3 {
			continue
		}

		matchMap := extractNamedGroups(re, name)

		indexStr, ok1 := matchMap["index"]
		field, ok2 := matchMap["field"]
		if !ok1 || !ok2 {
			continue
		}

		// parse index into integer
		index, err := strconv.Atoi(indexStr)
		if err != nil {
			continue
		}

		if !contains(l.RequiredFields, field) {
			continue
		}

		if _, ok := entryMap[index]; !ok {
			entryMap[index] = make(map[string]string)
		}

		entryMap[index][field] = value
	}

	var entries []Entry
	for index, fields := range entryMap {
		complete := true
		for _, f := range l.RequiredFields {
			if _, ok := fields[f]; !ok {
				complete = false
				break
			}
		}

		if !complete && !l.AllowPartial {
			logrus.Warnf("Skipping incomplete entry: index=%d fields=%v", index, fields)
			continue
		}

		entry := Entry{
			Index:  index,
			Fields: fields,
		}

		if val, ok := fields["KEY"]; ok {
			entry.Key = val
		}

		if val, ok := fields["SECRET"]; ok {
			entry.Secret = val
		}

		entries = append(entries, entry)
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Index < entries[j].Index
	})

	return &Source{Entries: entries}, nil
}

func extractNamedGroups(re *regexp.Regexp, input string) map[string]string {
	match := re.FindStringSubmatch(input)
	if match == nil {
		return nil
	}
	result := make(map[string]string)
	names := re.SubexpNames()
	for i, name := range names {
		if i > 0 && name != "" {
			result[name] = match[i]
		}
	}
	return result
}

func contains(list []string, item string) bool {
	for _, s := range list {
		if s == item {
			return true
		}
	}
	return false
}
