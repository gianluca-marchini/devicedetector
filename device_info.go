package devicedetector

import (
	"github.com/gianluca-marchini/devicedetector/parser"
	"github.com/gianluca-marchini/devicedetector/parser/client"
	"github.com/gianluca-marchini/devicedetector/parser/device"
)

var (
	touchRegex  = parser.NewRegex(fixUserAgentRegEx(`Touch`))
	adrTabRegex = parser.NewRegex(fixUserAgentRegEx(`Android( [\.0-9]+)?; Tablet;`))
	adrMobRegex = parser.NewRegex(fixUserAgentRegEx(`Android( [\.0-9]+)?; Mobile;`))

	touchReg  = touchRegex.Regexp()
	adrTabReg = adrTabRegex.Regexp()
	adrMobReg = adrMobRegex.Regexp()
)

type DeviceInfo struct {
	userAgent string
	device.DeviceMatchResult
	client *client.ClientMatchResult
	os     *parser.OsMatchResult
	bot    *parser.BotMatchResult
}

func (d *DeviceInfo) GetDeviceType() int {
	return parser.GetDeviceType(d.Type)
}

func (d *DeviceInfo) IsBot() bool {
	return d.bot != nil
}

func (d *DeviceInfo) IsTouchEnabled() bool {
	return touchRegex.IsMatch(parser.NewUserAgent(d.userAgent))
}

func (d *DeviceInfo) HasAndroidTableFragment() bool {
	return adrTabRegex.IsMatch(parser.NewUserAgent(d.userAgent))
}

func (d *DeviceInfo) HasAndroidMobileFragment() bool {
	return adrMobRegex.IsMatch(parser.NewUserAgent(d.userAgent))
}

func (d *DeviceInfo) UsesMobileBrowser() bool {
	return d.client != nil && d.client.Type == client.ParserNameBrowser && client.IsMobileOnlyBrowser(d.client.ShortName)
}

func (d *DeviceInfo) IsMobile() bool {
	if d.Type != "" {
		if deviceType := parser.GetDeviceType(d.Type); parser.DEVICE_TYPE_INVALID != deviceType {
			switch deviceType {
			case parser.DEVICE_TYPE_FEATURE_PHONE,
				parser.DEVICE_TYPE_SMARTPHONE,
				parser.DEVICE_TYPE_TABLET,
				parser.DEVICE_TYPE_PHABLET,
				parser.DEVICE_TYPE_CAMERA,
				parser.DEVICE_TYPE_PORTABLE_MEDIA_PAYER:
				return true
			case parser.DEVICE_TYPE_TV,
				parser.DEVICE_TYPE_SMART_DISPLAY,
				parser.DEVICE_TYPE_CONSOLE:
				return false
			}
		}
	}

	if d.UsesMobileBrowser() {
		return true
	}

	if d.os == nil || d.os.ShortName == "" || d.os.ShortName == UNKNOWN {
		return false
	}

	return !d.IsBot() && !d.IsDesktop()
}

func (d *DeviceInfo) IsDesktop() bool {
	if d.os == nil || d.os.ShortName == "" || d.os.ShortName == UNKNOWN {
		return false
	}

	// Check for browsers available for mobile devices only
	if d.UsesMobileBrowser() {
		return false
	}

	if decodedFamily := parser.GetOsFamily(d.os.ShortName); decodedFamily != "" {
		return parser.ArrayContainsString(desktopOsArray, decodedFamily)
	}
	return false
}

func (d *DeviceInfo) GetOs() *parser.OsMatchResult {
	if d.os != nil {
		return d.os
	}
	return &parser.OsMatchResult{}
}

func (d *DeviceInfo) GetClient() *client.ClientMatchResult {
	if d.client != nil {
		return d.client
	}
	return &client.ClientMatchResult{}
}

func (d *DeviceInfo) GetBrowserClient() *client.BrowserMatchResult {
	if d.client != nil && d.client.Type == client.ParserNameBrowser {
		return d.client
	}
	return &client.BrowserMatchResult{}
}

func (d *DeviceInfo) GetDevice() *device.DeviceMatchResult {
	return &d.DeviceMatchResult
}

func (d *DeviceInfo) GetDeviceName() string {
	return d.Type
}

func (d *DeviceInfo) GetBrand() string {
	return d.Brand
}

func (d *DeviceInfo) GetBrandName() string {
	return parser.GetFullName(d.Brand)
}

func (d *DeviceInfo) GetModel() string {
	return d.Model
}

func (d *DeviceInfo) GetUserAgent() string {
	return d.userAgent
}

func (d *DeviceInfo) GetBot() *parser.BotMatchResult {
	return d.bot
}

func (d *DeviceInfo) GetOsFamily() string {
	if d.os != nil {
		return parser.GetOsFamily(d.os.ShortName)
	}
	return ""
}

func (d *DeviceInfo) GetBrowserFamily() string {
	if d.client != nil {
		if v, ok := client.GetBrowserFamily(d.client.ShortName); ok {
			return v
		}
	}
	return ""
}
