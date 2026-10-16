package parser

import (
	"strings"
	"sync"
)

const ParserNameOs = "os"
const FixtureFileOs = "oss.yml"

type OsReg struct {
	Regular `yaml:",inline" json:",inline"`
	Name    string `yaml:"name" json:"name"`
	Version string `yaml:"version" json:"version"`
}

// Known operating systems mapped to their internal short codes
var OperatingSystems = map[string]string{
	`AIX`: `AIX`,
	`AND`: `Android`,
	`AMG`: `AmigaOS`,
	`ATV`: `Apple TV`,
	`ARL`: `Arch Linux`,
	`BTR`: `BackTrack`,
	`SBA`: `Bada`,
	`BEO`: `BeOS`,
	`BLB`: `BlackBerry OS`,
	`QNX`: `BlackBerry Tablet OS`,
	`BMP`: `Brew`,
	`CES`: `CentOS`,
	`COS`: `Chrome OS`,
	`CYN`: `CyanogenMod`,
	`DEB`: `Debian`,
	`DFB`: `DragonFly`,
	`FED`: `Fedora`,
	`FOS`: `Firefox OS`,
	`FIR`: `Fire OS`,
	`BSD`: `FreeBSD`,
	`GNT`: `Gentoo`,
	`GTV`: `Google TV`,
	`HPX`: `HP-UX`,
	`HAI`: `Haiku OS`,
	`IRI`: `IRIX`,
	`INF`: `Inferno`,
	`KOS`: `KaiOS`,
	`KNO`: `Knoppix`,
	`KBT`: `Kubuntu`,
	`LIN`: `GNU/Linux`,
	`LBT`: `Lubuntu`,
	`VLN`: `VectorLinux`,
	`MAC`: `Mac`,
	`MAE`: `Maemo`,
	`MDR`: `Mandriva`,
	`SMG`: `MeeGo`,
	`MCD`: `MocorDroid`,
	`MIN`: `Mint`,
	`MLD`: `MildWild`,
	`MOR`: `MorphOS`,
	`NBS`: `NetBSD`,
	`MTK`: `MTK / Nucleus`,
	`WII`: `Nintendo`,
	`NDS`: `Nintendo Mobile`,
	`OS2`: `OS/2`,
	`T64`: `OSF1`,
	`OBS`: `OpenBSD`,
	`ORD`: `Ordissimo`,
	`PSP`: `PlayStation Portable`,
	`PS3`: `PlayStation`,
	`RHT`: `Red Hat`,
	`ROS`: `RISC OS`,
	`REM`: `Remix OS`,
	`RZD`: `RazoDroiD`,
	`SAB`: `Sabayon`,
	`SSE`: `SUSE`,
	`SAF`: `Sailfish OS`,
	`SLW`: `Slackware`,
	`SOS`: `Solaris`,
	`SYL`: `Syllable`,
	`SYM`: `Symbian`,
	`SYS`: `Symbian OS`,
	`S40`: `Symbian OS Series 40`,
	`S60`: `Symbian OS Series 60`,
	`SY3`: `Symbian^3`,
	`TDX`: `ThreadX`,
	`TIZ`: `Tizen`,
	`TOS`: `TmaxOS`,
	`UBT`: `Ubuntu`,
	`WTV`: `WebTV`,
	`WIN`: `Windows`,
	`WCE`: `Windows CE`,
	`WIO`: `Windows IoT`,
	`WMO`: `Windows Mobile`,
	`WPH`: `Windows Phone`,
	`WRT`: `Windows RT`,
	`XBX`: `Xbox`,
	`XBT`: `Xubuntu`,
	`YNS`: `YunOs`,
	`IOS`: `iOS`,
	`POS`: `palmOS`,
	`WOS`: `webOS`,
}

// Operating system families mapped to the short codes of the associated operating systems
var OsFamilies = map[string][]string{
	`Android`:               {`AND`, `CYN`, `FIR`, `REM`, `RZD`, `MLD`, `MCD`, `YNS`},
	`AmigaOS`:               {`AMG`, `MOR`},
	`Apple TV`:              {`ATV`},
	`BlackBerry`:            {`BLB`, `QNX`},
	`Brew`:                  {`BMP`},
	`BeOS`:                  {`BEO`, `HAI`},
	`Chrome OS`:             {`COS`},
	`Firefox OS`:            {`FOS`, `KOS`},
	`Gaming Console`:        {`WII`, `PS3`},
	`Google TV`:             {`GTV`},
	`IBM`:                   {`OS2`},
	`iOS`:                   {`IOS`},
	`RISC OS`:               {`ROS`},
	`GNU/Linux`:             {`LIN`, `ARL`, `DEB`, `KNO`, `MIN`, `UBT`, `KBT`, `XBT`, `LBT`, `FED`, `RHT`, `VLN`, `MDR`, `GNT`, `SAB`, `SLW`, `SSE`, `CES`, `BTR`, `SAF`, `ORD`, `TOS`},
	`Mac`:                   {`MAC`},
	`Mobile Gaming Console`: {`PSP`, `NDS`, `XBX`},
	`Real-time OS`:          {`MTK`, `TDX`},
	`Other Mobile`:          {`WOS`, `POS`, `SBA`, `TIZ`, `SMG`, `MAE`},
	`Symbian`:               {`SYM`, `SYS`, `SY3`, `S60`, `S40`},
	`Unix`:                  {`SOS`, `AIX`, `HPX`, `BSD`, `NBS`, `OBS`, `DFB`, `SYL`, `IRI`, `T64`, `INF`},
	`WebTV`:                 {`WTV`},
	`Windows`:               {`WIN`},
	`Windows Mobile`:        {`WPH`, `WMO`, `WCE`, `WRT`, `WIO`},
}

