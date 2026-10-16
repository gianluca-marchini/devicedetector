package parser

import (
	"bufio"
	"os"
	"path/filepath"
	"reflect"
	stdregexp "regexp"
	"runtime"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"unicode"

	regexp "github.com/dlclark/regexp2"
	"gopkg.in/yaml.v2"
)

func TestTranslatePattern(t *testing.T) {
	cases := []struct {
		in, out     string
		exact, ok   bool
		description string
	}{
		{`Chrome/(\d+)`, `Chrome/(\d+)`, true, true, "plain"},
		{`(?:a|b)c`, `(?:a|b)c`, true, true, "non capturing group"},
		{`Chrome/[\.0-9]* (?!Mobile)`, `Chrome/[\.0-9]* (?:)`, false, true, "negative lookahead"},
		{`(?<!HTC)[ _]Butterfly/`, `(?:)[ _]Butterfly/`, false, true, "negative lookbehind"},
		{`a(?=(b|c))d`, `a(?:)d`, false, true, "nested lookahead"},
		{`(?>ab)c`, `(?:ab)c`, false, true, "atomic group"},
		{`[[:alpha:]]`, `[\[:alpha:]]`, true, true, "no POSIX class"},
		{`[]a]`, `[]a]`, true, true, "] first in class"},
		{`[^]a]`, `[^]a]`, true, true, "] first in negated class"},
		{`[(]x`, `[(]x`, true, true, "( in class"},
		{`\x41`, `\x41`, true, true, "hex"},
		{`é`, `é`, false, true, "non-ASCII literal"},
		{`(a)\1`, ``, false, false, "backreference"},
		{`[a-z-[aeiou]]`, ``, false, false, "class subtraction"},
		{`(?<name>a)`, ``, false, false, "named group"},
		{`(?i)a`, ``, false, false, "inline options"},
		{`(?(?=a)a|b)`, ``, false, false, "conditional"},
		{`\p{L}`, ``, false, false, "unicode class"},
		{`\Qa\E`, ``, false, false, "quote"},
		{`\Z`, ``, false, false, "\\Z"},
		{`[\b]`, ``, false, false, "backspace in class"},
		{`[abc`, ``, false, false, "unterminated class"},
	}
	for _, c := range cases {
		out, exact, ok := translatePattern(c.in, false)
		if ok != c.ok || (ok && (out != c.out || exact != c.exact)) {
			t.Errorf("%s: translatePattern(%q) = %q, %v, %v; want %q, %v, %v",
				c.description, c.in, out, exact, ok, c.out, c.exact, c.ok)
		}
	}
}

func TestTranslatePatternUnicodeClasses(t *testing.T) {
	cases := map[string]string{
		`a\sb`:   `a[` + unicodeSpace + `]b`,
		`a\Sb`:   `a[^` + unicodeSpace + `]b`,
		`[\s_-]`: `[` + unicodeSpace + `_-]`,
		`(\d+)`:  `(\p{Nd}+)`,
		`[^\d.]`: `[^\p{Nd}.]`,
		`\w\W`:   `[` + unicodeWord + `][^` + unicodeWord + `]`,
		`[\W]`:   ``,
		`[a\S]`:  ``,
	}
	for in, want := range cases {
		out, _, ok := translatePattern(in, true)
		if want == "" {
			if ok {
				t.Errorf("translatePattern(%q) = %q, want not ok", in, out)
			}
			continue
		}
		if !ok || out != want {
			t.Errorf("translatePattern(%q) = %q, %v; want %q", in, out, ok, want)
		}
	}
	// \s of regexp2 is unicode.IsSpace: the converted class must not miss
	// any of its characters.
	re := stdregexp.MustCompile(`^[` + unicodeSpace + `]$`)
	for r := rune(0); r <= unicode.MaxRune; r++ {
		if unicode.IsSpace(r) && !re.MatchString(string(r)) {
			t.Errorf("%U is a space for regexp2", r)
		}
	}
}

