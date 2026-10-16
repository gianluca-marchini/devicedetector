package devicedetector

import (
	"path/filepath"
	"strings"

	gover "github.com/mcuadros/go-version"

	"github.com/gianluca-marchini/devicedetector/parser"
	"github.com/gianluca-marchini/devicedetector/parser/client"
	"github.com/gianluca-marchini/devicedetector/parser/device"
)

const UNKNOWN = "UNK"
const VERSION = `3.12.5`

var desktopOsArray = []string{
	`AmigaOS`,
	`IBM`,
	`GNU/Linux`,
	`Mac`,
	`Unix`,
	`Windows`,
	`BeOS`,
	`Chrome OS`,
}

var (
	chrMobRegex = parser.NewRegex(fixUserAgentRegEx(`Chrome/[\.0-9]* Mobile`))
	chrTabRegex = parser.NewRegex(fixUserAgentRegEx(`Chrome/[\.0-9]* (?!Mobile)`))
	opaTabRegex = parser.NewRegex(fixUserAgentRegEx(`Opera Tablet`))
	opaTvRegex  = parser.NewRegex(fixUserAgentRegEx(`Opera TV Store`))

	chrMobReg = chrMobRegex.Regexp()
	chrTabReg = chrTabRegex.Regexp()
	opaTabReg = opaTabRegex.Regexp()
	opaTvReg  = opaTvRegex.Regexp()
)

func fixUserAgentRegEx(regex string) string {
	reg := strings.ReplaceAll(regex, `/`, `\/`)
	reg = strings.ReplaceAll(reg, `++`, `+`)
	return `(?:^|[^A-Z_-])(?:` + reg + `)`
}

type DeviceDetector struct {
	cache                 *Cache
	deviceParsers         []device.DeviceParser
	clientParsers         []client.ClientParser
	botParsers            []parser.BotParser
	osParsers             []parser.OsParser
	vendorParser          *parser.VendorFragments
	DiscardBotInformation bool
	SkipBotDetection      bool
}

type CacheParams struct {
	IsEnabled bool
	Size      int
}

// Initialize the device detector.
// - dir: path of the folder containing the regexes to parse the userAgent
// - cacheParams: if IsEnabled is true, the cache will be enabled. A nil value disables the cache.
func NewDeviceDetector(dir string, cacheParams *CacheParams) (*DeviceDetector, error) {
	vp, err := parser.NewVendor(filepath.Join(dir, parser.FixtureFileVendor))
	if err != nil {
		return nil, err
	}

	osp, err := parser.NewOss(filepath.Join(dir, parser.FixtureFileOs))
	if err != nil {
		return nil, err
	}

	d := &DeviceDetector{
		cache:        nil,
		vendorParser: vp,
		osParsers:    []parser.OsParser{osp},
	}

	if cacheParams != nil && cacheParams.IsEnabled {
		if cacheParams.Size == 0 {
			cacheParams.Size = CACHE_DEFAULT_SIZE
		}

		d.cache = NewCache(cacheParams.Size)
	}

	clientDir := filepath.Join(dir, "client")
	d.clientParsers = client.NewClientParsers(clientDir,
		[]string{
			client.ParserNameFeedReader,
			client.ParserNameMobileApp,
			client.ParserNameMediaPlayer,
			client.ParserNamePim,
			client.ParserNameBrowser,
			client.ParserNameLibrary,
		})

	deviceDir := filepath.Join(dir, "device")
	d.deviceParsers = device.NewDeviceParsers(deviceDir,
		[]string{
			device.ParserNameHbbTv,
			device.ParserNameConsole,
			device.ParserNameCar,
			device.ParserNameCamera,
			device.ParserNamePortableMediaPlayer,
			device.ParserNameMobile,
		})

	d.botParsers = []parser.BotParser{
		parser.NewBot(filepath.Join(dir, parser.FixtureFileBot)),
	}

	return d, nil
}

func (d *DeviceDetector) AddClientParser(cp client.ClientParser) {
	d.clientParsers = append(d.clientParsers, cp)
}

func (d *DeviceDetector) GetClientParser() []client.ClientParser {
	return d.clientParsers
}

func (d *DeviceDetector) AddDeviceParser(dp device.DeviceParser) {
	d.deviceParsers = append(d.deviceParsers, dp)
}

