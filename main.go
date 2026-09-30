// CLI: JSON on stdout (feeds a UI later) or human-readable formatting.
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	tea "github.com/charmbracelet/bubbletea"
)

// ---- config -----------------------------------------------------------------

func configPath() string {
	h, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(h, ".config", "vvs.json")
}

func loadConfig() map[string]any {
	data, err := os.ReadFile(configPath())
	if err != nil {
		return map[string]any{}
	}
	var m map[string]any
	if json.Unmarshal(data, &m) != nil || m == nil {
		return map[string]any{}
	}
	return m
}

func saveConfig(cfg map[string]any) {
	os.MkdirAll(filepath.Dir(configPath()), 0o755)
	data, _ := json.MarshalIndent(cfg, "", " ")
	os.WriteFile(configPath(), append(data, '\n'), 0o644)
}

func homeStop() Stop {
	h := obj(loadConfig()["home"])
	if h != nil && (str(h["name"]) != "" || str(h["id"]) != "") {
		return Stop{Name: str(h["name"]), Place: str(h["place"]), ID: str(h["id"])}
	}
	return Stop{} // unset home surfaces as a clear error at call sites
}

func saveHome(s Stop) {
	cfg := loadConfig()
	hm := map[string]any{"name": s.Name, "id": s.ID}
	if s.Place != "" {
		hm["place"] = s.Place
	}
	cfg["home"] = hm
	saveConfig(cfg)
}

// ---- queries ----------------------------------------------------------------

type DepsResult struct {
	Station *Stop       `json:"station,omitempty"`
	Rows    []Departure `json:"rows"`
	Error   string      `json:"error,omitempty"`
}

type TripsResult struct {
	From         *Stop  `json:"from,omitempty"`
	To           *Stop  `json:"to,omitempty"`
	ArriveBy     bool   `json:"arriveBy,omitempty"`
	ArriveByTime string `json:"arriveByTime,omitempty"`
	Trips        []Trip `json:"trips"`
	Error        string `json:"error,omitempty"`
}

func departures(query any, limit int, localOnly bool) DepsResult {
	stop := homeStop()
	if q, ok := query.(string); !ok || q != "" {
		s := asStopQ(query)
		if s == nil {
			return DepsResult{Error: "unknown station: " + fmt.Sprint(query)}
		}
		stop = *s
	}
	if stop.ID == "" {
		return DepsResult{Error: "no home station set — run `vvs home <station>`"}
	}
	j, err := httpJSON(departuresURL(stop.ID, limit), limitTimetable)
	if err != nil {
		return DepsResult{Error: err.Error()}
	}
	rows := parseDepartures(j)
	if localOnly {
		kept := make([]Departure, 0, len(rows))
		for _, r := range rows {
			if r.Local {
				kept = append(kept, r)
			}
		}
		rows = kept
	}
	if rows == nil {
		rows = []Departure{}
	}
	return DepsResult{Station: &stop, Rows: rows}
}

func errTrips(msg string) TripsResult { return TripsResult{Trips: []Trip{}, Error: msg} }

func tripsQuery(direction string, query any, date, timeStr string, localOnly, arriveBy bool) TripsResult {
	h := homeStop()
	var from, to *Stop
	if direction == "to" {
		from, to = &h, asStopQ(query)
	} else {
		from, to = asStopQ(query), &h
	}
	if from == nil {
		return errTrips("unknown station: " + fmt.Sprint(query))
	}
	if h.ID == "" {
		return errTrips("no home station set — run `vvs home <station>`")
	}
	if to == nil {
		return errTrips("unknown station: " + fmt.Sprint(query))
	}
	return runTrips(from, to, date, timeStr, localOnly, arriveBy)
}

func tripsRoute(route [2]string, date, timeStr string, localOnly, arriveBy bool) TripsResult {
	from := asStop(route[0])
	if from == nil {
		return errTrips("unknown station: " + route[0])
	}
	to := asStop(route[1])
	if to == nil {
		return errTrips("unknown station: " + route[1])
	}
	return runTrips(from, to, date, timeStr, localOnly, arriveBy)
}

