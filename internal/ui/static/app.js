"use strict";

/* Pasla web interface. Plain JavaScript, no build step, no external
   requests: everything here is served by the local Pasla process.
   Texts go through t()/tn() from i18n.js; English is the source language. */

const { t, tn } = I18N;

const S = {
  me: null,
  peers: [],
  pairs: [],
  transfers: [],
  messages: [],
  phone: { active: false, links: [] },
  phones: [],
  selected: null,
  unread: {},
  dismissedPairs: new Set(),
  uploads: new Map(), // transfer id -> AbortController
  online: false,
  attention: 0,
};

const $ = (sel, root = document) => root.querySelector(sel);

/* ---------- tiny DOM helper ---------- */
function h(tag, attrs, ...kids) {
  const el = document.createElement(tag);
  for (const [k, v] of Object.entries(attrs || {})) {
    if (v == null || v === false) continue;
    if (k === "class") el.className = v;
    else if (k.startsWith("on")) el.addEventListener(k.slice(2), v);
    else if (k === "style") for (const [p, val] of Object.entries(v)) el.style.setProperty(p, val);
    else if (v === true) el.setAttribute(k, "");
    else el.setAttribute(k, v);
  }
  for (const kid of kids.flat()) {
    if (kid == null || kid === false) continue;
    el.append(kid instanceof Node ? kid : document.createTextNode(String(kid)));
  }
  return el;
}
function icon(name, cls) {
  const svg = document.createElementNS("http://www.w3.org/2000/svg", "svg");
  if (cls) svg.setAttribute("class", cls);
  svg.setAttribute("aria-hidden", "true");
  const use = document.createElementNS("http://www.w3.org/2000/svg", "use");
  use.setAttribute("href", "#i-" + name);
  svg.append(use);
  return svg;
}

/* ---------- formatting ---------- */
const bytes = I18N.bytes;
function duration(sec) {
  if (!isFinite(sec) || sec < 0) return "";
  if (sec < 60) return t("{n} s", { n: Math.max(1, Math.round(sec)) });
  if (sec < 3600) return t("{n} min", { n: Math.round(sec / 60) });
  return t("{h} h {m} min", { h: Math.floor(sec / 3600), m: Math.round((sec % 3600) / 60) });
}
function clock(d) { return I18N.time(new Date(d)); }
function hueOf(fp) { return parseInt(fp.slice(0, 6), 16) % 360; }
function tiltOf(fp) { return ((parseInt(fp.slice(6, 8), 16) % 7) - 3) + "deg"; }
function osIcon(os) {
  return { windows: "windows", linux: "linux", macos: "macos", android: "phone", ios: "phone", other: "phone" }[os] || "device";
}
function osLabel(os) { return { windows: "Windows", linux: "Linux", macos: "macOS" }[os] || t("Unknown system"); }
function shortFp(fp) { return fp.slice(0, 16).replace(/(.{4})/g, "$1 ").trim(); }

function stamp(peer, size, online) {
  const k = peer.hueKey || peer.fp;
  return h("span", { class: "stamp" + (size ? " " + size : ""), style: { "--hue": hueOf(k), "--tilt": tiltOf(k) } },
    h("span", { class: "stamp-face" }, icon(osIcon(peer.os))),
    online ? h("span", { class: "dot", title: t("Online") }) : null);
}

/* ---------- API ---------- */
async function api(method, path, body) {
  const opt = { method, headers: { "X-Pasla-Lang": I18N.lang() } };
  if (body !== undefined) {
    opt.headers["Content-Type"] = "application/json";
    opt.body = JSON.stringify(body);
  }
  const res = await fetch(path, opt);
  if (res.status === 401) { showLocked(); throw new Error(t("No session")); }
  let data = null;
  try { data = await res.json(); } catch { /* empty */ }
  if (!res.ok) throw new Error((data && data.error) || res.statusText);
  return data;
}

function toast(text, kind) {
  const el = h("div", { class: "toast" + (kind === "err" ? " err" : ""), role: kind === "err" ? "alert" : "status" },
    icon(kind === "err" ? "x" : "check"), h("span", null, text));
  $("#toasts").append(el);
  setTimeout(() => el.remove(), kind === "err" ? 6500 : 3500);
}

/* ---------- state sync ---------- */
let loadTimer = 0;
function scheduleLoad() {
  clearTimeout(loadTimer);
  loadTimer = setTimeout(load, 80);
}
async function load() {
  try {
    const st = await api("GET", "/api/state");
    Object.assign(S, { me: st.me, peers: st.peers, pairs: st.pairs, transfers: st.transfers, messages: st.messages, phone: st.phone, phones: st.phones });
    // The language lives in the device settings; follow it if another
    // window (or the first start) changed it.
    if (st.me && st.me.lang && st.me.lang !== I18N.lang()) applyLang(st.me.lang);
    renderAll();
  } catch (e) {
    console.warn(e);
  }
}

function applyLang(lang) {
  I18N.setLang(lang);
  I18N.applyDOM(document);
  stageKey = "";
  phoneStageKey = "";
  pairKey = "";
  for (const el of [...$("#offers").children]) el.remove();
}

let locked = false;
function showLocked() {
  if (locked) return;
  locked = true;
  document.body.replaceChildren(h("main", { class: "locked" },
    h("img", { src: "icon.svg", alt: "", width: "72", height: "72" }),
    h("h1", null, t("This window is not signed in")),
    h("p", null, t("For your security the Pasla interface only works in the window the program opens itself. Start Pasla again, or click its shortcut again, and the interface opens on its own."))));
}

function connect() {
  if (locked) return;
  const es = new EventSource("/api/events");
  es.onopen = () => { S.online = true; load(); renderScan(); };
  es.onerror = () => {
    S.online = false;
    renderScan();
    // An auth failure looks like a plain error to EventSource; ask the API.
    fetch("/api/state").then((r) => { if (r.status === 401) { es.close(); showLocked(); } }).catch(() => {});
  };
  es.onmessage = (m) => {
    let ev;
    try { ev = JSON.parse(m.data); } catch { return; }
    switch (ev.type) {
      case "transfer": upsertTransfers([ev.data]); break;
      case "progress": upsertTransfers(ev.data); break;
      case "message": onMessage(ev.data); break;
      case "paired": onPaired(ev.data); scheduleLoad(); break;
      case "attention": onAttention(ev.data); break;
      case "peer-renamed": toast(t("Paired device renamed: {old} → {name}", ev.data)); scheduleLoad(); break;
      default: scheduleLoad();
    }
  };
}

