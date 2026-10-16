package devicedetector

import (
	"bufio"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gianluca-marchini/devicedetector/parser"
	"github.com/gianluca-marchini/devicedetector/parser/client"
	"github.com/gianluca-marchini/devicedetector/parser/device"
)

// This file contains the user agent corpus shared by the benchmarks and the
// snapshot equivalence test.
//
// The corpus is made of every user agent of the fixtures (deduplicated, in
// file order) plus testdata/extra_user_agents.txt.
//
//   - DD_SNAPSHOT_WRITE=<path> go test -run TestSnapshotWrite
//     writes the snapshot of the current code to <path> (gzipped JSON lines).
//   - TestSnapshotEquivalence compares the current code against
//     testdata/snapshot.json.gz.

const snapshotReferenceFile = "testdata/snapshot.json.gz"

// corpusFixture is one entry of a fixture file. The expected values are kept
// generic because the fixtures use `[]` for unknown values.
type corpusFixture struct {
	UserAgent string `yaml:"user_agent"`
	Os        any    `yaml:"os"`
	Client    any    `yaml:"client"`
	Device    any    `yaml:"device"`
	Bot       any    `yaml:"bot"`
	File      string `yaml:"-"`
}

// fixtureField returns the value of key in a fixture section, "" when absent.
func fixtureField(section any, key string) string {
	m, ok := section.(map[any]any)
	if !ok {
		return ""
	}
	if v, ok := m[key]; ok && v != nil {
		return fmt.Sprint(v)
	}
	return ""
}

var (
	corpusOnce     sync.Once
	corpusFixtures []corpusFixture
	corpusUAs      []string
	corpusErr      error
)

func loadCorpus(tb testing.TB) ([]string, []corpusFixture) {
	tb.Helper()
	corpusOnce.Do(func() {
		files, err := filepath.Glob("fixtures/*.yml")
		if err != nil {
			corpusErr = err
			return
		}
		sort.Strings(files)
		seen := make(map[string]bool)
		for _, file := range files {
			var list []corpusFixture
			if err := parser.ReadYamlFile(file, &list); err != nil {
				corpusErr = fmt.Errorf("%s: %w", file, err)
				return
			}
			for _, f := range list {
				f.File = filepath.Base(file)
				corpusFixtures = append(corpusFixtures, f)
				if !seen[f.UserAgent] {
					seen[f.UserAgent] = true
					corpusUAs = append(corpusUAs, f.UserAgent)
				}
			}
		}
		extra, err := os.Open("testdata/extra_user_agents.txt")
		if err != nil {
			corpusErr = err
			return
		}
		defer extra.Close()
		sc := bufio.NewScanner(extra)
		for sc.Scan() {
			line := strings.TrimSpace(sc.Text())
			if line == "" || strings.HasPrefix(line, "#") || seen[line] {
				continue
			}
			seen[line] = true
			corpusUAs = append(corpusUAs, line)
		}
		corpusErr = sc.Err()
	})
	if corpusErr != nil {
		tb.Fatal(corpusErr)
	}
	return corpusUAs, corpusFixtures
}

// benchCorpus returns a deterministic sample of the corpus: one fixture user
// agent every 10 plus all the extra user agents.
func benchCorpus(tb testing.TB) []string {
	uas, fixtures := loadCorpus(tb)
	nFixtureUAs := 0
	seen := make(map[string]bool)
	for _, f := range fixtures {
		if !seen[f.UserAgent] {
			seen[f.UserAgent] = true
			nFixtureUAs++
		}
	}
	sample := make([]string, 0, nFixtureUAs/10+len(uas)-nFixtureUAs)
	for i := 0; i < nFixtureUAs; i += 10 {
		sample = append(sample, uas[i])
	}
	return append(sample, uas[nFixtureUAs:]...)
}

