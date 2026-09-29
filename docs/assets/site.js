"use strict";

// Copy to Clipboard buttons with live status announcements
for (const button of document.querySelectorAll("[data-copy]")) {
  button.hidden = false;
  let timer;
  button.addEventListener("click", async () => {
    const feedback = document.getElementById(button.dataset.feedback);
    const code = document.getElementById(button.dataset.copy);
    if (!code) return;
    const text = code.textContent.trim().replace(/\s+/g, " ");

    try {
      await navigator.clipboard.writeText(text);
      button.textContent = "Copied";
      if (feedback) {
        feedback.textContent = "Command copied. Paste it into your terminal.";
      }
    } catch {
      const range = document.createRange();
      range.selectNodeContents(code);
      const selection = window.getSelection();
      selection.removeAllRanges();
      selection.addRange(range);
      if (feedback) {
        feedback.textContent = "Select and copy the command with your browser.";
      }
    }

    clearTimeout(timer);
    timer = setTimeout(() => {
      button.textContent = "Copy";
      if (feedback) {
        feedback.textContent = "";
      }
    }, 4000);
  });
}

// Interactive Desktop Demo Tabs (Keyboard accessible: Left, Right, Home, End)
const tabs = [...document.querySelectorAll("[data-demo-tab]")];

function selectTab(tab) {
  for (const item of tabs) {
    const selected = item === tab;
    item.setAttribute("aria-selected", String(selected));
    item.tabIndex = selected ? 0 : -1;
    const targetId = item.getAttribute("aria-controls");
    const targetPanel = document.getElementById(targetId);
    if (targetPanel) {
      targetPanel.hidden = !selected;
    }
  }
}

for (const tab of tabs) {
  tab.addEventListener("click", () => selectTab(tab));
  tab.addEventListener("keydown", (event) => {
    const index = tabs.indexOf(tab);
    let next;
    if (event.key === "ArrowRight") next = tabs[(index + 1) % tabs.length];
    if (event.key === "ArrowLeft") next = tabs[(index + tabs.length - 1) % tabs.length];
    if (event.key === "Home") next = tabs[0];
    if (event.key === "End") next = tabs[tabs.length - 1];
    if (next) {
      event.preventDefault();
      selectTab(next);
      next.focus();
    }
  });
}

const demoTabs = document.querySelector("[data-demo-tabs]");
if (demoTabs) {
  demoTabs.hidden = false;
}

// Interactive Station Resolver Live Demo
const stationInput = document.getElementById("station-search-input");
const stationResults = document.getElementById("station-search-results");
const sampleChips = document.querySelectorAll("[data-search-chip]");

const STATIONS_SAMPLE = [
  { name: "Schlossplatz", place: "Stuttgart", id: "de:08111:6022", lines: "U5, U6, U7, U12, U15, 42, 44" },
  { name: "Hauptbahnhof (tief)", place: "Stuttgart", id: "de:08111:6001", lines: "S1, S2, S3, S4, S5, S6, S60" },
  { name: "Hauptbahnhof (oben)", place: "Stuttgart", id: "de:08111:6002", lines: "U5, U6, U7, U9, U12, U14, U29" },
  { name: "Charlottenplatz", place: "Stuttgart", id: "de:08111:6018", lines: "U1, U2, U4, U9, U14, 42, 43, 44" },
  { name: "Feuerbach", place: "Stuttgart", id: "de:08111:6157", lines: "S4, S5, S6, S60, U6, U13, U16" },
  { name: "Feuerbacher Weg", place: "Stuttgart", id: "de:08111:2454", lines: "43" },
  { name: "Feuerbach Bosch", place: "Stuttgart", id: "de:08111:6423", lines: "91" },
  { name: "Feuerbach Friedhof", place: "Stuttgart", id: "de:08111:2422", lines: "91" },
  { name: "Sportpark Feuerbach", place: "Stuttgart", id: "de:08111:153", lines: "U6, U13, 91" },
  { name: "Feuerbach Pfostenwäldle", place: "Stuttgart", id: "de:08111:152", lines: "U6, U13, U16" },
  { name: "Bad Cannstatt", place: "Stuttgart", id: "de:08111:6006", lines: "S1, S2, S3, U1, U2, U13, U19" },
  { name: "Flughafen/Messe", place: "Stuttgart", id: "de:08111:6118", lines: "S2, S3, U6" },
  { name: "Marienplatz", place: "Stuttgart", id: "de:08111:6015", lines: "U1, U14, U21, Zacke (10)" },
  { name: "Südheimer Platz", place: "Stuttgart", id: "de:08111:6011", lines: "U1, U14, Seilbahn (20)" },
  { name: "Vaihingen", place: "Stuttgart", id: "de:08111:6056", lines: "S1, S2, S3, U1, U3, U8, U12" },
  { name: "Universität", place: "Stuttgart", id: "de:08111:6073", lines: "S1, S2, S3, 82, 84, 91, 92" },
  { name: "Degerloch", place: "Stuttgart", id: "de:08111:6030", lines: "U5, U6, U8, U12, Zacke (10)" },
  { name: "Stadtbibliothek", place: "Stuttgart", id: "de:08111:6040", lines: "U5, U6, U7, U12, U15" },
  { name: "Wilhelma", place: "Stuttgart", id: "de:08111:6008", lines: "U14" },
  { name: "Mineralbäder", place: "Stuttgart", id: "de:08111:6010", lines: "U1, U2, U14" }
];

function normalize(s) {
  return s
    .toLowerCase()
    .normalize("NFD")
    .replace(/[\u0300-\u036f]/g, "")
    .replace(/ß/g, "ss")
    .replace(/[^a-z0-9]/g, "");
}

function renderResults(query) {
  if (!stationResults) return;
  const qNorm = normalize(query.trim());
  if (!qNorm) {
    stationResults.innerHTML = '<div class="station-empty">Type a station name or click one of the quick suggestions above.</div>';
    return;
  }

  const matches = STATIONS_SAMPLE.filter((s) => {
    return normalize(s.name).includes(qNorm) || normalize(s.place).includes(qNorm) || s.id.includes(query.trim());
  }).slice(0, 6);

  if (matches.length === 0) {
    stationResults.innerHTML = `<div class="station-empty">No quick matches for &ldquo;${query}&rdquo; in demo sample (full index has 10,000+ stops).</div>`;
    return;
  }

  stationResults.innerHTML = matches
    .map(
      (s, idx) => `
    <div class="station-row">
      <span class="station-idx">${idx + 1}.</span>
      <span class="station-name">${s.name}</span>
      <span class="station-place">${s.place}</span>
      <span class="station-id"><code>${s.id}</code></span>
      <span class="station-lines">${s.lines}</span>
    </div>
  `
    )
    .join("");
}

if (stationInput && stationResults) {
  stationInput.addEventListener("input", (e) => {
    renderResults(e.target.value);
  });

  for (const chip of sampleChips) {
    chip.addEventListener("click", () => {
      stationInput.value = chip.dataset.searchChip;
      renderResults(chip.dataset.searchChip);
      stationInput.focus();
    });
  }

  renderResults(stationInput.value || "feuerb");
}
