/* ============================================================
   Malice SPA — vanilla JS, hash router, no framework.
   ============================================================ */
"use strict";

const $ = (sel, root = document) => root.querySelector(sel);
const $$ = (sel, root = document) => Array.from(root.querySelectorAll(sel));
const view = $("#view");

/* ---------- Inline SVG icons (Lucide-style, no emoji) ---------- */
const I = {
  shield: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z"/><path d="M9 12l2 2 4-4"/></svg>',
  shieldAlert: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z"/><path d="M12 8v4"/><path d="M12 16h.01"/></svg>',
  shieldCheck: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z"/><path d="M9 12l2 2 4-4"/></svg>',
  radar: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M19.07 4.93A10 10 0 0 0 6.99 3.34"/><path d="M4 6h.01"/><path d="M2.29 9.62A10 10 0 1 0 21.31 8.35"/><path d="M16.24 7.76A6 6 0 1 0 8.23 16.67"/><path d="M12 18h.01"/><path d="M17.99 11.66A6 6 0 0 1 15.77 16.67"/><circle cx="12" cy="12" r="2"/><path d="m13.41 10.59 5.66-5.66"/></svg>',
  file: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M15 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V7z"/><path d="M14 2v4a2 2 0 0 0 2 2h4"/></svg>',
  upload: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/><path d="M17 8l-5-5-5 5"/><path d="M12 3v12"/></svg>',
  back: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M19 12H5"/><path d="M12 19l-7-7 7-7"/></svg>',
  copy: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><rect x="9" y="9" width="13" height="13" rx="2"/><path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"/></svg>',
  check: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M20 6 9 17l-5-5"/></svg>',
  alert: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M10.29 3.86 1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z"/><path d="M12 9v4"/><path d="M12 17h.01"/></svg>',
  search: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><circle cx="11" cy="11" r="8"/><path d="m21 21-4.3-4.3"/></svg>',
  plus: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M5 12h14"/><path d="M12 5v14"/></svg>',
  cpu: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><rect x="4" y="4" width="16" height="16" rx="2"/><rect x="9" y="9" width="6" height="6"/><path d="M9 2v2M15 2v2M9 20v2M15 20v2M2 9h2M2 15h2M20 9h2M20 15h2"/></svg>',
};

/* ---------- API client ---------- */
const api = {
  async get(path) {
    const r = await fetch(path, { headers: { Accept: "application/json" } });
    if (!r.ok) throw new Error((await r.json().catch(() => ({}))).error || r.statusText);
    return r.json();
  },
  // XHR (not fetch) so we get upload.onprogress for the progress bar.
  upload(file, onProgress) {
    return new Promise((resolve, reject) => {
      const fd = new FormData();
      fd.append("file", file);
      const xhr = new XMLHttpRequest();
      xhr.open("POST", "/api/scans");
      xhr.upload.onprogress = (e) => {
        if (e.lengthComputable && onProgress) onProgress(e.loaded, e.total);
      };
      xhr.onload = () => {
        let j = {};
        try { j = JSON.parse(xhr.responseText); } catch (_) {}
        if (xhr.status >= 200 && xhr.status < 300) resolve(j);
        else reject(new Error(j.error || "HTTP " + xhr.status));
      };
      xhr.onerror = () => reject(new Error("network error"));
      xhr.send(fd);
    });
  },
};

/* ---------- State ---------- */
const state = {
  totalEngines: 17,
  active: new Map(), // sha256 -> { lastReported, stable, startedAt }
  pollTimer: null,
};

/* ---------- Helpers ---------- */
function esc(s) {
  return String(s ?? "").replace(/[&<>"']/g, (c) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" }[c]));
}
function fmtSize(n) {
  if (n == null) return "—";
  if (typeof n === "string") return n;
  if (n < 1024) return n + " B";
  if (n < 1024 * 1024) return (n / 1024).toFixed(2) + " kB";
  return (n / 1024 / 1024).toFixed(2) + " MB";
}
function fmtDate(iso) {
  if (!iso) return "—";
  const d = new Date(iso);
  if (isNaN(d)) return iso;
  return d.toLocaleString(undefined, { year: "2-digit", month: "short", day: "2-digit", hour: "2-digit", minute: "2-digit" });
}
function shortHash(h, n = 16) {
  if (!h) return "—";
  return h.length > n ? h.slice(0, n) + "…" : h;
}