function upsertTransfers(list) {
  for (const tv of list) {
    const i = S.transfers.findIndex((x) => x.id === tv.id);
    if (i >= 0) S.transfers[i] = tv; else S.transfers.unshift(tv);
    if (["cancelled", "failed", "rejected"].includes(tv.state) && S.uploads.has(tv.id)) {
      S.uploads.get(tv.id).abort();
      S.uploads.delete(tv.id);
    }
  }
  renderOffers();
  renderTransfers();
}

function onMessage(m) {
  S.messages.push(m);
  if (m.dir === "in" && (S.selected !== m.peerFp || document.hidden)) {
    S.unread[m.peerFp] = (S.unread[m.peerFp] || 0) + 1;
    renderDevices();
  }
  renderThread();
}

function onPaired(ev) {
  if (ev.ok) toast(t("Paired with {name}. You can send files now.", { name: ev.name }));
  else if (ev.error) toast(`${ev.name}: ${ev.error}`, "err");
}

function onAttention(a) {
  const text = {
    pair: t("{name} wants to pair", { name: a.name }),
    phone: t("{name} wants to connect", { name: a.name }),
    offer: tn(a.count, "{name} wants to send {n} file", "{name} wants to send {n} files", { name: a.name }),
    message: t("{name} sent a text", { name: a.name }),
    done: a.dir === "in"
      ? tn(a.count, "Received {n} file from {name}", "Received {n} files from {name}", { name: a.name })
      : tn(a.count, "Sent {n} file to {name}", "Sent {n} files to {name}", { name: a.name }),
  }[a.kind];
  if (!text || !document.hidden) return;
  S.attention++;
  document.title = `(${S.attention}) Pasla`;
  try {
    if ("Notification" in window && Notification.permission === "granted") new Notification("Pasla", { body: text, icon: "icon.png" });
  } catch { /* notifications unavailable */ }
}
document.addEventListener("visibilitychange", () => {
  if (!document.hidden) {
    S.attention = 0;
    document.title = "Pasla";
    if (S.selected && S.unread[S.selected]) { delete S.unread[S.selected]; renderDevices(); }
  }
});

/* ---------- rendering ---------- */
// Approved phones appear next to computers as pseudo-devices.
function phonePeers() {
  return (S.phones || []).filter((ph) => ph.approved).map((ph) => ({
    fp: "phone:" + ph.id, hueKey: ph.id, id: ph.id, name: ph.name, os: ph.os, online: true, trusted: true,
    isPhone: true, outbox: ph.outbox || [],
  }));
}
function findPeer(fp) {
  return S.peers.find((x) => x.fp === fp) || phonePeers().find((x) => x.fp === fp) || null;
}

function renderAll() {
  if (S.selected && !findPeer(S.selected)) S.selected = null;
  renderMe();
  renderScan();
  renderDevices();
  renderStage();
  renderOffers();
  renderPairDialog();
  renderPhoneDialog();
}

function renderMe() {
  if (!S.me) return;
  $("#me-name").textContent = S.me.name;
  $("#me-addr").textContent = (S.me.addrs || []).join(", ");
}

function renderScan() {
  const el = $("#scan-indicator");
  const on = S.online && S.me && S.me.discovery;
  el.classList.toggle("off", !on);
  el.title = !S.online ? t("Cannot reach Pasla") : on ? t("Listening on the network") : t("Automatic discovery is off");
}

// A device that is not paired but carries the name of a paired one may be
// trying to pass itself off as it.
function twinOf(p) {
  if (p.trusted || !p.name) return null;
  const n = p.name.toLocaleLowerCase();
  return S.peers.find((x) => x.trusted && x.fp !== p.fp && (x.name || "").toLocaleLowerCase() === n) || null;
}

function peerStatus(p) {
  if (!p.online) return t("Offline");
  if (p.trusted) return t("Paired");
  return t("Not paired");
}

function renderDevices() {
  const list = $("#device-list");
  list.replaceChildren();
  const phones = phonePeers();
  if (!S.peers.length && !phones.length) {
    list.append(h("li", { class: "list-empty" },
      h("strong", null, t("Nobody here yet")),
      t("Open Pasla on your other computers. They show up here within a few seconds.")));
    return;
  }
  for (const p of S.peers) {
    const unread = S.unread[p.fp];
    list.append(h("li", null,
      h("button", {
        class: "device" + (p.online ? "" : " offline"), type: "button",
        "aria-current": S.selected === p.fp ? "true" : "false",
        onclick: () => select(p.fp),
      },
        stamp(p, "", p.online),
        h("span", { class: "device-text" },
          h("span", { class: "device-name" }, p.name || t("Unnamed device")),
          h("span", { class: "device-sub" },
            p.trusted ? icon("lock", "lock") : null,
            twinOf(p) ? h("span", { class: "twin" }, t("Same name as another device!")) : h("span", null, peerStatus(p) + (p.addr ? ", " + p.addr : "")))),
        unread ? h("span", { class: "badge", "aria-label": tn(unread, "{n} unread", "{n} unread") }, unread) : null)));
  }
  if (phones.length) list.append(h("li", { class: "list-head" }, t("Phones")));
  for (const p of phones) {
    const unread = S.unread[p.fp];
    list.append(h("li", null,
      h("button", { class: "device", type: "button", "aria-current": S.selected === p.fp ? "true" : "false", onclick: () => select(p.fp) },
        stamp(p, "", true),
        h("span", { class: "device-text" },
          h("span", { class: "device-name" }, p.name),
          h("span", { class: "device-sub" }, icon("lock", "lock"), h("span", null, t("Connected from the browser")))),
        unread ? h("span", { class: "badge", "aria-label": tn(unread, "{n} unread", "{n} unread") }, unread) : null)));
  }
}

function select(fp) {
  S.selected = fp;
  delete S.unread[fp];
  try { localStorage.setItem("pasla.selected", fp || ""); } catch { /* storage unavailable */ }
  stageKey = "";
  renderDevices();
  renderStage();
}

