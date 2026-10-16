package parser

import (
	"regexp/syntax"
	"sort"
	"strings"
	"unicode"
)

// This file computes the prefilter of a Regex: the literals that must appear
// in a user agent for the regex to match it.
//
// Lower-casing is the one of regexp2 with IgnoreCase: a pattern character p
// matches an input character c when unicode.ToLower(c) equals
// unicode.ToLower(p), so the lower-cased user agent contains the lower-cased
// literals of every match.

const (
	// maxExactSet bounds the size of the sets of strings combined by a cross
	// product.
	maxExactSet = 64
	// maxClassChars bounds the characters of a class used as a set of
	// strings ([0-9] has 10).
	maxClassChars = 10
	// maxClauses bounds the clauses of a prefilter.
	maxClauses = 4
)

// prefilter is a conjunction of clauses: a regex can only match a user agent
// containing, for every clause, at least one of the literals of the clause.
type prefilter struct {
	clauses [][]literal
}

// literal is a lower-cased string with the hashes of some of its bigrams,
// checked against the bigram bitmap of the user agent before searching it.
type literal struct {
	s      string
	grams  [3]uint16
	ngrams uint8
}

func bigram(a, b byte) uint16 {
	return (uint16(a)<<5 ^ uint16(b)) & (gramBits - 1)
}

func newLiteral(s string) literal {
	l := literal{s: s}
	if len(s) < 2 {
		return l
	}
	for _, i := range []int{0, (len(s) - 2) / 2, len(s) - 2} {
		h := bigram(s[i], s[i+1])
		dup := false
		for k := uint8(0); k < l.ngrams; k++ {
			dup = dup || l.grams[k] == h
		}
		if !dup {
			l.grams[l.ngrams] = h
			l.ngrams++
		}
	}
	return l
}

// newPrefilter returns the prefilter of goPattern (see translatePattern),
// nil when no literal is required.
func newPrefilter(goPattern string) *prefilter {
	clauses := requiredClauses(goPattern)
	if clauses == nil {
		return nil
	}
	p := &prefilter{}
	for _, c := range clauses {
		lits := make([]literal, len(c))
		for i, s := range c {
			lits[i] = newLiteral(s)
		}
		p.clauses = append(p.clauses, lits)
	}
	return p
}

