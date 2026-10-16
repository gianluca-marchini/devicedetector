package parser

import (
	stdregexp "regexp"
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"
	"unsafe"

	regexp "github.com/dlclark/regexp2"

	"github.com/gianluca-marchini/devicedetector/internal/alternation"
)

// UserAgent is a user agent prepared for matching against many regexes:
// the lower-cased copy used by the prefilters is computed once.
//
// A UserAgent is not safe for concurrent use; create one per parse.
type UserAgent struct {
	ua    string
	lower string
	// simple is true when the user agent only contains printable ASCII
	// characters: on such inputs the standard library engine gives the same
	// results as regexp2 (see Regex).
	simple bool
	runes  []rune
	grams  *gramBitmap
}

// gramBits is the size of the bigram bitmap of a user agent.
const gramBits = 4096

// gramBitmap has a bit set for the hash of every bigram of a lower-cased
// user agent (collisions only make the prefilter less selective).
type gramBitmap [gramBits / 64]uint64

// NewUserAgent prepares ua for matching.
func NewUserAgent(ua string) *UserAgent {
	u := &UserAgent{ua: ua, simple: true}
	upper := false
	for i := 0; i < len(ua); i++ {
		c := ua[i]
		if c < 0x20 || c > 0x7e {
			u.simple = false
			break
		}
		if 'A' <= c && c <= 'Z' {
			upper = true
		}
	}
	switch {
	case !u.simple:
		// regexp2 lower-cases rune by rune with unicode.ToLower when
		// matching with IgnoreCase: do exactly the same.
		u.lower = strings.Map(unicode.ToLower, ua)
	case upper:
		b := make([]byte, len(ua))
		for i := 0; i < len(ua); i++ {
			c := ua[i]
			if 'A' <= c && c <= 'Z' {
				c += 'a' - 'A'
			}
			b[i] = c
		}
		// b is never modified again.
		u.lower = unsafe.String(&b[0], len(b))
	default:
		u.lower = ua
	}
	return u
}

// String returns the original user agent.
func (u *UserAgent) String() string {
	return u.ua
}

func (u *UserAgent) getGrams() *gramBitmap {
	if u.grams == nil {
		g := new(gramBitmap)
		s := u.lower
		for i := 0; i+1 < len(s); i++ {
			h := bigram(s[i], s[i+1])
			g[h>>6] |= 1 << (h & 63)
		}
		u.grams = g
	}
	return u.grams
}

func (u *UserAgent) getRunes() []rune {
	if u.runes == nil {
		u.runes = []rune(u.ua)
	}
	return u.runes
}

// Regex is a case-insensitive regex in the .NET syntax of regexp2 with two
// fast paths that never change the result:
//
//   - a prefilter: groups of lower-cased literals of which at least one per
//     group must appear in the user agent for the regex to match (see
//     literals.go). Otherwise the regex is not executed at all;
//   - an equivalent regex for the standard library engine (RE2), used when
//     the pattern has no construct with a different meaning in the two
//     engines and the user agent only contains printable ASCII characters.
//
// In all other cases the regexp2 regex is executed. A Regex is safe for
// concurrent use.
type Regex struct {
	re2x *regexp.Regexp
	pre  *prefilter
	// std is equivalent to re2x on printable ASCII user agents.
	std *lazyRegexp
	// superset matches a superset of the printable ASCII user agents matched
	// by re2x (its lookarounds are removed): when it does not match, re2x
	// does not match either.
	superset *lazyRegexp
	// alts are the top-level alternatives of a long regex: on printable
	// ASCII user agents the regex matches exactly when one of them matches.
	// Each one has its own prefilter and engine, much more selective than
	// the ones of the whole regex. The regexp2 regex of an alternative is
	// only compiled if needed.
	alts       []*Regex
	re2Pattern string
	re2Once    sync.Once
}

