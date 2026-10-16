package client

import (
	"fmt"
	"path/filepath"
	"sort"

	"gopkg.in/yaml.v2"

	gover "github.com/mcuadros/go-version"

	"github.com/gianluca-marchini/devicedetector/parser"
)

// Known browsers mapped to their internal short codes
var availableBrowsers = map[string]string{
	`1B`: `115 Browser`,
	`2B`: `2345 Browser`,
	`36`: `360 Phone Browser`,
	`3B`: `360 Browser`,
	`AA`: `Avant Browser`,
	`AB`: `ABrowse`,
	`AF`: `ANT Fresco`,
	`AG`: `ANTGalio`,
	`AL`: `Aloha Browser`,
	`AH`: `Aloha Browser Lite`,
	`AM`: `Amaya`,
	`AO`: `Amigo`,
	`AN`: `Android Browser`,
	`AE`: `AOL Desktop`,
	`AD`: `AOL Shield`,
	`AR`: `Arora`,
	`AX`: `Arctic Fox`,
	`AV`: `Amiga Voyager`,
	`AW`: `Amiga Aweb`,
	`A0`: `Atom`,
	`AT`: `Atomic Web Browser`,
	`AS`: `Avast Secure Browser`,
	`VG`: `AVG Secure Browser`,
	`BA`: `Beaker Browser`,
	`BM`: `Beamrise`,
	`BB`: `BlackBerry Browser`,
	`BD`: `Baidu Browser`,
	`BS`: `Baidu Spark`,
	`BI`: `Basilisk`,
	`BE`: `Beonex`,
	`BH`: `BlackHawk`,
	`BJ`: `Bunjalloo`,
	`BL`: `B-Line`,
	`BR`: `Brave`,
	`BK`: `BriskBard`,
	`BX`: `BrowseX`,
	`CA`: `Camino`,
	`CL`: `CCleaner`,
	`C0`: `Centaury`,
	`CC`: `Coc Coc`,
	`C2`: `Colibri`,
	`CD`: `Comodo Dragon`,
	`C1`: `Coast`,
	`CX`: `Charon`,
	`CE`: `CM Browser`,
	`CF`: `Chrome Frame`,
	`HC`: `Headless Chrome`,
	`CH`: `Chrome`,
	`CI`: `Chrome Mobile iOS`,
	`CK`: `Conkeror`,
	`CM`: `Chrome Mobile`,
	`CN`: `CoolNovo`,
	`CO`: `CometBird`,
	`CB`: `COS Browser`,
	`CP`: `ChromePlus`,
	`CR`: `Chromium`,
	`CY`: `Cyberfox`,
	`CS`: `Cheshire`,
	`CT`: `Crusta`,
	`CU`: `Cunaguaro`,
	`CV`: `Chrome Webview`,
	`DB`: `dbrowser`,
	`DE`: `Deepnet Explorer`,
	`DT`: `Delta Browser`,
	`DF`: `Dolphin`,
	`DO`: `Dorado`,
	`DL`: `Dooble`,
	`DI`: `Dillo`,
	`DD`: `DuckDuckGo Privacy Browser`,
	`EC`: `Ecosia`,
	`EI`: `Epic`,
	`EL`: `Elinks`,
	`EB`: `Element Browser`,
	`EE`: `Elements Browser`,
	`EZ`: `eZ Browser`,
	`EU`: `EUI Browser`,
	`EP`: `GNOME Web`,
	`ES`: `Espial TV Browser`,
	`FA`: `Falkon`,
	`FX`: `Faux Browser`,
	`F1`: `Firefox Mobile iOS`,
	`FB`: `Firebird`,
	`FD`: `Fluid`,
	`FE`: `Fennec`,
	`FF`: `Firefox`,
	`FK`: `Firefox Focus`,
	`FY`: `Firefox Reality`,
	`FR`: `Firefox Rocket`,
	`FL`: `Flock`,
	`FM`: `Firefox Mobile`,
	`FW`: `Fireweb`,
	`FN`: `Fireweb Navigator`,
	`FU`: `FreeU`,
	`GA`: `Galeon`,
	`GB`: `Glass Browser`,
	`GE`: `Google Earth`,
	`HA`: `Hawk Turbo Browser`,
	`HO`: `hola! Browser`,
	`HJ`: `HotJava`,
	`HU`: `Huawei Browser`,
	`IB`: `IBrowse`,
	`IC`: `iCab`,
	`I2`: `iCab Mobile`,
	`I1`: `Iridium`,
	`I3`: `Iron Mobile`,
	`I4`: `IceCat`,
	`ID`: `IceDragon`,
	`IV`: `Isivioo`,
	`IW`: `Iceweasel`,
	`IE`: `Internet Explorer`,
	`IM`: `IE Mobile`,
	`IR`: `Iron`,
	`JS`: `Jasmine`,
	`JI`: `Jig Browser`,
	`JP`: `Jig Browser Plus`,
	`JO`: `Jio Browser`,
	`KB`: `K.Browser`,
	`KI`: `Kindle Browser`,
	`KM`: `K-meleon`,
	`KO`: `Konqueror`,
	`KP`: `Kapiko`,
	`KN`: `Kinza`,
	`KW`: `Kiwi`,
	`KY`: `Kylo`,
	`KZ`: `Kazehakase`,
	`LB`: `Cheetah Browser`,
	`LF`: `LieBaoFast`,
	`LG`: `LG Browser`,
	`LH`: `Light`,
	`LI`: `Links`,
	`LO`: `Lovense Browser`,
	`LU`: `LuaKit`,
	`LL`: `Lulumi`,
	`LS`: `Lunascape`,
	`LN`: `Lunascape Lite`,
	`LX`: `Lynx`,
	`M1`: `mCent`,
	`MB`: `MicroB`,
	`MC`: `NCSA Mosaic`,
	`MZ`: `Meizu Browser`,
	`ME`: `Mercury`,
	`MF`: `Mobile Safari`,
	`MI`: `Midori`,
	`MO`: `Mobicip`,
	`MU`: `MIUI Browser`,
	`MS`: `Mobile Silk`,
	`MN`: `Minimo`,
	`MT`: `Mint Browser`,
	`MX`: `Maxthon`,
	`MY`: `Mypal`,
	`NB`: `Nokia Browser`,
	`NO`: `Nokia OSS Browser`,
	`NV`: `Nokia Ovi Browser`,
	`NX`: `Nox Browser`,
	`NE`: `NetSurf`,
	`NF`: `NetFront`,
	`NL`: `NetFront Life`,
	`NP`: `NetPositive`,
	`NS`: `Netscape`,
	`NT`: `NTENT Browser`,
	`OC`: `Oculus Browser`,
	`O1`: `Opera Mini iOS`,
	`OB`: `Obigo`,
	`OD`: `Odyssey Web Browser`,
	`OF`: `Off By One`,
	`HH`: `OhHai Browser`,
	`OE`: `ONE Browser`,
	`OX`: `Opera GX`,
	`OG`: `Opera Neon`,
	`OH`: `Opera Devices`,
	`OI`: `Opera Mini`,
	`OM`: `Opera Mobile`,
	`OP`: `Opera`,
	`ON`: `Opera Next`,
	`OO`: `Opera Touch`,
	`OS`: `Ordissimo`,
	`OR`: `Oregano`,
	`O0`: `Origin In-Game Overlay`,
	`OY`: `Origyn Web Browser`,
	`OV`: `Openwave Mobile Browser`,
	`OW`: `OmniWeb`,
	`OT`: `Otter Browser`,
	`PL`: `Palm Blazer`,
	`PM`: `Pale Moon`,
	`PY`: `Polypane`,
	`PP`: `Oppo Browser`,
	`PR`: `Palm Pre`,
	`PU`: `Puffin`,
	`PW`: `Palm WebPro`,
	`PA`: `Palmscape`,
	`PX`: `Phoenix`,
	`PO`: `Polaris`,
	`PT`: `Polarity`,
	`PS`: `Microsoft Edge`,
	`Q1`: `QQ Browser Mini`,
	`QQ`: `QQ Browser`,
	`QT`: `Qutebrowser`,
	`QZ`: `QupZilla`,
	`QM`: `Qwant Mobile`,
	`QW`: `QtWebEngine`,
	`RE`: `Realme Browser`,
	`RK`: `Rekonq`,
	`RM`: `RockMelt`,
	`SB`: `Samsung Browser`,
	`SA`: `Sailfish Browser`,
	`SC`: `SEMC-Browser`,
	`SE`: `Sogou Explorer`,
	`SF`: `Safari`,
	`S5`: `Safe Exam Browser`,
	`SW`: `SalamWeb`,
	`SH`: `Shiira`,
	`S1`: `SimpleBrowser`,
	`SY`: `Sizzy`,
	`SK`: `Skyfire`,
	`SS`: `Seraphic Sraf`,
	`SL`: `Sleipnir`,
	`SN`: `Snowshoe`,
	`SO`: `Sogou Mobile Browser`,
	`S2`: `Splash`,
	`SI`: `Sputnik Browser`,
	`SR`: `Sunrise`,
	`SP`: `SuperBird`,
	`SU`: `Super Fast Browser`,
	`S3`: `surf`,
	`S0`: `START Internet Browser`,
	`S4`: `Steam In-Game Overlay`,
	`ST`: `Streamy`,
	`SX`: `Swiftfox`,
	`SZ`: `Seznam Browser`,
	`TO`: `t-online.de Browser`,
	`TA`: `Tao Browser`,
	`TF`: `TenFourFox`,
	`TB`: `Tenta Browser`,
	`TZ`: `Tizen Browser`,
	`TU`: `Tungsten`,
	`TG`: `ToGate`,
	`TS`: `TweakStyle`,
	`TV`: `TV Bro`,
	`UB`: `UBrowser`,
	`UC`: `UC Browser`,
	`UM`: `UC Browser Mini`,
	`UT`: `UC Browser Turbo`,
	`UZ`: `Uzbl`,
	`VI`: `Vivaldi`,
	`VV`: `vivo Browser`,
	`VB`: `Vision Mobile Browser`,
	`VM`: `VMware AirWatch`,
	`WI`: `Wear Internet Browser`,
	`WP`: `Web Explorer`,
	`WE`: `WebPositive`,
	`WF`: `Waterfox`,
	`WH`: `Whale Browser`,
	`WO`: `wOSBrowser`,
	`WT`: `WeTab Browser`,
	`YJ`: `Yahoo! Japan Browser`,
	`YA`: `Yandex Browser`,
	`YL`: `Yandex Browser Lite`,
	`XI`: `Xiino`,
	`XV`: `Xvast`,

	// detected browsers in older versions
	// `IA` : `Iceape`,  : pim
	// `SM` : `SeaMonkey`,  : pim
}