// newMiddlewareDetector returns a detector configured like intelligence-middleware.
func newMiddlewareDetector(tb testing.TB) *DeviceDetector {
	tb.Helper()
	d, err := NewDeviceDetector("regexes", &CacheParams{IsEnabled: false})
	if err != nil {
		tb.Fatal(err)
	}
	d.DiscardBotInformation = true
	d.SkipBotDetection = true
	return d
}

func BenchmarkNewDeviceDetector(b *testing.B) {
	for b.Loop() {
		if _, err := NewDeviceDetector("regexes", &CacheParams{IsEnabled: false}); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkParse measures a single uncached parse (one user agent per op,
// cycling over the sample corpus) with the middleware configuration.
func BenchmarkParse(b *testing.B) {
	parser.ResetParserAbstract()
	uas := benchCorpus(b)
	d := newMiddlewareDetector(b)
	b.ReportAllocs()
	i := 0
	for b.Loop() {
		d.Parse(uas[i%len(uas)])
		i++
	}
}

// BenchmarkParseWithBots is BenchmarkParse with bot detection enabled.
func BenchmarkParseWithBots(b *testing.B) {
	parser.ResetParserAbstract()
	uas := benchCorpus(b)
	d, err := NewDeviceDetector("regexes", &CacheParams{IsEnabled: false})
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	i := 0
	for b.Loop() {
		d.Parse(uas[i%len(uas)])
		i++
	}
}

// BenchmarkParseParallel measures uncached parses from GOMAXPROCS goroutines
// sharing one detector. ns/op is the throughput cost, ns/parse the average
// latency of a single Parse call under load.
func BenchmarkParseParallel(b *testing.B) {
	parser.ResetParserAbstract()
	uas := benchCorpus(b)
	d := newMiddlewareDetector(b)
	// Warm up single-threaded: v1.1.1 compiles some regexes lazily without
	// synchronization, so the first parses must not run concurrently there.
	d.Parse(uas[0])
	var start, busy atomic.Int64
	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := int(start.Add(7919))
		for pb.Next() {
			t0 := time.Now()
			d.Parse(uas[i%len(uas)])
			busy.Add(int64(time.Since(t0)))
			i++
		}
	})
	b.ReportMetric(float64(busy.Load())/float64(b.N), "ns/parse")
}

// snapshotRecord is the complete output of the detector for one user agent.
type snapshotRecord struct {
	UA         string                    `json:"ua"`
	Nil        bool                      `json:"nil,omitempty"`
	Os         *parser.OsMatchResult     `json:"os"`
	Client     *client.ClientMatchResult `json:"client"`
	Device     device.DeviceMatchResult  `json:"device"`
	DeviceType int                       `json:"device_type"`
	IsMobile   bool                      `json:"is_mobile"`
	IsDesktop  bool                      `json:"is_desktop"`
	// Bot detection enabled, with details (default configuration).
	Bot *parser.BotMatchResult `json:"bot"`
	// Bot detection enabled, DiscardBotInformation = true.
	IsBotDiscarded bool `json:"is_bot_discarded"`
}

func snapshotOne(d *DeviceDetector, ua string) snapshotRecord {
	rec := snapshotRecord{UA: ua}

	// Middleware configuration.
	d.SkipBotDetection = true
	d.DiscardBotInformation = true
	info := d.Parse(ua)
	if info == nil {
		rec.Nil = true
	} else {
		rec.Os = info.os
		rec.Client = info.client
		rec.Device = info.DeviceMatchResult
		rec.DeviceType = info.GetDeviceType()
		rec.IsMobile = info.IsMobile()
		rec.IsDesktop = info.IsDesktop()
	}

	// Bot detection.
	d.SkipBotDetection = false
	d.DiscardBotInformation = false
	if bot := d.ParseBot(ua); bot != nil {
		cp := *bot
		rec.Bot = &cp
	}
	d.DiscardBotInformation = true
	rec.IsBotDiscarded = d.ParseBot(ua) != nil
	return rec
}

// computeSnapshot parses the whole corpus with one detector per worker, so it
// also works with code that is not safe for concurrent use.
func computeSnapshot(tb testing.TB, uas []string) []snapshotRecord {
	parser.ResetParserAbstract()
	out := make([]snapshotRecord, len(uas))
	workers := runtime.GOMAXPROCS(0)
	var next atomic.Int64
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		d, err := NewDeviceDetector("regexes", &CacheParams{IsEnabled: false})
		if err != nil {
			tb.Fatal(err)
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				i := int(next.Add(1)) - 1
				if i >= len(uas) {
					return
				}
				out[i] = snapshotOne(d, uas[i])
			}
		}()
	}
	wg.Wait()
	return out
}