func runTrips(from, to *Stop, date, timeStr string, localOnly, arriveBy bool) TripsResult {
	queryTrips := func(d, t string) ([]Trip, error) {
		j, err := httpJSON(tripURL(from.ID, to.ID, d, t), limitTimetable)
		if err != nil {
			return nil, err
		}
		list := parseTrips(j)
		if localOnly {
			kept := make([]Trip, 0, len(list))
			for _, x := range list {
				ok := true
				for _, l := range x.Legs {
					if !l.Walk && (l.Mot == nil || !isLocalMot(*l.Mot)) {
						ok = false
						break
					}
				}
				if ok {
					kept = append(kept, x)
				}
			}
			list = kept
		}
		if list == nil {
			list = []Trip{}
		}
		return list, nil
	}

	// Arrival-oriented search: efa-bw.de ignores itdTripDateTimeDep=arr, and a
	// wide lookback misses the latest departures (the server returns only a
	// handful of trips per query). Start 20 min before the target and widen
	// until something arrives in time; report the latest feasible connections.
	if arriveBy && timeStr != "" {
		target := timeStr[:2] + ":" + timeStr[2:4]
		hh, _ := strconv.Atoi(timeStr[:2])
		mm, _ := strconv.Atoi(timeStr[2:4])
		var candidates []Trip
		for w := arriveStep; w <= 480; w *= 2 {
			total := hh*60 + mm - w
			day := time.Now()
			if date != "" {
				day, _ = time.ParseInLocation("20060102", date, time.Local)
			}
			if total < 0 {
				day = day.AddDate(0, 0, -1)
			}
			qDate := ""
			if date != "" {
				qDate = day.Format("20060102")
			}
			m := ((total % 1440) + 1440) % 1440
			qTime := pad2(m/60) + pad2(m%60)
			list, err := queryTrips(qDate, qTime)
			if err != nil {
				return errTrips(err.Error())
			}
			candidates = filterArriveBy(list, target)
			if len(candidates) > 0 {
				break
			}
		}
		if len(candidates) == 0 {
			return TripsResult{From: from, To: to, ArriveBy: true, Trips: []Trip{},
				Error: fmt.Sprintf("nothing arrives by %s within 8 h of searching — check the connection", target)}
		}
		if len(candidates) > 4 {
			candidates = candidates[len(candidates)-4:]
		}
		return TripsResult{From: from, To: to, ArriveBy: true, ArriveByTime: target, Trips: candidates}
	}

	list, err := queryTrips(date, timeStr)
	if err != nil {
		return errTrips(err.Error())
	}
	return TripsResult{From: from, To: to, Trips: list}
}

// ---- human-readable output --------------------------------------------------

var useColor = false

func isTTY(f *os.File) bool {
	fi, err := f.Stat()
	return err == nil && fi.Mode()&os.ModeCharDevice != 0
}

func paint(code, s string) string {
	if useColor {
		return "\x1b[" + code + "m" + s + "\x1b[0m"
	}
	return s
}
func bold(s string) string { return paint("1", s) }
func dim(s string) string  { return paint("2", s) }
func red(s string) string  { return paint("31", s) }

// JS .length counts UTF-16 units; for BMP strings that equals rune count
func rc(s string) int { return utf8.RuneCountInString(s) }

func pad(s string, n int) string {
	if rc(s) >= n {
		return s
	}
	return s + strings.Repeat(" ", n-rc(s))
}
func padL(s string, n int) string {
	if rc(s) >= n {
		return s
	}
	return strings.Repeat(" ", n-rc(s)) + s
}

func errHint(msg string) string {
	s := red("error:") + " " + msg
	if strings.Contains(msg, "unknown station") {
		s += dim(" — try: vvs search <term> or vvs to ?")
	}
	return s
}

func fmtDepartures(r DepsResult) string {
	if r.Error != "" {
		return errHint(r.Error)
	}
	rows := r.Rows
	if len(rows) == 0 {
		return r.Station.Name + " — no local departures right now"
	}
	live := false
	for _, x := range rows {
		if x.RTTime != "" {
			live = true
		}
	}
	head := r.Station.Name + " — departures"
	if live {
		head += " (live)"
	}
	lines := []string{bold(head)}
	eff := func(x Departure) string {
		if x.RTTime != "" {
			return x.RTTime
		}
		return x.Time
	}
	wTime, wLine, wDir, wPl := 5, 4, 11, 2
	for _, x := range rows {
		if n := rc(eff(x)); n > wTime {
			wTime = n
		}
		if n := rc(x.Line); n > wLine {
			wLine = n
		}
		if n := rc(x.Dir); n > wDir {
			wDir = n
		}
		if n := rc(x.Platform); n > wPl {
			wPl = n
		}
	}
	for _, x := range rows {
		status := dim("on time")
		switch {
		case x.Cancelled:
			status = red("cancelled")
		case x.Delay != nil && *x.Delay > 0:
			status = "+" + strconv.Itoa(*x.Delay) + "'"
		}
		lines = append(lines, "  "+padL(eff(x), wTime)+"  "+bold(pad(x.Line, wLine))+"  "+pad(x.Dir, wDir)+"  "+pad(x.Platform, wPl)+"  "+status)
	}
	return strings.Join(lines, "\n")
}

