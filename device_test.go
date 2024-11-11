package devicedetector

import (
	"strconv"
	"sync"
	"testing"

	regexp "github.com/dlclark/regexp2"
	"github.com/gianluca-marchini/devicedetector/parser"
	"github.com/gianluca-marchini/devicedetector/parser/client"
	"github.com/gianluca-marchini/devicedetector/parser/device"
	"github.com/stretchr/testify/require"
)

var dd, _ = NewDeviceDetector("regexes", &CacheParams{false, 0})

func TestParseInvalidUA(t *testing.T) {
	info := dd.Parse(`12345`)
	if info != nil {
		t.Fatal("testParseInvalidUA fail")
	}
}

func TestInstanceReusage(t *testing.T) {
	userAgents := [][]string{
		{
			`Sraf/3.0 (Linux i686 ; U; HbbTV/1.1.1 (+PVR+DL;NEXtUS; TV44; sw1.0) CE-HTML/1.0 Config(L:eng,CC:DEU); en/de)`,
			``,
			``,
		},
		{
			`Mozilla/5.0 (Linux; Android 4.2.2; ARCHOS 101 PLATINUM Build/JDQ39) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/34.0.1847.114 Safari/537.36`,
			`Archos`,
			`101 PLATINUM`,
		},
		{
			`Opera/9.80 (Linux mips; U; HbbTV/1.1.1 (; Vestel; MB95; 1.0; 1.0; ); en) Presto/2.10.287 Version/12.00`,
			`Vestel`,
			`MB95`,
		},
	}

	for i, item := range userAgents {
		info := dd.Parse(item[0])
		require.Equal(t, info.GetBrandName(), item[1], i)
		require.Equal(t, info.GetModel(), item[2], i)
	}
}

func TestVersionTruncation(t *testing.T) {
	data := map[int][]string{
		parser.VERSION_TRUNCATION_NONE:  {`Mozilla/5.0 (Linux; Android 4.2.2; ARCHOS 101 PLATINUM Build/JDQ39) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/34.0.1847.114 Safari/537.36`, `4.2.2`, `34.0.1847.114`},
		parser.VERSION_TRUNCATION_BUILD: {`Mozilla/5.0 (Linux; Android 4.2.2; ARCHOS 101 PLATINUM Build/JDQ39) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/34.0.1847.114 Safari/537.36`, `4.2.2`, `34.0.1847.114`},
		parser.VERSION_TRUNCATION_PATCH: {`Mozilla/5.0 (Linux; Android 4.2.2; ARCHOS 101 PLATINUM Build/JDQ39) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/34.0.1847.114 Safari/537.36`, `4.2.2`, `34.0.1847`},
		parser.VERSION_TRUNCATION_MINOR: {`Mozilla/5.0 (Linux; Android 4.2.2; ARCHOS 101 PLATINUM Build/JDQ39) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/34.0.1847.114 Safari/537.36`, `4.2`, `34.0`},
		parser.VERSION_TRUNCATION_MAJOR: {`Mozilla/5.0 (Linux; Android 4.2.2; ARCHOS 101 PLATINUM Build/JDQ39) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/34.0.1847.114 Safari/537.36`, `4`, `34`},
	}
	for k, v := range data {
		parser.SetVersionTruncation(k)
		info := dd.Parse(v[0])
		require.Equal(t, info.GetOs().Version, v[1])
		require.Equal(t, info.GetClient().Version, v[2])
	}
}

func TestBot(t *testing.T) {
	parser.ResetParserAbstract()

	type BotTest struct {
		Ua  string                `yaml:"user_agent"`
		Bot parser.BotMatchResult `yaml:"bot"`
	}
	var listBotTest []BotTest
	err := parser.ReadYamlFile(`fixtures/bots.yml`, &listBotTest)
	if err != nil {
		t.Error(err)
	}

	for _, item := range listBotTest {
		info := dd.Parse(item.Ua)
		bot := info.GetBot()
		if bot == nil {
			t.Error("bot is null")
		}

		if item.Bot.Equal(bot) == false {
			t.Error("bot is null")
		}

		osName := info.GetOs().ShortName
		clientName := info.GetClient().ShortName
		require.Equal(t, osName, "", osName)
		require.Equal(t, clientName, "", clientName)
	}
}