func TestSplitAlternatives(t *testing.T) {
	cases := map[string][]string{
		`a|b`:            {"a", "b"},
		`(a|b)|c`:        {"(a|b)", "c"},
		`[|]|\||x`:       {"[|]", `\|`, "x"},
		`[]|]|y`:         {"[]|]", "y"},
		`(?:x(?!a|b))|z`: {"(?:x(?!a|b))", "z"},
		`abc`:            {"abc"},
	}
	for in, want := range cases {
		if got := splitAlternatives(in); !reflect.DeepEqual(got, want) {
			t.Errorf("splitAlternatives(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestRequiredClauses(t *testing.T) {
	cases := map[string][][]string{
		`Chrome/(\d+)`:                {{"chrome/"}},
		`(?:SAMSUNG|Galaxy)[ _-]?`:    {{"galaxy", "samsung"}},
		`Opera Mini|OPiOS/`:           {{"opera mini", "opios/"}},
		`x(?:abc|abd)y`:               {{"xabcy", "xabdy"}},
		`(?:ab|a)c`:                   {{"abc", "ac"}},
		`SM[- ]?G9`:                   {{"sm g9", "sm-g9", "smg9"}},
		`Chrome/[\.0-9]* (?:)`:        {{"chrome/"}},
		`(?:Nokia|Lumia)[ _]?\d+`:     {{"lumia", "nokia"}},
		`a(?:bc)+d`:                   {{"abc"}, {"bcd"}},
		`foo\bbar`:                    {{"foobar"}},
		`Build\s`:                     {{"build"}}, // \s is not a small class in regexp2
		`[Ss]ony`:                     {{"sony"}},
		`Android.*Z1([234])0[);\/ ]`:  {{"z120 ", "z120)", "z120/", "z120;", "z130 ", "z130)", "z130/", "z130;", "z140 ", "z140)", "z140/", "z140;"}, {"android"}},
		`R([0-9]{3,4})[KSTW]?[);\/ ]`: {{"r0", "r1", "r2", "r3", "r4", "r5", "r6", "r7", "r8", "r9"}},
		`(?:Phone){2}`:                {{"phonephone"}},
		`[a-z]+`:                      nil,
		`a?b*`:                        nil,
		`(?:abc|[0-9]+)`:              {{"0", "1", "2", "3", "4", "5", "6", "7", "8", "9", "abc"}},
		`(?:abc|[a-z]+)`:              nil, // an alternative without literals
		UserAgentPattern(`Nexus`):     {{"nexus"}},
		`K`:                           {{"k"}},
	}
	for in, want := range cases {
		goPattern, _, ok := translatePattern(in, true)
		if !ok {
			t.Fatalf("%q not translated", in)
		}
		if got := requiredClauses(goPattern); !reflect.DeepEqual(got, want) {
			t.Errorf("requiredClauses(%q) = %q, want %q", in, got, want)
		}
	}
}

// classLiterals relies on it: the characters of a lower-cased user agent are
// exactly the fixed points of unicode.ToLower.
func TestToLowerIdempotent(t *testing.T) {
	for r := rune(0); r <= unicode.MaxRune; r++ {
		if l := unicode.ToLower(r); unicode.ToLower(l) != l {
			t.Errorf("ToLower(%U) = %U, ToLower(%U) = %U", r, l, l, unicode.ToLower(l))
		}
	}
}

func TestNewUserAgent(t *testing.T) {
	u := NewUserAgent("Mozilla/5.0 (Linux; Android 10)")
	if !u.simple || u.lower != "mozilla/5.0 (linux; android 10)" {
		t.Errorf("got %+v", u)
	}
	// regexp2 lower-cases rune by rune: İ (U+0130) is i, K (U+212A) is k.
	u = NewUserAgent("ANDROİD KİT \u212A")
	if u.simple || u.lower != "androİd kİt k" && u.lower != "android kit k" {
		t.Errorf("got %+v", u)
	}
	if !strings.Contains(u.lower, "kit") || !strings.HasSuffix(u.lower, " k") {
		t.Errorf("lower %q", u.lower)
	}
	for _, s := range []string{"a\tb", "a\nb", "\x7f", "ü"} {
		if NewUserAgent(s).simple {
			t.Errorf("%q is not simple", s)
		}
	}
}

// corpusPatterns returns every pattern compiled for the YAML files: the
// regexes of all the entries, the vendor fragments, the aggregated regexes
// of PreMatch and the regexes of DeviceDetector.
// testPattern is a pattern to compile; raw, when set, is the regex of a YAML
// file it comes from (the pattern is then UserAgentPattern(raw)).
type testPattern struct {
	pattern, raw string
}

func (p testPattern) compile() *Regex {
	if p.raw != "" {
		return newUserAgentRegex(regexp.MustCompile(p.pattern, regexp.IgnoreCase), p.raw)
	}
	return NewRegex(p.pattern)
}

func corpusPatterns(t testing.TB) (patterns []testPattern, aggregates map[string][]string) {
	raw := func(r string) testPattern { return testPattern{UserAgentPattern(r), r} }
	aggregates = make(map[string][]string)
	files, _ := filepath.Glob(filepath.Join(dir, "*.yml"))
	more, _ := filepath.Glob(filepath.Join(dir, "*", "*.yml"))
	files = append(files, more...)
	sort.Strings(files)
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		var doc interface{}
		if err := yaml.Unmarshal(data, &doc); err != nil {
			t.Fatal(err)
		}
		var top []string // regexes of the entries of the file, for the aggregates
		if strings.HasSuffix(file, FixtureFileVendor) {
			for _, brands := range doc.(map[interface{}]interface{}) {
				for _, b := range brands.([]interface{}) {
					patterns = append(patterns, raw(b.(string)+"[^a-z0-9]+"))
				}
			}
			continue
		}
		switch d := doc.(type) {
		case []interface{}:
			for _, e := range d {
				if r, ok := e.(map[interface{}]interface{})["regex"].(string); ok {
					top = append(top, r)
				}
			}
		case map[interface{}]interface{}:
			for _, e := range d {
				if r, ok := e.(map[interface{}]interface{})["regex"].(string); ok {
					top = append(top, r)
				}
			}
		}
		aggregates[file] = top
		walkRegexes(doc, func(r string) { patterns = append(patterns, raw(r)) })
		patterns = append(patterns, raw(strings.Join(top, "|")))
	}
	for _, r := range []string{"arm", "WOW64|x64|win64|amd64|x86_64", "i[0-9]86|i86pc", `HbbTV/([1-9]{1}(?:\.[0-9]{1}){1,2})`, ``} {
		patterns = append(patterns, raw(r))
	}
	// DeviceDetector (fixUserAgentRegEx).
	for _, r := range []string{`Chrome/[\.0-9]* Mobile`, `Chrome/[\.0-9]* (?!Mobile)`, `Opera Tablet`, `Opera TV Store`, `Touch`, `Android( [\.0-9]+)?; Tablet;`, `Android( [\.0-9]+)?; Mobile;`} {
		patterns = append(patterns, testPattern{pattern: `(?:^|[^A-Z_-])(?:` + strings.ReplaceAll(r, `/`, `\/`) + `)`})
	}
	return patterns, aggregates
}

func walkRegexes(v interface{}, f func(string)) {
	switch v := v.(type) {
	case map[interface{}]interface{}:
		for k, e := range v {
			if r, ok := e.(string); ok && k == "regex" {
				f(r)
			} else {
				walkRegexes(e, f)
			}
		}
	case []interface{}:
		for _, e := range v {
			walkRegexes(e, f)
		}
	}
}

// corpusInputs returns the user agents of the fixtures and of the extra
// corpus, plus variants: upper case, lower case and with non-ASCII
// characters that regexp2 lower-cases to ASCII letters.
func corpusInputs(t testing.TB, every int) []*UserAgent {
	var uas []string
	files, _ := filepath.Glob("../fixtures/*.yml")
	sort.Strings(files)
	seen := map[string]bool{}
	n := 0
	for _, file := range files {
		var list []struct {
			UA string `yaml:"user_agent"`
		}
		if err := ReadYamlFile(file, &list); err != nil {
			t.Fatal(err)
		}
		for _, e := range list {
			if !seen[e.UA] {
				seen[e.UA] = true
				if n%every == 0 {
					uas = append(uas, e.UA)
				}
				n++
			}
		}
	}
	f, err := os.Open("../testdata/extra_user_agents.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		if l := strings.TrimSpace(sc.Text()); l != "" && !strings.HasPrefix(l, "#") {
			uas = append(uas, l)
		}
	}
	base := len(uas)
	for i := 0; i < base; i++ {
		ua := uas[i]
		if i%3 == 0 {
			uas = append(uas, strings.ToUpper(ua))
		}
		if i%3 == 1 {
			uas = append(uas, strings.ToLower(ua))
		}
		if i%5 == 0 {
			uas = append(uas, strings.NewReplacer("i", "İ", "k", "\u212A", "K", "\u212A").Replace(ua))
		}
	}
	uas = append(uas, "", "-", "a\tb Chrome/1.0", "Android 4.4; Mobile;\n", "ü")
	inputs := make([]*UserAgent, len(uas))
	for i, ua := range uas {
		inputs[i] = NewUserAgent(ua)
		// Read-only from now on: the inputs are shared by the goroutines.
		inputs[i].getRunes()
		inputs[i].getGrams()
	}
	return inputs
}

func withFastPaths(t testing.TB) {
	prefilter, std := usePrefilter, useStdEngine
	usePrefilter, useStdEngine = true, true
	t.Cleanup(func() { usePrefilter, useStdEngine = prefilter, std })
}

// TestRegexFastPathsEquivalence checks that, for every pattern of the YAML
// files and every user agent of a sample of the corpus, Regex gives exactly
// the same result as regexp2 (match and all the groups). Set DD_EXHAUSTIVE=1
// to use the whole corpus.
func TestRegexFastPathsEquivalence(t *testing.T) {
	withFastPaths(t)
	every := 25
	if os.Getenv("DD_EXHAUSTIVE") != "" {
		every = 1
	} else if testing.Short() || raceEnabled {
		every = 400
	}
	patterns, _ := corpusPatterns(t)
	inputs := corpusInputs(t, every)

	var withLiterals, withStd, withAlts, checks, prefiltered atomic.Int64
	var next atomic.Int64
	var wg sync.WaitGroup
	var mu sync.Mutex
	failures := 0
	for w := 0; w < runtime.GOMAXPROCS(0); w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				i := int(next.Add(1)) - 1
				if i >= len(patterns) {
					return
				}
				p := patterns[i].pattern
				ref := regexp.MustCompile(p, regexp.IgnoreCase)
				x := patterns[i].compile()
				if x.pre != nil {
					withLiterals.Add(1)
				}
				if x.std != nil {
					withStd.Add(1)
				}
				if x.alts != nil {
					withAlts.Add(1)
				}
				for _, u := range inputs {
					checks.Add(1)
					if !x.mayMatch(u) {
						prefiltered.Add(1)
					}
					want := matchRegexp2(ref, u.getRunes())
					got := x.Match(u)
					isMatch := x.IsMatch(u)
					if !reflect.DeepEqual(got, want) || isMatch != (want != nil) {
						mu.Lock()
						failures++
						if failures <= 20 {
							t.Errorf("pattern %q, user agent %q:\n\tregexp2 %q\n\tRegex   %q (IsMatch %v, prefilter %v, std %v)",
								p, u.ua, want, got, isMatch, x.pre, x.std != nil)
						}
						mu.Unlock()
					}
				}
			}
		}()
	}
	wg.Wait()
	t.Logf("%d patterns (%d with literals, %d with the standard engine, %d split), %d user agents, %d checks, %.1f%% prefiltered",
		len(patterns), withLiterals.Load(), withStd.Load(), withAlts.Load(), len(inputs), checks.Load(),
		100*float64(prefiltered.Load())/float64(checks.Load()))
}