func fmtTrips(r TripsResult) string {
	if r.Error != "" {
		return errHint(r.Error)
	}
	by := ""
	if r.ArriveBy {
		by = " (arriving by " + r.ArriveByTime + ")"
	}
	lines := []string{bold(r.From.Name + " → " + r.To.Name + by)}
	if len(r.Trips) == 0 {
		return strings.Join(lines, "\n") + "\n  no local connections found"
	}
	wSpan := 0
	for _, t := range r.Trips {
		if n := rc(t.Dep + "–" + t.Arr); n > wSpan {
			wSpan = n
		}
	}
	for _, t := range r.Trips {
		var parts []string
		for _, l := range t.Legs {
			if l.Walk {
				min := "?"
				if l.Minutes != nil {
					min = strconv.Itoa(*l.Minutes)
				}
				parts = append(parts, dim("walk "+min+" min"))
			} else {
				parts = append(parts, l.Line)
			}
		}
		span := pad(t.Dep+"–"+t.Arr, wSpan)
		lines = append(lines, "  "+span+"  "+pad(t.Duration, 5)+"  "+strconv.Itoa(t.Changes)+"×  "+strings.Join(parts, " · "))
	}
	return strings.Join(lines, "\n")
}

// ---- interactive picker (bubbletea) ------------------------------------------

type pickResult struct {
	stop *Stop
	when *When
}

type wizardModel struct {
	label     string
	allowTime bool
	mode      string // "station" | "time"
	buffer    string
	timeBuf   string
	results   []Stop
	sel       int
	when      *When
	lines     int // last rendered line count, cleared after the program ends
	ctrlc     bool
	result    *pickResult
}

func (m *wizardModel) Init() tea.Cmd { return nil }

func (m *wizardModel) search() {
	q := strings.TrimSpace(m.buffer)
	if utf8.RuneCountInString(q) < 2 {
		m.results = nil
		return
	}
	ranked := searchStations(q) // local index — instant, no debounce needed
	if len(ranked) > 8 {
		ranked = ranked[:8]
	}
	m.results = ranked
	m.sel = 0
}

func trimLastRune(s string) string {
	r := []rune(s)
	if len(r) == 0 {
		return s
	}
	return string(r[:len(r)-1])
}

func (m *wizardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			m.ctrlc = true
			return m, tea.Quit
		case "esc":
			if m.mode == "time" {
				m.mode = "station"
				return m, nil
			}
			return m, tea.Quit
		}
		if m.mode == "station" {
			switch msg.Type {
			case tea.KeyTab:
				if m.allowTime {
					m.mode = "time"
				}
			case tea.KeyEnter:
				if m.sel < len(m.results) {
					m.result = &pickResult{stop: &m.results[m.sel], when: m.when}
					return m, tea.Quit
				}
			case tea.KeyDown:
				if m.sel < len(m.results)-1 {
					m.sel++
				}
			case tea.KeyUp:
				if m.sel > 0 {
					m.sel--
				}
			case tea.KeyBackspace:
				if m.buffer != "" {
					m.buffer = trimLastRune(m.buffer)
					m.search()
				}
			case tea.KeyRunes, tea.KeySpace:
				m.buffer += msg.String()
				m.search()
			}
		} else {
			switch msg.Type {
			case tea.KeyTab:
				m.mode = "station"
			case tea.KeyEnter:
				if strings.TrimSpace(m.timeBuf) != "" {
					m.when = parseWhen(m.timeBuf)
				} else {
					m.when = nil
				}
				m.mode = "station"
			case tea.KeyBackspace:
				m.timeBuf = trimLastRune(m.timeBuf)
			case tea.KeyRunes, tea.KeySpace:
				m.timeBuf += msg.String()
			}
		}
	}
	return m, nil
}

func fmtWhen(w *When) string {
	if w == nil {
		return "now"
	}
	s := ""
	if len(w.Date) == 8 {
		s = w.Date[:4] + "-" + w.Date[4:6] + "-" + w.Date[6:8]
	}
	if len(w.Time) == 4 {
		s += " " + w.Time[:2] + ":" + w.Time[2:4]
	}
	return s
}

