// VVS data source — Stuttgart region local transit (VVS) via the EFA-BW API.
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

const baseURL = "https://www.efa-bw.de/nvbw"

// no personal default — home is chosen on first interactive run or via `vvs home <station>`

const limitTimetable = int64(2 * 1024 * 1024)

// "arriving by T" first looks this many minutes back, then widens
const arriveStep = 20

// motType: 1 S-Bahn, 3 Stadtbahn (U-Bahn), 5 Bus; 0/15/16 = DB long distance
var localMot = map[int]bool{1: true, 3: true, 4: true, 5: true, 6: true, 7: true, 8: true, 9: true}

func isLocalMot(mot int) bool { return localMot[mot] }

// VVS area (Einzugsgebiet), by AGS-style omc prefix: Stadt Stuttgart 8111,
// Böblingen 8115, Esslingen 8116, Ludwigsburg 8118, Rems-Murr 8119.
var vvsOmc = regexp.MustCompile(`^811[15689]`)

type Stop struct {
	Name  string `json:"name"`
	Place string `json:"place,omitempty"`
	ID    string `json:"id"`
}

// ---- HTTP -------------------------------------------------------------------

var client = &http.Client{Timeout: 20 * time.Second}

// the server is slow and rate-limits: one retry with a pause keeps lookups
// from dying on a single hiccup
func httpJSON(url string, limit int64) (map[string]any, error) {
	var lastErr error
	for try := 0; try < 2; try++ {
		if try > 0 {
			time.Sleep(2 * time.Second)
		}
		data, err := fetch(url, limit)
		if err != nil {
			lastErr = err
			continue
		}
		var v map[string]any
		if err := json.Unmarshal(data, &v); err != nil {
			lastErr = errors.New("invalid json")
			continue
		}
		return v, nil
	}
	return nil, lastErr
}

func fetch(url string, limit int64) ([]byte, error) {
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("http %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("response exceeds %d bytes", limit)
	}
	return data, nil
}

func departuresURL(id string, limit int) string {
	return fmt.Sprintf(baseURL+"/XML_DM_REQUEST?outputFormat=JSON&coord=EUR&nameInfoActive=1&mode=direct&type_dm=stop&ptOptionsActive=1&useRealtime=1&name_dm=%s&limit=%d", url.QueryEscape(id), limit)
}

func tripURL(fromID, toID, date, tm string) string {
	u := fmt.Sprintf(baseURL+"/XML_TRIP_REQUEST2?outputFormat=JSON&coord=EUR&type_origin=stop&name_origin=%s&type_destination=stop&name_destination=%s&ptOptionsActive=1&useRealtime=1", url.QueryEscape(fromID), url.QueryEscape(toID))
	if date != "" {
		u += "&itdDate=" + date
	}
	if tm != "" {
		u += "&itdTime=" + tm
	}
	return u
}

// ---- dynamic-JSON helpers (EFA shapes are irregular: object or array) -------

func obj(v any) map[string]any {
	m, _ := v.(map[string]any)
	return m
}

func str(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64)
	default:
		return ""
	}
}

func num(v any) int {
	switch x := v.(type) {
	case float64:
		return int(x)
	case string:
		n, _ := strconv.Atoi(strings.TrimSpace(x))
		return n
	default:
		return 0
	}
}

// [].concat semantics: array spreads, single object wraps, missing → nil
func oneOrMany(v any) []any {
	switch x := v.(type) {
	case []any:
		return x
	case map[string]any:
		return []any{x}
	default:
		return nil
	}
}

// ---- stopfinder -------------------------------------------------------------

// raw stop ids are accepted as queries ("vvs to 5006500", "de:08111:…")
func asStop(x string) *Stop {
	s := strings.TrimSpace(x)
	if regexp.MustCompile(`^(\d{6,}|de:.+)$`).MatchString(s) {
		return &Stop{Name: s, ID: s}
	}
	return resolveStop(s)
}