// Browser families mapped to the short codes of the associated browsers
var browserFamilies = map[string][]string{
	`Android Browser`:    {`AN`, `MU`},
	`BlackBerry Browser`: {`BB`},
	`Baidu`:              {`BD`, `BS`},
	`Amiga`:              {`AV`, `AW`},
	`Chrome`:             {`CH`, `BA`, `BR`, `CC`, `CD`, `CM`, `CI`, `CF`, `CN`, `CR`, `CP`, `DD`, `IR`, `RM`, `AO`, `TS`, `VI`, `PT`, `AS`, `TB`, `AD`, `SB`, `WP`, `I3`, `CV`, `WH`, `SZ`, `QW`, `LF`, `KW`, `2B`, `CE`, `EC`, `MT`, `MS`, `HA`, `OC`, `MZ`, `BM`, `KN`, `SW`, `M1`, `FA`, `TA`, `AH`, `CL`, `SU`, `EU`, `UB`, `LO`, `VG`, `TV`, `A0`, `1B`, `S4`, `EE`, `AE`, `VM`, `O0`, `TG`, `GB`, `SY`, `HH`, `YJ`, `LL`, `TU`, `XV`, `C2`},
	`Firefox`:            {`FF`, `FE`, `FM`, `SX`, `FB`, `PX`, `MB`, `EI`, `WF`, `CU`, `TF`, `QM`, `FR`, `I4`, `GZ`, `MO`, `F1`, `BI`, `MN`, `BH`, `TO`, `OS`, `MY`, `FY`, `AX`, `C0`, `LH`, `S5`},
	`Internet Explorer`:  {`IE`, `IM`, `PS`},
	`Konqueror`:          {`KO`},
	`NetFront`:           {`NF`},
	`NetSurf`:            {`NE`},
	`Nokia Browser`:      {`NB`, `NO`, `NV`, `DO`},
	`Opera`:              {`OP`, `OM`, `OI`, `ON`, `OO`, `OG`, `OH`, `O1`, `OX`},
	`Safari`:             {`SF`, `MF`, `SO`},
	`Sailfish Browser`:   {`SA`},
}

