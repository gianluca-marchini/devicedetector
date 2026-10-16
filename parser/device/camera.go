package device

import (
	"path/filepath"

	"github.com/gianluca-marchini/devicedetector/parser"
)

const ParserNameCamera = `camera`
const FixtureFileCamera = `cameras.yml`

func init() {
	RegDeviceParser(ParserNameCamera,
		func(dir string) DeviceParser {
			return NewCamera(filepath.Join(dir, FixtureFileCamera))
		})
}

func NewCamera(fileName string) *Camera {
	c := &Camera{}
	if err := c.Load(fileName); err != nil {
		return nil
	}
	return c
}

// Device parser for camera detection
type Camera struct {
	DeviceParserAbstract
}

func (c *Camera) Parse(ua string) *DeviceMatchResult {
	return c.ParseUserAgent(parser.NewUserAgent(ua))
}

// ParseUserAgent is Parse for a prepared user agent.
func (c *Camera) ParseUserAgent(u *parser.UserAgent) *DeviceMatchResult {
	if !c.gate(u) {
		return nil
	}
	return c.parse(u)
}
