#!/usr/bin/env python3
"""Regenerate data/stations.json from the official VVS stop registry.

Source: opendata-oepnv.de, dataset "haltestellen-vvs" (VVS Haltestellen).
Fetch the details page, pick the newest vvs_haltestelle_jNN.csv link, convert
to a minimal JSON list of {name, place, id} where id is the DHID the EFA API
accepts directly ("de:08111:6169").

    curl -sS -L -o /tmp/h.csv <url> && python3 tools/mkstations.py /tmp/h.csv
"""
import csv, json, sys

src = sys.argv[1] if len(sys.argv) > 1 else "/tmp/h.csv"
raw = open(src, encoding="latin-1").read().splitlines()
rows = list(csv.reader(raw[1:], delimiter=";"))  # first line is a #-comment header
out, seen = [], set()
for r in rows:
    if len(r) < 7:
        continue
    name, gid, gemeinde = r[0].strip(), r[3].strip(), r[5].strip()
    if not gid.startswith("de:") or gid in seen:
        continue
    seen.add(gid)
    out.append({"name": name, "place": gemeinde, "id": gid})
json.dump(out, open("data/stations.json", "w"), ensure_ascii=False, separators=(",", ":"))
print(f"{len(out)} stops -> data/stations.json")