func (m *wizardModel) View() string {
	var lines []string
	if m.mode == "station" {
		hint := ""
		if m.allowTime {
			hint = "[Tab] time: " + fmtWhen(m.when) + "  "
		}
		lines = append(lines, bold(m.label)+" "+m.buffer+"\u2588   "+dim(hint+"[\u2193\u2191] pick  [Enter] go  [Esc] quit"))
		if len(m.results) == 0 {
			lines = append(lines, dim("  …type to search"))
		}
		for i, s := range m.results {
			prefix := "  "
			if i == m.sel {
				prefix = bold("> ")
			}
			lines = append(lines, prefix+s.Name)
		}
	} else {
		extra := "(now)"
		if pw := parseWhen(m.timeBuf); pw != nil {
			extra = "\u2192 " + fmtWhen(pw)
		}
		lines = append(lines, bold("Time")+" "+m.timeBuf+"\u2588   "+dim(extra+"  [Enter] ok  [Tab] back"))
	}
	m.lines = len(lines)
	return strings.Join(lines, "\n")
}

func clearLines(n int) {
	if n > 0 {
		fmt.Printf("\x1b[%dA\x1b[J", n)
	}
}

// interactive trip wizard for bare "vvs to" / "vvs from": fuzzy-find the
// station while typing, Tab switches to time entry, Enter runs the query
func runWizard(direction string) *pickResult {
	if !isTTY(os.Stdin) {
		return nil
	}
	label := "To"
	if direction == "from" {
		label = "From"
	}
	m := &wizardModel{label: label, allowTime: true, mode: "station"}
	runPicker(m)
	return m.result
}

// non-wizard picker for inline "vvs to ?"
func pickStation() *Stop {
	if !isTTY(os.Stdin) {
		return nil
	}
	m := &wizardModel{label: "Pick station", mode: "station"}
	runPicker(m)
	if m.result == nil {
		return nil
	}
	return m.result.stop
}

func runPicker(m *wizardModel) {
	p := tea.NewProgram(m)
	_, err := p.Run()
	clearLines(m.lines)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if m.ctrlc {
		os.Exit(130)
	}
}

// first run: ask once (interactive only; scripts and --json keep the default)
func chooseHome() {
	fmt.Println(dim("Welcome — pick your home station (stored in " + configPath() + ")"))
	sc := bufio.NewScanner(os.Stdin)
	var chosen *Stop
	for chosen == nil {
		fmt.Print("Home station: ")
		if !sc.Scan() {
			fmt.Fprintln(os.Stderr, "\nno home station set — run `vvs home <station>`")
			os.Exit(1)
		}
		answer := strings.TrimSpace(sc.Text())
		if answer == "" {
			continue
		}
		stations := searchStations(answer)
		if len(stations) > 6 {
			stations = stations[:6]
		}
		if len(stations) == 0 {
			fmt.Println(red("  no match — try again"))
			continue
		}
		pick := 0
		if len(stations) > 1 {
			for i, s := range stations {
				fmt.Printf("  %d) %s\n", i+1, s.Name)
			}
			fmt.Print("Choose [1]: ")
			if sc.Scan() {
				if n, err := strconv.Atoi(strings.TrimSpace(sc.Text())); err == nil && n >= 1 && n <= len(stations) {
					pick = n - 1
				}
			}
		}
		chosen = &stations[pick]
	}
	saveHome(*chosen)
	fmt.Println(dim("saved: " + chosen.Name))
}

// ---- CLI dispatch -------------------------------------------------------------

func has(args []string, name string) bool { return indexOf(args, name) >= 0 }
func indexOf(args []string, name string) int {
	for i, a := range args {
		if a == name {
			return i
		}
	}
	return -1
}

var (
	kwRe         = regexp.MustCompile(`(?i)\s+(at|um|am|@|arriving|ankunft|arr)\s+(.+)$`)
	bareRe       = regexp.MustCompile(`(?i)\s+(\d{1,2}:\d{2})\s*$`)
	dayRe        = regexp.MustCompile(`(?i)\s+(tomorrow|morgen|heute|today|übermorgen|montag|dienstag|mittwoch|donnerstag|freitag|samstag|sonntag|monday|tuesday|wednesday|thursday|friday|saturday|sunday)\s*$`)
	departuresRe = regexp.MustCompile(`(?i)^departures\b`)
	routeWordRe  = regexp.MustCompile(`^(?:to|>)$`)
)

