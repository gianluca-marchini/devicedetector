// Package alternation tells when the alternation of regexes can be evaluated
// one regex at a time.
package alternation

import "strings"

// IsUnion reports whether the regexes are independent: then the
// alternation of the regexes matches a user agent exactly when one of the
// regexes matches it. They are not when a regex has backreferences, inline
// options or conditionals, whose meaning depends on the other regexes.
func IsUnion(regexes []string) bool {
	for _, r := range regexes {
		for i := 0; i < len(r); i++ {
			switch r[i] {
			case '\\':
				if i+1 < len(r) && (r[i+1] >= '0' && r[i+1] <= '9' || r[i+1] == 'k') {
					return false
				}
				i++
			case '(':
				if strings.HasPrefix(r[i:], "(?") && !strings.HasPrefix(r[i:], "(?:") &&
					!strings.HasPrefix(r[i:], "(?=") && !strings.HasPrefix(r[i:], "(?!") &&
					!strings.HasPrefix(r[i:], "(?<=") && !strings.HasPrefix(r[i:], "(?<!") &&
					!strings.HasPrefix(r[i:], "(?>") {
					return false
				}
			}
		}
	}
	return true
}
