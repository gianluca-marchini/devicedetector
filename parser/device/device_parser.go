package device

import (
	"errors"
	"os"
	"sort"
	"strings"
	"sync"

	"gopkg.in/yaml.v2"

	"github.com/gianluca-marchini/devicedetector/internal/ordered"
	"github.com/gianluca-marchini/devicedetector/parser"
)

const UnknownBrand = "Unknown"

type DeviceMatchResult struct {
	Type  string `yaml:"type"`
	Model string `yaml:"model"`
	Brand string `yaml:"brand"`
}

type DeviceParser interface {
	PreMatch(string) bool
	Parse(string) *DeviceMatchResult
}

type Model struct {
	parser.Regular `yaml:",inline" json:",inline"`
	Model          string `yaml:"model" json:"model"`
	Device         string `yaml:"device" json:"device"` //mobile
	Brand          string `yaml:"brand" json:"brand"`   //mobile
}

type DeviceReg struct {
	parser.Regular `yaml:",inline" json:",inline"`
	Model          string   `yaml:"model" json:"model"`
	Device         string   `yaml:"device" json:"device"`
	Models         []*Model `yaml:"models" json:"models"`
}

// brandReg is a brand of the YAML file with its precomputed short code.
type brandReg struct {
	brand string
	// id is the short code of the brand, "" for UnknownBrand.
	id string
	// known is false when the brand is not in the list of known brands: a
	// match then gives no result.
	known bool
	reg   *DeviceReg
}

type DeviceParserAbstract struct {
	// Regexes are the regexes of the YAML file by brand. They are applied in
	// the order of the file, like Matomo does.
	Regexes map[string]*DeviceReg

	brands       []brandReg // Regexes in the order of the YAML file
	brandsOnce   sync.Once
	overAllOnce  sync.Once
	overAllMatch parser.Regular
}

func (d *DeviceParserAbstract) Load(file string) error {
	data, err := os.ReadFile(file)
	if err != nil {
		return errors.New("not exists:" + file)
	}
	var v map[string]*DeviceReg
	if err := yaml.Unmarshal(data, &v); err != nil {
		return err
	}
	keys, err := ordered.Keys(data, v)
	if err != nil {
		return err
	}
	for _, item := range v {
		item.Compile()
		for _, m := range item.Models {
			m.Compile()
		}
	}
	d.Regexes = v
	d.brands = newBrandRegs(v, keys)
	d.brandsOnce.Do(func() {})
	d.overAllMatch = parser.Regular{}
	d.buildOverAllMatch()
	d.overAllOnce.Do(func() {})
	return nil
}

func newBrandRegs(regexes map[string]*DeviceReg, keys []string) []brandReg {
	brands := make([]brandReg, 0, len(keys))
	for _, brand := range keys {
		b := brandReg{brand: brand, reg: regexes[brand], known: true}
		if brand != UnknownBrand {
			b.id = parser.FindBrand(brand)
			b.known = b.id != ""
		}
		brands = append(brands, b)
	}
	return brands
}

// orderedBrands returns the brands in the order of the YAML file. When the
// parser was not loaded with Load, the brands of Regexes are used in lexical
// order.
func (d *DeviceParserAbstract) orderedBrands() []brandReg {
	d.brandsOnce.Do(func() {
		keys := make([]string, 0, len(d.Regexes))
		for k, item := range d.Regexes {
			keys = append(keys, k)
			item.Compile()
			for _, m := range item.Models {
				m.Compile()
			}
		}
		sort.Strings(keys)
		d.brands = newBrandRegs(d.Regexes, keys)
	})
	return d.brands
}

// buildOverAllMatch builds the regex matching when any regex of the file
// matches.
func (d *DeviceParserAbstract) buildOverAllMatch() {
	count := len(d.Regexes)
	if count == 0 {
		return
	}
	sortKeys := make([]string, 0, count)
	for k := range d.Regexes {
		sortKeys = append(sortKeys, k)
	}
	sort.Strings(sortKeys)
	sb := strings.Builder{}
	sb.WriteString(d.Regexes[sortKeys[count-1]].Regex)
	for i := count - 2; i >= 0; i-- {
		sb.WriteString("|")
		sb.WriteString(d.Regexes[sortKeys[i]].Regex)
	}
	d.overAllMatch.Regex = sb.String()
	d.overAllMatch.Compile()
}

func (d *DeviceParserAbstract) PreMatch(ua string) bool {
	return d.preMatch(parser.NewUserAgent(ua))
}

func (d *DeviceParserAbstract) preMatch(ua *parser.UserAgent) bool {
	d.overAllOnce.Do(d.buildOverAllMatch)
	if d.overAllMatch.Regexp == nil {
		return false
	}
	return d.overAllMatch.IsMatchUA(ua)
}

func (d *DeviceParserAbstract) Parse(ua string) *DeviceMatchResult {
	return d.parse(parser.NewUserAgent(ua))
}

func (d *DeviceParserAbstract) parse(ua *parser.UserAgent) *DeviceMatchResult {
	var b *brandReg
	var matches []string
	brands := d.orderedBrands()
	for i := range brands {
		if matches = brands[i].reg.MatchUA(ua); len(matches) > 0 {
			b = &brands[i]
			break
		}
	}

	if b == nil || !b.known {
		return nil
	}
	regex := b.reg

	r := &DeviceMatchResult{
		Type:  regex.Device,
		Brand: b.id,
	}

	if regex.Model != "" {
		r.Model = parser.BuildModel(regex.Model, matches)
	}

	for _, modelRegex := range regex.Models {
		modelMatches := modelRegex.MatchUA(ua)
		if len(modelMatches) > 0 {
			r.Model = strings.TrimSpace(parser.BuildModel(modelRegex.Model, modelMatches))
			if modelRegex.Brand != "" {
				if brandId := parser.FindBrand(modelRegex.Brand); brandId != "" {
					r.Brand = brandId
				}
			}
			if modelRegex.Device != "" {
				r.Type = modelRegex.Device
			}
			return r
		}
	}
	return r
}