// lazyRegexp is a standard library regex compiled on first use: thanks to
// the prefilters, most regexes are never executed.
type lazyRegexp struct {
	pattern string
	once    sync.Once
	re      *stdregexp.Regexp
}

func newLazyRegexp(pattern string) *lazyRegexp {
	return &lazyRegexp{pattern: pattern}
}

// get returns the regex, nil if it cannot be compiled.
func (l *lazyRegexp) get() *stdregexp.Regexp {
	if l == nil {
		return nil
	}
	l.once.Do(func() {
		l.re, _ = stdregexp.Compile(l.pattern)
	})
	return l.re
}

// Fast paths of Regex. They are read when a Regex is built and are disabled
// only by tests, to compare the results with plain regexp2.
var (
	usePrefilter = true
	useStdEngine = true
)

// NewRegex compiles pattern with regexp2 (IgnoreCase) and builds its fast
// paths. It panics if pattern cannot be compiled by regexp2.
func NewRegex(pattern string) *Regex {
	return newRegex(regexp.MustCompile(pattern, regexp.IgnoreCase), pattern)
}

func newRegex(re2x *regexp.Regexp, pattern string) *Regex {
	x := &Regex{re2x: re2x}
	x.setFastPaths(pattern)
	return x
}

func (x *Regex) setFastPaths(pattern string) {
	if usePrefilter {
		if goPattern, _, ok := translatePattern(pattern, true); ok {
			x.pre = newPrefilter(goPattern)
		}
	}
	if useStdEngine && isASCII(pattern) {
		if goPattern, exact, ok := translatePattern(pattern, false); ok {
			if exact {
				x.std = newLazyRegexp(`(?i)` + goPattern)
			} else {
				x.superset = newLazyRegexp(`(?i)` + goPattern)
			}
		}
	}
}

func isASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] >= utf8.RuneSelf {
			return false
		}
	}
	return true
}

// Regexp returns the regexp2 regex.
func (x *Regex) Regexp() *regexp.Regexp {
	return x.getRe2()
}

func (x *Regex) getRe2() *regexp.Regexp {
	if x.re2Pattern != "" {
		x.re2Once.Do(func() {
			x.re2x = regexp.MustCompile(x.re2Pattern, regexp.IgnoreCase)
		})
	}
	return x.re2x
}

// minSplitLen is the length from which the alternatives of a regex are
// evaluated one by one.
const minSplitLen = 100

// newUserAgentRegex compiles the pattern of a user agent regex of the YAML
// files (see UserAgentPattern).
func newUserAgentRegex(re2x *regexp.Regexp, regex string) *Regex {
	x := newRegex(re2x, UserAgentPattern(regex))
	if !useStdEngine || len(regex) < minSplitLen {
		return x
	}
	alts := splitAlternatives(regex)
	if len(alts) < 2 || !alternation.IsUnion(alts) {
		return x
	}
	for _, alt := range alts {
		a := &Regex{re2Pattern: UserAgentPattern(alt)}
		a.setFastPaths(a.re2Pattern)
		x.alts = append(x.alts, a)
	}
	return x
}

// splitAlternatives returns the top-level alternatives of regex.
func splitAlternatives(regex string) []string {
	var alts []string
	depth, start := 0, 0
	inClass, classStart := false, false
	for i := 0; i < len(regex); i++ {
		c := regex[i]
		if c == '\\' {
			i++
			classStart = false
			continue
		}
		if inClass {
			if c == ']' && !classStart {
				inClass = false
			}
			classStart = false
			continue
		}
		switch c {
		case '[':
			inClass, classStart = true, true
			if i+1 < len(regex) && regex[i+1] == '^' {
				i++
			}
		case '(':
			depth++
		case ')':
			depth--
		case '|':
			if depth == 0 {
				alts = append(alts, regex[start:i])
				start = i + 1
			}
		}
	}
	return append(alts, regex[start:])
}