const usage = `vvs — Stuttgart region local transit (VVS) via the EFA-BW API

  vvs departures [station] [--limit N] [--all]   departures board, local lines only
  vvs to <station> [--at WHEN] [--arrive]        trips home → station
  vvs from <station> [--at WHEN]                 trips station → home
  vvs <from> to <to> [--at WHEN] [--arrive]      trips between two stations
  vvs search <term>                              list matching stations
  vvs to                                         interactive station picker
  vvs to ? / <from> to ?                         picker for a route endpoint
  vvs home [station]                             show or set the home station

  --at WHEN    date/time: "8:00", "morgen 7:45", "friday", "2026-10-01 08:00", "1.10."
  --arrive     arrive by WHEN instead of departing at it
  --all        include non-local lines (ICE/IC/…)
  --json       JSON output for scripts/UIs
  --limit N    max departures (default 10)

Station names resolve offline against the embedded VVS stop index; raw ids
("5006022", "de:08111:6022") work too. When-phrases work inline:
"vvs Feuerbach to Schlossplatz um morgen 8:00". Home config: ~/.config/vvs.json
`

func splitRoute(words []string) *[2]string {
	for i := 1; i < len(words)-1; i++ {
		if routeWordRe.MatchString(words[i]) {
			return &[2]string{strings.Join(words[:i], " "), strings.Join(words[i+1:], " ")}
		}
	}
	return nil
}

func printJSON(v any) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", " ")
	enc.Encode(v)
}

var version = "dev"