// Engines where found=true is itself a positive hit (threat-intel / AV). nsrl
// and shadow-server are handled specially below because their found=true also
// fires on known-GOOD matches (NSRL catalog / shadow-server whitelist).
const FOUND_IS_THREAT = new Set(["kvrt", "lmd", "hashlookup"]);
function isDetection(name, res) {
  if (!res || typeof res !== "object") return false;
  if (res.infected === true) return true;
  if (["infected", "threat", "malicious"].includes(res.status)) return true;
  if ((res.matches && res.matches.length) || (res.detections && res.detections.length)) return true;
  if (name === "nsrl") return false; // NSRL hit = known-good NIST software
  if (name === "shadow-server") {
    const sb = res.sandbox || {};
    return (sb.antivirus && Object.keys(sb.antivirus).length) || (sb.metadata && Object.keys(sb.metadata).length);
  }
  if (name === "virustotal") return (res.positives || 0) > 0;
  return res.found === true && FOUND_IS_THREAT.has(name);
}
const INTEL_ENGINES = new Set(["hashlookup", "nsrl", "shadow-server", "virustotal"]);
function detectionLabel(name, res) {
  let d = null;
  if (name === "virustotal" && (res.positives || 0) > 0) {
    d = res.positives + " vendors detected" + (res.ratio ? " · " + res.ratio : "");
  } else if (Array.isArray(res.detections) && res.detections.length) {
    const x = res.detections[0];
    d = (typeof x === "object") ? (x.name || x.detection || x.info || x.result) : x;
  } else if (Array.isArray(res.matches) && res.matches.length) {
    const x = res.matches[0];
    d = (typeof x === "object") ? (x.name || x.rule || (x.meta && x.meta.description)) : x;
  } else if (Array.isArray(res.hits) && res.hits.length) {
    const x = res.hits[0];
    d = (typeof x === "object") ? (x.signature || x.name || x.rule) : x;
  } else if (res.name) d = res.name;
  else if (res.detection) d = res.detection;
  else if (typeof res.result === "string") d = res.result;
  else if (res.found === true && FOUND_IS_THREAT.has(name)) d = INTEL_ENGINES.has(name) ? ("Found in threat intel" + (res.source ? " · " + res.source : "")) : "Detected";
  else if (["infected", "threat", "malicious"].includes(res.status)) d = res.status;
  if (d == null) d = "flagged";
  if (typeof d === "object") d = JSON.stringify(d);
  // maldet reports hash-rule hits as "{SHA256}sig.name" — render as "sig.name (SHA256 match)"
  const m = String(d).match(/^\{([A-Za-z0-9]+)\}(.+)$/);
  if (m) d = m[2] + " (" + m[1] + " match)";
  d = String(d).replace(/\s+/g, " ").trim();
  if (d.length > 90) d = d.slice(0, 87) + "…";
  return d;
}

function toast(msg, isErr = false) {
  const t = $("#toast");
  t.innerHTML = (isErr ? I.alert : I.check) + "<span>" + esc(msg) + "</span>";
  t.className = "toast show" + (isErr ? " err" : "");
  t.hidden = false;
  clearTimeout(toast._t);
  toast._t = setTimeout(() => { t.className = "toast" + (isErr ? " err" : ""); }, 3200);
}

async function copyText(txt) {
  try { await navigator.clipboard.writeText(txt); toast("Copied to clipboard"); }
  catch { toast("Copy failed", true); }
}

/* ---------- Verdict ---------- */
function verdictOf(scan) {
  if (scan.verdict === "threat") return "threat";
  const sha = scan.file && scan.file.sha256;
  if (sha && state.active.has(sha) && !state.active.get(sha).stable) return "scanning";
  return "clean";
}
function verdictBadge(v) {
  if (v === "threat") return '<span class="badge badge-threat">' + I.shieldAlert + "Threat</span>";
  if (v === "scanning") return '<span class="badge badge-scan"><span class="spinner" style="width:11px;height:11px;border-width:1.5px;margin:0"></span>Scanning</span>';
  return '<span class="badge badge-clean">' + I.shieldCheck + "Clean</span>";
}

