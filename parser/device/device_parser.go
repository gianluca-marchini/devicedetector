package device

import (
	"errors"
	"os"
	"sort"
	"strings"
	"sync"

	"gopkg.in/yaml.v2"

	"github.com/gianluca-marchini/devicedetector/internal/alternation"
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
	// models indexes reg.Models when there are many.
	models *parser.Index
	// captures is false when the model does not use the groups of the regex:
	// knowing whether it matches is enough.
	captures bool
	// modelCaptures is the same for every model regex.
	modelCaptures []bool
	// modelBrands are the short codes of the brands of the models.
	modelBrands []string
}

func usesGroups(item string) bool {
	return strings.IndexByte(item, '$') >= 0
}

// minIndexLen is the length from which a list of regexes is indexed.
const minIndexLen = 8

type DeviceParserAbstract struct {
	// Regexes are the regexes of the YAML file by brand. They are applied in
	// the order of the file, like Matomo does.
	Regexes map[string]*DeviceReg

	brands       []brandReg // Regexes in the order of the YAML file
	index        *parser.Index
	brandsOnce   sync.Once
	overAllOnce  sync.Once
	overAllMatch parser.Regular
	// union is true when overAllMatch matches exactly when one of the
	// regexes matches: it is then evaluated one regex at a time.
	union bool
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
	d.brands, d.index = newBrandRegs(v, keys)
	d.brandsOnce.Do(func() {})
	d.overAllMatch = parser.Regular{}
	d.buildOverAllMatch()
	d.overAllOnce.Do(func() {})
	return nil
}

func newBrandRegs(regexes map[string]*DeviceReg, keys []string) ([]brandReg, *parser.Index) {
	brands := make([]brandReg, 0, len(keys))
	regs := make([]*parser.Regular, 0, len(keys))
	for _, brand := range keys {
		b := brandReg{brand: brand, reg: regexes[brand], known: true}
		if brand != UnknownBrand {
			b.id = parser.FindBrand(brand)
			b.known = b.id != ""
		}
		b.captures = usesGroups(b.reg.Model)
		b.modelCaptures = make([]bool, len(b.reg.Models))
		b.modelBrands = make([]string, len(b.reg.Models))
		for i, m := range b.reg.Models {
			b.modelCaptures[i] = usesGroups(m.Model)
			if m.Brand != "" {
				b.modelBrands[i] = parser.FindBrand(m.Brand)
			}
		}
		if len(b.reg.Models) >= minIndexLen {
			models := make([]*parser.Regular, len(b.reg.Models))
			for i, m := range b.reg.Models {
				models[i] = &m.Regular
			}
			b.models = parser.NewIndex(models)
		}
		brands = append(brands, b)
		regs = append(regs, &b.reg.Regular)
	}
	return brands, parser.NewIndex(regs)
}

// orderedBrands returns the brands in the order of the YAML file and their
// index. When the parser was not loaded with Load, the brands of Regexes are
// used in lexical order.
func (d *DeviceParserAbstract) orderedBrands() ([]brandReg, *parser.Index) {
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
		d.brands, d.index = newBrandRegs(d.Regexes, keys)
	})
	return d.brands, d.index
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
	regexes := make([]string, count)
	for i, k := range sortKeys {
		regexes[i] = d.Regexes[k].Regex
	}
	if d.union = alternation.IsUnion(regexes); d.union {
		return
	}
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
	if d.union {
		brands, index := d.orderedBrands()
		candidates := index.Candidates(ua)
		for i := range brands {
			if candidates.Has(i) && brands[i].reg.IsMatchUA(ua) {
				return true
			}
		}
		return false
	}
	if d.overAllMatch.Regexp == nil {
		return false
	}
	return d.overAllMatch.IsMatchUA(ua)
}

// gate is preMatch when it can give a different result from parse: with
// union, parse finds a brand exactly when preMatch is true.
func (d *DeviceParserAbstract) gate(ua *parser.UserAgent) bool {
	d.overAllOnce.Do(d.buildOverAllMatch)
	return d.union || d.preMatch(ua)
}

func (d *DeviceParserAbstract) Parse(ua string) *DeviceMatchResult {
	return d.parse(parser.NewUserAgent(ua))
}

func (d *DeviceParserAbstract) parse(ua *parser.UserAgent) *DeviceMatchResult {
	var b *brandReg
	var matches []string
	brands, index := d.orderedBrands()
	candidates := index.Candidates(ua)
	for i := range brands {
		if !candidates.Has(i) {
			continue
		}
		if brands[i].captures {
			if matches = brands[i].reg.MatchUA(ua); len(matches) > 0 {
				b = &brands[i]
				break
			}
		} else if brands[i].reg.IsMatchUA(ua) {
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

	var models parser.Candidates
	if b.models != nil {
		models = b.models.Candidates(ua)
	}
	for i, modelRegex := range regex.Models {
		if models != nil && !models.Has(i) {
			continue
		}
		var modelMatches []string
		if b.modelCaptures[i] {
			modelMatches = modelRegex.MatchUA(ua)
		} else if modelRegex.IsMatchUA(ua) {
			// BuildModel does not read the groups.
			modelMatches = []string{}
		}
		if modelMatches != nil {
			r.Model = strings.TrimSpace(parser.BuildModel(modelRegex.Model, modelMatches))
			if brandId := b.modelBrands[i]; brandId != "" {
				r.Brand = brandId
			}
			if modelRegex.Device != "" {
				r.Type = modelRegex.Device
			}
			return r
		}
	}
	return r
}
