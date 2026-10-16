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
		return nil
	}
	// Like Matomo, only the user agents with the HbbTV fragment are parsed.
	// (v1.1.1 set the regex only when Load failed: the empty regex matched
	// every user agent, so phones with e.g. "Samsung" became TVs.)
	h.hbbTvRegx.Regex = `HbbTV/([1-9]{1}(?:\.[0-9]{1}){1,2})`
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
	return h.ParseUserAgent(parser.NewUserAgent(ua))
}

// ParseUserAgent is Parse for a prepared user agent.
func (h *HbbTv) ParseUserAgent(u *parser.UserAgent) *DeviceMatchResult {
	// only parse user agents containing hbbtv fragment
	if !h.hbbTvRegx.IsMatchUA(u) {
		return nil
	}
	r := h.DeviceParserAbstract.parse(u)
	// always set device type to tv, even if no model/brand could be found
	if r == nil {
		r = &DeviceMatchResult{}
	}
	r.Type = ParserNameHbbTv
	return r
}

// Returns if the parsed UA was identified as a HbbTV device
func (h *HbbTv) IsHbbTv(ua string) bool {
	return h.hbbTvRegx.IsMatchUserAgent(ua)
}