/* ---------- Health ---------- */
async function pollHealth() {
  try {
    const h = await api.get("/api/health");
    state.totalEngines = h.engines_enabled || h.engines || 17;
    const dot = $("#es-dot");
    dot.className = "dot " + (h.elasticsearch ? "ok" : "err");
    $("#es-label").textContent = h.elasticsearch ? "ES online" : "ES down";
    $("#engine-count").textContent = (h.engines_enabled ?? h.engines) + " engines";
    $("#footer-version").textContent = "v" + (h.version || "dev");
  } catch {
    $("#es-dot").className = "dot err";
    $("#es-label").textContent = "offline";
  }
}

/* ---------- Views ---------- */
function renderScans(data) {
  const rows = data.scans || [];
  let html = `
    <div class="page-head">
      <div>
        <h1 class="page-title">Scans</h1>
        <p class="page-sub">${data.total} file${data.total === 1 ? "" : "s"} analyzed across ${state.totalEngines} engines</p>
      </div>
    </div>
    <div class="dropzone" id="dropzone">
      <div class="dz-icon">${I.upload}</div>
      <div class="dz-title">Drop a file to scan, or <b style="color:var(--primary)">browse</b></div>
      <div class="dz-hint">Runs through all ${state.totalEngines} engines · AV, static analysis, intel &amp; document parsers</div>
      <input type="file" id="file-input">
    </div>
    <div class="table-wrap">`;
  if (rows.length === 0) {
    html += `<div class="empty"><div class="e-ico">${I.search}</div><div class="e-title">No scans yet</div><div class="e-sub">Upload a file above to run your first multi-engine scan.</div></div>`;
  } else {
    html += `<div class="scan-row head stagger"><div>File</div><div>SHA-256</div><div>Size</div><div>Scanned</div><div>Verdict</div></div>`;
    for (const s of rows) {
      const v = verdictOf(s);
      const f = s.file || {};
      const detCount = (s.detections || []).length;
      html += `
      <div class="scan-row" data-id="${esc(s.id || "")}" data-sha="${esc(f.sha256 || "")}">
        <div class="cell-name"><span class="file-ico">${I.file}</span><span class="fname" title="${esc(f.name)}">${esc(f.name || "unnamed")}</span></div>
        <div class="cell-hash" title="${esc(f.sha256)}">${esc(shortHash(f.sha256, 24))}</div>
        <div class="cell-size">${esc(fmtSize(f.size_human || f.size))}</div>
        <div class="cell-date">${esc(fmtDate(s.scan_date))}</div>
        <div>${verdictBadge(v)}<div class="engines-pill" style="margin-top:6px"><b>${detCount}</b>/${state.totalEngines} flagged</div></div>
      </div>`;
    }
  }
  html += `</div>`;
  view.innerHTML = html;
  wireScans();
}

function wireScans() {
  $$(".scan-row[data-id]").forEach((row) => {
    row.addEventListener("click", () => {
      const id = row.dataset.id;
      if (id) location.hash = "#/scans/" + id;
    });
  });
  wireDropzone();
}

function wireDropzone() {
  const dz = $("#dropzone");
  const input = $("#file-input");
  if (!dz || !input) return;
  dz.addEventListener("click", (e) => { if (e.target === dz || e.target.closest(".dz-title,.dz-hint,.dz-icon")) input.click(); });
  input.addEventListener("change", () => { if (input.files[0]) uploadFile(input.files[0]); });
  ["dragenter", "dragover"].forEach((ev) => dz.addEventListener(ev, (e) => { e.preventDefault(); dz.classList.add("drag"); }));
  ["dragleave", "drop"].forEach((ev) => dz.addEventListener(ev, (e) => { e.preventDefault(); dz.classList.remove("drag"); }));
  dz.addEventListener("drop", (e) => {
    const f = e.dataTransfer.files && e.dataTransfer.files[0];
    if (f) uploadFile(f);
  });
}