let stageKey = "";
function renderStage() {
  const stage = $("#stage");
  const app = $("#app");
  const p = findPeer(S.selected);
  app.classList.toggle("has-peer", !!p);
  if (!p) {
    if (stageKey !== "empty") {
      stageKey = "empty";
      const tpl = $("#t-empty").content.cloneNode(true);
      I18N.applyDOM(tpl);
      stage.replaceChildren(h("div", { class: "stage-inner" }, tpl));
    }
    return;
  }
  if (p.isPhone) { renderPhoneStage(p); return; }
  const key = [p.fp, p.trusted, p.online, p.name, p.autoAccept, p.addr].join("|");
  if (key === stageKey) { renderTransfers(); renderThread(); return; }
  const draft = $("#composer-text")?.value || "";
  stageKey = key;

  const twin = twinOf(p);
  const meta = h("div", { class: "peer-meta" },
    p.trusted ? h("span", { class: "chip ok" }, icon("lock"), t("Paired")) : h("span", { class: "chip warn" }, t("Not paired")),
    twin ? h("span", { class: "chip danger" }, t("Possible impersonation")) : null,
    p.online ? null : h("span", { class: "chip off" }, t("Offline")),
    h("span", null, osLabel(p.os) + (p.addr ? ", " + p.addr : "")));

  const head = h("div", { class: "peer-head" },
    h("button", { class: "icon-btn back", type: "button", "aria-label": t("Back to the device list"), onclick: () => select(null) }, icon("back")),
    stamp(p, "lg", p.online),
    h("div", { class: "peer-title" }, h("h1", null, p.name || t("Unnamed device")), meta));

  const body = [];
  if (!p.trusted) {
    if (twin) body.push(h("section", { class: "card alert", role: "alert" },
      h("h2", null, t("You already have a paired device called “{name}”", { name: twin.name })),
      h("p", null, t("This device uses the same name but has a different identity. That is normal if the computer was reinstalled or Pasla's settings folder was deleted. Otherwise someone may be trying to pose as that device: ask the person at that computer before pairing."))));
    body.push(h("section", { class: "card invite" },
      h("div", null,
        h("h2", null, t("Pair once to get started")),
        h("p", null, t("Pairing makes sure your files only ever go to this exact device. The connection is always encrypted.")),
        h("ol", null,
          h("li", null, t("Press Pair.")),
          h("li", null, t("The same 6-digit code appears on both screens.")),
          h("li", null, t("If the codes match, confirm on both devices.")))),
      h("button", { class: "btn big", type: "button", disabled: !p.online, onclick: () => startPair(p) }, icon("lock"), t("Pair"))));
    if (p.manual) body.push(manualRemoveRow(p));
  } else {
    body.push(dropZone(p));
    body.push(h("section", { class: "section" },
      h("div", { class: "section-head" }, h("h2", null, t("Text and links"))),
      h("div", { class: "thread", id: "thread" }),
      composer(p, draft)));
    body.push(h("section", { class: "section" },
      h("div", { class: "section-head" }, h("h2", null, t("Transfers")),
        h("button", { class: "linkish", type: "button", onclick: clearFinished }, t("Clear finished"))),
      h("div", { class: "transfers", id: "peer-transfers" })));
    body.push(h("section", { class: "section" },
      h("div", { class: "section-head" }, h("h2", null, t("About this device"))),
      h("div", { class: "card", style: { padding: "4px 20px 18px" } },
        h("label", { class: "switch-row" },
          h("span", null, t("Accept files from this device without asking"),
            h("small", null, t("When on, what this device sends goes straight into your received folder."))),
          h("span", { class: "switch" },
            h("input", { type: "checkbox", checked: p.autoAccept, onchange: (e) => setAutoAccept(p, e.target.checked) }), h("i"))),
        h("div", { class: "safety" },
          h("span", { class: "field-label" }, t("Safety number")),
          h("span", { class: "safety-num" }, p.safety || ""),
          h("small", null, t("The other device shows the same number. Whenever you want peace of mind, compare the two screens; if they differ, remove the pairing."))),
        h("div", { class: "menu-row" },
          h("span", { class: "fp", title: t("Identity fingerprint") }, shortFp(p.fp)),
          h("button", { class: "linkish danger", type: "button", onclick: () => unpair(p) }, t("Remove pairing")),
          p.manual ? manualRemoveButton(p) : null))));
  }
  stage.replaceChildren(h("div", { class: "stage-inner" }, head, ...body));
  renderTransfers();
  renderThread();
}

function manualAddrOf(p) {
  return (S.me?.manualPeers || []).find((a) => a === p.addr || a.startsWith(p.addr + ":"));
}
function manualRemoveButton(p) {
  const addr = manualAddrOf(p);
  if (!addr) return null;
  return h("button", { class: "linkish", type: "button", onclick: async () => {
    try { await api("DELETE", "/api/manual-peers", { addr }); toast(t("{addr} removed from manually added addresses", { addr })); load(); }
    catch (e) { toast(e.message, "err"); }
  } }, t("Remove from manual addresses"));
}
function manualRemoveRow(p) {
  const b = manualRemoveButton(p);
  return b ? h("div", { class: "menu-row" }, b) : null;
}

function dropZone(p) {
  const fileInput = h("input", { type: "file", multiple: true, hidden: true });
  const dirInput = h("input", { type: "file", hidden: true, webkitdirectory: true });
  fileInput.addEventListener("change", () => {
    sendItems(p, [...fileInput.files].map((f) => ({ file: f, rel: f.name })));
    fileInput.value = "";
  });
  dirInput.addEventListener("change", () => {
    sendItems(p, [...dirInput.files].map((f) => ({ file: f, rel: f.webkitRelativePath || f.name })));
    dirInput.value = "";
  });
  const zone = h("section", { class: "drop" + (p.online ? "" : " disabled") },
    h("span", { class: "drop-icon" }, icon("plane")),
    h("h2", null, p.online ? t("Send to {name}", { name: p.name }) : t("{name} is offline right now", { name: p.name })),
    h("p", null, p.online ? t("Drop files or folders here, or paste them (Ctrl+V).") : t("You can send again once Pasla is open on that device.")),
    h("div", { class: "drop-actions" },
      h("button", { class: "btn", type: "button", disabled: !p.online, onclick: () => fileInput.click() }, icon("file"), t("Choose files")),
      h("button", { class: "btn ghost", type: "button", disabled: !p.online, onclick: () => dirInput.click() }, icon("folder"), t("Choose a folder"))),
    h("span", { class: "drop-note" }, t("Keep this tab open while sending.")),
    fileInput, dirInput);
  let depth = 0;
  zone.addEventListener("dragenter", (e) => { e.preventDefault(); depth++; if (p.online) zone.classList.add("over"); });
  zone.addEventListener("dragleave", () => { if (--depth <= 0) { depth = 0; zone.classList.remove("over"); } });
  zone.addEventListener("dragover", (e) => { e.preventDefault(); e.dataTransfer.dropEffect = p.online ? "copy" : "none"; });
  zone.addEventListener("drop", async (e) => {
    e.preventDefault();
    depth = 0;
    zone.classList.remove("over");
    if (!p.online) return;
    const items = await itemsFromDrop(e.dataTransfer);
    sendItems(p, items);
  });
  return zone;
}

