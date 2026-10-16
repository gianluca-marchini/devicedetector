package devicedetector

import (
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gianluca-marchini/devicedetector/parser"
)

// More brands of mobiles.yml match these user agents: the first one in the
// order of the file wins, like in Matomo (expected values from the fixtures).
func TestDeviceBrandOrder(t *testing.T) {
	parser.ResetParserAbstract()
	d := newMiddlewareDetector(t)
	cases := []struct {
		ua, typ, brand, model string
	}{
		{`Pulp/1.5.2 (iPad; http://www.acrylicapps.com/pulp/)`, `tablet`, `AP`, `iPad`},
		{`FlyCast/1.34 (BlackBerry; 8330/4.5.0.131 Profile/MIDP-2.0 Configuration/CLDC-1.1 VendorID/-1)`, `smartphone`, `RM`, `BlackBerry`},
		{`Mozilla/5.0 (Windows NT 10.0; ARM; Lumia 950 XL) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/46.0.2486.0 Safari/537.36 Edge/14.14295`, `phablet`, `MS`, `Lumia 950 XL`},
		{`Mozilla/5.0 (Linux; Android 4.3; ja-jp; SC-01F Build/JSS15J) AppleWebKit/537.36 (KHTML, like Gecko) Version/1.5 Chrome/28.0.1500.94 Mobile Safari/537.36`, `phablet`, `SA`, `GALAXY Note 3`},
		{`Mozilla/4.0 (compatible; MSIE 7.0; Windows Phone OS 7.0; Trident/3.1; IEMobile/ 7.0; WpsLondonTest; Asus;Galaxy6)`, `smartphone`, `AU`, `Galaxy6`},
	}
	for _, c := range cases {
		// The order of a Go map changes at every iteration: repeat.
		for i := 0; i < 20; i++ {
			info := d.Parse(c.ua)
			require.Equal(t, c.typ, info.Type, c.ua)
			require.Equal(t, c.brand, info.Brand, c.ua)
			require.Equal(t, c.model, info.Model, c.ua)
		}
	}
}

// The engine versions of a browser are applied in the order of the YAML file:
// Opera 12 is Presto, not Elektra.
func TestBrowserEngineVersionOrder(t *testing.T) {
	parser.ResetParserAbstract()
	d := newMiddlewareDetector(t)
	cases := map[string]string{
		`Opera/9.80 (X11; Linux zbov) Presto/2.11.355 Version/12.10`:                                                                    `Presto`,
		`Opera/9.80 (Windows NT 5.1; U; de) Presto/2.2.15 Version/10.10`:                                                                `Presto`,
		`Mozilla/5.0 (Windows NT 6.3; WOW64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/31.0.1650.63 Safari/537.36 OPR/18.0.1284.68`: `Blink`,
	}
	for ua, engine := range cases {
		for i := 0; i < 20; i++ {
			require.Equal(t, engine, d.Parse(ua).GetClient().Engine, ua)
		}
	}
}

// One detector shared by many goroutines must give the same results as a
// sequential parse. Run with -race.
func TestParseConcurrent(t *testing.T) {
	parser.ResetParserAbstract()
	uas := benchCorpus(t)
	if testing.Short() || raceEnabled {
		uas = uas[:300]
	}

	type result struct {
		os, client, device, bot string
	}
	run := func(mw, bots *DeviceDetector, ua string) result {
		var r result
		if info := mw.Parse(ua); info != nil {
			r.os = fmt.Sprint(info.os)
			if info.client != nil {
				r.client = fmt.Sprint(*info.client)
			}
			r.device = fmt.Sprint(info.DeviceMatchResult, info.GetDeviceType())
		}
		if info := bots.Parse(ua); info != nil && info.bot != nil {
			r.bot = fmt.Sprint(*info.bot)
		}
		return r
	}
	newBots := func() *DeviceDetector {
		d, err := NewDeviceDetector("regexes", &CacheParams{IsEnabled: true, Size: 64})
		require.NoError(t, err)
		return d
	}

	refMw, refBots := newMiddlewareDetector(t), newBots()
	expected := make([]result, len(uas))
	for i, ua := range uas {
		expected[i] = run(refMw, refBots, ua)
	}

	mw, bots := newMiddlewareDetector(t), newBots()
	const goroutines = 16
	var wg sync.WaitGroup
	errs := make(chan string, goroutines)
	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for k := range uas {
				i := (k + g*len(uas)/goroutines) % len(uas)
				if got := run(mw, bots, uas[i]); got != expected[i] {
					errs <- fmt.Sprintf("%s: got %+v, want %+v", uas[i], got, expected[i])
					return
				}
			}
		}(g)
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		t.Error(e)
	}
}

// Concurrent Add of the same keys must not duplicate them in key_list, so
// that the eviction always removes the oldest entry of the map.
func TestCacheConcurrentAdd(t *testing.T) {
	cache := NewCache(8)
	var wg sync.WaitGroup
	for g := 0; g < 16; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for i := 0; i < 2000; i++ {
				// Fewer keys than the size of the cache: every Add after the
				// first ones is a duplicate.
				ua := fmt.Sprintf("ua-%d", (i+g)%6)
				cache.Add(ua, &DeviceInfo{userAgent: ua})
				cache.Lookup(ua)
			}
		}(g)
	}
	wg.Wait()

	require.Equal(t, len(cache.cache), cache.key_list.Len())
	require.Equal(t, 6, cache.key_list.Len())
	seen := map[string]bool{}
	for e := cache.key_list.Front(); e != nil; e = e.Next() {
		ua := e.Value.(string)
		require.False(t, seen[ua], "duplicated key %s", ua)
		seen[ua] = true
		_, ok := cache.cache[ua]
		require.True(t, ok, "key %s not in the map", ua)
	}
}

func TestCacheAddExistingKey(t *testing.T) {
	cache := NewCache(2)
	a1, a2, b, c := &DeviceInfo{}, &DeviceInfo{}, &DeviceInfo{}, &DeviceInfo{}
	cache.Add("a", a1)
	cache.Add("a", a2)
	cache.Add("b", b)
	require.Equal(t, 2, cache.key_list.Len())
	got, hit := cache.Lookup("a")
	require.True(t, hit)
	require.Same(t, a2, got)

	// "a" is the oldest entry: adding "c" evicts it and keeps "b".
	cache.Add("c", c)
	_, hit = cache.Lookup("a")
	require.False(t, hit)
	got, hit = cache.Lookup("b")
	require.True(t, hit)
	require.Same(t, b, got)
	require.Len(t, cache.cache, 2)
}

func TestNilCacheParams(t *testing.T) {
	d, err := NewDeviceDetector("regexes", nil)
	require.NoError(t, err)
	require.Nil(t, d.cache)
}