async function uploadFile(file) {
  showUploadProgress(file.name, file.size);
  const onScansList = () => {
    const h = location.hash || "#/scans";
    return h.startsWith("#/scans") && !h.includes("#/scans/");
  };
  try {
    const res = await api.upload(file, (loaded, total) => updateUploadProgress(loaded, total));
    if (res.sha256) {
      state.active.set(res.sha256, { lastReported: 0, stable: 0, startedAt: Date.now() });
    }
    ensurePolling();
    // refresh the list (also restores the dropzone after the progress bar)
    const data = await api.get("/api/scans?size=50");
    if (onScansList()) renderScans(data); else resetDropzone();
  } catch (e) {
    resetDropzone();
    toast("Upload failed: " + e.message, true);
  }
}

/* ---------- Upload progress ---------- */
function showUploadProgress(name, size) {
  const dz = $("#dropzone");
  if (!dz) return;
  dz.classList.add("uploading");
  dz.innerHTML = `
    <div class="dz-icon">${I.upload}</div>
    <div class="dz-title">Uploading <b class="up-name" style="color:var(--primary)">${esc(name)}</b></div>
    <div class="up-bar"><div class="up-fill"></div></div>
    <div class="dz-hint up-meta"><span class="up-pct">0%</span> · <span class="up-bytes">0 B</span> of ${esc(fmtSize(size))}</div>
    <input type="file" id="file-input">`;
}
function updateUploadProgress(loaded, total) {
  const dz = $("#dropzone");
  if (!dz) return;
  const pct = total ? Math.min(100, Math.round((loaded / total) * 100)) : 0;
  const fill = dz.querySelector(".up-fill");
  const pctEl = dz.querySelector(".up-pct");
  const bytesEl = dz.querySelector(".up-bytes");
  if (fill) fill.style.width = pct + "%";
  if (pctEl) pctEl.textContent = pct + "%";
  if (bytesEl) bytesEl.textContent = fmtSize(loaded);
}
function resetDropzone() {
  const dz = $("#dropzone");
  if (!dz) return;
  dz.classList.remove("uploading");
  dz.innerHTML = `
    <div class="dz-icon">${I.upload}</div>
    <div class="dz-title">Drop a file to scan, or <b style="color:var(--primary)">browse</b></div>
    <div class="dz-hint">Runs through all ${state.totalEngines} engines · AV, static analysis, intel &amp; document parsers</div>
    <input type="file" id="file-input">`;
  wireDropzone();
}