// already-resolved stops pass through untouched (JS: object with .id → itself)
func asStopQ(x any) *Stop {
	if s, ok := x.(*Stop); ok {
		return s
	}
	return asStop(x.(string))
}

// ---- departures -------------------------------------------------------------

type Departure struct {
	Time      string `json:"time"`
	RTTime    string `json:"rtTime"`
	Delay     *int   `json:"delay"`
	Line      string `json:"line"`
	Dir       string `json:"dir"`
	Mot       int    `json:"mot"`
	Local     bool   `json:"local"`
	Platform  string `json:"platform"`
	Cancelled bool   `json:"cancelled"`
}

func hm(t map[string]any) string {
	if t == nil {
		return ""
	}
	return pad2(num(t["hour"])) + ":" + pad2(num(t["minute"]))
}

func dayMin(t map[string]any) int64 {
	if t == nil {
		return 0
	}
	epoch := time.Date(num(t["year"]), time.Month(num(t["month"])), num(t["day"]), 0, 0, 0, 0, time.UTC)
	return epoch.Unix() / 60
}

// departures json -> []Departure
func parseDepartures(json map[string]any) []Departure {
	dl := json["departureList"]
	if dl == nil {
		return nil
	}
	var raw []any
	if m := obj(dl); m != nil {
		raw = oneOrMany(m["departure"])
	} else {
		raw = oneOrMany(dl)
	}
	out := make([]Departure, 0, len(raw))
	for _, item := range raw {
		d := obj(item)
		if d == nil {
			continue
		}
		l := obj(d["servingLine"])
		dt := obj(d["dateTime"])
		rt := obj(d["realDateTime"])
		var delay *int
		if rt != nil {
			dl := int(dayMin(rt)+int64(num(rt["hour"])*60+num(rt["minute"]))) -
				int(dayMin(dt)+int64(num(dt["hour"])*60+num(dt["minute"])))
			delay = &dl
		}
		status := str(d["realtimeTripStatus"])
		line := str(l["number"])
		if line == "" {
			line = str(l["name"])
		}
		platform := str(d["platformName"])
		if platform == "" {
			p := obj(d["platform"])
			platform = str(p["name"])
			if platform == "" {
				platform = str(p["text"])
			}
		}
		mot := num(l["motType"])
		out = append(out, Departure{
			Time:      hm(dt),
			RTTime:    hm(rt),
			Delay:     delay,
			Line:      line,
			Dir:       str(l["direction"]),
			Mot:       mot,
			Local:     isLocalMot(mot),
			Platform:  platform,
			Cancelled: strings.Contains(status, "CANCELLED") || (delay != nil && *delay < 0),
		})
	}
	// sort by effective time of day; wrap after midnight keeps the board order
	minOf := func(r Departure) int {
		s := r.RTTime
		if s == "" {
			s = r.Time
		}
		h, _ := strconv.Atoi(s[:2])
		m, _ := strconv.Atoi(s[3:5])
		return h*60 + m
	}
	if len(out) > 0 {
		firstMin := minOf(out[0])
		sort.SliceStable(out, func(i, j int) bool {
			key := func(m int) int {
				if m < firstMin-720 {
					return m + 1440
				}
				return m
			}
			return key(minOf(out[i])) < key(minOf(out[j]))
		})
	}
	return out
}

// ---- trips ------------------------------------------------------------------

type Leg struct {
	Line    string `json:"line"`
	Mot     *int   `json:"mot"`
	Walk    bool   `json:"walk"`
	Minutes *int   `json:"minutes"`
	From    string `json:"from"`
	To      string `json:"to"`
	Dep     string `json:"dep"`
	Arr     string `json:"arr"`
}

type Trip struct {
	Dep      string `json:"dep"`
	Arr      string `json:"arr"`
	Duration string `json:"duration"`
	Changes  int    `json:"changes"`
	Legs     []Leg  `json:"legs"`
}

