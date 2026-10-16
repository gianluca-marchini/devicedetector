package client

import (
	"sort"
	"strings"
	"sync"

	"github.com/gianluca-marchini/devicedetector/parser"
)

type ClientMatchResult struct {
	Type    string `yaml:"type" json:"type"`
	Name    string `yaml:"name" json:"name"`
	Version string `yaml:"version" json:"version"`

	ShortName     string `yaml:"short_name" json:"short_name"`
	Engine        string `yaml:"engine" json:"engine"`
	EngineVersion string `yaml:"engine_version" json:"engine_version"`
}

type ClientParser interface {
	PreMatch(string) bool
	Parse(string) *ClientMatchResult
}

type ClientReg struct {
	parser.Regular `yaml:",inline" json:",inline"`
	Name           string `yaml:"name" json:"name"`
	Version        string `yaml:"version" json:"version"`
}

// Parses the current UA and checks whether it contains any client information
type ClientParserAbstract struct {
	Regexes      []*ClientReg
	ParserName   string
	overAllOnce  sync.Once
	overAllMatch parser.Regular
}

func (c *ClientParserAbstract) Load(file string) error {
	var v []*ClientReg
	err := parser.ReadYamlFile(file, &v)
	if err != nil {
		return err
	}
	for _, item := range v {
		item.Compile()
	}
	c.Regexes = v
	c.overAllMatch = parser.Regular{}
	c.buildOverAllMatch()
	c.overAllOnce.Do(func() {})
	return nil
}

// buildOverAllMatch builds the regex matching when any regex matches.
func (c *ClientParserAbstract) buildOverAllMatch() {
	count := len(c.Regexes)
	if count == 0 {
		return
	}
	sb := strings.Builder{}
	sb.WriteString(c.Regexes[count-1].Regex)
	for i := count - 2; i >= 0; i-- {
		sb.WriteString("|")
		sb.WriteString(c.Regexes[i].Regex)
	}
	c.overAllMatch.Regex = sb.String()
	c.overAllMatch.Compile()
}

func (c *ClientParserAbstract) PreMatch(ua string) bool {
	return c.preMatch(parser.NewUserAgent(ua))
}

func (c *ClientParserAbstract) preMatch(ua *parser.UserAgent) bool {
	c.overAllOnce.Do(c.buildOverAllMatch)
	if c.overAllMatch.Regexp == nil {
		return false
	}
	return c.overAllMatch.IsMatchUA(ua)
}

// Parses the current UA and checks whether it contains any client information
func (c *ClientParserAbstract) Parse(ua string) *ClientMatchResult {
	u := parser.NewUserAgent(ua)
	if c.preMatch(u) {
		for _, regex := range c.Regexes {
			matches := regex.MatchUA(u)
			if len(matches) > 0 {
				return &ClientMatchResult{
					Type:    c.ParserName,
					Name:    parser.BuildByMatch(regex.Name, matches),
					Version: parser.BuildVersion(regex.Version, matches),
				}
			}
		}
	}
	return nil
}

// Returns all names defined in the regexes
// Attention: This method might not return all names of detected clients
func (c *ClientParserAbstract) GetAvailableClients() []string {
	names := make([]string, 0, len(c.Regexes))
	for _, regex := range c.Regexes {
		n := regex.Name
		if n != `$1` && !parser.ArrayContainsString(names, n) {
			names = append(names, n)
		}
	}
	sort.Strings(names)
	return names
}
