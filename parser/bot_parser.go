package parser

import (
	"strings"
	"sync"
	"sync/atomic"
)

type Producer struct {
	Name string `yaml:"name" json:"name"`
	Url  string `yaml:"url" json:"url"`
}

type BotMatchResult struct {
	Name     string   `yaml:"name" json:"name"`
	Category string   `yaml:"category" json:"category"`
	Url      string   `yaml:"url" json:"url"`
	Producer Producer `yaml:"producer" json:"producer"`
}

func (b *BotMatchResult) Equal(a *BotMatchResult) bool {
	return b.Name == a.Name &&
		b.Category == a.Category &&
		b.Url == a.Url &&
		b.Producer.Name == a.Producer.Name &&
		b.Producer.Url == a.Producer.Url
}

type BotReg struct {
	Regular        `yaml:",inline" json:",inline"`
	BotMatchResult `yaml:",inline" json:",inline"`
}

type BotParser interface {
	PreMatch(string) bool
	Parse(string) *BotMatchResult
	DiscardDetails(bool)
}

// Abstract class for all bot parsers
type BotParserAbstract struct {
	Regexes        []*BotReg
	ParserName     string
	discardDetails atomic.Bool
	overAllOnce    sync.Once
	overAllMatch   Regular
}

func (b *BotParserAbstract) DiscardDetails(v bool) {
	b.discardDetails.Store(v)
}

func (b *BotParserAbstract) Load(file string) error {
	var v []*BotReg
	err := ReadYamlFile(file, &v)
	if err != nil {
		return err
	}
	for _, item := range v {
		item.Compile()
	}
	b.Regexes = v
	b.overAllMatch = Regular{}
	buildOverAllMatch(&b.overAllMatch, len(v), func(i int) string { return v[i].Regex })
	b.overAllOnce.Do(func() {})
	return nil
}

// buildOverAllMatch compiles into r the alternation of the count regexes
// returned by regex, in reverse order. It does nothing when count is 0.
func buildOverAllMatch(r *Regular, count int, regex func(int) string) {
	if count == 0 {
		return
	}
	sb := strings.Builder{}
	sb.WriteString(regex(count - 1))
	for i := count - 2; i >= 0; i-- {
		sb.WriteString("|")
		sb.WriteString(regex(i))
	}
	r.Regex = sb.String()
	r.Compile()
}

func (b *BotParserAbstract) PreMatch(ua string) bool {
	return b.preMatch(NewUserAgent(ua))
}

func (b *BotParserAbstract) preMatch(ua *UserAgent) bool {
	b.overAllOnce.Do(func() {
		buildOverAllMatch(&b.overAllMatch, len(b.Regexes), func(i int) string { return b.Regexes[i].Regex })
	})
	if b.overAllMatch.Regexp == nil {
		return false
	}
	return b.overAllMatch.IsMatchUA(ua)
}

var EmptyBotMatchResult = new(BotMatchResult)

// Parses the current UA and checks whether it contains bot information
func (b *BotParserAbstract) Parse(ua string) *BotMatchResult {
	u := NewUserAgent(ua)
	if b.preMatch(u) {
		if b.discardDetails.Load() {
			return EmptyBotMatchResult
		}
		for _, regex := range b.Regexes {
			matches := regex.MatchUA(u)
			if len(matches) > 0 {
				return &regex.BotMatchResult
			}
		}
	}
	return nil
}