function composer(p, draft) {
  const ta = h("textarea", { id: "composer-text", rows: "1", placeholder: t("Type a text, note or link…"), "aria-label": t("Text to send") });
  ta.value = draft;
  const grow = () => { ta.style.height = "auto"; ta.style.height = Math.min(ta.scrollHeight, 200) + "px"; };
  const send = async () => {
    const text = ta.value;
    if (!text.trim()) return;
    btn.disabled = true;
    try {
      await api("POST", p.isPhone ? `/api/phones/${p.id}/message` : `/api/peers/${p.fp}/message`, { text });
      ta.value = "";
      grow();
    } catch (e) {
      toast(e.message, "err");
    } finally {
      btn.disabled = false;
      ta.focus();
    }
  };
  ta.addEventListener("input", grow);
  ta.addEventListener("keydown", (e) => {
    if (e.key === "Enter" && !e.shiftKey && !e.isComposing) { e.preventDefault(); send(); }
  });
  const btn = h("button", { class: "icon-btn solid", type: "button", "aria-label": t("Send text"), disabled: !p.online, onclick: send }, icon("send"));
  requestAnimationFrame(grow);
  return h("div", { class: "composer" }, ta, btn);
}

function renderThread() {
  const box = $("#thread");
  if (!box || !S.selected) return;
  const msgs = S.messages.filter((m) => m.peerFp === S.selected).slice(-60);
  const key = msgs.length + "|" + S.selected + "|" + I18N.lang();
  if (box.dataset.key === key) return;
  box.dataset.key = key;
  box.replaceChildren(...msgs.map((m) => h("div", { class: "bubble " + m.dir },
    m.text,
    h("div", { class: "bubble-foot" },
      h("span", null, clock(m.at)),
      h("button", { type: "button", onclick: (e) => copyText(m.text, e.currentTarget) }, icon("copy"), t("Copy"))))));
  if (!msgs.length) box.append(h("p", { class: "empty-line" }, t("Type a password, link or note; it appears on the other screen right away.")));
}

async function copyText(text, btn) {
  try {
    await navigator.clipboard.writeText(text);
  } catch {
    const ta = h("textarea", null);
    ta.value = text;
    document.body.append(ta);
    ta.select();
    document.execCommand("copy");
    ta.remove();
  }
  const old = btn.lastChild.textContent;
  btn.lastChild.textContent = t("Copied");
  setTimeout(() => (btn.lastChild.textContent = old), 1500);
}

function transferRow(x) {
  const out = x.dir === "out";
  const first = x.names.join(", ");
  const extra = x.count > x.names.length ? " (" + tn(x.count, "{n} file", "{n} files") + ")" : "";
  const pct = x.total > 0 ? (x.done / x.total) * 100 : x.state === "done" ? 100 : 0;
  let sub = "", err = false;
  const why = x.error ? ` (${x.error})` : "";
  switch (x.state) {
    case "pending": sub = out ? t("Waiting for the other side to accept") : t("Waiting for your answer"); break;
    case "active": {
      const left = x.speed > 0 ? duration((x.total - x.done) / x.speed) : "";
      sub = `${bytes(x.done)} / ${bytes(x.total)}` + (x.speed > 0 ? `, ${bytes(x.speed)}/s` : "") + (left ? ", " + t("{time} left", { time: left }) : "");
      break;
    }
    case "done":
      sub = (out ? tn(x.count, "Sent {n} file", "Sent {n} files") : tn(x.count, "Received {n} file", "Received {n} files"))
        + `, ${bytes(x.total)}, ${clock(x.finished)}`;
      break;
    case "rejected": sub = t("Declined") + why; err = true; break;
    case "cancelled": sub = t("Cancelled") + why; err = true; break;
    case "failed": sub = t("Could not finish: {why}", { why: x.error || t("unknown error") }); err = true; break;
  }
  const live = x.state === "active" || x.state === "pending";
  return h("article", { class: "card tr", "data-id": x.id },
    h("span", { class: "tr-dir " + x.dir, title: out ? t("Outgoing") : t("Incoming") }, icon(out ? "up" : "down")),
    h("div", { class: "tr-main" },
      h("div", { class: "tr-title" }, first + extra),
      live ? h("div", { class: "bar" + (x.state === "pending" ? " wait" : ""), role: "progressbar", "aria-valuenow": Math.round(pct), "aria-valuemin": 0, "aria-valuemax": 100 },
        h("i", { style: { width: pct.toFixed(1) + "%" } })) : null,
      h("div", { class: "tr-sub" + (err ? " err" : "") }, sub)),
    h("div", { class: "tr-actions" },
      live ? h("button", { class: "icon-btn small", type: "button", "aria-label": t("Cancel"), title: t("Cancel"), onclick: () => cancelTransfer(x) }, icon("x")) : null,
      x.canReveal ? h("button", { class: "btn small quiet", type: "button", onclick: () => api("POST", `/api/transfers/${x.id}/reveal`).catch((e) => toast(e.message, "err")) }, icon("folder"), t("Show")) : null));
}

function renderTransfers() {
  const box = $("#peer-transfers");
  if (!box) return;
  const list = S.transfers.filter((x) => x.peerFp === S.selected).slice(0, 40);
  if (!list.length) {
    box.replaceChildren(h("p", { class: "empty-line" }, S.selected?.startsWith("phone:") ? t("Nothing has come from this phone yet.") : t("Nothing shared with this device yet.")));
    return;
  }
  box.replaceChildren(...list.map(transferRow));
}

