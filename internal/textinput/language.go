// Package textinput owns Unicode editing, keyboard geometry and IME processes.
// It has no dependency on SDL or archive readers.
package textinput

import (
	_ "embed"
	"encoding/json"
	"sort"
	"strings"
)

//go:embed languages.json
var languageData []byte

type Profile struct {
	Name     string   `json:"name"`
	Language string   `json:"language"`
	Methods  []string `json:"methods"`
}

var registry struct {
	Aliases  map[string]string  `json:"aliases"`
	Profiles map[string]Profile `json:"profiles"`
}

func init() {
	if err := json.Unmarshal(languageData, &registry); err != nil {
		panic(err)
	}
}

// Languages preserves the archive's order and always includes English once.
// Unknown languages remain visible instead of being silently discarded.
func Languages(raw string) []string {
	result := []string{}
	seen := map[string]bool{}
	for _, value := range strings.FieldsFunc(raw, func(r rune) bool { return r == ',' || r == ';' || r == ' ' || r == '\n' || r == '\t' }) {
		code := strings.ToLower(strings.TrimSpace(value))
		if alias, ok := registry.Aliases[code]; ok {
			code = alias
		}
		if code == "" || code == "und" || code == "mul" || code == "zxx" || len(code) > 16 || seen[code] {
			continue
		}
		seen[code] = true
		result = append(result, code)
	}
	if !seen["eng"] {
		result = append(result, "eng")
	}
	return result
}
func Lookup(code string) (Profile, bool) {
	p, ok := registry.Profiles[code]
	return p, ok
}
func Name(code string) string {
	if p, ok := Lookup(code); ok {
		return p.Name
	}
	return code
}

func AvailableLanguages() []string {
	values := make([]string, 0, len(registry.Profiles))
	for code := range registry.Profiles {
		values = append(values, code)
	}
	sort.Strings(values)
	return values
}
