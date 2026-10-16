package device

import (
	"path/filepath"

	"github.com/gianluca-marchini/devicedetector/parser"
)

const ParserNamePortableMediaPlayer = `portablemediaplayer`
const FixtureFilePortableMediaPlayer = `portable_media_player.yml`

func init() {
	RegDeviceParser(ParserNamePortableMediaPlayer,
		func(dir string) DeviceParser {
			return NewPortableMediaPlayer(filepath.Join(dir, FixtureFilePortableMediaPlayer))
		})
}

func NewPortableMediaPlayer(fileName string) *PortableMediaPlayer {
	p := &PortableMediaPlayer{}
	if err := p.Load(fileName); err != nil {
		return nil
	}
	return p
}

// Device parser for portable media player detection
type PortableMediaPlayer struct {
	DeviceParserAbstract
}

func (p *PortableMediaPlayer) Parse(ua string) *DeviceMatchResult {
	return p.ParseUserAgent(parser.NewUserAgent(ua))
}

// ParseUserAgent is Parse for a prepared user agent.
func (p *PortableMediaPlayer) ParseUserAgent(u *parser.UserAgent) *DeviceMatchResult {
	if !p.gate(u) {
		return nil
	}
	return p.parse(u)
}
