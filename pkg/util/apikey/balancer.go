package apikey

import (
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"

	log "github.com/sirupsen/logrus"
)

type EnvKeyLoader struct {
	Prefix       string
	Postfix      string
	Fields       []string
	AllowPartial bool

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
		Prefix:  prefix,
		Postfix: postfix,
		Fields:  fields,
		pattern: pattern,
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

		if !contains(l.Fields, field) {
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
		for _, f := range l.Fields {
			if _, ok := fields[f]; !ok {
				complete = false
				break
			}
		}

		if !complete && !l.AllowPartial {
			log.Warnf("Skipping incomplete entry: index=%d fields=%v", index, fields)
			continue
		}

		if complete {
			entry := Entry{Index: index, Fields: fields}
			entry.Key = fields["KEY"]
			entry.Secret = fields["SECRET"]
			entries = append(entries, entry)
		}
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Index < entries[j].Index
	})

	return &Source{Entries: entries}, nil
}

type Balancer struct {
	source *Source
	mu     sync.Mutex
	pos    int
}

func NewBalancer(source *Source) *Balancer {
	return &Balancer{source: source}
}

func (b *Balancer) Next() *Entry {
	b.mu.Lock()
	defer b.mu.Unlock()

	if len(b.source.Entries) == 0 {
		return nil
	}

	entry := &b.source.Entries[b.pos]
	b.pos = (b.pos + 1) % len(b.source.Entries)
	return entry
}

func (b *Balancer) Reset() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.pos = 0
}

func (b *Balancer) Peek() *Entry {
	b.mu.Lock()
	defer b.mu.Unlock()

	if len(b.source.Entries) == 0 {
		return nil
	}
	return &b.source.Entries[b.pos]
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