const (
	PlatformTypeARM  = "ARM"
	PlatformTypeX64  = "x64"
	PlatformTypeX86  = "x86"
	PlatformTypeNONE = ""
)

type PlatformReg struct {
	Name string
	Regular
}

type OsMatchResult struct {
	Name      string `yaml:"name" json:"name"`
	ShortName string `yaml:"short_name" json:"short_name"`
	Version   string `yaml:"version" json:"version"`
	Platform  string `yaml:"platform" json:"platform"`
}

type OsParser interface {
	PreMatch(string) bool
	Parse(string) *OsMatchResult
}

// operatingSystemsByName maps the lower-cased (ASCII) names of
// OperatingSystems to their short codes.
var operatingSystemsByName = func() map[string]string {
	m := make(map[string]string, len(OperatingSystems))
	for short, name := range OperatingSystems {
		m[asciiLower(name)] = short
	}
	return m
}()

func asciiLower(s string) string {
	for i := 0; i < len(s); i++ {
		if 'A' <= s[i] && s[i] <= 'Z' {
			b := []byte(s)
			for j := i; j < len(b); j++ {
				if 'A' <= b[j] && b[j] <= 'Z' {
					b[j] += 'a' - 'A'
				}
			}
			return string(b)
		}
	}
	return s
}

// Parses the useragent for operating system information
type Oss struct {
	Regexes      []*OsReg
	platforms    []*PlatformReg
	overAllOnce  sync.Once
	overAllMatch Regular
}

func NewOss(file string) (*Oss, error) {
	var v []*OsReg
	err := ReadYamlFile(file, &v)
	if err != nil {
		return nil, err
	}
	ps := []*PlatformReg{
		{Name: PlatformTypeARM, Regular: Regular{Regex: "arm"}},
		{Name: PlatformTypeX64, Regular: Regular{Regex: "WOW64|x64|win64|amd64|x86_64"}},
		{Name: PlatformTypeX86, Regular: Regular{Regex: "i[0-9]86|i86pc"}},
	}
	for _, pp := range ps {
		pp.Compile()
	}
	// Compile now: compiling on first use is not safe for concurrent use.
	for _, item := range v {
		item.Compile()
	}
	return &Oss{
		Regexes:   v,
		platforms: ps,
	}, nil
}

func (o *Oss) ParsePlatform(ua string) string {
	return o.parsePlatform(NewUserAgent(ua))
}

func (o *Oss) parsePlatform(ua *UserAgent) string {
	for _, p := range o.platforms {
		if p.IsMatchUA(ua) {
			return p.Name
		}
	}
	return PlatformTypeNONE
}

func (o *Oss) buildOverAllMatch() {
	buildOverAllMatch(&o.overAllMatch, len(o.Regexes), func(i int) string { return o.Regexes[i].Regex })
}

func (o *Oss) PreMatch(ua string) bool {
	o.overAllOnce.Do(o.buildOverAllMatch)
	if o.overAllMatch.Regexp == nil {
		return false
	}
	return o.overAllMatch.IsMatchUserAgent(ua)
}

func (o *Oss) Parse(ua string) *OsMatchResult {
	u := NewUserAgent(ua)
	var matches []string
	var osRegex *OsReg
	for _, osRegex = range o.Regexes {
		matches = osRegex.MatchUA(u)
		if len(matches) > 0 {
			break
		}
	}

	if len(matches) == 0 || osRegex == nil {
		return nil
	}

	name := BuildByMatch(osRegex.Name, matches)
	short := UnknownShort
	if osShort, ok := operatingSystemsByName[asciiLower(name)]; ok {
		name = OperatingSystems[osShort]
		short = osShort
	}

	result := &OsMatchResult{
		Name:      name,
		ShortName: short,
		Version:   BuildVersion(osRegex.Version, matches),
		Platform:  o.parsePlatform(u),
	}
	return result
}

func GetOsFamily(osLabel string) string {
	for k, vs := range OsFamilies {
		for _, v := range vs {
			if v == osLabel {
				return k
			}
		}
	}
	return ""
}

func GetOsNameFromId(os, ver string) string {
	if osFullName, ok := OperatingSystems[os]; ok {
		return strings.TrimSpace(osFullName + " " + ver)
	}
	return ""
}