function renderOffers() {
  const box = $("#offers");
  const pending = S.transfers.filter((x) => x.dir === "in" && x.state === "pending");
  const ids = new Set(pending.map((x) => x.id));
  for (const el of [...box.children]) if (!el.dataset.phone && !ids.has(el.dataset.id)) el.remove();
  for (const x of pending) {
    if (box.querySelector(`[data-id="${x.id}"]`)) continue;
    const peer = S.peers.find((p) => p.fp === x.peerFp) || { fp: x.peerFp, os: "" };
    const more = x.count - x.names.length;
    box.append(h("section", { class: "card offer", "data-id": x.id, role: "alertdialog", "aria-label": t("{name} wants to send files", { name: x.peerName }) },
      h("div", { class: "offer-head" }, stamp(peer),
        h("div", null, h("h3", null, t("{name} is sending files", { name: x.peerName })),
          h("div", { class: "offer-sub" }, tn(x.count, "{n} file, {size} in total", "{n} files, {size} in total", { size: bytes(x.total) })))),
      x.risky ? h("p", { class: "offer-warn" }, icon("shield"),
        x.risky === x.count
          ? t("These files can run code (programs, scripts or installers). Decline if you were not expecting them.")
          : tn(x.risky, "{n} file can run code (a program, script or installer). Decline if you were not expecting it.",
            "{n} files can run code (programs, scripts or installers). Decline if you were not expecting them.")) : null,
      h("ul", { class: "offer-files" },
        ...x.names.map((n) => h("li", null, icon(x.count > 1 && !n.includes(".") ? "folder" : "file"), h("span", null, n))),
        more > 0 && x.names.length < x.count ? h("li", { class: "more" }, tn(x.count, "{n} file including folder contents", "{n} files including folder contents")) : null),
      h("div", { class: "offer-actions" },
        h("button", { class: "btn ghost small", type: "button", onclick: () => decideOffer(x, false) }, t("Decline")),
        h("button", { class: "btn small", type: "button", onclick: () => decideOffer(x, true) }, icon("check"), t("Accept")))));
  }
}

/* ---------- pairing dialog ---------- */
let pairKey = "";
function renderPairDialog() {
  const dlg = $("#pair-dialog");
  const live = S.pairs.filter((p) => !S.dismissedPairs.has(p.id + p.state));
  const s = live.find((p) => p.state === "code") || live[0];
  if (!s) {
    pairKey = "";
    if (dlg.open) dlg.close();
    return;
  }
  const key = s.id + s.state + I18N.lang();
  if (key === pairKey && dlg.open) return;
  pairKey = key;
  const peer = { fp: s.peerFp, os: s.peerOs };
  const meStamp = S.me ? stamp({ fp: S.me.fp, os: S.me.os }) : null;
  const digits = h("span", { class: "code-face", "aria-label": t("Code {digits}", { digits: s.code.split("").join(" ") }) },
    s.code.slice(0, 3), h("span", { class: "gap" }), s.code.slice(3));
  let content;
  if (s.state === "code") {
    const incoming = s.role === "in";
    content = [
      h("h2", null, incoming ? t("{name} wants to pair", { name: s.peerName }) : t("Compare the codes")),
      h("p", null, incoming
        ? t("{name} should be showing this same code. If it does, confirm.", { name: s.peerName })
        : t("The same code just appeared on {name}. If they match, confirm on both devices.", { name: s.peerName })),
      h("div", { class: "code-stamp" }, digits),
      h("div", { class: "pair-who" }, meStamp, h("span", null, t("with")), stamp(peer), h("span", null, s.peerName)),
      (() => {
        const tw = twinOf({ fp: s.peerFp, name: s.peerName, trusted: false });
        return tw ? h("p", { class: "offer-warn" }, icon("shield"), t("Careful: you are already paired with a device called “{name}”; this request comes from a different identity.", { name: tw.name })) : null;
      })(),
      h("p", { class: "pair-hint" }, t("If the codes differ, decline: another device may be trying to get in between.")),
      h("div", { class: "sheet-actions" },
        h("button", { class: "btn ghost", type: "button", onclick: () => decidePair(s, false) }, incoming ? t("Decline") : t("Cancel")),
        h("button", { class: "btn", type: "button", onclick: () => decidePair(s, true), autofocus: true }, icon("check"), t("Codes match, pair"))),
    ];
  } else {
    content = [
      h("h2", null, t("Almost done")),
      h("p", null, t("Now confirm “Codes match” on {name} too.", { name: s.peerName })),
      h("div", { class: "code-stamp" }, digits),
      h("div", { class: "waiting" }, h("span", { class: "spinner" }), t("Waiting for {name}…", { name: s.peerName })),
      h("div", { class: "sheet-actions" },
        h("button", { class: "btn ghost", type: "button", onclick: () => { S.dismissedPairs.add(s.id + s.state); renderPairDialog(); } }, t("Wait in the background"))),
    ];
  }
  dlg.replaceChildren(h("div", { class: "sheet-body" }, ...content));
  if (!dlg.open) dlg.showModal();
}
$("#pair-dialog").addEventListener("cancel", (e) => e.preventDefault());

async function startPair(p) {
  try {
    const v = await api("POST", `/api/peers/${p.fp}/pair`);
    if (!S.pairs.some((x) => x.id === v.id)) S.pairs.push(v);
    renderPairDialog();
  } catch (e) {
    toast(e.message, "err");
  }
}
async function decidePair(s, ok) {
  try {
    await api("POST", `/api/pairs/${s.id}/${ok ? "accept" : "reject"}`);
  } catch (e) {
    toast(e.message, "err");
  }
  scheduleLoad();
}

/* ---------- actions ---------- */
async function decideOffer(x, ok) {
  try {
    await api("POST", `/api/transfers/${x.id}/${ok ? "accept" : "reject"}`);
    if (ok) select(x.peerFp);
  } catch (e) {
    toast(e.message, "err");
  }
}
async function cancelTransfer(x) {
  S.uploads.get(x.id)?.abort();
  try { await api("POST", `/api/transfers/${x.id}/cancel`); } catch (e) { toast(e.message, "err"); }
}
async function clearFinished() {
  try { await api("DELETE", "/api/transfers"); } catch (e) { toast(e.message, "err"); }
}
async function setAutoAccept(p, on) {
  try {
    await api("PUT", `/api/peers/${p.fp}/auto-accept`, { on });
    toast(on ? t("Files from {name} will now be accepted without asking", { name: p.name }) : t("You will be asked before accepting files from {name}", { name: p.name }));
  } catch (e) { toast(e.message, "err"); }
}
async function unpair(p) {
  if (!confirm(t("Remove the pairing with {name}? To send files again you will need to pair again.", { name: p.name }))) return;
  try { await api("DELETE", `/api/peers/${p.fp}/trust`); toast(t("Pairing with {name} removed", { name: p.name })); }
  catch (e) { toast(e.message, "err"); }
}

