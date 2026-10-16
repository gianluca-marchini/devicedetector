package client

import (
	"path/filepath"

	"github.com/gianluca-marchini/devicedetector/parser"
)

const ParserNamePim = `pim`
const FixtureFilePim = `pim.yml`

func init() {
	RegClientParser(ParserNamePim,
		func(dir string) ClientParser {
			return NewPim(filepath.Join(dir, FixtureFilePim))
		})
}

func NewPim(fileName string) *Pim {
	c := &Pim{}
	c.ParserName = ParserNamePim
	if err := c.Load(fileName); err != nil {
		return nil
	}
	return c
}

// Client parser for pim (personal information manager) detection
type Pim struct {
	ClientParserAbstract
}

// ParseUserAgent is Parse for a prepared user agent.
func (p *Pim) ParseUserAgent(ua *parser.UserAgent) *ClientMatchResult {
	return p.parse(ua)
}
