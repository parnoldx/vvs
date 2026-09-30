// Offline checks against the saved EFA responses in tests/fixtures. Run: go test ./...
package main

import (
	"encoding/json"
	"os"
	"strconv"
	"strings"
	"testing"
)

func load(t *testing.T, f string) map[string]any {
	t.Helper()
	data, err := os.ReadFile("tests/fixtures/" + f)
	if err != nil {
		t.Fatal(err)
	}
	var v map[string]any
	if err := json.Unmarshal(data, &v); err != nil {
		t.Fatal(err)
	}
	return v
}

func TestParseDepartures(t *testing.T) {
	rows := parseDepartures(load(t, "departures.json"))
	if len(rows) <= 10 {
		t.Fatalf("expected many rows, got %d", len(rows))
	}
	var first *Departure
	for i := range rows {
		if rows[i].Line == "U7" {
			first = &rows[i]
		}
	}
	if first == nil {
		t.Fatal("U7 row missing")
	}
	if !first.Local || first.Mot != 3 {
		t.Fatalf("U7 not local: %+v", first)
	}
	okDir := first.Dir == "Mönchfeld" || first.Dir == "Nellingen Ostfildern"
	if !okDir {
		t.Fatalf("unexpected U8 direction %q", first.Dir)
	}
	for _, r := range rows {
		if r.Delay != nil && (*r.Delay >= 60 || *r.Delay <= -60) {
			t.Fatalf("implausible delay %d on %s", *r.Delay, r.Line)
		}
		if len(r.Time) != 5 || r.Time[2] != ':' {
			t.Fatalf("bad time %q", r.Time)
		}
	}
	// effective-time ordering
	minuteOf := func(r Departure) int {
		s := r.RTTime
		if s == "" {
			s = r.Time
		}
		h, _ := strconv.Atoi(s[0:2])
		m, _ := strconv.Atoi(s[3:5])
		return h*60 + m
	}
	for i := 1; i < len(rows); i++ {
		if minuteOf(rows[i-1]) > minuteOf(rows[i]) {
			t.Fatalf("rows out of order at %d", i)
		}
	}
}

func TestIsLocalMot(t *testing.T) {
	for _, mot := range []int{1, 3, 5} {
		if !isLocalMot(mot) {
			t.Fatalf("mot %d should be local", mot)
		}
	}
	if isLocalMot(16) {
		t.Fatal("ICE (16) must not be local")
	}
}

func TestParseTrips(t *testing.T) {
	trips := parseTrips(load(t, "trips.json"))
	if len(trips) < 3 {
		t.Fatalf("expected several trips, got %d", len(trips))
	}
	tr := trips[0]
	if len(tr.Dep) != 5 || tr.Dep[2] != ':' || len(tr.Arr) != 5 || tr.Arr[2] != ':' {
		t.Fatalf("bad trip times %q %q", tr.Dep, tr.Arr)
	}
	if tr.Duration != "0:11" || tr.Changes != 0 || len(tr.Legs) != 1 {
		t.Fatalf("bad trip head: %+v", tr)
	}
	leg := tr.Legs[0]
	if leg.Line != "U6" || leg.Mot == nil || *leg.Mot != 3 || leg.Walk {
		t.Fatalf("bad leg: %+v", leg)
	}
	if leg.From != "Schlossplatz" {
		t.Fatalf("bad from %q", leg.From)
	}
	if !strings.Contains(leg.To, "Feuerbach") {
		t.Fatalf("bad to %q", leg.To)
	}
	if tr.Dep > tr.Arr {
		t.Fatal("dep after arr")
	}
	// every trip must be SSB/local (U-Bahn/S-Bahn to Feuerbach)
	for _, trip := range trips {
		for _, l := range trip.Legs {
			if !l.Walk && (l.Mot == nil || !isLocalMot(*l.Mot)) {
				t.Fatalf("non-local leg %q", l.Line)
			}
		}
	}
}

func TestFilterArriveBy(t *testing.T) {
	list := []Trip{{Arr: "07:55"}, {Arr: "08:00"}, {Arr: "08:01"}, {Arr: "00:05"}}
	feas := filterArriveBy(list, "08:00")
	want := []string{"07:55", "08:00", "00:05"}
	if len(feas) != len(want) {
		t.Fatalf("arrive-by filter wrong: %+v", feas)
	}
	for i, tr := range feas {
		if tr.Arr != want[i] {
			t.Fatalf("arrive-by filter wrong: %+v", feas)
		}
	}
}

func TestParseWhen(t *testing.T) {
	cases := []struct{ in, date, time string }{
		{"8:00", "", "0800"},
		{"morgen 7:45", "", "0745"},
		{"tomorrow 17:30", "", "1730"},
		{"2026-10-01 08:00", "20261001", "0800"},
		{"1.10.", "20261001", ""},
		{"2026-1-2", "20260102", ""},
	}
	for _, c := range cases {
		w := parseWhen(c.in)
		if w == nil {
			t.Fatalf("parseWhen(%q) = nil", c.in)
		}
		if c.date != "" && w.Date != c.date {
			t.Fatalf("parseWhen(%q).Date = %q, want %q", c.in, w.Date, c.date)
		}
		if w.Time != c.time {
			t.Fatalf("parseWhen(%q).Time = %q, want %q", c.in, w.Time, c.time)
		}
	}
	if parseWhen("") != nil {
		t.Fatal("empty input must be nil")
	}
	// "übermorgen" = +2 days, "heute" = today
	w := parseWhen("heute")
	today := ""
	if w != nil {
		today = w.Date
	}
	if today == "" {
		t.Fatal("heute must parse")
	}
}

func TestLocalSearch(t *testing.T) {
	// embedded index: exact names, abbreviations, umlaut folding
	if s := resolveStop("Feuerbach"); s == nil || s.ID != "de:08111:6157" {
		t.Fatalf("feuerbach -> %+v", s)
	}
	if s := resolveStop("muhlhausen"); s == nil || s.Name != "Mühlhausen" {
		t.Fatalf("folded muhlhausen -> %+v", s)
	}
	if s := resolveStop("schlossplatz"); s == nil || s.ID != "de:08111:6022" {
		t.Fatalf("schlossplatz -> %+v (want the Stuttgart one)", s)
	}
	if s := resolveStop("hauptbahnhof"); s == nil || s.Place != "Stuttgart" {
		t.Fatalf("hauptbahnhof -> %+v", s)
	}
	if s := resolveStop("gibts nicht hhxy"); s != nil {
		t.Fatalf("nonsense should miss, got %+v", s)
	}
	// raw ids still pass through untouched
	if s := asStop("5006022"); s == nil || s.ID != "5006022" {
		t.Fatalf("raw id -> %+v", s)
	}
	// "echterdingen bf" finds the stop named just "Echterdingen" (de:08116:7003),
	// not place-substring matches like "Leinfelden-Echterdingen, Hof"
	if s := resolveStop("echterdingen bf"); s == nil || s.ID != "de:08116:7003" {
		t.Fatalf("echterdingen bf -> %+v (want Echterdingen de:08116:7003)", s)
	}
	if s := resolveStop("echterdingen"); s == nil || s.ID != "de:08116:7003" {
		t.Fatalf("echterdingen -> %+v (want Echterdingen de:08116:7003)", s)
	}
	// top-8 wizard results lead with the exact-name match for a prefix query
	hits := searchStations("feuerb")
	if len(hits) == 0 || hits[0].Name != "Feuerbach" {
		t.Fatal("prefix query lost")
	}
}