// The aggregated regexes of PreMatch match exactly when one of the regexes
// of the file matches.
func TestAggregateEquivalence(t *testing.T) {
	withFastPaths(t)
	every := 25
	if testing.Short() || raceEnabled {
		every = 400
	}
	_, aggregates := corpusPatterns(t)
	inputs := corpusInputs(t, every)
	for file, regexes := range aggregates {
		if len(regexes) == 0 {
			continue
		}
		all := NewRegex(UserAgentPattern(strings.Join(regexes, "|")))
		var single []*Regex
		for _, r := range regexes {
			single = append(single, testPattern{UserAgentPattern(r), r}.compile())
		}
		for _, u := range inputs {
			any := false
			for _, s := range single {
				if s.IsMatch(u) {
					any = true
					break
				}
			}
			if got := all.IsMatch(u); got != any {
				t.Errorf("%s, %q: aggregate %v, any regex %v", file, u.ua, got, any)
			}
		}
	}
}

// Every regex of a list matching a user agent is a candidate of the index.
func TestIndexCandidates(t *testing.T) {
	withFastPaths(t)
	every := 25
	if testing.Short() || raceEnabled {
		every = 400
	}
	inputs := corpusInputs(t, every)
	files, _ := filepath.Glob(filepath.Join(dir, "*.yml"))
	more, _ := filepath.Glob(filepath.Join(dir, "*", "*.yml"))
	var lists [][]*Regular
	for _, file := range append(files, more...) {
		data, _ := os.ReadFile(file)
		var doc interface{}
		if err := yaml.Unmarshal(data, &doc); err != nil {
			t.Fatal(err)
		}
		var top []*Regular
		add := func(e interface{}) {
			m, ok := e.(map[interface{}]interface{})
			if !ok {
				return
			}
			if r, ok := m["regex"].(string); ok {
				top = append(top, &Regular{Regex: r})
			}
			if models, ok := m["models"].([]interface{}); ok {
				var list []*Regular
				for _, mm := range models {
					list = append(list, &Regular{Regex: mm.(map[interface{}]interface{})["regex"].(string)})
				}
				lists = append(lists, list)
			}
		}
		switch d := doc.(type) {
		case []interface{}:
			for _, e := range d {
				add(e)
			}
		case map[interface{}]interface{}:
			for _, e := range d {
				add(e)
			}
		}
		lists = append(lists, top)
	}
	checks := 0
	for _, list := range lists {
		for _, r := range list {
			r.Compile()
		}
		ix := NewIndex(list)
		for _, u := range inputs {
			c := ix.Candidates(u)
			for i, r := range list {
				checks++
				if !c.Has(i) && r.IsMatchUA(u) {
					t.Errorf("%q matches %q but is not a candidate", r.Regex, u.ua)
				}
			}
		}
	}
	t.Logf("%d lists, %d checks", len(lists), checks)
}
