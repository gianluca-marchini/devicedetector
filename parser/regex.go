package parser

import (
	stdregexp "regexp"
	"regexp/syntax"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	regexp "github.com/dlclark/regexp2"
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
}

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
		u.lower = string(b)
	default:
		u.lower = ua
	}
	return u
}

// String returns the original user agent.
func (u *UserAgent) String() string {
	return u.ua
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
//   - a prefilter: the lower-cased literals of which at least one must appear
//     in the user agent for the regex to match. When none appears, the regex
//     is not executed at all;
//   - an equivalent regex for the standard library engine (RE2), used when
//     the pattern has no construct with a different meaning in the two
//     engines and the user agent only contains printable ASCII characters.
//
// In all other cases the regexp2 regex is executed. A Regex is safe for
// concurrent use.
type Regex struct {
	re2x     *regexp.Regexp
	literals []string
	std      *stdregexp.Regexp
}

// Fast paths of Regex. They are read when a Regex is built and are disabled
// only by tests, to compare the results with plain regexp2.
var (
	usePrefilter = false
	useStdEngine = false
)

// NewRegex compiles pattern with regexp2 (IgnoreCase) and builds its fast
// paths. It panics if pattern cannot be compiled by regexp2.
func NewRegex(pattern string) *Regex {
	return newRegex(regexp.MustCompile(pattern, regexp.IgnoreCase), pattern)
}

func newRegex(re2x *regexp.Regexp, pattern string) *Regex {
	x := &Regex{re2x: re2x}
	goPattern, exact, ok := translatePattern(pattern)
	if !ok {
		return x
	}
	if usePrefilter {
		x.literals = requiredLiterals(goPattern)
	}
	if exact && useStdEngine {
		if std, err := stdregexp.Compile(`(?i)` + goPattern); err == nil {
			x.std = std
		}
	}
	return x
}

// Regexp returns the regexp2 regex.
func (x *Regex) Regexp() *regexp.Regexp {
	return x.re2x
}

// mayMatch reports whether the regex can match u, according to the prefilter.
func (x *Regex) mayMatch(u *UserAgent) bool {
	if x.literals == nil {
		return true
	}
	for _, l := range x.literals {
		if strings.Contains(u.lower, l) {
			return true
		}
	}
	return false
}

// IsMatch reports whether the regex matches u.
func (x *Regex) IsMatch(u *UserAgent) bool {
	if !x.mayMatch(u) {
		return false
	}
	if x.std != nil && u.simple {
		return x.std.MatchString(u.ua)
	}
	m, _ := x.re2x.MatchRunes(u.getRunes())
	return m
}

// Match returns the text of the match and of all the groups, nil when the
// regex does not match u.
func (x *Regex) Match(u *UserAgent) []string {
	if !x.mayMatch(u) {
		return nil
	}
	if x.std != nil && u.simple {
		return x.std.FindStringSubmatch(u.ua)
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

// translatePattern converts a regexp2 (.NET) pattern to the syntax of the
// standard library.
//
// When ok is false the pattern cannot be converted. Otherwise the returned
// pattern matches a superset of the inputs matched by the original one
// (lookarounds are removed, atomic groups become plain groups), so its
// required literals are required by the original pattern too. exact reports
// whether the two patterns are equivalent, captures included, on inputs made
// of printable ASCII characters only.
func translatePattern(p string) (out string, exact bool, ok bool) {
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

// requiredLiterals returns a set of lower-cased strings of which at least one
// is contained in every input matched by goPattern, nil when there is none.
func requiredLiterals(goPattern string) []string {
	re, err := syntax.Parse(goPattern, syntax.Perl)
	if err != nil {
		return nil
	}
	lits, ok := required(re)
	if !ok || len(lits) == 0 {
		return nil
	}
	sort.Strings(lits)
	out := lits[:0]
	for i, l := range lits {
		if l == "" {
			return nil
		}
		if i == 0 || l != lits[i-1] {
			out = append(out, l)
		}
	}
	return removeRedundant(out)
}

// removeRedundant drops the literals containing another literal of the set:
// they cannot make the prefilter pass on their own.
func removeRedundant(lits []string) []string {
	byLen := append([]string(nil), lits...)
	sort.SliceStable(byLen, func(i, j int) bool { return len(byLen[i]) < len(byLen[j]) })
	var out []string
	for _, l := range byLen {
		redundant := false
		for _, o := range out {
			if strings.Contains(l, o) {
				redundant = true
				break
			}
		}
		if !redundant {
			out = append(out, l)
		}
	}
	sort.Strings(out)
	return out
}

func required(re *syntax.Regexp) ([]string, bool) {
	switch re.Op {
	case syntax.OpLiteral:
		if len(re.Rune) == 0 {
			return nil, false
		}
		r := make([]rune, len(re.Rune))
		for i, c := range re.Rune {
			r[i] = unicode.ToLower(c)
		}
		return []string{string(r)}, true
	case syntax.OpCapture, syntax.OpPlus:
		return required(re.Sub[0])
	case syntax.OpRepeat:
		if re.Min >= 1 {
			return required(re.Sub[0])
		}
		return nil, false
	case syntax.OpConcat:
		var best []string
		found := false
		for _, sub := range re.Sub {
			if lits, ok := required(sub); ok && (!found || betterLiterals(lits, best)) {
				best, found = lits, true
			}
		}
		return best, found
	case syntax.OpAlternate:
		var all []string
		for _, sub := range re.Sub {
			lits, ok := required(sub)
			if !ok {
				return nil, false
			}
			all = append(all, lits...)
		}
		return all, true
	}
	return nil, false
}

// betterLiterals reports whether a is a more selective prefilter than b.
func betterLiterals(a, b []string) bool {
	ma, mb := minLen(a), minLen(b)
	if ma != mb {
		return ma > mb
	}
	return len(a) < len(b)
}

func minLen(lits []string) int {
	m := -1
	for _, l := range lits {
		if m < 0 || len(l) < m {
			m = len(l)
		}
	}
	return m
}