func TestTypeMethods(t *testing.T) {
	parser.ResetParserAbstract()

	data := map[string][]bool{
		`Googlebot/2.1 (http://www.googlebot.com/bot.html)`: {true, false, false},
		`Mozilla/5.0 (Linux; Android 4.4.2; Nexus 4 Build/KOT49H) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/33.0.1750.136 Mobile Safari/537.36`: {false, true, false},
		`Mozilla/5.0 (Linux; Android 4.4.3; Build/KTU84L) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/37.0.2062.117 Mobile Safari/537.36`:         {false, true, false},
		`Mozilla/5.0 (compatible; MSIE 9.0; Windows NT 6.1; WOW64; Trident/5.0)`:                                                                    {false, false, true},
		`Mozilla/3.01 (compatible;)`: {false, false, false},
		// Mobile only browsers:
		`Opera/9.80 (J2ME/MIDP; Opera Mini/9.5/37.8069; U; en) Presto/2.12.423 Version/12.16`:                                                                                          {false, true, false},
		`Mozilla/5.0 (X11; U; Linux i686; th-TH@calendar=gregorian) AppleWebKit/534.12 (KHTML, like Gecko) Puffin/1.3.2665MS Safari/534.12`:                                            {false, true, false},
		`Mozilla/5.0 (Linux; Android 4.4.4; MX4 Pro Build/KTU84P) AppleWebKit/537.36 (KHTML, like Gecko) Version/4.0 Chrome/33.0.0.0 Mobile Safari/537.36; 360 Aphone Browser (6.9.7)`: {false, true, false},
		`Mozilla/5.0 (Macintosh; U; Intel Mac OS X 10_5_7; xx) AppleWebKit/530.17 (KHTML, like Gecko) Version/4.0 Safari/530.17 Skyfire/6DE`:                                           {false, true, false},
		// useragent containing non unicode chars
		`Mozilla/5.0 (Linux; U; Android 4.1.2; ru-ru; PMP7380D3G Build/JZO54K) AppleWebKit/534.30 (KHTML, ÃÂºÃÂ°ÃÂº Gecko) Version/4.0 Safari/534.30`: {false, true, false},
	}
	for k, v := range data {
		dd.DiscardBotInformation = true
		info := dd.Parse(k)
		require.Equal(t, info.IsBot(), v[0], k)
		require.Equal(t, info.IsMobile(), v[1], k)
		require.Equal(t, info.IsDesktop(), v[2], k)
	}
}

func TestGetOs(t *testing.T) {
	parser.ResetParserAbstract()

	ua := `Mozilla/5.0 (compatible; MSIE 9.0; Windows NT 6.1; WOW64; Trident/5.0)`
	info := dd.Parse(ua)
	os := info.GetOs()
	require.Equal(t, os.Name, `Windows`)
	require.Equal(t, os.ShortName, `WIN`)
	require.Equal(t, os.Version, `7`)
	require.Equal(t, os.Platform, `x64`)
}

func TestGetClient(t *testing.T) {
	parser.ResetParserAbstract()

	ua := `Mozilla/5.0 (compatible; MSIE 9.0; Windows NT 6.1; WOW64; Trident/5.0)`
	info := dd.Parse(ua)
	client := info.GetClient()
	require.Equal(t, client.Type, `browser`)
	require.Equal(t, client.Name, `Internet Explorer`)
	require.Equal(t, client.ShortName, `IE`)
	require.Equal(t, client.Version, `9.0`)
	require.Equal(t, client.Engine, `Trident`)
	require.Equal(t, client.EngineVersion, `5.0`)
}

func TestGetBrandName(t *testing.T) {
	parser.ResetParserAbstract()

	ua := `Mozilla/5.0 (Linux; Android 4.4.2; Nexus 4 Build/KOT49H) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/33.0.1750.136 Mobile Safari/537.36`
	info := dd.Parse(ua)
	require.Equal(t, info.GetBrandName(), `Google`)
}

func TestIsTouchEnabled(t *testing.T) {
	parser.ResetParserAbstract()

	ua := `Mozilla/5.0 (compatible; MSIE 10.0; Windows NT 6.2; ARM; Trident/6.0; Touch; ARMBJS)`
	info := dd.Parse(ua)
	require.True(t, info.IsTouchEnabled())
}

func TestSkipBotDetection(t *testing.T) {
	parser.ResetParserAbstract()

	ua := `Mozilla/5.0 (iPhone; CPU iPhone OS 6_0 like Mac OS X) AppleWebKit/536.26 (KHTML, like Gecko) Version/6.0 Mobile/10A5376e Safari/8536.25 (compatible; Googlebot/2.1; +http://www.google.com/bot.html)`
	info := dd.Parse(ua)
	require.False(t, info.IsMobile())
	require.True(t, info.IsBot())
	dd.SkipBotDetection = true
	info = dd.Parse(ua)
	require.True(t, info.IsMobile())
	require.False(t, info.IsBot())
}

type SmartFixture struct {
	UserAgent     string                    `yaml:"user_agent"`
	Os            *parser.OsMatchResult     `yaml:"os"`
	Client        *client.ClientMatchResult `yaml:"client"`
	Device        *device.DeviceMatchResult `yaml:"device"`
	OsFamily      string                    `yaml:"os_family"`
	BrowserFamily string                    `yaml:"browser_family"`
}

func TestRegThread(t *testing.T) {
	parser.ResetParserAbstract()

	// read file
	var lists [][]*SmartFixture
	for i := 0; i <= 12; i++ {
		var list []*SmartFixture
		var name string
		if i == 0 {
			name = `smartphone.yml`
		} else {
			name = `smartphone-` + strconv.Itoa(i) + `.yml`
		}
		err := parser.ReadYamlFile(`fixtures/`+name, &list)
		if err == nil {
			lists = append(lists, list)
		}
	}
	rs := []*regexp.Regexp{adrMobReg, touchReg, adrTabReg, chrMobReg, chrTabReg, opaTabReg, opaTvReg}
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		for _, list := range lists {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for _, f := range list {
					ua := f.UserAgent
					for _, reg := range rs {
						reg.MatchString(ua)
						reg.FindStringMatch(ua)
					}
				}
			}()
		}
	}
	wg.Wait()
}
