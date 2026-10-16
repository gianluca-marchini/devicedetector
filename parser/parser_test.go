package parser

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

const dir = "../regexes"

func TestVendors(t *testing.T) {
	v, err := NewVendor(filepath.Join(dir, FixtureFileVendor))
	require.NoError(t, err)
	str, _ := json.Marshal(v)
	require.Equal(t, "{}", string(str))
}

func TestReg(t *testing.T) {
	name := `Chrome(?:/(\d+[\.\d]+))?`
	ua := `Mozilla/5.0 (Linux; Android 4.2.2; ARCHOS 101 PLATINUM Build/JDQ39) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/34.0.1847.114 Safari/537.36`
	matches := MatchUserAgent(ua, name)

	require.Equal(t, " Chrome/34.0.1847.114", matches[0])
	require.Equal(t, "34.0.1847.114", matches[1])
}

func TestRemoveTD(t *testing.T) {
	inputs := []string{"", "TD", " TD", " td", " Td", "x TD", "x tD", "x TD TD", "TD x", "x TD\n", "x TD\n\n",
		"x  TD", "xTD", " T D", "x TDD", "\n TD", " TD\n", "Galaxy TD", "Ä TD", " ŦD", "x TＤ"}
	for _, in := range inputs {
		want, err := tdReg.Replace(in, "", 0, -1)
		if err != nil {
			t.Fatal(err)
		}
		if got := removeTD(in); got != want {
			t.Errorf("removeTD(%q) = %q, regexp2 gives %q", in, got, want)
		}
	}
}