func writeSnapshot(path string, recs []snapshotRecord) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	zw := gzip.NewWriter(f)
	enc := json.NewEncoder(zw)
	enc.SetEscapeHTML(false)
	for _, r := range recs {
		if err := enc.Encode(r); err != nil {
			return err
		}
	}
	return zw.Close()
}

func readSnapshot(path string) ([]snapshotRecord, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	zr, err := gzip.NewReader(f)
	if err != nil {
		return nil, err
	}
	var recs []snapshotRecord
	dec := json.NewDecoder(zr)
	for dec.More() {
		var r snapshotRecord
		if err := dec.Decode(&r); err != nil {
			return nil, err
		}
		recs = append(recs, r)
	}
	return recs, nil
}

func TestSnapshotWrite(t *testing.T) {
	path := os.Getenv("DD_SNAPSHOT_WRITE")
	if path == "" {
		t.Skip("set DD_SNAPSHOT_WRITE=<path> to write a snapshot")
	}
	uas, _ := loadCorpus(t)
	start := time.Now()
	recs := computeSnapshot(t, uas)
	t.Logf("parsed %d user agents in %v", len(uas), time.Since(start))
	if err := writeSnapshot(path, recs); err != nil {
		t.Fatal(err)
	}
}

func snapshotDiff(a, b snapshotRecord) []string {
	ja, _ := json.Marshal(a)
	jb, _ := json.Marshal(b)
	if string(ja) == string(jb) {
		return nil
	}
	var ma, mb map[string]any
	_ = json.Unmarshal(ja, &ma)
	_ = json.Unmarshal(jb, &mb)
	var diffs []string
	for k := range ma {
		va, _ := json.Marshal(ma[k])
		vb, _ := json.Marshal(mb[k])
		if string(va) != string(vb) {
			diffs = append(diffs, fmt.Sprintf("%s: %s -> %s", k, va, vb))
		}
	}
	sort.Strings(diffs)
	return diffs
}

// TestSnapshotEquivalence checks that the output for every user agent of the
// corpus is identical to the reference snapshot.
func TestSnapshotEquivalence(t *testing.T) {
	if testing.Short() {
		t.Skip("skipped in short mode")
	}
	ref, err := readSnapshot(snapshotReferenceFile)
	if os.IsNotExist(err) {
		t.Skip("no reference snapshot")
	}
	if err != nil {
		t.Fatal(err)
	}
	uas, _ := loadCorpus(t)
	if len(ref) != len(uas) {
		t.Fatalf("reference snapshot has %d records, corpus has %d user agents", len(ref), len(uas))
	}
	got := computeSnapshot(t, uas)
	n := 0
	for i := range ref {
		if ref[i].UA != uas[i] {
			t.Fatalf("record %d: reference UA %q, corpus UA %q", i, ref[i].UA, uas[i])
		}
		if d := snapshotDiff(ref[i], got[i]); d != nil {
			n++
			if n <= 30 {
				t.Errorf("%s\n\t%s", uas[i], strings.Join(d, "\n\t"))
			}
		}
	}
	if n > 0 {
		t.Errorf("%d/%d user agents differ from %s", n, len(uas), snapshotReferenceFile)
	}
}