// anyAlternative reports whether one of the alternatives matches the
// printable ASCII user agent u.
func (x *Regex) anyAlternative(u *UserAgent) bool {
	for _, a := range x.alts {
		if a.pre != nil && !a.pre.mayMatch(u) {
			continue
		}
		if std := a.std.get(); std != nil {
			if std.MatchString(u.ua) {
				return true
			}
			continue
		}
		if superset := a.superset.get(); superset != nil && !superset.MatchString(u.ua) {
			continue
		}
		if m, _ := a.getRe2().MatchRunes(u.getRunes()); m {
			return true
		}
	}
	return false
}

// mayMatch reports whether the regex can match u, according to the prefilter.
func (x *Regex) mayMatch(u *UserAgent) bool {
	if x.pre != nil && !x.pre.mayMatch(u) {
		return false
	}
	if !u.simple {
		return true
	}
	superset := x.superset.get()
	return superset == nil || superset.MatchString(u.ua)
}

// IsMatch reports whether the regex matches u.
func (x *Regex) IsMatch(u *UserAgent) bool {
	if x.alts != nil && u.simple {
		return x.anyAlternative(u)
	}
	if !x.mayMatch(u) {
		return false
	}
	if u.simple {
		if std := x.std.get(); std != nil {
			return std.MatchString(u.ua)
		}
	}
	m, _ := x.re2x.MatchRunes(u.getRunes())
	return m
}

// Match returns the text of the match and of all the groups, nil when the
// regex does not match u.
func (x *Regex) Match(u *UserAgent) []string {
	if x.alts != nil && u.simple {
		if !x.anyAlternative(u) {
			return nil
		}
	} else if !x.mayMatch(u) {
		return nil
	}
	if u.simple {
		if std := x.std.get(); std != nil {
			return std.FindStringSubmatch(u.ua)
		}
	}
	return matchRegexp2(x.re2x, u.getRunes())
}

func matchRegexp2(re *regexp.Regexp, runes []rune) []string {
	match, err := re.FindRunesMatch(runes)
	if err != nil || match == nil {
		return nil
	}
	groups := match.Groups()
	matches := make([]string, len(groups))
	for i := range groups {
		matches[i] = groups[i].String()
	}
	return matches
}

// Unicode versions of \s, \w and \d of regexp2: \s is unicode.IsSpace, \w
// the categories L, Mn, Nd and Pc plus ZWNJ and ZWJ, \d the category Nd.
// The standard library uses ASCII classes for \s, \w and \d.
const (
	unicodeSpace = `\t\n\v\f\r\x{85}\p{Z}`
	unicodeWord  = `\p{L}\p{Mn}\p{Nd}\p{Pc}\x{200C}\x{200D}`
)

