package parser

import "strings"

// Index finds, with a single scan of a user agent, the regexes of a list that
// can match it: those with a string of the first clause of their prefilter
// (see literals.go) in the user agent, and those without a prefilter. The
// other regexes cannot match, so Index never changes which regex of the list
// matches first.
//
// An Index is safe for concurrent use.
type Index struct {
	n int
	// always has a bit set for the regexes that are always candidates.
	always []uint64
	// The literals of length 1 by their character, and the longer ones by
	// the hash of their first 2 or 3 bytes, as offsets in lits.
	charStart [257]int32
	hashStart []int32
	mask      uint32
	lits      []indexLiteral
}

// hash2 and hash3 are the hashes of the first 2 and 3 bytes of a literal.
func hash2(a, b byte) uint32 {
	return (uint32(a)<<8 | uint32(b)) * 0x9E3779B1
}

func hash3(a, b, c byte) uint32 {
	return (uint32(a)<<16 | uint32(b)<<8 | uint32(c)) * 0x85EBCA6B
}

// key returns the bucket of a literal: its character for a literal of length
// 1, 256 plus its hash otherwise.
func (ix *Index) key(l string) uint32 {
	switch len(l) {
	case 1:
		return uint32(l[0])
	case 2:
		return 256 + hash2(l[0], l[1])>>16&ix.mask
	}
	return 256 + hash3(l[0], l[1], l[2])>>16&ix.mask
}

type indexLiteral struct {
	s string
	// owners are the regexes that a match of s makes candidates.
	owners []int32
}

// Candidates is the result of Index.Candidates.
type Candidates []uint64

// Has reports whether the regex at position i of the list can match.
func (c Candidates) Has(i int) bool {
	return c[i>>6]&(1<<(uint(i)&63)) != 0
}

func (c Candidates) set(i int) {
	c[i>>6] |= 1 << (uint(i) & 63)
}

// NewIndex builds the index of a list of regexes. The regexes must be
// compiled.
func NewIndex(regexes []*Regular) *Index {
	ix := &Index{n: len(regexes), always: make([]uint64, (len(regexes)+63)/64)}
	owners := map[string][]int32{}
	for i, r := range regexes {
		lits := firstClause(r.x)
		if lits == nil {
			Candidates(ix.always).set(i)
			continue
		}
		for _, l := range lits {
			owners[l] = append(owners[l], int32(i))
		}
	}

	// Bucket the literals (counting sort).
	size := 64
	for size < 2*len(owners) && size < 1<<16 {
		size *= 2
	}
	ix.mask = uint32(size - 1)
	ix.hashStart = make([]int32, size+1)
	counts := make([]int32, 256+size)
	for l := range owners {
		counts[ix.key(l)]++
	}
	next := int32(0)
	for k := range counts {
		if k < 256 {
			ix.charStart[k] = next
		} else {
			ix.hashStart[k-256] = next
		}
		next += counts[k]
		counts[k] = 0
	}
	ix.charStart[256] = ix.hashStart[0]
	ix.hashStart[size] = next
	ix.lits = make([]indexLiteral, next)
	for l, o := range owners {
		k := ix.key(l)
		start := ix.charStart[k&255]
		if k >= 256 {
			start = ix.hashStart[k-256]
		}
		ix.lits[start+counts[k]] = indexLiteral{s: l, owners: o}
		counts[k]++
	}
	return ix
}

// firstClause returns the strings of which one is needed for x to match, nil
// when there are none.
func firstClause(x *Regex) []string {
	if x == nil {
		return nil
	}
	if x.alts != nil {
		// x matches when an alternative matches.
		var all []string
		for _, a := range x.alts {
			lits := firstClause(a)
			if lits == nil {
				return nil
			}
			all = append(all, lits...)
		}
		return all
	}
	if x.pre == nil {
		return nil
	}
	lits := make([]string, len(x.pre.clauses[0]))
	for i, l := range x.pre.clauses[0] {
		lits[i] = l.s
	}
	return lits
}

// Candidates returns the regexes of the list that can match u.
func (ix *Index) Candidates(u *UserAgent) Candidates {
	c := make(Candidates, len(ix.always))
	copy(c, ix.always)
	s := u.lower
	var seen [256]bool
	for i := 0; i < len(s); i++ {
		if !seen[s[i]] {
			seen[s[i]] = true
			for _, l := range ix.lits[ix.charStart[s[i]]:ix.charStart[int(s[i])+1]] {
				for _, o := range l.owners {
					c.set(int(o))
				}
			}
		}
		if i+1 == len(s) {
			break
		}
		k := hash2(s[i], s[i+1]) >> 16 & ix.mask
		for _, l := range ix.lits[ix.hashStart[k]:ix.hashStart[k+1]] {
			if len(l.s) == 2 && s[i] == l.s[0] && s[i+1] == l.s[1] {
				for _, o := range l.owners {
					c.set(int(o))
				}
			}
		}
		if i+2 == len(s) {
			continue
		}
		k = hash3(s[i], s[i+1], s[i+2]) >> 16 & ix.mask
		for j := ix.hashStart[k]; j < ix.hashStart[k+1]; j++ {
			l := &ix.lits[j]
			if len(l.s) > 2 && strings.HasPrefix(s[i:], l.s) {
				for _, o := range l.owners {
					c.set(int(o))
				}
			}
		}
	}
	return c
}