/* ---------- Scan detail ---------- */
async function renderScanDetail(id) {
  view.innerHTML = `<div class="empty"><div class="spinner"></div><div class="e-sub" style="margin-top:16px">Loading scan…</div></div>`;
  let data, engines;
  try {
    data = await api.get("/api/scans/" + id);
    engines = (await api.get("/api/plugins")).engines;
  } catch (e) {
    view.innerHTML = `<div class="empty"><div class="e-ico">${I.alert}</div><div class="e-title">Scan not found</div><div class="e-sub">${esc(e.message)}</div><a class="back-link" href="#/scans" style="margin-top:18px">${I.back}Back to scans</a></div>`;
    return;
  }
  const f = data.file || {};
  const v = verdictOf(data);
  const plugins = data.plugins || {};

  const detCount = (data.detections || []).length;
  let banner;
  if (v === "threat") {
    banner = `<div class="verdict verdict-threat"><div class="v-ico">${I.shieldAlert}</div><div><div class="v-title">Threat detected</div><div class="v-sub">${detCount} engine${detCount === 1 ? "" : "s"} flagged this file · ${data.engines_reported}/${state.totalEngines} engines reported</div></div></div>`;
  } else if (v === "scanning") {
    banner = `<div class="verdict verdict-scan"><div class="v-ico">${I.radar}</div><div><div class="v-title">Scanning in progress</div><div class="v-sub">${data.engines_reported}/${state.totalEngines} engines reported · this page refreshes automatically</div></div></div>`;
  } else {
    banner = `<div class="verdict verdict-clean"><div class="v-ico">${I.shieldCheck}</div><div><div class="v-title">No threats detected</div><div class="v-sub">All ${data.engines_reported} reporting engines returned clean · ${data.engines_reported}/${state.totalEngines} engines</div></div></div>`;
  }

  const hashes = [
    ["MD5", f.md5], ["SHA-1", f.sha1], ["SHA-256", f.sha256], ["SHA-512", f.sha512],
  ].filter(([, val]) => val).map(([k, val]) => `
    <div class="hash-row"><span class="hk">${k}</span><span class="hv">${esc(val)}</span>
    <button class="copy-btn" data-copy="${esc(val)}">${I.copy}copy</button></div>`).join("");

  // merge all engines with their per-scan status
  const cards = engines.map((eng) => {
    const catEngines = plugins[eng.category] || {};
    const res = catEngines[eng.name];
    let dot = "s-off", label = "Not run for this file type";
    let detail = "";
    if (res) {
      if (isDetection(eng.name, res)) {
        dot = "s-threat"; label = "Detection";
        detail = `<div class="e-detail" title="${esc(detectionLabel(eng.name, res))}">${esc(detectionLabel(eng.name, res))}</div>`;
      } else if (res.status === "skipped") {
        dot = "s-off"; label = "Skipped";
        if (res.reason) detail = `<div class="e-detail skip" title="${esc(res.reason)}">${esc(res.reason)}</div>`;
      } else if (res.status === "error") {
        dot = "s-err"; label = "Error";
        const msg = res.error || res.reason || "engine error";
        detail = `<div class="e-detail err" title="${esc(msg)}">${esc(msg)}</div>`;
      } else {
        dot = "s-clean"; label = "Clean";
      }
    }
    return `<div class="engine-card">
      <div class="e-top"><span class="e-name">${esc(eng.name)}</span><span class="e-cat" data-cat="${esc(eng.category)}">${esc(eng.category)}</span></div>
      <div class="e-desc">${esc(eng.description || "")}</div>
      <div class="e-status"><span class="s-dot ${dot}"></span>${esc(label)}</div>
      ${detail}
    </div>`;
  }).join("");

  view.innerHTML = `
    <a class="back-link" href="#/scans">${I.back}All scans</a>
    ${banner}
    <div class="card">
      <h3>File</h3>
      <div class="kv" style="margin-bottom:16px">
        <span class="k">Name</span><span class="v">${esc(f.name || "—")}</span>
        <span class="k">Size</span><span class="v">${esc(fmtSize(f.size_human || f.size))}</span>
        <span class="k">Path</span><span class="v mono">${esc(f.path || "—")}</span>
        <span class="k">Scanned</span><span class="v mono">${esc(fmtDate(data.scan_date))}</span>
      </div>
      <h3 style="margin-top:22px">Hashes</h3>
      ${hashes || '<div class="cell-size">No hashes recorded</div>'}
    </div>
    <div class="page-head" style="margin-bottom:16px"><h3 class="page-title" style="font-size:18px">Engine results</h3></div>
    <div class="engines-grid stagger">${cards}</div>`;

  $$(".copy-btn").forEach((b) => b.addEventListener("click", (e) => { e.stopPropagation(); copyText(b.dataset.copy); }));

  // auto-refresh while scanning
  const sha = f.sha256;
  if (v === "scanning" && sha) {
    state.active.set(sha, state.active.get(sha) || { lastReported: data.engines_reported, stable: 0, startedAt: Date.now() });
    ensurePolling();
  }
}