// stamp times come as "1605"/"160518" (realtime, padded) or "758"/"75800"
// (scheduled, unpadded) — normalize both to "HH:MM"
func stampHM(s string) string {
	if s == "" {
		return ""
	}
	if len(s) >= 5 {
		s = strings.Repeat("0", 6-len(s)) + s
		s = s[:4]
	} else {
		s = strings.Repeat("0", 4-len(s)) + s
	}
	return s[:2] + ":" + s[2:4]
}

func durFormat(d string) string {
	// "00:15" = h:mm, can exceed 24h
	parts := strings.Split(d, ":")
	h := 0
	fmt.Sscan(parts[0], &h)
	m := "00"
	if len(parts) > 1 && parts[1] != "" {
		m = parts[1]
	}
	return fmt.Sprintf("%d:%s", h, m)
}

// trip json -> []Trip
func parseTrips(json map[string]any) []Trip {
	trips, _ := json["trips"].([]any)
	if trips == nil {
		return nil
	}
	out := make([]Trip, 0, len(trips))
	for _, item := range trips {
		t := obj(item)
		if t == nil {
			continue
		}
		legsAny, _ := t["legs"].([]any)
		trip := Trip{Duration: durFormat(str(t["duration"])), Changes: num(t["interchange"])}
		if n := len(legsAny); n > 0 {
			firstPts, _ := obj(legsAny[0])["points"].([]any)
			lastPts, _ := obj(legsAny[n-1])["points"].([]any)
			if len(firstPts) > 0 && len(lastPts) > 0 {
				trip.Dep = stampHM(pointStamp(obj(firstPts[0])))
				trip.Arr = stampHM(pointStamp(obj(lastPts[len(lastPts)-1])))
			}
		}
		for _, li := range legsAny {
			l := obj(li)
			if l == nil {
				continue
			}
			m := obj(l["mode"])
			points, _ := l["points"].([]any)
			if len(points) == 0 {
				continue
			}
			first := obj(points[0])
			last := obj(points[len(points)-1])
			leg := Leg{
				Line: str(m["number"]),
				From: str(first["nameWithPlace"]),
				To:   str(last["nameWithPlace"]),
				Dep:  stampHM(pointStamp(first)),
				Arr:  stampHM(pointStamp(last)),
			}
			if leg.From == "" {
				leg.From = str(first["name"])
			}
			if leg.To == "" {
				leg.To = str(last["name"])
			}
			var fp map[string]any
			if fpa, ok := l["footpath"].([]any); ok && len(fpa) > 0 {
				fp = obj(fpa[0])
			} else if fpm := obj(l["footpath"]); fpm != nil {
				fp = fpm
			}
			leg.Walk = fp != nil
			if fp != nil && str(fp["duration"]) != "" {
				min := num(fp["duration"])
				leg.Minutes = &min
			}
			if code := str(m["code"]); code != "" {
				mot := num(m["code"])
				leg.Mot = &mot
			}
			trip.Legs = append(trip.Legs, leg)
		}
		out = append(out, trip)
	}
	return out
}

func pointStamp(p map[string]any) string {
	st := obj(p["stamp"])
	if st == nil {
		return ""
	}
	if rt := str(st["rtTime"]); rt != "" {
		return rt
	}
	return str(st["time"])
}

// keep connections arriving no later than "HH:MM" (same-day assumption;
// midnight-crossing arrivals count as in time, which is what you want)
func filterArriveBy(list []Trip, target string) []Trip {
	mins := func(s string) int {
		h, _ := strconv.Atoi(s[0:2])
		m, _ := strconv.Atoi(s[3:5])
		return h*60 + m
	}
	t := mins(target)
	var out []Trip
	for _, tr := range list {
		if mins(tr.Arr) <= t {
			out = append(out, tr)
		}
	}
	return out
}
