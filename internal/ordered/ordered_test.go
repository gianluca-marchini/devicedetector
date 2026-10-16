package ordered

import (
	"testing"

	"gopkg.in/yaml.v2"
)

func keys(t *testing.T, doc string) []string {
	t.Helper()
	var m map[string]interface{}
	if err := yaml.Unmarshal([]byte(doc), &m); err != nil {
		t.Fatal(err)
	}
	k, err := Keys([]byte(doc), m)
	if err != nil {
		t.Fatal(err)
	}
	if len(k) != len(m) {
		t.Fatalf("got %d keys, the map has %d", len(k), len(m))
	}
	return k
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestKeysDocumentOrder(t *testing.T) {
	got := keys(t, "zeta: 1\nalpha: 2\n'Tunisie Telecom': 3\nmid: 4\n")
	want := []string{"zeta", "alpha", "Tunisie Telecom", "mid"}
	if !equal(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestKeysDuplicated(t *testing.T) {
	// A duplicated key takes the position of its first occurrence.
	got := keys(t, "b: 1\na: 2\nb: 3\n")
	want := []string{"b", "a"}
	if !equal(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestKeysNotStrings(t *testing.T) {
	got := keys(t, "28: Blink\n'3.5': Elektra\n7: Presto\n")
	want := []string{"28", "3.5", "7"}
	if !equal(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestKeysNotAMap(t *testing.T) {
	if _, err := Keys([]byte("a: 1"), []string{}); err == nil {
		t.Fatal("expected an error")
	}
}
