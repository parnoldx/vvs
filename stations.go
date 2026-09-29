// Embedded station index — all VVS stops (data/stations.json, regenerated
// from the official "haltestellen-vvs" registry via tools/mkstations.py).
// Station resolution is fully local: no stopfinder lookup, no flaky API.
package main

import (
	_ "embed"
	"encoding/json"
	"sort"
	"strings"
)

//go:embed data/stations.json
var stationsJSON []byte

type indexedStop struct {
	Stop
	// folded whole-label strings and word lists, both umlaut variants
	labelE, labelD string
	wordsE, wordsD []string
}

var stationIndex []indexedStop

func init() {
	var list []Stop
	if err := json.Unmarshal(stationsJSON, &list); err != nil {
		panic("stations.json: " + err.Error())
	}
	stationIndex = make([]indexedStop, len(list))
	for i, s := range list {
		st := indexedStop{Stop: s}
		st.labelE = fold(label(s), true)
		st.labelD = fold(label(s), false)
		for _, w := range splitWords(st.labelE) {
			st.wordsE = append(st.wordsE, expandAbbrev(w))
		}
		for _, w := range splitWords(st.labelD) {
			st.wordsD = append(st.wordsD, expandAbbrev(w))
		}
		stationIndex[i] = st
	}
}

// label matches the EFA display style: "Place, Name" (kept when the name
// already starts with the place)
func label(s Stop) string {
	if s.Place != "" && s.Name != "" && !strings.HasPrefix(s.Name, s.Place) {
		return s.Place + ", " + s.Name
	}
	return s.Name
}

// fold lowercases and folds umlauts two ways: expand (ö→oe, matches user
// input "moehringen") and drop (ö→o, matches "mohringen")
func fold(s string, expand bool) string {
	var b strings.Builder
	b.Grow(len(s) + 8)
	for _, r := range strings.ToLower(s) {
		switch r {
		case 'ä':
			if expand {
				b.WriteString("ae")
			} else {
				b.WriteByte('a')
			}
		case 'ö':
			if expand {
				b.WriteString("oe")
			} else {
				b.WriteByte('o')
			}
		case 'ü':
			if expand {
				b.WriteString("ue")
			} else {
				b.WriteByte('u')
			}
		case 'ß':
			b.WriteString("ss")
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

func isWordRune(r rune) bool { return r >= 'a' && r <= 'z' || r >= '0' && r <= '9' }

func splitWords(s string) []string {
	return strings.FieldsFunc(s, func(r rune) bool { return !isWordRune(r) })
}

// common German station abbreviations, expanded on both sides so "bf"
// matches "bahnhof" and vice versa
var abbrevs = map[string]string{
	"bf": "bahnhof", "hbf": "hauptbahnhof", "ob": "oberzentrum",
}

func expandAbbrev(w string) string {
	if a, ok := abbrevs[w]; ok {
		return a
	}
	return w
}

// query appears in label as a complete chunk (non-word chars around it)
func boundedMatch(label, key string) bool {
	for i := 0; ; {
		j := strings.Index(label[i:], key)
		if j < 0 {
			return false
		}
		i += j
		before := i == 0 || !isWordRune(rune(label[i-1]))
		after := i+len(key) >= len(label) || !isWordRune(rune(label[i+len(key)]))
		if before && after {
			return true
		}
		i++
	}
}

func wordPrefix(words []string, key string) bool {
	for _, w := range words {
		if strings.HasPrefix(w, key) {
			return true
		}
	}
	return false
}

// every query token matches some label word (prefix either way, e.g.
// "bahnhof" ~ "bf"), exact whole-word hits count double
func tokensMatch(words []string, key string) (bool, int) {
	toks := splitWords(key)
	if len(toks) == 0 {
		return false, 0
	}
	exact := 0
	for _, t := range toks {
		t = expandAbbrev(t)
		hit := false
		for _, w := range words {
			if w == t {
				hit = true
				exact++
				break
			}
			if strings.HasPrefix(w, t) || strings.HasPrefix(t, w) {
				hit = true
				break
			}
		}
		if !hit {
			return false, 0
		}
	}
	return true, exact
}

// searchStations scores the embedded list: whole-query word match (100) >
// word prefix (80) > substring (60) > all tokens matched (40 + 10/exact),
// plus the Stuttgart place bias from the old stopfinder scoring. Stable on
// list order for ties.
func searchStations(q string) []Stop {
	key := strings.ToLower(strings.TrimSpace(q))
	if key == "" {
		return nil
	}
	ke, kd := fold(key, true), fold(key, false)
	type scored struct {
		i int
		v int
	}
	var hits []scored
	for i, st := range stationIndex {
		v := 0
		switch {
		case boundedMatch(st.labelE, ke) || boundedMatch(st.labelD, kd):
			v = 100
		case wordPrefix(st.wordsE, ke) || wordPrefix(st.wordsD, kd):
			v = 80
		case strings.Contains(st.labelE, ke) || strings.Contains(st.labelD, kd):
			v = 60
		default:
			if ok, exact := tokensMatch(st.wordsE, ke); ok {
				v = 40 + 10*exact
			} else if ok, exact := tokensMatch(st.wordsD, kd); ok {
				v = 40 + 10*exact
			}
		}
		if v > 0 {
			if st.Place == "Stuttgart" {
				v += 50
			}
			hits = append(hits, scored{i, v})
		}
	}
	sort.SliceStable(hits, func(a, b int) bool {
		if hits[a].v != hits[b].v {
			return hits[a].v > hits[b].v
		}
		// equal score: the shorter (more specific) name first
		return len([]rune(stationIndex[hits[a].i].Name)) < len([]rune(stationIndex[hits[b].i].Name))
	})
	out := make([]Stop, len(hits))
	for k, h := range hits {
		out[k] = stationIndex[h.i].Stop
	}
	return out
}

// resolveStop is now a local index lookup — the EFA stopfinder is gone
func resolveStop(q string) *Stop {
	if hits := searchStations(q); len(hits) > 0 {
		return &hits[0]
	}
	return nil
}