/* ---------- sending files from the browser ---------- */
async function itemsFromDrop(dt) {
  const entries = [...(dt.items || [])].map((it) => (it.webkitGetAsEntry ? it.webkitGetAsEntry() : null)).filter(Boolean);
  if (!entries.length) return [...dt.files].map((f) => ({ file: f, rel: f.name }));
  const out = [];
  const fileOf = (entry) => new Promise((res, rej) => entry.file(res, rej));
  const readAll = async (reader) => {
    const all = [];
    for (;;) {
      const batch = await new Promise((res, rej) => reader.readEntries(res, rej));
      if (!batch.length) return all;
      all.push(...batch);
    }
  };
  async function walk(entry, prefix) {
    if (entry.isFile) {
      out.push({ file: await fileOf(entry), rel: prefix + entry.name });
    } else if (entry.isDirectory) {
      for (const child of await readAll(entry.createReader())) await walk(child, prefix + entry.name + "/");
    }
  }
  for (const e of entries) await walk(e, "");
  return out;
}

async function sendItems(peer, items) {
  if (!items.length) return;
  const files = items.map((i) => ({ name: i.rel, size: i.file.size }));
  let tv;
  try {
    tv = await api("POST", `/api/peers/${peer.fp}/offer`, { files });
  } catch (e) {
    toast(e.message, "err");
    return;
  }
  if (tv.state !== "active") {
    if (tv.state === "rejected") toast(t("{name} did not accept the files", { name: peer.name }) + (tv.error ? ` (${tv.error})` : ""), "err");
    else if (tv.state === "failed") toast(tv.error || t("Could not start sending"), "err");
    return;
  }
  const ctrl = new AbortController();
  S.uploads.set(tv.id, ctrl);
  // Small files first so the count climbs quickly; three at a time keeps the link busy.
  const order = items.map((it, i) => i).sort((a, b) => items[a].file.size - items[b].file.size);
  let next = 0, failed = null;
  const worker = async () => {
    while (next < order.length && !failed && !ctrl.signal.aborted) {
      const i = order[next++];
      try {
        const res = await fetch(`/api/transfers/${tv.id}/files/${i}`, {
          method: "PUT", body: items[i].file, signal: ctrl.signal,
          headers: { "Content-Type": "application/octet-stream", "X-Pasla-Lang": I18N.lang() },
        });
        if (!res.ok) {
          const data = await res.json().catch(() => ({}));
          failed = data.error || res.statusText;
        }
      } catch (e) {
        if (!ctrl.signal.aborted) failed = e.message;
      }
    }
  };
  await Promise.all([worker(), worker(), worker()]);
  S.uploads.delete(tv.id);
  if (failed && !ctrl.signal.aborted) {
    const x = S.transfers.find((y) => y.id === tv.id);
    if (!x || x.state !== "cancelled") toast(failed, "err");
  }
}

window.addEventListener("beforeunload", (e) => {
  if (S.uploads.size) { e.preventDefault(); e.returnValue = ""; }
});

// Pasting files (screenshots, copied files) sends them to the open device.
document.addEventListener("paste", (e) => {
  const p = findPeer(S.selected);
  if (!p || !p.trusted || !p.online) return;
  const files = [...(e.clipboardData?.files || [])];
  if (!files.length) return;
  e.preventDefault();
  if (p.isPhone) { sendToPhone(p, files); return; }
  const stampName = new Date().toISOString().slice(0, 19).replace(/[T:]/g, "-");
  sendItems(p, files.map((f, i) => ({ file: f, rel: f.name && f.name !== "image.png" ? f.name : `pasted-${stampName}${i ? "-" + i : ""}.png` })));
});

/* ---------- settings ---------- */
function openSettings() {
  const me = S.me;
  if (!me) return;
  const dlg = $("#settings-dialog");
  const name = h("input", { id: "set-name", value: me.name, maxlength: "64", required: true });
  const dir = h("input", { id: "set-dir", value: me.receiveDir, spellcheck: "false" });
  const auto = h("input", { type: "checkbox", checked: me.autoAccept });
  const keep = h("input", { type: "checkbox", checked: me.keepRunning });
  const lang = h("select", { id: "set-lang" },
    ...I18N.languages.map((l) => h("option", { value: l.code, selected: l.code === I18N.lang() }, l.name)));
  const manual = (me.manualPeers || []).length
    ? h("ul", { class: "manual-list" }, ...me.manualPeers.map((a) => h("li", null, a,
      h("button", { class: "linkish danger", type: "button", onclick: async (e) => {
        try { await api("DELETE", "/api/manual-peers", { addr: a }); e.target.closest("li").remove(); load(); }
        catch (err) { toast(err.message, "err"); }
      } }, t("Remove")))))
    : h("small", null, t("None. You can add devices that automatic discovery cannot reach by IP, in the box on the left."));
  const notifyBtn = "Notification" in window && Notification.permission === "default"
    ? h("button", { class: "btn ghost small", type: "button", onclick: async (e) => { await Notification.requestPermission(); e.target.remove(); } }, t("Allow desktop notifications"))
    : null;
  const save = async (e) => {
    e.preventDefault();
    try {
      const changedLang = lang.value !== I18N.lang();
      S.me = await api("PUT", "/api/settings", { name: name.value, receiveDir: dir.value, autoAccept: auto.checked, keepRunning: keep.checked, lang: lang.value });
      if (changedLang) applyLang(lang.value);
      dlg.close();
      renderAll();
      toast(t("Settings saved"));
    } catch (err) {
      toast(err.message, "err");
    }
  };
  dlg.replaceChildren(h("form", { class: "sheet-body", onsubmit: save },
    h("h2", null, t("This device")),
    h("p", null, t("Other computers see this device under this name.")),
    h("div", { class: "field" }, h("label", { for: "set-name" }, t("Device name")), name),
    h("div", { class: "field" }, h("label", { for: "set-lang" }, t("Language")), lang),
    h("div", { class: "field" }, h("label", { for: "set-dir" }, t("Received files folder")), dir,
      h("small", null, t("Incoming files are saved here. Nothing is overwritten: if a name is taken, “(2)” is added."))),
    h("label", { class: "switch-row" },
      h("span", null, t("Accept files from all paired devices without asking"),
        h("small", null, t("When off, you are asked before every transfer."))),
      h("span", { class: "switch" }, auto, h("i"))),
    h("label", { class: "switch-row" },
      h("span", null, t("Keep running after I close the window"),
        h("small", null, t(me.os === "windows"
          ? "So other devices can send you files at any time. Pasla stays next to the clock; right-click its icon to quit. When off, closing the window quits Pasla."
          : "So other devices can send you files at any time. When off, closing the window quits Pasla."))),
      h("span", { class: "switch" }, keep, h("i"))),
    h("div", { class: "field" }, h("span", { class: "field-label" }, t("Manually added addresses")), manual),
    h("div", { class: "field" }, h("span", { class: "field-label" }, t("Identity fingerprint")),
      h("div", { class: "fp" }, me.fp.replace(/(.{4})/g, "$1 ").trim()),
      h("small", null, t("This device's identity. Pairing trusts this identity, so it keeps working when the IP address changes."))),
    h("dl", { class: "facts" },
      h("dt", null, t("IP addresses")), h("dd", null, (me.addrs || []).join(", ") || t("None found")),
      h("dt", null, t("Transfer port")), h("dd", null, `TCP ${me.port}`),
      h("dt", null, t("Automatic discovery")), h("dd", null, me.discovery ? t("On") : t("Off")),
      h("dt", null, t("Version")), h("dd", null, me.version)),
    h("div", { class: "sheet-actions" },
      h("button", { class: "btn ghost danger-text push-left", type: "button", onclick: quitPasla }, t("Quit Pasla")),
      notifyBtn,
      h("button", { class: "btn ghost", type: "button", onclick: () => dlg.close() }, t("Close")),
      h("button", { class: "btn", type: "submit" }, t("Save")))));
  dlg.showModal();
}

