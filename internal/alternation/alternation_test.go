package alternation

import "testing"

func TestIsUnion(t *testing.T) {
	cases := map[string]bool{
		`a|b`:           true,
		`(?:a)(b)`:      true,
		`x(?!y)(?<!z)w`: true,
		`(?=a)(?<=b)`:   true,
		`(?>a)`:         true,
		`\(?i\)`:        true,
		`(a)\1`:         false,
		`(?<n>a)\k<n>`:  false,
		`(?i)a`:         false,
		`(?-i:a)`:       false,
		`(?(a)b|c)`:     false,
		`(?<name>a)`:    false,
	}
	for r, want := range cases {
		if got := IsUnion([]string{"ok", r}); got != want {
			t.Errorf("IsUnion(%q) = %v, want %v", r, got, want)
		}
	}
}