func (d *DeviceDetector) GetDeviceParsers() []device.DeviceParser {
	return d.deviceParsers
}

func (d *DeviceDetector) AddBotParser(op parser.BotParser) {
	d.botParsers = append(d.botParsers, op)
}

func (d *DeviceDetector) GetBotParsers() []parser.BotParser {
	return d.botParsers
}

func (d *DeviceDetector) ParseBot(ua string) *parser.BotMatchResult {
	if !d.SkipBotDetection {
		for _, parser := range d.botParsers {
			parser.DiscardDetails(d.DiscardBotInformation)
			if r := parser.Parse(ua); r != nil {
				return r
			}
		}
	}
	return nil
}

func (d *DeviceDetector) ParseOs(ua string) *parser.OsMatchResult {
	return d.parseOs(parser.NewUserAgent(ua))
}

func (d *DeviceDetector) parseOs(u *parser.UserAgent) *parser.OsMatchResult {
	for _, p := range d.osParsers {
		var r *parser.OsMatchResult
		// Only the built-in parsers take the prepared user agent: a custom
		// parser (even one embedding a built-in one) gets its own Parse called.
		if o, ok := p.(*parser.Oss); ok {
			r = o.ParseUserAgent(u)
		} else {
			r = p.Parse(u.String())
		}
		if r != nil {
			return r
		}
	}
	return nil
}

func (d *DeviceDetector) ParseClient(ua string) *client.ClientMatchResult {
	return d.parseClient(parser.NewUserAgent(ua))
}

func (d *DeviceDetector) parseClient(u *parser.UserAgent) *client.ClientMatchResult {
	for _, p := range d.clientParsers {
		var r *client.ClientMatchResult
		switch p := p.(type) {
		case *client.FeedReader:
			r = p.ParseUserAgent(u)
		case *client.MobileApp:
			r = p.ParseUserAgent(u)
		case *client.MediaPlayer:
			r = p.ParseUserAgent(u)
		case *client.Pim:
			r = p.ParseUserAgent(u)
		case *client.Browser:
			r = p.ParseUserAgent(u)
		case *client.Library:
			r = p.ParseUserAgent(u)
		default:
			r = p.Parse(u.String())
		}
		if r != nil {
			return r
		}
	}
	return nil
}

func (d *DeviceDetector) ParseDevice(ua string) *device.DeviceMatchResult {
	return d.parseDevice(parser.NewUserAgent(ua))
}

func (d *DeviceDetector) parseDevice(u *parser.UserAgent) *device.DeviceMatchResult {
	for _, p := range d.deviceParsers {
		var r *device.DeviceMatchResult
		switch p := p.(type) {
		case *device.HbbTv:
			r = p.ParseUserAgent(u)
		case *device.Console:
			r = p.ParseUserAgent(u)
		case *device.Car:
			r = p.ParseUserAgent(u)
		case *device.Camera:
			r = p.ParseUserAgent(u)
		case *device.PortableMediaPlayer:
			r = p.ParseUserAgent(u)
		case *device.Mobile:
			r = p.ParseUserAgent(u)
		default:
			r = p.Parse(u.String())
		}
		if r != nil {
			return r
		}
	}
	return nil
}