// Browsers that are available for mobile devices only
var mobileOnlyBrowsers = []string{
	`36`, `OC`, `PU`, `SK`, `MF`, `OI`, `OM`, `DD`, `DB`, `ST`, `BL`, `IV`, `FM`, `C1`, `AL`, `SA`, `SB`, `FR`, `WP`, `HA`, `NX`, `HU`, `VV`, `RE`, `CB`, `MZ`, `UM`, `FK`, `FX`, `WI`, `MN`, `M1`, `AH`, `SU`, `EU`, `EZ`, `UT`, `DT`, `S0`,
}

func GetBrowserFamily(browserLabel string) (string, bool) {
	for k, vs := range browserFamilies {
		for _, v := range vs {
			if v == browserLabel {
				return k, true
			}
		}
	}
	return "", false
}

// Returns if the given browser is mobile only
func IsMobileOnlyBrowser(browser string) bool {
	if parser.ArrayContainsString(mobileOnlyBrowsers, browser) {
		return true
	}
	if v, ok := availableBrowsers[browser]; ok {
		return parser.ArrayContainsString(mobileOnlyBrowsers, v)
	}
	return false
}

type BrowserMatchResult = ClientMatchResult

// browsersByName maps the lower-cased (ASCII) names of availableBrowsers to
// their short codes.
var browsersByName = func() map[string]string {
	m := make(map[string]string, len(availableBrowsers))
	for short, name := range availableBrowsers {
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

type Engine struct {
	Default  string            `yaml:"default" json:"default"`
	Versions map[string]string `yaml:"versions" json:"versions"`
	// versions are the Versions in the order of the YAML file.
	versions []engineVersion
}

type engineVersion struct {
	version string
	engine  string
}

func (e *Engine) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var raw struct {
		Default  string            `yaml:"default"`
		Versions map[string]string `yaml:"versions"`
	}
	if err := unmarshal(&raw); err != nil {
		return err
	}
	var order struct {
		Versions yaml.MapSlice `yaml:"versions"`
	}
	if err := unmarshal(&order); err != nil {
		return err
	}
	e.Default = raw.Default
	e.Versions = raw.Versions
	e.versions = make([]engineVersion, 0, len(raw.Versions))
	used := make(map[string]bool, len(raw.Versions))
	for _, item := range order.Versions {
		k, ok := item.Key.(string)
		if !ok {
			k = fmt.Sprint(item.Key)
		}
		if engine, ok := raw.Versions[k]; ok && !used[k] {
			used[k] = true
			e.versions = append(e.versions, engineVersion{version: k, engine: engine})
		}
	}
	if len(e.versions) < len(raw.Versions) {
		e.versions = sortedEngineVersions(raw.Versions, used)
	}
	return nil
}

// sortedEngineVersions returns the versions not in skip by ascending version.
func sortedEngineVersions(versions map[string]string, skip map[string]bool) []engineVersion {
	r := make([]engineVersion, 0, len(versions))
	for k, v := range versions {
		if !skip[k] {
			r = append(r, engineVersion{version: k, engine: v})
		}
	}
	sort.Slice(r, func(i, j int) bool {
		if c := gover.CompareSimple(r[i].version, r[j].version); c != 0 {
			return c < 0
		}
		return r[i].version < r[j].version
	})
	return r
}

// orderedVersions returns the versions in the order of the YAML file. When
// the Engine was not decoded from YAML they are sorted by ascending version.
func (e *Engine) orderedVersions() []engineVersion {
	if e.versions != nil || len(e.Versions) == 0 {
		return e.versions
	}
	return sortedEngineVersions(e.Versions, nil)
}

type BrowserItem struct {
	parser.Regular `yaml:",inline" json:",inline"`
	Name           string  `yaml:"name" json:"name"`
	Version        string  `yaml:"version" json:"version"`
	Engine         *Engine `yaml:"engine" json:"engine"`
}

// Client parser for browser detection
type Browser struct {
	Regexes []*BrowserItem
	engine  BrowserEngine
	// verCache holds the engine version regexes. It is only written by Load.
	verCache map[string]*Version
}

const ParserNameBrowser = `browser`
const FixtureFileBrowser = `browsers.yml`

func init() {
	RegClientParser(ParserNameBrowser,
		func(dir string) ClientParser {
			return NewBrowser(filepath.Join(dir, FixtureFileBrowser))
		})
}

func NewBrowser(fileName string) *Browser {
	b := &Browser{}
	b.engine.ParserName = ParserNameBrowserEngine
	if err := b.Load(fileName); err != nil {
		return nil
	}
	return b
}

func (b *Browser) Load(file string) error {
	var v []*BrowserItem
	err := parser.ReadYamlFile(file, &v)
	if err != nil {
		return err
	}
	engineFile := file[0:len(file)-len(FixtureFileBrowser)] + FixtureFileBrowserEngine
	err = b.engine.Load(engineFile)
	if err != nil {
		return err
	}

	// Compile everything now: the parser must be safe for concurrent use.
	verCache := make(map[string]*Version)
	addEngine := func(engine string) {
		if _, ok := verCache[engine]; !ok && engine != "" {
			ver := &Version{Engine: engine}
			ver.Compile()
			verCache[engine] = ver
		}
	}
	for _, engine := range availableEngines {
		addEngine(engine)
	}
	for _, item := range v {
		item.Compile()
		if item.Engine != nil {
			addEngine(item.Engine.Default)
			for _, engine := range item.Engine.Versions {
				addEngine(engine)
			}
		}
	}
	b.verCache = verCache
	b.Regexes = v
	return nil
}

func (b *Browser) PreMatch(ua string) bool {
	return true
}

func (b *Browser) Parse(ua string) *BrowserMatchResult {
	u := parser.NewUserAgent(ua)
	for _, regex := range b.Regexes {
		matches := regex.MatchUA(u)
		if len(matches) > 0 {
			name := parser.BuildByMatch(regex.Name, matches)
			if browserShort, ok := browsersByName[asciiLower(name)]; ok {
				version := parser.BuildVersion(regex.Version, matches)
				engine := b.buildEngine(regex.Engine, version, u)
				engineVersion := b.BuildEngineVersion(engine, ua)
				return &BrowserMatchResult{
					Type:          ParserNameBrowser,
					Name:          availableBrowsers[browserShort],
					ShortName:     browserShort,
					Version:       version,
					Engine:        engine,
					EngineVersion: engineVersion,
				}
			}
		}
	}
	return nil
}

func (b *Browser) BuildEngine(engineData *Engine, browserVersion, ua string) string {
	return b.buildEngine(engineData, browserVersion, parser.NewUserAgent(ua))
}

func (b *Browser) buildEngine(engineData *Engine, browserVersion string, ua *parser.UserAgent) string {
	engine := ""
	if engineData != nil {
		engine = engineData.Default
		// The last version lower than or equal to the browser version wins.
		for _, v := range engineData.orderedVersions() {
			if gover.CompareSimple(browserVersion, v.version) >= 0 {
				engine = v.engine
			}
		}
	}
	if engine == "" {
		if engineResult := b.engine.parse(ua); engineResult != nil {
			engine = engineResult.Name
		}
	}
	return engine
}

func (b *Browser) BuildEngineVersion(engine, ua string) string {
	if engine == "" {
		return ""
	}
	v, ok := b.verCache[engine]
	if !ok {
		// Not known at load time: compile it without caching.
		v = &Version{Engine: engine}
		v.Compile()
	}
	return v.Parse(ua)
}