func main() {
	useColor = isTTY(os.Stdout)
	args := os.Args[1:]
	if has(args, "--version") || has(args, "-v") {
		fmt.Println("vvs " + version)
		return
	}
	if has(args, "--help") || has(args, "-h") {
		fmt.Print(usage)
		return
	}
	asJson := has(args, "--json")
	localOnly := !has(args, "--all")
	get := func(name string) string {
		if i := indexOf(args, name); i >= 0 && i+1 < len(args) {
			return args[i+1]
		}
		return ""
	}
	var at *When
	if has(args, "--at") {
		at = parseWhen(get("--at"))
	}
	// positional args = everything that is not a flag and not a flag's value
	optValues := map[int]bool{}
	for _, f := range []string{"--at", "--limit"} {
		if i := indexOf(args, f); i >= 0 {
			optValues[i+1] = true
		}
	}
	var rest []string
	for i, a := range args {
		if !strings.HasPrefix(a, "--") && !optValues[i] {
			rest = append(rest, a)
		}
	}

	// Positional words may carry a when-phrase: "to Schlossplatz at 19:00",
	// "Feuerbach to Schlossplatz um morgen 8:00", "… at friday". Peel it off
	// the end before dispatching; --at wins when both are given.
	joined := strings.Join(rest, " ")
	var when *When
	arrive := has(args, "--arrive")
	text := joined
	if m := kwRe.FindStringSubmatchIndex(joined); m != nil {
		keyword := joined[m[2]:m[3]]
		if parsed := parseWhen(joined[m[4]:m[5]]); parsed != nil {
			when = parsed
			if !regexp.MustCompile(`(?i)^(at|um|am|@)$`).MatchString(keyword) {
				arrive = true
			}
			text = joined[:m[0]]
		}
	}
	if when == nil {
		if m := bareRe.FindStringSubmatchIndex(joined); m != nil {
			when = parseWhen(joined[m[2]:m[3]])
			text = joined[:m[0]]
		}
	}
	if when == nil {
		if m := dayRe.FindStringSubmatchIndex(joined); m != nil {
			when = parseWhen(joined[m[2]:m[3]])
			text = joined[:m[0]]
		}
	}
	var date, timeStr string
	if at != nil {
		date, timeStr = at.Date, at.Time
	} else if when != nil {
		date, timeStr = when.Date, when.Time
	}
	limit := 10
	if n, err := strconv.Atoi(get("--limit")); err == nil && n > 0 {
		limit = n
	}
	words := strings.Fields(text)

	// dispatch on the joined text so multi-word station names survive
	var mSub []string
	if m := regexp.MustCompile(`(?i)^(departures)\s*(.*)$`).FindStringSubmatch(text); m != nil {
		mSub = m
	} else if m := regexp.MustCompile(`(?i)^(to|from)\s+(.+)$`).FindStringSubmatch(text); m != nil {
		mSub = m
	}
	route := splitRoute(words)
	print := func(raw any, fmtFn func() string) {
		if asJson {
			printJSON(raw)
		} else {
			fmt.Println(fmtFn())
		}
	}

	// ---- home station ---------------------------------------------------------
	if len(rest) > 0 && rest[0] == "search" && len(rest) > 1 {
		q := strings.Join(rest[1:], " ")
		stations := searchStations(q)
		if len(stations) > 10 {
			stations = stations[:10]
		}
		if len(stations) == 0 {
			fmt.Println("no matches")
		} else {
			for i, s := range stations {
				fmt.Printf("%d. %s  (%s)\n", i+1, label(s), s.ID)
			}
		}
		return
	}

	if len(rest) > 0 && rest[0] == "home" {
		if len(rest) > 1 {
			s := resolveStop(strings.Join(rest[1:], " "))
			if s == nil {
				fmt.Fprintln(os.Stderr, "unknown station: "+strings.Join(rest[1:], " "))
				os.Exit(1)
			}
			saveHome(Stop{Name: s.Name, ID: s.ID})
			fmt.Println("saved: " + s.Name)
		} else {
			h := homeStop()
			if h.ID == "" {
				fmt.Println("not set — run `vvs home <station>` (config: " + configPath() + ")")
			} else {
				fmt.Printf("%s (%s) — config: %s\n", h.Name, h.ID, configPath())
			}
		}
		return
	}
	usesHome := route == nil && !departuresRe.MatchString(text)
	if usesHome && obj(loadConfig()["home"]) == nil && isTTY(os.Stdin) && !asJson {
		chooseHome()
	}

	// bare "vvs to" / "vvs from": interactive fuzzy picker with time entry
	if route == nil && mSub == nil && len(words) == 1 && regexp.MustCompile(`(?i)^(to|from)$`).MatchString(words[0]) {
		dir := strings.ToLower(words[0])
		w := runWizard(dir)
		if w == nil {
			return
		}
		var date2, time2 string
		if w.when != nil {
			date2, time2 = w.when.Date, w.when.Time
		}
		raw := tripsQuery(dir, w.stop, date2, time2, localOnly, arrive)
		print(raw, func() string { return fmtTrips(raw) })
		return
	}
	var pickedFrom, pickedTo *Stop
	if route != nil && (route[0] == "?" || route[1] == "?") {
		if route[0] == "?" {
			pickedFrom = pickStation()
			if pickedFrom == nil {
				return
			}
		}
		if route[1] == "?" {
			pickedTo = pickStation()
			if pickedTo == nil {
				return
			}
		}
	}
	if mSub != nil && strings.TrimSpace(mSub[2]) == "?" {
		pickedFrom = pickStation()
		if pickedFrom == nil {
			return
		}
	}
	if route != nil {
		var raw TripsResult
		if pickedFrom != nil || pickedTo != nil {
			if pickedFrom == nil {
				pickedFrom = asStop(route[0])
			}
			if pickedTo == nil {
				pickedTo = asStop(route[1])
			}
			if pickedFrom == nil {
				raw = errTrips("unknown station: " + route[0])
			} else if pickedTo == nil {
				raw = errTrips("unknown station: " + route[1])
			} else {
				raw = runTrips(pickedFrom, pickedTo, date, timeStr, localOnly, arrive)
			}
		} else {
			raw = tripsRoute(*route, date, timeStr, localOnly, arrive)
		}
		print(raw, func() string { return fmtTrips(raw) })
	} else if mSub != nil && strings.EqualFold(mSub[1], "departures") {
		l := limit
		if localOnly {
			l = limit * 3
		}
		var q any = mSub[2]
		if pickedFrom != nil {
			q = pickedFrom
		}
		raw := departures(q, l, localOnly)
		print(raw, func() string { return fmtDepartures(raw) })
	} else if mSub != nil {
		var q any = mSub[2]
		if pickedFrom != nil {
			q = pickedFrom
		}
		raw := tripsQuery(strings.ToLower(mSub[1]), q, date, timeStr, localOnly, arrive)
		print(raw, func() string { return fmtTrips(raw) })
	} else if len(words) == 0 {
		l := limit
		if localOnly {
			l = limit * 3
		}
		raw := departures("", l, localOnly)
		print(raw, func() string { return fmtDepartures(raw) })
	} else {
		l := limit
		if localOnly {
			l = limit * 3
		}
		raw := departures(strings.Join(words, " "), l, localOnly)
		print(raw, func() string { return fmtDepartures(raw) })
	}
}
