# vvs

Stuttgart region local transit from the terminal — departures and door-to-door
trips (VVS/SSB) as a single static Go binary, querying the EFA-BW API directly.
No API key, no runtime dependencies.

```
go install github.com/parnoldx/vvs@latest
```

## Usage

```
vvs departures Schlossplatz            # departures board, local lines only
vvs departures Schlossplatz --all      # include ICE/IC/…
vvs to Feuerbach                       # trips home → Feuerbach
vvs from Feuerbach --at 17:30          # trips Feuerbach → home
vvs Feuerbach to Schlossplatz          # trips between two stations
vvs to Schlossplatz --arrive           # arrive by a given time instead
vvs search feuerb                      # list matching stations
vvs to                                 # interactive station picker
```

The first interactive run asks for your home station once and stores it in
`~/.config/vvs.json` (change later with `vvs home <station>`).

Time phrases work inline — German or English:

```
vvs to Schlossplatz um morgen 8:00
vvs to Schlossplatz --at friday
vvs to Schlossplatz --arrive --at 2026-10-01 08:00
```

Station names resolve offline against an embedded index of all VVS stops
(fuzzy, umlaut-insensitive); raw EFA ids (`5006022`, `de:08111:6022`) work
too. Output is plain text, `--json` gives machine-readable results for
scripts and UIs.

## Development

```
go test ./...    # offline checks against saved EFA responses in tests/fixtures
go build         # single binary
```

`tools/mkstations.py` regenerates the embedded stop index
(`data/stations.json`) from the official VVS Haltestellen registry.
