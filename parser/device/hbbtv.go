package device

import (
	"path/filepath"

	"github.com/gianluca-marchini/devicedetector/parser"
)

const ParserNameHbbTv = `tv`
const FixtureFileHbbTv = `televisions.yml`

func init() {
	RegDeviceParser(ParserNameHbbTv,
		func(dir string) DeviceParser {
			return NewHbbTv(filepath.Join(dir, FixtureFileHbbTv))
		})
}

func NewHbbTv(fileName string) *HbbTv {
	h := &HbbTv{}
	if err := h.Load(fileName); err != nil {
		h.hbbTvRegx.Regex = `HbbTV/([1-9]{1}(?:.[0-9]{1}){1,2})`
		return nil
	}
	// Compile now: compiling on first use is not safe for concurrent use.
	h.hbbTvRegx.Compile()
	return h
}

// Device parser for hbbtv detection
type HbbTv struct {
	DeviceParserAbstract
	hbbTvRegx parser.Regular
}

func (h *HbbTv) Parse(ua string) *DeviceMatchResult {
	u := parser.NewUserAgent(ua)
	// only parse user agents containing hbbtv fragment
	if !h.hbbTvRegx.IsMatchUA(u) {
		return nil
	}
	r := h.DeviceParserAbstract.parse(u)
	// always set device type to tv, even if no model/brand could be found
	if r != nil {
		r.Type = ParserNameHbbTv
	}
	return r
}

// Returns if the parsed UA was identified as a HbbTV device
func (h *HbbTv) IsHbbTv(ua string) bool {
	return h.hbbTvRegx.IsMatchUserAgent(ua)
}