async function quitPasla() {
  const busy = S.transfers.some((x) => x.state === "active") || S.uploads.size > 0;
  if (busy && !confirm(t("Transfers are still running. Quit Pasla anyway?"))) return;
  try { await api("POST", "/api/quit"); } catch { /* it may already be gone */ }
  locked = true;
  document.body.replaceChildren(h("main", { class: "locked" },
    h("img", { src: "icon.svg", alt: "", width: "72", height: "72" }),
    h("h1", null, t("Pasla is closed")),
    h("p", null, t("You can close this tab. To use Pasla again, start it the same way you did before."))));
}

/* ---------- phones ---------- */
let phoneStageKey = "";
function renderPhoneStage(p) {
  const key = "phone|" + p.id + "|" + JSON.stringify(p.outbox) + "|" + p.name + "|" + I18N.lang();
  if (key === phoneStageKey && stageKey === "phone") { renderTransfers(); renderThread(); return; }
  const draft = $("#composer-text")?.value || "";
  phoneStageKey = key;
  stageKey = "phone";

  const input = h("input", { type: "file", multiple: true, hidden: true });
  input.addEventListener("change", () => { sendToPhone(p, [...input.files]); input.value = ""; });
  const zone = h("section", { class: "drop" },
    h("span", { class: "drop-icon" }, icon("phone")),
    h("h2", null, t("Send to the phone")),
    h("p", null, t("Drop files here. They appear in the phone's “From the computer” list, ready to download.")),
    h("div", { class: "drop-actions" },
      h("button", { class: "btn", type: "button", onclick: () => input.click() }, icon("file"), t("Choose files"))),
    input);
  let depth = 0;
  zone.addEventListener("dragenter", (e) => { e.preventDefault(); depth++; zone.classList.add("over"); });
  zone.addEventListener("dragleave", () => { if (--depth <= 0) { depth = 0; zone.classList.remove("over"); } });
  zone.addEventListener("dragover", (e) => { e.preventDefault(); e.dataTransfer.dropEffect = "copy"; });
  zone.addEventListener("drop", async (e) => {
    e.preventDefault();
    depth = 0;
    zone.classList.remove("over");
    const items = await itemsFromDrop(e.dataTransfer);
    sendToPhone(p, items.map((i) => i.file));
  });

  const outbox = p.outbox.length
    ? h("div", { class: "transfers" }, ...p.outbox.slice().reverse().map((it) => h("article", { class: "card tr" },
      h("span", { class: "tr-dir", title: t("To the phone") }, icon("up")),
      h("div", { class: "tr-main" },
        h("div", { class: "tr-title" }, it.name),
        h("div", { class: "tr-sub" }, `${bytes(it.size)}, ${it.downloads ? t("downloaded on the phone") : t("waiting to be downloaded on the phone")}`)),
      h("div", { class: "tr-actions" },
        h("button", { class: "icon-btn small", type: "button", "aria-label": t("Remove from list"), title: t("Remove from list"),
          onclick: () => api("DELETE", `/api/phones/${p.id}/outbox/${it.id}`).catch((e) => toast(e.message, "err")) }, icon("x"))))))
    : h("p", { class: "empty-line" }, t("Nothing sent to the phone yet."));

  $("#stage").replaceChildren(h("div", { class: "stage-inner" },
    h("div", { class: "peer-head" },
      h("button", { class: "icon-btn back", type: "button", "aria-label": t("Back to the device list"), onclick: () => select(null) }, icon("back")),
      stamp(p, "lg", true),
      h("div", { class: "peer-title" }, h("h1", null, p.name),
        h("div", { class: "peer-meta" }, h("span", { class: "chip ok" }, icon("lock"), t("Approved")), h("span", null, t("Phone connected from the browser"))))),
    zone,
    h("section", { class: "section" },
      h("div", { class: "section-head" }, h("h2", null, t("Sent to the phone"))), outbox),
    h("section", { class: "section" },
      h("div", { class: "section-head" }, h("h2", null, t("Text and links"))),
      h("div", { class: "thread", id: "thread" }),
      composer(p, draft)),
    h("section", { class: "section" },
      h("div", { class: "section-head" }, h("h2", null, t("From the phone"))),
      h("div", { class: "transfers", id: "peer-transfers" })),
    h("div", { class: "menu-row" },
      h("button", { class: "linkish danger", type: "button", onclick: async () => {
        try { await api("DELETE", `/api/phones/${p.id}`); toast(t("{name} disconnected", { name: p.name })); select(null); }
        catch (e) { toast(e.message, "err"); }
      } }, t("Disconnect")))));
  renderTransfers();
  renderThread();
}

async function sendToPhone(p, files) {
  for (const f of files) {
    try {
      const res = await fetch(`/api/phones/${p.id}/outbox`, {
        method: "PUT", body: f,
        headers: { "Content-Type": "application/octet-stream", "X-File-Name": encodeURIComponent(f.name), "X-Pasla-Lang": I18N.lang() },
      });
      if (res.status === 401) { showLocked(); return; }
      if (!res.ok) throw new Error((await res.json().catch(() => ({}))).error || res.statusText);
    } catch (e) {
      toast(`${f.name}: ${e.message}`, "err");
      return;
    }
  }
  toast(files.length === 1 ? t("{file} is ready on the phone", { file: files[0].name }) : tn(files.length, "{n} file is ready on the phone", "{n} files are ready on the phone"));
}