func (d *DeviceDetector) parseInfo(info *DeviceInfo, u *parser.UserAgent) {
	if r := d.parseDevice(u); r != nil {
		info.Type = r.Type
		info.Model = r.Model
		info.Brand = r.Brand
	}
	// If no brand has been assigned try to match by known vendor fragments
	if info.Brand == "" && d.vendorParser != nil {
		info.Brand = d.vendorParser.ParseUserAgent(u)
	}

	os := info.GetOs()
	osShortName := os.ShortName
	osFamily := parser.GetOsFamily(osShortName)
	osVersion := os.Version
	cmr := info.GetClient()

	if info.Brand == "" && (osShortName == `ATV` || osShortName == `IOS` || osShortName == `MAC`) {
		info.Brand = `AP`
	}

	deviceType := parser.GetDeviceType(info.Type)
	// Chrome on Android passes the device type based on the keyword 'Mobile'
	// If it is present the device should be a smartphone, otherwise it's a tablet
	// See https://developer.chrome.com/multidevice/user-agent#chrome_for_android_user_agent
	if deviceType == parser.DEVICE_TYPE_INVALID && osFamily == `Android` {
		if browserName, ok := client.GetBrowserFamily(cmr.ShortName); ok && browserName == `Chrome` {
			if chrMobRegex.IsMatch(u) {
				deviceType = parser.DEVICE_TYPE_SMARTPHONE
			} else if chrTabRegex.IsMatch(u) {
				deviceType = parser.DEVICE_TYPE_TABLET
			}
		}
	}

	if deviceType == parser.DEVICE_TYPE_INVALID {
		if adrTabRegex.IsMatch(u) {
			deviceType = parser.DEVICE_TYPE_TABLET
		} else if opaTabRegex.IsMatch(u) {
			deviceType = parser.DEVICE_TYPE_TABLET
		} else if adrMobRegex.IsMatch(u) {
			deviceType = parser.DEVICE_TYPE_SMARTPHONE
		} else if osShortName == "AND" && osVersion != "" {
			if gover.CompareSimple(osVersion, `2.0`) == -1 {
				deviceType = parser.DEVICE_TYPE_SMARTPHONE
			} else if gover.CompareSimple(osVersion, `3.0`) >= 0 &&
				gover.CompareSimple(osVersion, `4.0`) == -1 {
				deviceType = parser.DEVICE_TYPE_TABLET
			}
		}
	}

	// All detected feature phones running android are more likely a smartphone
	if deviceType == parser.DEVICE_TYPE_FEATURE_PHONE && osFamily == `Android` {
		deviceType = parser.DEVICE_TYPE_SMARTPHONE
	}

	// According to http://msdn.microsoft.com/en-us/library/ie/hh920767(v=vs.85).aspx
	if deviceType == parser.DEVICE_TYPE_INVALID &&
		(osShortName == `WRT` || (osShortName == `WIN` && gover.CompareSimple(osVersion, `8`) >= 0)) &&
		touchRegex.IsMatch(u) {
		deviceType = parser.DEVICE_TYPE_TABLET
	}

	// All devices running Opera TV Store are assumed to be a tv
	if opaTvRegex.IsMatch(u) {
		deviceType = parser.DEVICE_TYPE_TV
	}

	// Devices running Kylo or Espital TV Browsers are assumed to be a TV
	if deviceType == parser.DEVICE_TYPE_INVALID {
		if cmr.Name == `Kylo` || cmr.Name == `Espial TV Browser` {
			deviceType = parser.DEVICE_TYPE_TV
		} else if info.IsDesktop() {
			deviceType = parser.DEVICE_TYPE_DESKTOP
		}
	}

	if deviceType != parser.DEVICE_TYPE_INVALID {
		info.Type = parser.GetDeviceName(deviceType)
	}
}

// Cache the deviceInfo if the cache is enabled
func (d *DeviceDetector) cacheDeviceInfo(ua string, deviceInfo *DeviceInfo) *DeviceInfo {
	if d.cache != nil {
		d.cache.Add(ua, deviceInfo)
	}

	return deviceInfo
}

// Purge the cache.
// It may be used in case of dynamic update of the referenced regexes.
func (d *DeviceDetector) PurgeCache() {
	if d.cache != nil {
		d.cache.Purge()
	}
}

// Parse the userAgent and retrieve information, if it is valid
func (d *DeviceDetector) Parse(ua string) *DeviceInfo {
	// Skip parsing for empty useragents or those not containing any letter
	if !parser.StringContainsLetter(ua) {
		return nil
	}

	// Try to search for the userAgent in the cache
	if d.cache != nil {
		if deviceInfo, hit := d.cache.Lookup(ua); hit {
			return deviceInfo
		}
	}

	// Start parsing
	info := &DeviceInfo{
		userAgent: ua,
	}

	info.bot = d.ParseBot(ua)
	if info.IsBot() {
		return d.cacheDeviceInfo(ua, info)
	}

	u := parser.NewUserAgent(ua)
	info.os = d.parseOs(u)

	// Parse Clients
	// Clients might be browsers, Feed Readers, Mobile Apps, Media Players or
	// any other application accessing with an parseable UA
	info.client = d.parseClient(u)

	d.parseInfo(info, u)

	return d.cacheDeviceInfo(ua, info)
}
