(() => {
  "use strict";

  const $ = (s, r = document) => r.querySelector(s);
  const $$ = (s, r = document) => [...r.querySelectorAll(s)];
  const root = document.documentElement;
  root.classList.add("js");
  const motion = matchMedia("(prefers-reduced-motion: no-preference)").matches;
  const esc = (s) =>
    String(s).replace(/[&<>]/g, (c) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;" }[c]));
  const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

  /* ── Interactive Pixel Train Mark ─────────────────────────────────────── */
  const px = [
    [3, 0, 9, 1], // roof top
    [2, 1, 11, 1], // upper roof
    [1, 2, 2, 9], // left cab pillar
    [12, 2, 2, 9], // right cab pillar
    [5, 2, 5, 1], // destination display
    [7, 3, 1, 4], // center wiper
    [3, 7, 9, 1], // stripe under window
    [6, 8, 3, 1], // front chevron top
    [7, 9, 1, 2], // front chevron tip
    [3, 9, 2, 2], // left headlight
    [10, 9, 2, 2], // right headlight
    [1, 11, 13, 1], // bumper
    [6, 12, 3, 1], // coupler
    [2, 12, 2, 1], // left wheel arch
    [11, 12, 2, 1], // right wheel arch
    [2, 13, 2, 1], // left wheel
    [11, 13, 2, 1], // right wheel
    [0, 14, 15, 1], // track rail
  ];

  const cells = [];
  px.forEach(([x, y, w, h]) => {
    for (let i = 0; i < w; i++) {
      for (let j = 0; j < h; j++) {
        cells.push([x + i, y + j]);
      }
    }
  });
  cells.sort((a, b) => a[1] - b[1] || a[0] - b[0]);

  const markSvg = $("#marksvg");
  if (markSvg) {
    markSvg.innerHTML = cells
      .map(
        ([x, y], i) =>
          `<rect x="${x}" y="${y}" width="1" height="1" style="--i:${i}"/>`
      )
      .join("");
  }

  const boot = () => {
    const m = $("#mark");
    if (!m) return;
    m.classList.remove("boot");
    void m.offsetWidth;
    m.classList.add("boot");
  };

  /* Physics loot burst on clicking brand */
  let breaking = false;
  const smash = async () => {
    breaking = true;
    const m = $("#mark");
    if (m) m.classList.remove("boot");
    const rects = $$("#marksvg rect"),
      pile = {},
      anims = [];
    let landed = 0;
    const lean = Math.random() * 4 - 2,
      spread = 0.1 + Math.random() * 0.45,
      scatter = 1 + Math.random() * 2.5;
    const ok = (c) => c >= -3 && c <= 17,
      ht = (c) => pile[c] ?? -1,
      pick = (a) => a[Math.floor(Math.random() * a.length)];

    rects
      .map((r, i) => [r, ...cells[i]])
      .sort((a, b) => b[2] - a[2] || Math.random() - 0.5)
      .forEach(([r, x, y]) => {
        let col = Math.max(
          -3,
          Math.min(
            17,
            Math.round(x + (x - 7) * spread + lean + (Math.random() * 2 - 1) * scatter)
          )
        );
        for (let k = 0; k < 4; k++) {
          const n = [col - 1, col + 1].filter((c) => ok(c) && ht(c) < ht(col) - 1);
          if (!n.length) break;
          col = pick(n);
        }
        const h = (pile[col] = ht(col) + 1);
        const dx = col - x,
          fy = 14 - h - y,
          up = 1 + Math.random() * 2,
          spin = 90 * (Math.floor(Math.random() * 5) - 2);
        const delay = (14 - y) * 22 + Math.random() * 60,
          rise = 150,
          fall = 110 * Math.sqrt(fy + up),
          bounce = 90;
        const T = rise + fall + 2 * bounce,
          t = (a, b = 0) => `translate(${a}px, ${b}px)`;
        landed = Math.max(landed, delay + T);
        anims.push(
          r.animate(
            [
              { transform: t(0) + " rotate(0deg)", easing: "cubic-bezier(.2,.6,.4,1)" },
              {
                transform: t(dx * 0.25, -up) + ` rotate(${spin * 0.25}deg)`,
                offset: rise / T,
                easing: "cubic-bezier(.5,0,1,.6)",
              },
              {
                transform: t(dx, fy) + ` rotate(${spin}deg)`,
                offset: (rise + fall) / T,
                easing: "ease-out",
              },
              {
                transform: t(dx, fy - 0.7) + ` rotate(${spin}deg)`,
                offset: (rise + fall + bounce) / T,
                easing: "ease-in",
              },
              { transform: t(dx, fy) + ` rotate(${spin}deg)` },
            ],
            { duration: T, delay, fill: "forwards" }
          )
        );
      });

    const blink = rects.map((r) =>
      r.animate(
        [
          { opacity: 1 },
          { opacity: 1, offset: 0.5 },
          { opacity: 0, offset: 0.5 },
          { opacity: 0 },
        ],
        { duration: 110, delay: landed + 350, iterations: 9, fill: "forwards" }
      )
    );
    await Promise.all(blink.map((a) => a.finished)).catch(() => {});
    [...anims, ...blink].forEach((a) => a.cancel());
    boot();
    breaking = false;
  };

  const brandEl = $("#brand");
  if (brandEl) {
    brandEl.addEventListener("click", (e) => {
      e.preventDefault();
      if (scrollY > 0) scrollTo({ top: 0, behavior: motion ? "smooth" : "auto" });
      if (!breaking) (motion ? smash() : boot());
    });
  }

  /* ── Interactive Hero Terminal Tabs ───────────────────────────────────── */
  const terminalTabs = $$(".t-tab");
  const terminalBody = $("#term-body");

  const terminalModes = {
    dep: `
      <div class="t-prompt"><span class="p">~ ❯</span> vvs departures Schlossplatz --limit 5</div>
      <div style="color:var(--cyan);font-weight:600;margin-bottom:6px">Schlossplatz — departures · LIVE</div>
      <table class="deptable">
        <thead>
          <tr><th>TIME</th><th>LINE</th><th>DESTINATION</th><th>TRACK</th><th>STATUS</th></tr>
        </thead>
        <tbody>
          <tr><td class="tm">20:04</td><td><span class="tag-u">U15</span></td><td>Ruhbank (Fernsehturm)</td><td>Gleis 2</td><td class="ok">on time</td></tr>
          <tr><td class="tm">20:05</td><td><span class="tag-u">U6</span></td><td>Gerlingen</td><td>Gleis 1</td><td class="del">+7'</td></tr>
          <tr><td class="tm">20:06</td><td><span class="tag-u">U12</span></td><td>Dürrlewang</td><td>Gleis 1</td><td class="del">+4'</td></tr>
          <tr><td class="tm">20:06</td><td><span class="tag-u">U7</span></td><td>Mönchfeld</td><td>Gleis 2</td><td class="del">+4'</td></tr>
          <tr><td class="tm">20:06</td><td><span class="tag-bus">42</span></td><td>Erwin-Schoettle-Platz</td><td>Pos. 1</td><td class="ok">on time</td></tr>
        </tbody>
      </table>
    `,
    trip: `
      <div class="t-prompt"><span class="p">~ ❯</span> vvs to Feuerbach um morgen 8:00</div>
      <div style="color:var(--cyan);font-weight:600;margin-bottom:6px">Schlossplatz → Feuerbach · at 08:00</div>
      <div style="display:grid;gap:8px;margin-top:6px">
        <div style="background:var(--bg-2);border:1px solid var(--line);padding:8px 10px">
          <div style="display:flex;justify-content:space-between;align-items:baseline;margin-bottom:4px">
            <span style="font-weight:600;color:var(--ink)">08:06–08:16</span>
            <span style="color:var(--brand)">0:10 · 0× direct</span>
          </div>
          <div style="font-size:11px;color:var(--ink-2)"><span class="tag-u">U6</span> direct to Feuerbach Bahnhof</div>
        </div>
        <div style="background:var(--bg-2);border:1px solid var(--line);padding:8px 10px">
          <div style="display:flex;justify-content:space-between;align-items:baseline;margin-bottom:4px">
            <span style="font-weight:600;color:var(--ink)">08:01–08:14</span>
            <span style="color:var(--accent)">0:13 · 1× transfer</span>
          </div>
          <div style="font-size:11px;color:var(--ink-2)">walk 0m · <span class="tag-u">U15</span> to Pragsattel · <span class="tag-u">U6</span> to Feuerbach</div>
        </div>
      </div>
    `,
    wiz: `
      <div class="t-prompt"><span class="p">~ ❯</span> vvs to</div>
      <div style="background:var(--bg-2);border:1px solid var(--line);padding:10px 12px">
        <div style="margin-bottom:4px"><span style="color:var(--accent);font-weight:600">To </span>feuerb<span style="display:inline-block;width:7px;height:12px;background:var(--brand);margin-left:2px;vertical-align:middle;animation:blink 1s steps(1) infinite"></span></div>
        <div style="font-size:10px;color:var(--muted);margin-bottom:8px">[Tab] time: now · [↓↑] pick · [Enter] go · [Esc] quit</div>
        <div style="display:grid;gap:4px">
          <div style="background:color-mix(in srgb, var(--brand) 15%, var(--panel));padding:3px 6px;color:var(--brand);font-weight:600">&gt; Feuerbach <span style="float:right;color:var(--muted);font-weight:normal">de:08111:6157</span></div>
          <div style="padding:3px 6px;color:var(--ink-2)">&nbsp;&nbsp;Feuerbacher Weg <span style="float:right;color:var(--muted)">de:08111:2454</span></div>
          <div style="padding:3px 6px;color:var(--ink-2)">&nbsp;&nbsp;Feuerbach Bosch <span style="float:right;color:var(--muted)">de:08111:6423</span></div>
          <div style="padding:3px 6px;color:var(--ink-2)">&nbsp;&nbsp;Feuerbach Friedhof <span style="float:right;color:var(--muted)">de:08111:2422</span></div>
        </div>
      </div>
    `,
  };

  function setTerminalMode(modeKey) {
    terminalTabs.forEach((tab) =>
      tab.classList.toggle("active", tab.dataset.tab === modeKey)
    );
    if (terminalBody && terminalModes[modeKey]) {
      terminalBody.innerHTML = terminalModes[modeKey];
    }
  }

  terminalTabs.forEach((tab) => {
    tab.addEventListener("click", () => {
      setTerminalMode(tab.dataset.tab);
    });
  });

  /* ── Clipboard Copy ───────────────────────────────────────────────────── */
  $$("[data-copy]").forEach((b) =>
    b.addEventListener("click", async () => {
      const el = document.getElementById(b.dataset.copy);
      if (!el) return;
      try {
        await navigator.clipboard.writeText(el.textContent.trim());
        b.textContent = "Copied";
      } catch {
        const r = document.createRange();
        r.selectNodeContents(el);
        getSelection().removeAllRanges();
        getSelection().addRange(r);
        b.textContent = "Selected";
      }
      setTimeout(() => {
        b.textContent = "Copy";
      }, 1600);
    })
  );

  /* ── Peek Modal Dialog ────────────────────────────────────────────────── */
  const layer = $("#layer"),
    pop = $(".pop"),
    popbody = $("#popbody"),
    poptitle = $("#poptitle"),
    popcap = $("#popcap"),
    popclose = $("#popclose");
  let opener = null;

  function openPop(from, title, body, cap) {
    opener = from;
    if (poptitle) poptitle.textContent = title;
    if (popbody) popbody.innerHTML = body;
    if (popcap) popcap.innerHTML = cap;
    if (layer) layer.hidden = false;
    if (motion && pop && from) {
      const a = from.getBoundingClientRect(),
        b = pop.getBoundingClientRect();
      pop.animate(
        [
          {
            transform: `translate(${a.left - b.left}px, ${a.top - b.top}px) scale(${
              a.width / b.width
            }, ${a.height / b.height})`,
            opacity: 0.5,
          },
          { transform: "none", opacity: 1 },
        ],
        { duration: 340, easing: "cubic-bezier(.2,.8,.2,1)" }
      );
      layer.animate([{ opacity: 0 }, { opacity: 1 }], { duration: 200 });
    }
    if (popclose) popclose.focus();
  }

  function closePop() {
    if (layer) layer.hidden = true;
    if (popbody) popbody.innerHTML = "";
    if (opener) opener.focus();
  }

  if (popclose) popclose.addEventListener("click", closePop);
  if (layer) {
    layer.addEventListener("click", (e) => {
      if (e.target === layer) closePop();
    });
  }
  addEventListener("keydown", (e) => {
    if (e.key === "Escape" && layer && !layer.hidden) closePop();
  });

  const demoScreens = [
    {
      name: "vvs departures Schlossplatz --limit 8",
      body: `<pre class="pop-terminal"><span class="p">$</span> vvs departures Schlossplatz --limit 8
<span style="color:var(--cyan);font-weight:600">Schlossplatz — departures · LIVE</span>

20:04   <span style="color:var(--cyan)">U15</span>   Ruhbank (Fernsehturm)      Gleis 2   <span style="color:var(--brand)">on time</span>
20:05   <span style="color:var(--cyan)">U6</span>    Gerlingen                  Gleis 1   <span style="color:var(--yellow)">+7'</span>
20:06   <span style="color:var(--cyan)">U12</span>   Dürrlewang                 Gleis 1   <span style="color:var(--yellow)">+4'</span>
20:06   <span style="color:var(--cyan)">U7</span>    Mönchfeld                  Gleis 2   <span style="color:var(--yellow)">+4'</span>
20:06   <span style="color:var(--magenta)">42</span>    Erwin-Schoettle-Platz      Pos. 1    <span style="color:var(--brand)">on time</span>
20:08   <span style="color:var(--cyan)">U12</span>   Neckargröningen Remseck    Gleis 2   <span style="color:var(--yellow)">+4'</span>
20:11   <span style="color:var(--cyan)">U5</span>    Leinfelden Bahnhof         Gleis 1   <span style="color:var(--brand)">on time</span>
20:14   <span style="color:var(--cyan)">U15</span>   Stammheim                  Gleis 1   <span style="color:var(--brand)">on time</span></pre>`,
      cap: `<b>$ vvs departures Schlossplatz --limit 8</b> · Real-time delays, tracks &amp; bus bays directly from EFA-BW.`,
    },
    {
      name: "vvs to Feuerbach um morgen 8:00",
      body: `<pre class="pop-terminal"><span class="p">$</span> vvs to Feuerbach um morgen 8:00
<span style="color:var(--cyan);font-weight:600">Schlossplatz → Feuerbach · at 08:00</span>

08:06–08:16   <span style="color:var(--brand)">0:10</span>   0× direct
  <span style="color:var(--cyan)">U6</span> direct to Feuerbach Bahnhof

08:01–08:14   <span style="color:var(--accent)">0:13</span>   1× transfer
  walk 0 min · <span style="color:var(--cyan)">U15</span> to Pragsattel · <span style="color:var(--cyan)">U6</span> to Feuerbach Bahnhof

08:16–08:26   <span style="color:var(--brand)">0:10</span>   0× direct
  <span style="color:var(--cyan)">U6</span> direct to Feuerbach Bahnhof</pre>`,
      cap: `<b>$ vvs to Feuerbach um morgen 8:00</b> · Natural language German/English times parsed offline.`,
    },
    {
      name: "vvs to (Interactive Station & Time Wizard)",
      body: `<pre class="pop-terminal"><span class="p">$</span> vvs to
<span style="color:var(--accent);font-weight:600">To </span>feuerb▍
<span style="color:var(--muted)">[Tab] time: now  ·  [↓↑] pick  ·  [Enter] go  ·  [Esc] quit</span>

<span style="color:var(--brand);font-weight:bold">&gt; Feuerbach</span>                <span style="color:var(--muted)">de:08111:6157</span>
  Feuerbacher Weg          <span style="color:var(--muted)">de:08111:2454</span>
  Feuerbach Bosch          <span style="color:var(--muted)">de:08111:6423</span>
  Feuerbach Friedhof       <span style="color:var(--muted)">de:08111:2422</span>
  Sportpark Feuerbach      <span style="color:var(--muted)">de:08111:153</span></pre>`,
      cap: `<b>$ vvs to</b> · Interactive fuzzy stop selector matching 10,000+ stations offline.`,
    },
  ];

  $$("[data-peek]").forEach((t) =>
    t.addEventListener("click", () => {
      const idx = +t.dataset.peek;
      const d = demoScreens[idx] || demoScreens[0];
      openPop(t, `Terminal Output · ${d.name}`, d.body, d.cap);
    })
  );

  /* ── Versus Comparison Animation ───────────────────────────────────────── */
  const scBad = $("#sc-bad"),
    scGood = $("#sc-good"),
    steps = $$("#steps i");
  const steps5 = [
    {
      bad: "firefox https://vvs.de",
      good: "vvs departures Schlossplatz",
      badOut: "loading 34 scripts, 18 tracking pixels...",
      goodOut: "5 departures in 118ms",
      tally: { ram: 280, cookies: 14, latency: 1.8, clicks: 1 },
    },
    {
      bad: "click 'Cookie Settings' → 'Reject Non-Essential'",
      good: "vvs to Feuerbach um morgen 8:00",
      badOut: "vendor consent dialog re-prompting",
      goodOut: "direct route: 08:06–08:16 (10m)",
      tally: { ram: 45, cookies: 8, latency: 0.9, clicks: 3 },
    },
    {
      bad: "type 'feuerb' (waiting for network autocompletion)",
      good: "vvs search feuerb",
      badOut: "server roundtrip latency 450ms",
      goodOut: "10 matching stops resolved locally in 2ms",
      tally: { ram: 30, cookies: 4, latency: 0.5, clicks: 2 },
    },
    {
      bad: "scroll past dynamic advertising overlays",
      good: "vvs departures --json | jq .",
      badOut: "DOM reflow: full page re-render",
      goodOut: "piped directly to status bar",
      tally: { ram: 65, cookies: 6, latency: 0.4, clicks: 2 },
    },
    {
      bad: "result: 4 tabs left open, 450MB memory footprint",
      good: "process exited cleanly · 0 linger",
      badOut: "heavy browser process running in background",
      goodOut: "done in 12ms",
      tally: { ram: 30, cookies: 10, latency: 0.2, clicks: 0 },
    },
  ];

  const tallies = { bad: {}, good: {} };
  function setTally(el, kind, add) {
    if (!el) return;
    for (const [k, n] of Object.entries(add || {})) {
      const b = $(`[data-k="${k}"]`, el);
      if (!b) continue;
      if (kind === "bad") {
        tallies.bad[k] = (tallies.bad[k] || 0) + n;
        b.textContent =
          k === "ram"
            ? `${tallies.bad[k]} MB`
            : k === "latency"
            ? `${tallies.bad[k].toFixed(1)}s`
            : tallies.bad[k];
      } else {
        b.textContent = k === "ram" ? "12 MB" : k === "latency" ? "0.1s" : "0";
      }
      b.classList.remove("bump");
      void b.offsetWidth;
      b.classList.add("bump");
    }
  }

  const cmdLine = (c, out) =>
    `<span class="p">❯</span> ${esc(c)}${
      out ? `\n<span class="o">${esc(out)}</span>` : ""
    }`;

  let vsRun = 0;
  function vsReset() {
    for (const el of [scBad, scGood]) {
      if (!el) continue;
      el.className = el.className.replace(/\bs\d\b/g, "").trim();
      $$("[data-k]", el).forEach((b) => {
        b.textContent = b.dataset.k === "ram" ? "0 MB" : b.dataset.k === "latency" ? "0s" : "0";
      });
    }
    tallies.bad = {};
    steps.forEach((i) => i.classList.remove("on"));
    const cb = $("#cmd-bad"),
      cg = $("#cmd-good");
    if (cb) cb.innerHTML = "";
    if (cg) cg.innerHTML = "";
  }

  function vsStep(i) {
    const st = steps5[i],
      cls = "s" + (i + 1);
    if (scBad) scBad.classList.add(cls);
    if (scGood) scGood.classList.add(cls);
    const cb = $("#cmd-bad"),
      cg = $("#cmd-good");
    if (cb) cb.innerHTML = cmdLine(st.bad, st.badOut);
    if (cg) cg.innerHTML = cmdLine(st.good, st.goodOut);
    setTally(scBad, "bad", st.tally);
    setTally(scGood, "good", st.tally);
    if (steps[i]) steps[i].classList.add("on");
  }

  async function vsPlay() {
    const me = ++vsRun;
    vsReset();
    if (!motion) {
      steps5.forEach((_, i) => vsStep(i));
      return;
    }
    await sleep(600);
    for (let i = 0; i < steps5.length; i++) {
      if (me !== vsRun) return;
      vsStep(i);
      await sleep(2200);
    }
  }

  const replayBtn = $("#vsreplay");
  if (replayBtn) replayBtn.addEventListener("click", vsPlay);

  const compareSection = $("#compare");
  if (compareSection) {
    new IntersectionObserver(
      (es, o) => {
        if (es[0].isIntersecting) {
          o.disconnect();
          vsPlay();
        }
      },
      { threshold: 0.35 }
    ).observe(compareSection);
  }

  /* ── Section Glint & Reveal Animations ─────────────────────────────────── */
  const rvSel = ".sec .head, .versus > *, .sec .card, .search-widget, .cmd-card-grid";
  const rvEls = $$(rvSel).filter((el) => !el.closest(".hero"));
  rvEls.forEach((el) => {
    const sib = [...el.parentElement.children].filter((c) => rvEls.includes(c));
    el.style.setProperty("--d", Math.min(sib.indexOf(el), 5));
    el.classList.add("rv");
  });

  const rvIO = new IntersectionObserver(
    (es) =>
      es.forEach((e) => {
        if (e.isIntersecting) {
          e.target.classList.add("on");
          rvIO.unobserve(e.target);
        }
      }),
    { rootMargin: "0px 0px -8% 0px" }
  );
  rvEls.forEach((el) => rvIO.observe(el));

  const secIO = new IntersectionObserver(
    (es) =>
      es.forEach((e) => {
        if (e.isIntersecting) {
          e.target.classList.add("seen");
          secIO.unobserve(e.target);
        }
      }),
    { rootMargin: "-40% 0px -55% 0px" }
  );
  $$(".sec").forEach((s) => secIO.observe(s));

  /* ── Header Scroll Progress & Compression ─────────────────────────────── */
  const prog = $("#prog"),
    top = $(".top");
  let ticking = false,
    lastT = -1;
  const onScroll = () => {
    const h = document.documentElement.scrollHeight - innerHeight;
    if (prog) prog.style.setProperty("--p", h > 0 ? Math.min(1, scrollY / h) : 0);
    const t = Math.min(1, Math.max(0, scrollY / 120));
    if (top && t !== lastT) {
      top.style.setProperty("--t", t);
      top.classList.toggle("scrolled", t > 0.3);
      lastT = t;
    }
    ticking = false;
  };

  addEventListener(
    "scroll",
    () => {
      if (!ticking) {
        ticking = true;
        requestAnimationFrame(onScroll);
      }
    },
    { passive: true }
  );
  onScroll();

  /* ── Interactive Station Resolver Live Demo ────────────────────────────── */
  const STATIONS_SAMPLE = [
    {
      name: "Schlossplatz",
      place: "Stuttgart",
      id: "de:08111:6022",
      lines: "U5, U6, U7, U12, U15, 42, 44",
    },
    {
      name: "Hauptbahnhof (tief)",
      place: "Stuttgart",
      id: "de:08111:6001",
      lines: "S1, S2, S3, S4, S5, S6, S60",
    },
    {
      name: "Hauptbahnhof (oben)",
      place: "Stuttgart",
      id: "de:08111:6002",
      lines: "U5, U6, U7, U9, U12, U14, U29",
    },
    {
      name: "Charlottenplatz",
      place: "Stuttgart",
      id: "de:08111:6018",
      lines: "U1, U2, U4, U9, U14, 42, 43, 44",
    },
    {
      name: "Feuerbach",
      place: "Stuttgart",
      id: "de:08111:6157",
      lines: "S4, S5, S6, S60, U6, U13, U16",
    },
    {
      name: "Feuerbacher Weg",
      place: "Stuttgart",
      id: "de:08111:2454",
      lines: "43",
    },
    {
      name: "Feuerbach Bosch",
      place: "Stuttgart",
      id: "de:08111:6423",
      lines: "91",
    },
    {
      name: "Feuerbach Friedhof",
      place: "Stuttgart",
      id: "de:08111:2422",
      lines: "91",
    },
    {
      name: "Sportpark Feuerbach",
      place: "Stuttgart",
      id: "de:08111:153",
      lines: "U6, U13, 91",
    },
    {
      name: "Bad Cannstatt",
      place: "Stuttgart",
      id: "de:08111:6006",
      lines: "S1, S2, S3, U1, U2, U13, U19",
    },
    {
      name: "Flughafen/Messe",
      place: "Stuttgart",
      id: "de:08111:6118",
      lines: "S2, S3, U6",
    },
    {
      name: "Marienplatz",
      place: "Stuttgart",
      id: "de:08111:6015",
      lines: "U1, U14, U21, Zacke (Linie 10)",
    },
    {
      name: "Südheimer Platz",
      place: "Stuttgart",
      id: "de:08111:6011",
      lines: "U1, U14, Seilbahn (Linie 20)",
    },
    {
      name: "Vaihingen",
      place: "Stuttgart",
      id: "de:08111:6056",
      lines: "S1, S2, S3, U1, U3, U8, U12",
    },
    {
      name: "Universität",
      place: "Stuttgart",
      id: "de:08111:6073",
      lines: "S1, S2, S3, 82, 84, 91, 92",
    },
    {
      name: "Degerloch",
      place: "Stuttgart",
      id: "de:08111:6030",
      lines: "U5, U6, U8, U12, Zacke (10)",
    },
  ];

  function normalize(s) {
    return s
      .toLowerCase()
      .normalize("NFD")
      .replace(/[\u0300-\u036f]/g, "")
      .replace(/ß/g, "ss")
      .replace(/[^a-z0-9]/g, "");
  }

  const stationInput = $("#station-search-input");
  const stationResults = $("#station-search-results");
  const sampleChips = $$("[data-search-chip]");

  function renderResults(query) {
    if (!stationResults) return;
    const qNorm = normalize(query.trim());
    if (!qNorm) {
      stationResults.innerHTML =
        '<div class="station-empty">Type a station name or click one of the quick suggestions above.</div>';
      return;
    }

    const matches = STATIONS_SAMPLE.filter((s) => {
      return (
        normalize(s.name).includes(qNorm) ||
        normalize(s.place).includes(qNorm) ||
        s.id.includes(query.trim())
      );
    }).slice(0, 5);

    if (matches.length === 0) {
      stationResults.innerHTML = `<div class="station-empty">No quick matches for &ldquo;${esc(
        query
      )}&rdquo; in sample (full index embeds 10,000+ stops).</div>`;
      return;
    }

    stationResults.innerHTML = matches
      .map(
        (s, idx) => `
      <div class="station-row">
        <span class="station-idx">${idx + 1}.</span>
        <span class="station-name">${esc(s.name)}</span>
        <span class="station-place">${esc(s.place)}</span>
        <span class="station-id"><code>${esc(s.id)}</code></span>
        <span class="station-lines">${esc(s.lines)}</span>
      </div>
    `
      )
      .join("");
  }

  if (stationInput && stationResults) {
    stationInput.addEventListener("input", (e) => {
      renderResults(e.target.value);
    });

    sampleChips.forEach((chip) => {
      chip.addEventListener("click", () => {
        stationInput.value = chip.dataset.searchChip;
        renderResults(chip.dataset.searchChip);
        stationInput.focus();
      });
    });

    renderResults(stationInput.value || "feuerb");
  }

  setTerminalMode("dep");
})();