/* ---------- Engines view ---------- */
async function renderEngines() {
  view.innerHTML = `<div class="empty"><div class="spinner"></div></div>`;
  let data;
  try {
    data = await api.get("/api/plugins");
  } catch (e) {
    view.innerHTML = `<div class="empty"><div class="e-ico">${I.alert}</div><div class="e-title">Cannot load engines</div><div class="e-sub">${esc(e.message)}</div></div>`;
    return;
  }
  const engines = data.engines || [];
  const byCat = {};
  for (const e of engines) (byCat[e.category] = byCat[e.category] || []).push(e);
  const catOrder = ["av", "static", "exe", "document", "intel", "metadata", "other"];
  const cats = Object.keys(byCat).sort((a, b) => (catOrder.indexOf(a) - catOrder.indexOf(b)));

  let html = `
    <div class="page-head">
      <div><h1 class="page-title">Engines</h1>
      <p class="page-sub">${engines.length} detection engines · ${engines.filter((e) => e.enabled).length} enabled</p></div>
    </div>`;
  for (const cat of cats) {
    html += `<h3 style="margin:0 0 14px;font-size:12px;font-weight:600;text-transform:uppercase;letter-spacing:.06em;color:var(--ink-faint);display:flex;align-items:center;gap:8px">${esc(cat)} <span style="font-family:var(--font-mono);font-size:11px">${byCat[cat].length}</span></h3>`;
    html += `<div class="engines-grid stagger" style="margin-bottom:22px">` + byCat[cat].map((e) => `
      <div class="engine-card">
        <div class="e-top"><span class="e-name">${esc(e.name)}</span><span class="e-cat ${e.enabled ? "cat-on" : "cat-off"}">${e.enabled ? "enabled" : "disabled"}</span></div>
        <div class="e-desc">${esc(e.description || "")}</div>
        <div class="e-status"><span class="s-dot ${e.enabled ? "s-clean" : "s-off"}"></span>${e.enabled ? "Ready" : "Disabled"}<span style="margin-left:auto;font-family:var(--font-mono);font-size:11px;color:var(--ink-faint)">${esc(e.image || "")}</span></div>
      </div>`).join("") + `</div>`;
  }
  view.innerHTML = html;
}

/* ---------- Router ---------- */
function route() {
  const hash = location.hash || "#/scans";
  const parts = hash.replace(/^#\//, "").split("/").filter(Boolean);
  $$(".nav a").forEach((a) => a.classList.toggle("active", a.dataset.nav === (parts[0] || "scans")));

  if (parts[0] === "engines") { renderEngines(); return; }
  if (parts[0] === "scans" && parts[1]) { renderScanDetail(parts[1]); return; }
  // default: scans list
  api.get("/api/scans?size=50").then(renderScans).catch((e) => {
    view.innerHTML = `<div class="empty"><div class="e-ico">${I.alert}</div><div class="e-title">Cannot reach backend</div><div class="e-sub">${esc(e.message)}</div></div>`;
  });
}

/* ---------- Polling (for in-progress scans) ---------- */
function ensurePolling() {
  if (state.pollTimer) return;
  state.pollTimer = setInterval(async () => {
    if (state.active.size === 0) { clearInterval(state.pollTimer); state.pollTimer = null; return; }
    // mark stable/timeout. On timeout set 99 so the entry is removed below —
    // previously it set stable=1, which never reached the 99 the removal checks,
    // so a scan absent from the list (ES down at upload, failed write, deleted)
    // kept polling /api/scans every 2.5s forever.
    for (const [sha, a] of state.active) {
      if (Date.now() - a.startedAt > 150000) a.stable = 99;
    }
    let data;
    try { data = await api.get("/api/scans?size=100"); } catch { return; }
    const bySha = {};
    for (const s of data.scans || []) if (s.file && s.file.sha256) bySha[s.file.sha256] = s;
    let changed = false;
    for (const [sha, a] of state.active) {
      const s = bySha[sha];
      if (s && s.engines_reported !== a.lastReported) { a.lastReported = s.engines_reported; a.stable = 0; changed = true; }
      else if (s) { a.stable = (a.stable || 0) + 1; if (a.stable >= 3) a.stable = 99; }
      if (a.stable === 99) state.active.delete(sha);
    }
    // re-render if on scans list or a detail of an active scan
    const hash = location.hash || "";
    if (changed || hash.startsWith("#/scans")) {
      const parts = hash.replace(/^#\//, "").split("/").filter(Boolean);
      if (parts[0] === "scans" && parts[1]) {
        const id = parts[1];
        try { const d = await api.get("/api/scans/" + id); renderScanDetail(id); } catch {}
      } else if (parts[0] === "scans") {
        renderScans(data);
      }
    }
    if (state.active.size === 0 && state.pollTimer) { clearInterval(state.pollTimer); state.pollTimer = null; }
  }, 2500);
}

/* ---------- Init ---------- */
window.addEventListener("hashchange", route);
window.addEventListener("DOMContentLoaded", () => {
  pollHealth();
  setInterval(pollHealth, 15000);
  route();
});
