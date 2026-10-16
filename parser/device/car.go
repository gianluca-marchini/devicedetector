package device

import (
	"path/filepath"

	"github.com/gianluca-marchini/devicedetector/parser"
)

const ParserNameCar = `car browser`
const FixtureFileCar = `car_browsers.yml`

func init() {
	RegDeviceParser(ParserNameCar,
		func(dir string) DeviceParser {
			return NewCar(filepath.Join(dir, FixtureFileCar))
		})
}

func NewCar(fileName string) *Car {
	c := &Car{}
	if err := c.Load(fileName); err != nil {
		return nil
	}
	return c
}

// Device parser for car browser detection
type Car struct {
	DeviceParserAbstract
}

func (c *Car) Parse(ua string) *DeviceMatchResult {
	return c.ParseUserAgent(parser.NewUserAgent(ua))
}

// ParseUserAgent is Parse for a prepared user agent.
func (c *Car) ParseUserAgent(u *parser.UserAgent) *DeviceMatchResult {
	if !c.gate(u) {
		return nil
	}
	return c.parse(u)
}