// mayMatch reports whether every clause has a literal contained in u.
func (p *prefilter) mayMatch(u *UserAgent) bool {
	grams := u.getGrams()
	for _, c := range p.clauses {
		found := false
		for i := range c {
			if l := &c[i]; containsLiteral(u, grams, l) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func containsLiteral(u *UserAgent, grams *gramBitmap, l *literal) bool {
	for k := uint8(0); k < l.ngrams; k++ {
		h := l.grams[k]
		if grams[h>>6]&(1<<(h&63)) == 0 {
			return false
		}
	}
	if len(l.s) == 1 {
		return strings.IndexByte(u.lower, l.s[0]) >= 0
	}
	return strings.Contains(u.lower, l.s)
}

// requiredClauses returns the clauses of lower-cased strings required by
// goPattern, nil when there is none.
func requiredClauses(goPattern string) [][]string {
	re, err := syntax.Parse(goPattern, syntax.Perl)
	if err != nil {
		return nil
	}
	var clauses [][]string
	seen := map[string]bool{}
	for _, c := range required(re).allClauses() {
		c = removeRedundant(c)
		if len(c) == 0 || c[0] == "" {
			continue
		}
		key := strings.Join(c, "\x00")
		if !seen[key] {
			seen[key] = true
			clauses = append(clauses, c)
		}
	}
	clauses = pruneClauses(clauses)
	if len(clauses) == 0 {
		return nil
	}
	return clauses
}

// commonTokens are found in most user agents: a clause matching them is
// checked last, since it rarely rejects anything.
const commonTokens = "mozilla/5.0 (linux; android ; u; build/ applewebkit/ (khtml, like gecko) " +
	"version/ chrome/ mobile safari/ windows nt ; win64; x64; wow64 macintosh; intel mac os x " +
	"iphone; cpu iphone os like mac os x ipad gecko/ firefox/ compatible; msie trident/ " +
	"opera/ presto/ edge/ samsungbrowser/ wv) en-us"

func weakClause(c []string) bool {
	for _, s := range c {
		if strings.Contains(commonTokens, s) {
			return true
		}
	}
	return false
}

// implies reports whether a clause a implies b: every string of a contains a
// string of b.
func implies(a, b []string) bool {
	for _, x := range a {
		found := false
		for _, y := range b {
			if strings.Contains(x, y) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

// pruneClauses orders the clauses by selectivity and removes the useless
// ones: those implied by another clause, those of single characters when
// there is a longer clause, and the least selective ones beyond maxClauses.
// Removing a clause only makes the prefilter accept more user agents.
func pruneClauses(clauses [][]string) [][]string {
	sort.SliceStable(clauses, func(i, j int) bool {
		return betterLiterals(clauses[i], clauses[j])
	})
	long := false
	for _, c := range clauses {
		long = long || minLen(c) > 1
	}
	var out [][]string
	for i, c := range clauses {
		if long && minLen(c) == 1 {
			continue
		}
		redundant := false
		for j, o := range clauses {
			// o implies c: c is useless. When they imply each other, keep
			// the first one.
			if i != j && implies(o, c) && (!implies(c, o) || j < i) {
				redundant = true
				break
			}
		}
		if !redundant {
			out = append(out, c)
		}
	}
	if len(out) > maxClauses {
		out = out[:maxClauses]
	}
	return out
}

// literalInfo describes the strings that a regex requires.
//
// When exact, every match is exactly one of set. Otherwise every match
// contains, for every clause, one of its strings, starts with one of prefix
// (when not nil) and ends with one of suffix (when not nil).
type literalInfo struct {
	exact   bool
	set     []string
	prefix  []string
	suffix  []string
	clauses [][]string
}

func exactInfo(set ...string) literalInfo {
	return literalInfo{exact: true, set: set, prefix: set, suffix: set}
}

func (i literalInfo) allClauses() [][]string {
	if i.exact {
		return [][]string{i.set}
	}
	c := i.clauses
	if i.prefix != nil {
		c = append(c, i.prefix)
	}
	if i.suffix != nil {
		c = append(c, i.suffix)
	}
	return c
}

// bestClause returns the most selective clause, false when there is none.
func (i literalInfo) bestClause() ([]string, bool) {
	var best []string
	found := false
	for _, c := range i.allClauses() {
		if hasEmpty(c) {
			continue
		}
		if !found || betterLiterals(c, best) {
			best, found = c, true
		}
	}
	return best, found
}

func hasEmpty(set []string) bool {
	for _, s := range set {
		if s == "" {
			return true
		}
	}
	return false
}

func required(re *syntax.Regexp) literalInfo {
	switch re.Op {
	case syntax.OpLiteral:
		r := make([]rune, len(re.Rune))
		for i, c := range re.Rune {
			r[i] = unicode.ToLower(c)
		}
		return exactInfo(string(r))
	case syntax.OpEmptyMatch, syntax.OpBeginLine, syntax.OpEndLine, syntax.OpBeginText,
		syntax.OpEndText, syntax.OpWordBoundary, syntax.OpNoWordBoundary:
		// Zero-width: ignoring the assertion only accepts more inputs.
		return exactInfo("")
	case syntax.OpCharClass:
		return classInfo(re.Rune)
	case syntax.OpCapture:
		return required(re.Sub[0])
	case syntax.OpQuest:
		if sub := required(re.Sub[0]); sub.exact && len(sub.set) < maxExactSet {
			return exactInfo(append(append([]string(nil), sub.set...), "")...)
		}
		return literalInfo{}
	case syntax.OpPlus:
		return repeatInfo(required(re.Sub[0]), 1, -1)
	case syntax.OpRepeat:
		return repeatInfo(required(re.Sub[0]), re.Min, re.Max)
	case syntax.OpConcat:
		return concatInfo(re.Sub)
	case syntax.OpAlternate:
		return alternateInfo(re.Sub)
	}
	return literalInfo{}
}

// classInfo returns the characters of a small character class: with
// IgnoreCase regexp2 tests the lower-cased input character against the class
// extended with the lower case of its characters.
func classInfo(ranges []rune) literalInfo {
	var chars []string
	seen := map[rune]bool{}
	for i := 0; i+1 < len(ranges); i += 2 {
		if ranges[i+1]-ranges[i] >= 2*maxClassChars {
			return literalInfo{}
		}
		for c := ranges[i]; c <= ranges[i+1]; c++ {
			for _, l := range []rune{c, unicode.ToLower(c)} {
				// The lower-cased user agent only contains the fixed points
				// of unicode.ToLower (it is idempotent, see the tests).
				if unicode.ToLower(l) == l && !seen[l] {
					seen[l] = true
					chars = append(chars, string(l))
				}
			}
			if len(chars) > maxClassChars {
				return literalInfo{}
			}
		}
	}
	if len(chars) == 0 {
		return literalInfo{}
	}
	return exactInfo(chars...)
}

// repeatInfo describes sub{min,max}; max < 0 means no maximum.
func repeatInfo(sub literalInfo, min, max int) literalInfo {
	if min < 1 {
		return literalInfo{}
	}
	if !sub.exact {
		return literalInfo{prefix: sub.prefix, suffix: sub.suffix, clauses: sub.allClauses()}
	}
	// The first and the last min copies are exact.
	ends := sub.set
	for n := 1; n < min && len(ends)*len(sub.set) <= maxExactSet; n++ {
		ends = crossProduct(ends, sub.set)
	}
	if min == max && len(ends) == pow(len(sub.set), min) {
		return exactInfo(ends...)
	}
	return literalInfo{prefix: ends, suffix: ends}
}

func pow(b, e int) int {
	r := 1
	for i := 0; i < e && r <= maxExactSet; i++ {
		r *= b
	}
	return r
}

// concatInfo combines the consecutive exact parts of a concatenation into
// longer strings and collects the clauses of all its parts.
func concatInfo(subs []*syntax.Regexp) literalInfo {
	var out literalInfo
	run := []string{""}
	allExact := true
	leading := true // run is the product of the leading exact parts
	for _, sub := range subs {
		info := required(sub)
		if info.exact {
			if productFits(run, info.set) {
				run = crossProduct(run, info.set)
				continue
			}
			allExact = false
			if leading {
				out.prefix = run
				leading = false
			}
			out.clauses = append(out.clauses, run)
			run = info.set
			continue
		}
		allExact = false
		joined := run
		if info.prefix != nil && productFits(run, info.prefix) {
			joined = crossProduct(run, info.prefix)
		}
		if leading {
			out.prefix = joined
			leading = false
		}
		out.clauses = append(out.clauses, joined)
		out.clauses = append(out.clauses, info.clauses...)
		if info.prefix != nil {
			out.clauses = append(out.clauses, info.prefix)
		}
		if info.suffix != nil {
			run = info.suffix
		} else {
			run = []string{""}
		}
	}
	if allExact {
		return exactInfo(run...)
	}
	out.suffix = run
	return out
}

// alternateInfo: a match of an alternation is a match of one of its
// alternatives, so it contains one of the strings of their best clauses.
func alternateInfo(subs []*syntax.Regexp) literalInfo {
	infos := make([]literalInfo, len(subs))
	allExact := true
	size := 0
	for i, sub := range subs {
		infos[i] = required(sub)
		allExact = allExact && infos[i].exact
		size += len(infos[i].set)
	}
	if allExact && size <= 4*maxExactSet {
		var set []string
		for _, info := range infos {
			set = append(set, info.set...)
		}
		return exactInfo(set...)
	}
	var out literalInfo
	var clause, prefix, suffix []string
	clauseOK, prefixOK, suffixOK := true, true, true
	for _, info := range infos {
		if c, ok := info.bestClause(); ok && clauseOK {
			clause = append(clause, c...)
		} else {
			clauseOK = false
		}
		if prefixOK && info.prefix != nil {
			prefix = append(prefix, info.prefix...)
		} else {
			prefixOK = false
		}
		if suffixOK && info.suffix != nil {
			suffix = append(suffix, info.suffix...)
		} else {
			suffixOK = false
		}
	}
	if clauseOK {
		out.clauses = [][]string{clause}
	}
	if prefixOK && len(prefix) <= 4*maxExactSet {
		out.prefix = prefix
	}
	if suffixOK && len(suffix) <= 4*maxExactSet {
		out.suffix = suffix
	}
	return out
}

// productFits reports whether the cross product of a and b is small enough.
// A product by a single string does not grow the set.
func productFits(a, b []string) bool {
	return len(a)*len(b) <= maxExactSet || len(b) == 1 && len(a) <= 4*maxExactSet
}

func crossProduct(a, b []string) []string {
	r := make([]string, 0, len(a)*len(b))
	for _, x := range a {
		for _, y := range b {
			r = append(r, x+y)
		}
	}
	return r
}

// removeRedundant sorts and deduplicates lits and drops the strings
// containing another string of the set: they cannot make the clause true on
// their own.
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

// betterLiterals reports whether a is a more selective clause than b: a
// clause without the common tokens of user agents, then with longer strings,
// then with fewer strings.
func betterLiterals(a, b []string) bool {
	if wa, wb := weakClause(a), weakClause(b); wa != wb {
		return !wa
	}
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
