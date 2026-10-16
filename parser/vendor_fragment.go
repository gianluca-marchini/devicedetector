package parser

import (
	"errors"
	"os"

	"gopkg.in/yaml.v2"

	"github.com/gianluca-marchini/devicedetector/internal/ordered"
)

const ParserNameVendor = "vendorfragments"
const FixtureFileVendor = "vendorfragments.yml"

type vendorFragment struct {
	shortName string
	regexes   []*Regular
}

// Device parser for vendor fragment detection
type VendorFragments struct {
	// vendors are applied in the order of the YAML file, like Matomo does.
	vendors []vendorFragment
}

func NewVendor(file string) (*VendorFragments, error) {
	data, err := os.ReadFile(file)
	if err != nil {
		return nil, errors.New("not exists:" + file)
	}
	var m map[string][]string
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	keys, err := ordered.Keys(data, m)
	if err != nil {
		return nil, err
	}

	vendors := make([]vendorFragment, 0, len(keys))
	for _, name := range keys {
		brands := m[name]
		if len(brands) == 0 {
			continue
		}
		v := vendorFragment{
			shortName: GetShortName(name),
			regexes:   make([]*Regular, 0, len(brands)),
		}
		for _, brand := range brands {
			regex := &Regular{Regex: brand + "[^a-z0-9]+"}
			regex.Compile()
			v.regexes = append(v.regexes, regex)
		}
		vendors = append(vendors, v)
	}

	return &VendorFragments{
		vendors: vendors,
	}, nil
}

func (v *VendorFragments) Parse(ua string) string {
	return v.ParseUserAgent(NewUserAgent(ua))
}

// ParseUserAgent is Parse for a prepared user agent.
func (v *VendorFragments) ParseUserAgent(u *UserAgent) string {
	for _, vendor := range v.vendors {
		for _, regex := range vendor.regexes {
			if regex.IsMatchUA(u) {
				return vendor.shortName
			}
		}
	}
	return ""
}