async function decidePhone(ph, ok) {
  try {
    await api("POST", `/api/phones/${ph.id}/${ok ? "accept" : "reject"}`);
    if (ok) toast(t("{name} connected", { name: ph.name }));
  } catch (e) { toast(e.message, "err"); }
  scheduleLoad();
}

function phoneRequestCard(ph) {
  return h("section", { class: "card offer", "data-phone": ph.id, role: "alertdialog", "aria-label": t("{name} wants to connect", { name: ph.name }) },
    h("div", { class: "offer-head" }, stamp({ fp: ph.id, os: ph.os }),
      h("div", null, h("h3", null, t("{name} wants to connect", { name: ph.name })),
        h("div", { class: "offer-sub" }, t("Does the phone show the same number?")))),
    h("div", { class: "phone-check" }, ph.check),
    h("div", { class: "offer-actions" },
      h("button", { class: "btn ghost small", type: "button", onclick: () => decidePhone(ph, false) }, t("Decline")),
      h("button", { class: "btn small", type: "button", onclick: () => decidePhone(ph, true) }, icon("check"), t("Same, allow"))));
}

let phoneTick = 0;
function renderPhoneDialog() {
  const dlg = $("#phone-dialog");
  // Requests show up as cards when the QR window is closed.
  const box = $("#offers");
  const pending = (S.phones || []).filter((ph) => !ph.approved);
  for (const el of [...box.querySelectorAll("[data-phone]")]) if (dlg.open || !pending.some((ph) => ph.id === el.dataset.phone)) el.remove();
  if (!dlg.open) {
    for (const ph of pending) if (!box.querySelector(`[data-phone="${ph.id}"]`)) box.append(phoneRequestCard(ph));
    return;
  }
  const v = S.phone || { active: false, links: [] };
  const chosen = Math.min(S.phoneLink || 0, Math.max(0, v.links.length - 1));
  const link = v.links[chosen];
  const qrBox = h("div", { class: "qr" });
  if (link && link.qr) {
    const doc = new DOMParser().parseFromString(link.qr, "image/svg+xml");
    qrBox.append(document.importNode(doc.documentElement, true));
  }
  const left = v.expires ? Math.max(0, Math.round((new Date(v.expires) - Date.now()) / 1000)) : 0;
  dlg.replaceChildren(h("div", { class: "sheet-body phone-sheet" },
    h("h2", null, t("Connect a phone")),
    v.keyLive && link ? [
      h("p", null, t("The phone must be on the same Wi-Fi as this computer. No app needed.")),
      qrBox,
      v.links.length > 1 ? h("div", { class: "ip-pick", role: "group", "aria-label": t("Network address") },
        h("span", null, t("Not scanning? Try another address:")),
        ...v.links.map((l, i) => h("button", { type: "button", class: "chip" + (i === chosen ? " ok" : ""),
          onclick: () => { S.phoneLink = i; renderPhoneDialog(); } }, new URL(l.url).hostname))) : null,
      h("ol", { class: "phone-steps" },
        h("li", null, t("Scan the QR code with the phone's camera.")),
        h("li", null, t("If the browser says the connection is not private, tap Advanced, then Proceed. The connection is still encrypted; the warning appears because this computer made its own certificate instead of buying one from a company.")),
        h("li", null, t("The same 4-digit number appears on the phone and here; if it matches, allow it."))),
      h("p", { class: "pair-hint", id: "phone-left" }, t("The code expires in {time}.", { time: fmtLeft(left) })),
    ] : h("p", null, t("The code has expired. Create a new one.")),
    pending.length ? h("div", { class: "phone-requests" }, ...pending.map(phoneRequestCard)) : null,
    h("div", { class: "sheet-actions" },
      v.active ? h("button", { class: "btn ghost", type: "button", onclick: async () => {
        await api("POST", "/api/phone/stop").catch((e) => toast(e.message, "err"));
        dlg.close();
        load();
      } }, t("Turn off phone connections")) : null,
      !v.keyLive ? h("button", { class: "btn ghost", type: "button", onclick: openPhoneDialog }, t("New code")) : null,
      h("button", { class: "btn", type: "button", onclick: () => dlg.close() }, t("Done")))));
}

function fmtLeft(sec) {
  const m = Math.floor(sec / 60), s = sec % 60;
  return m ? t("{m} min {s} s", { m, s }) : t("{n} s", { n: s });
}

async function openPhoneDialog() {
  try {
    S.phone = await api("POST", "/api/phone/start");
  } catch (e) {
    toast(e.message, "err");
    return;
  }
  const dlg = $("#phone-dialog");
  if (!dlg.open) dlg.showModal();
  renderPhoneDialog();
  clearInterval(phoneTick);
  phoneTick = setInterval(() => {
    if (!dlg.open) { clearInterval(phoneTick); return; }
    const v = S.phone;
    const el = $("#phone-left");
    if (!v || !v.expires) return;
    const left = Math.round((new Date(v.expires) - Date.now()) / 1000);
    if (left <= 0) { S.phone = { ...v, keyLive: false }; renderPhoneDialog(); return; }
    if (el) el.textContent = t("The code expires in {time}.", { time: fmtLeft(left) });
  }, 1000);
}

/* ---------- wiring ---------- */
$("#settings-btn").addEventListener("click", openSettings);
$("#me-btn").addEventListener("click", openSettings);
$("#phone-btn").addEventListener("click", openPhoneDialog);
$("#phone-dialog").addEventListener("close", () => { renderPhoneDialog(); });
$("#open-received").addEventListener("click", () => api("POST", "/api/reveal-receive-dir").catch((e) => toast(e.message, "err")));
$("#add-ip-form").addEventListener("submit", async (e) => {
  e.preventDefault();
  const input = $("#add-ip");
  const addr = input.value.trim();
  if (!addr) return;
  const btn = e.target.querySelector("button");
  btn.disabled = true;
  try {
    const info = await api("POST", "/api/manual-peers", { addr });
    input.value = "";
    toast(t("{name} added", { name: info.name }));
    await load();
    select(info.fp);
  } catch (err) {
    toast(err.message, "err");
  } finally {
    btn.disabled = false;
  }
});
// Dropping files anywhere outside the drop zone must not navigate away.
window.addEventListener("dragover", (e) => e.preventDefault());
window.addEventListener("drop", (e) => e.preventDefault());

I18N.applyDOM(document);
try { S.selected = localStorage.getItem("pasla.selected") || null; } catch { /* storage unavailable */ }
connect();
