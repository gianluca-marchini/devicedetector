package client

import (
	"path/filepath"

	"github.com/gianluca-marchini/devicedetector/parser"
)

const ParserNameMediaPlayer = `mediaplayer`
const FixtureFileMediaPlayer = `mediaplayers.yml`

func init() {
	RegClientParser(ParserNameMediaPlayer,
		func(dir string) ClientParser {
			return NewMediaPlayer(filepath.Join(dir, FixtureFileMediaPlayer))
		})
}

func NewMediaPlayer(fileName string) *MediaPlayer {
	c := &MediaPlayer{}
	c.ParserName = ParserNameMediaPlayer
	if err := c.Load(fileName); err != nil {
		return nil
	}
	return c
}

// Client parser for mediaplayer detection
type MediaPlayer struct {
	ClientParserAbstract
}

// ParseUserAgent is Parse for a prepared user agent.
func (m *MediaPlayer) ParseUserAgent(ua *parser.UserAgent) *ClientMatchResult {
	return m.parse(ua)
}