// translatePattern converts a regexp2 (.NET) pattern to the syntax of the
// standard library.
//
// When ok is false the pattern cannot be converted. Otherwise the returned
// pattern matches a superset of the inputs matched by the original one
// (lookarounds are removed, atomic groups become plain groups), so its
// required literals are required by the original pattern too.
//
// exact reports whether the two patterns are equivalent, captures included,
// on inputs made of printable ASCII characters only.
//
// With unicodeClasses, \s, \w and \d are converted to classes with the
// same characters as in regexp2, for any input: use it for the literals.
// Without, they are kept as they are: the ASCII classes of the standard
// library, faster and equivalent on printable ASCII inputs.
func translatePattern(p string, unicodeClasses bool) (out string, exact bool, ok bool) {
	var b strings.Builder
	b.Grow(len(p) + 8)
	exact = true
	inClass := false
	classStart := false // right after '[' or '[^': a ']' is a literal
	for i := 0; i < len(p); i++ {
		c := p[i]
		if c >= utf8.RuneSelf {
			// Non-ASCII literals: case folding differs between the engines.
			exact = false
		}
		if c == '\\' {
			if i+1 >= len(p) {
				return "", false, false
			}
			n := p[i+1]
			switch {
			case '0' <= n && n <= '9':
				// Backreference or octal escape.
				return "", false, false
			case unicodeClasses && strings.IndexByte("sSwWdD", n) >= 0:
				var class string
				switch n {
				case 's', 'S':
					class = unicodeSpace
				case 'w', 'W':
					class = unicodeWord
				case 'd':
					class = `\p{Nd}`
				case 'D':
					class = `\P{Nd}`
				}
				switch {
				case n == 'd' || n == 'D':
					b.WriteString(class)
				case n == 'S' || n == 'W':
					if inClass {
						// A negated class inside a class.
						return "", false, false
					}
					b.WriteString(`[^` + class + `]`)
				case inClass:
					b.WriteString(class)
				default:
					b.WriteString(`[` + class + `]`)
				}
				i++
				classStart = false
				continue
			case 'a' <= n && n <= 'z' || 'A' <= n && n <= 'Z':
				switch n {
				case 'd', 'D', 'w', 'W', 's', 'S', 't', 'n', 'r', 'f', 'v':
				case 'b', 'B', 'A', 'z':
					if inClass {
						return "", false, false
					}
				case 'x':
					if i+3 >= len(p) || !isHex(p[i+2]) || !isHex(p[i+3]) {
						return "", false, false
					}
				default:
					// \p, \k, \u, \e, \c, \G, \Z, \Q, ...: unsupported by
					// one of the engines or with a different meaning.
					return "", false, false
				}
			case n >= utf8.RuneSelf:
				return "", false, false
			}
			b.WriteByte(c)
			b.WriteByte(n)
			i++
			classStart = false
			continue
		}
		if inClass {
			switch {
			case c == '[':
				// A literal in .NET; escape it so that RE2 does not read a
				// POSIX class like [:alpha:].
				b.WriteString(`\[`)
			case c == '-' && i+1 < len(p) && p[i+1] == '[':
				// .NET character class subtraction.
				return "", false, false
			case c == ']' && !classStart:
				inClass = false
				b.WriteByte(c)
			default:
				b.WriteByte(c)
			}
			classStart = false
			continue
		}
		switch c {
		case '[':
			inClass = true
			classStart = true
			b.WriteByte(c)
			if i+1 < len(p) && p[i+1] == '^' {
				b.WriteByte('^')
				i++
			}
			continue
		case '(':
			if i+1 >= len(p) || p[i+1] != '?' {
				break
			}
			rest := p[i+2:]
			switch {
			case strings.HasPrefix(rest, ":"):
				b.WriteString(`(?:`)
				i += 2
			case strings.HasPrefix(rest, "="), strings.HasPrefix(rest, "!"),
				strings.HasPrefix(rest, "<="), strings.HasPrefix(rest, "<!"):
				end := closingParen(p, i)
				if end < 0 {
					return "", false, false
				}
				// Zero-width assertion: removing it accepts more inputs.
				b.WriteString(`(?:)`)
				exact = false
				i = end
			case strings.HasPrefix(rest, ">"):
				b.WriteString(`(?:`)
				exact = false
				i += 2
			default:
				// Named groups (numbered differently), inline options,
				// conditionals, comments.
				return "", false, false
			}
			continue
		}
		b.WriteByte(c)
	}
	if inClass {
		return "", false, false
	}
	return b.String(), exact, true
}

func isHex(c byte) bool {
	return '0' <= c && c <= '9' || 'a' <= c && c <= 'f' || 'A' <= c && c <= 'F'
}

// closingParen returns the index of the ')' closing the group opened at
// p[open], -1 if there is none.
func closingParen(p string, open int) int {
	depth := 0
	inClass := false
	classStart := false
	for i := open; i < len(p); i++ {
		c := p[i]
		if c == '\\' {
			i++
			classStart = false
			continue
		}
		if inClass {
			if c == ']' && !classStart {
				inClass = false
			}
			classStart = false
			continue
		}
		switch c {
		case '[':
			inClass = true
			classStart = true
			if i+1 < len(p) && p[i+1] == '^' {
				i++
			}
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}
