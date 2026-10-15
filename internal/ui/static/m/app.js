"use strict";

/* Phone page. Served by the computer over HTTPS; no outside requests.
   Texts go through t()/tn() from /i18n.js; English is the source language. */

const { t, tn } = I18N;
const main = document.getElementById("main");
const leaveBtn = document.getElementById("leave");
const langBtn = document.getElementById("lang");
let state = null;
let pollTimer = 0;
let uploading = null; // { index, count, name, done, total }

function h(tag, attrs, ...kids) {
  const el = document.createElement(tag);
  for (const [k, v] of Object.entries(attrs || {})) {
    if (v == null || v === false) continue;
    if (k === "class") el.className = v;
    else if (k.startsWith("on")) el.addEventListener(k.slice(2), v);
    else if (v === true) el.setAttribute(k, "");
    else el.setAttribute(k, v);
  }
  for (const kid of kids.flat()) {
    if (kid == null || kid === false) continue;
    el.append(kid instanceof Node ? kid : document.createTextNode(String(kid)));
  }
  return el;
}

const bytes = I18N.bytes;

let toastTimer = 0;
function toast(text, err) {
  const el = document.getElementById("toast");
  el.textContent = text;
  el.className = "toast" + (err ? " err" : "");
  el.hidden = false;
  clearTimeout(toastTimer);
  toastTimer = setTimeout(() => (el.hidden = true), err ? 6000 : 3000);
}

async function api(method, path, body) {
  const opt = { method, headers: { "X-Pasla-Lang": I18N.lang() }, credentials: "same-origin" };
  if (body !== undefined) {
    opt.headers["Content-Type"] = "application/json";
    opt.body = JSON.stringify(body);
  }
  const res = await fetch(path, opt);
  let data = null;
  try { data = await res.json(); } catch { /* empty */ }
  if (!res.ok) {
    const e = new Error((data && data.error) || res.statusText);
    e.status = res.status;
    throw e;
  }
  return data;
}

function guessDevice() {
  const ua = navigator.userAgent;
  if (/iPhone/.test(ua)) return { name: "iPhone", os: "ios" };
  if (/iPad/.test(ua) || (/Macintosh/.test(ua) && navigator.maxTouchPoints > 1)) return { name: "iPad", os: "ios" };
  if (/Android/.test(ua)) return { name: t("Android phone"), os: "android" };
  return { name: t("Phone"), os: "other" };
}

function savedName() {
  try { return localStorage.getItem("pasla.phoneName") || ""; } catch { return ""; }
}

/* ---------- language ---------- */
function renderLangBtn() {
  const other = I18N.languages.find((l) => l.code !== I18N.lang());
  langBtn.textContent = other.short;
  langBtn.setAttribute("aria-label", other.name);
  langBtn.setAttribute("lang", other.code);
}
langBtn.addEventListener("click", () => {
  const other = I18N.languages.find((l) => l.code !== I18N.lang());
  I18N.setLang(other.code);
  I18N.applyDOM(document);
  renderLangBtn();
  lastKey = "";
  if (current) current();
});

let current = null; // re-renders the visible screen after a language change

/* ---------- screens ---------- */
function screenConnect(key) {
  current = () => screenConnect(key);
  const dev = guessDevice();
  const input = h("input", { id: "name", maxlength: "40", value: document.getElementById("name")?.value || savedName() || dev.name, autocomplete: "off" });
  const btn = h("button", { class: "btn", type: "submit" }, t("Connect"));
  main.replaceChildren(
    h("h1", null, t("Connect to the computer")),
    h("p", null, t("This phone talks straight to the computer on the same network. Nothing goes to the internet.")),
    h("form", { class: "card", onsubmit: async (e) => {
      e.preventDefault();
      btn.disabled = true;
      try { localStorage.setItem("pasla.phoneName", input.value.trim()); } catch { /* private mode */ }
      try {
        await api("POST", "/p/login", { key, name: input.value.trim(), os: dev.os });
        history.replaceState(null, "", "/");
        poll();
      } catch (err) {
        btn.disabled = false;
        screenError(err.message);
      }
    } },
      h("label", { class: "field" }, h("span", null, t("Name shown on the computer")), input),
      btn));
}

function screenError(msg) {
  current = () => screenError(msg);
  leaveBtn.hidden = true;
  main.replaceChildren(
    h("h1", null, t("Not connected")),
    h("p", null, msg || t("On the computer, open Pasla, press “Connect a phone” and scan the QR code again.")));
}

function screenPending(s) {
  current = () => screenPending(s);
  leaveBtn.hidden = false;
  main.replaceChildren(
    h("h1", { class: "center" }, t("Confirm on the computer")),
    h("p", { class: "center" }, t("A prompt just opened on {computer}. If the number there matches this one, press “Same, allow”.", { computer: s.computer })),
    h("span", { class: "check", "aria-label": t("Check number {digits}", { digits: s.check.split("").join(" ") }) }, s.check),
    h("div", { class: "spinner", role: "progressbar", "aria-label": t("Waiting for approval") }));
}

let lastKey = "";
function screenReady(s) {
  current = () => { lastKey = ""; screenReady(s); };
  leaveBtn.hidden = false;
  const key = JSON.stringify([s.computer, s.outbox, s.messages.length, s.uploads.map((u) => [u.id, u.state, u.filesDone]), !!uploading, I18N.lang()]);
  if (key === lastKey && document.getElementById("send-card")) { updateProgress(); return; }
  lastKey = key;
  const draft = document.getElementById("text")?.value || "";

  const picker = h("input", { type: "file", multiple: true, hidden: true, onchange: () => { upload([...picker.files]); picker.value = ""; } });
  const sendCard = h("section", { class: "card", id: "send-card" },
    h("h2", null, t("Send to {computer}", { computer: s.computer })),
    h("p", { class: "note" }, t("Photos, videos or any file. They are saved in the computer's received files folder.")),
    h("button", { class: "btn", type: "button", disabled: !!uploading, onclick: () => picker.click() }, uploading ? t("Sending…") : t("Choose files and send")),
    uploading ? h("div", null,
      h("p", { class: "note up-label", id: "up-label" }, ""),
      h("div", { class: "bar-progress" }, h("i", { id: "up-bar" }))) : null,
    picker);

  const inbox = h("section", { class: "card" },
    h("h2", null, t("From the computer")),
    s.outbox.length
      ? h("ul", { class: "list" }, ...s.outbox.slice().reverse().map((it) => h("li", null,
        h("span", { class: "grow" }, h("span", { class: "name" }, it.name),
          h("span", { class: "sub" }, bytes(it.size) + (it.downloads ? ", " + t("downloaded") : ""))),
        h("a", { class: "dl", href: `/p/api/outbox/${it.id}`, download: it.name }, t("Download")))))
      : h("p", { class: "empty" }, t("Files the computer sends to this phone appear here.")));

  const ta = h("textarea", { id: "text", placeholder: t("Type a text, link or note for the computer"), "aria-label": t("Text") });
  ta.value = draft;
  const msgs = s.messages.slice(-30).reverse().map((m) => h("div", { class: "msg " + m.dir },
    m.text,
    h("div", { class: "foot" },
      h("span", null, (m.dir === "in" ? t("From the computer, {time}", { time: I18N.time(new Date(m.at)) }) : t("Sent, {time}", { time: I18N.time(new Date(m.at)) }))),
      h("button", { type: "button", onclick: (e) => copy(m.text, e.currentTarget) }, t("Copy")))));
  const textCard = h("section", { class: "card" },
    h("h2", null, t("Text")),
    ta,
    h("div", { class: "row" }, h("button", { class: "btn", type: "button", onclick: async () => {
      const text = ta.value;
      if (!text.trim()) return;
      try { await api("POST", "/p/api/message", { text }); ta.value = ""; poll(); }
      catch (e) { toast(e.message, true); }
    } }, t("Send text"))),
    ...msgs);

  const sent = s.uploads.length ? h("section", { class: "card" },
    h("h2", null, t("You sent")),
    h("ul", { class: "list" }, ...s.uploads.slice(0, 20).map((u) => {
      const label = {
        active: `${bytes(u.done)} / ${bytes(u.total)}`,
        done: tn(u.count, "{n} file, {size}, saved", "{n} files, {size}, saved", { size: bytes(u.total) }),
        failed: t("Could not finish: {why}", { why: u.error || "" }),
        cancelled: t("Cancelled"),
        rejected: u.error || t("Not accepted"),
      }[u.state] || "";
      return h("li", null, h("span", { class: "grow" },
        h("span", { class: "name" }, u.names.join(", ") + (u.count > u.names.length ? " (" + tn(u.count, "{n} file", "{n} files") + ")" : "")),
        h("span", { class: "sub" + (["failed", "cancelled", "rejected"].includes(u.state) ? " err" : "") }, label)));
    }))) : null;

  main.replaceChildren(
    h("h1", null, t("Connected to {computer}", { computer: s.computer })),
    h("p", null, t("The connection is encrypted. It closes by itself when you close this page or after 30 minutes without use.")),
    sendCard, inbox, textCard, sent);
  updateProgress();
}

function updateProgress() {
  if (!uploading) return;
  const lbl = document.getElementById("up-label");
  const bar = document.getElementById("up-bar");
  if (lbl) lbl.textContent = `${uploading.index}/${uploading.count}: ${uploading.name} (${bytes(uploading.done)} / ${bytes(uploading.total)})`;
  if (bar) bar.style.width = (uploading.total ? (uploading.done / uploading.total) * 100 : 0).toFixed(1) + "%";
}

async function copy(text, btn) {
  try { await navigator.clipboard.writeText(text); }
  catch {
    const ta = h("textarea", null);
    ta.value = text;
    document.body.append(ta);
    ta.select();
    document.execCommand("copy");
    ta.remove();
  }
  btn.textContent = t("Copied");
  setTimeout(() => (btn.textContent = t("Copy")), 1500);
}

/* ---------- uploading ---------- */
function putFile(url, file, onProgress) {
  return new Promise((resolve, reject) => {
    const xhr = new XMLHttpRequest();
    xhr.open("PUT", url);
    xhr.withCredentials = true;
    xhr.setRequestHeader("Content-Type", "application/octet-stream");
    xhr.setRequestHeader("X-Pasla-Lang", I18N.lang());
    xhr.upload.onprogress = (e) => onProgress(e.loaded);
    xhr.onload = () => {
      if (xhr.status >= 200 && xhr.status < 300) return resolve();
      let msg = xhr.statusText;
      try { msg = JSON.parse(xhr.responseText).error || msg; } catch { /* not json */ }
      reject(new Error(msg));
    };
    xhr.onerror = () => reject(new Error(t("The connection dropped")));
    xhr.send(file);
  });
}

async function upload(files) {
  if (!files.length) return;
  let offer;
  try {
    offer = await api("POST", "/p/api/offer", { files: files.map((f) => ({ name: f.name, size: f.size })) });
  } catch (e) {
    toast(e.message, true);
    return;
  }
  const total = files.reduce((a, f) => a + f.size, 0);
  let base = 0;
  uploading = { index: 0, count: files.length, name: "", done: 0, total };
  render();
  try {
    for (let i = 0; i < files.length; i++) {
      Object.assign(uploading, { index: i + 1, name: files[i].name });
      await putFile(`/p/api/upload/${offer.id}/${i}`, files[i], (n) => { uploading.done = base + n; updateProgress(); });
      base += files[i].size;
    }
    toast(files.length === 1 ? t("Sent") : tn(files.length, "{n} file sent", "{n} files sent"));
  } catch (e) {
    toast(e.message, true);
  } finally {
    uploading = null;
    lastKey = "";
    poll();
  }
}

window.addEventListener("beforeunload", (e) => { if (uploading) { e.preventDefault(); e.returnValue = ""; } });

/* ---------- state ---------- */
function render() {
  if (!state) return;
  if (!state.approved) screenPending(state);
  else { lastKey = ""; screenReady(state); }
}

async function poll() {
  clearTimeout(pollTimer);
  try {
    const s = await api("GET", "/p/api/state");
    const wasApproved = state && state.approved;
    state = s;
    if (!s.approved) screenPending(s);
    else if (!wasApproved) { lastKey = ""; screenReady(s); }
    else screenReady(s);
  } catch (e) {
    if (e.status === 401) { state = null; screenError(); return; }
  }
  pollTimer = setTimeout(poll, document.hidden ? 5000 : 1500);
}

document.addEventListener("visibilitychange", () => { if (!document.hidden && state) poll(); });

leaveBtn.addEventListener("click", async () => {
  try { await api("POST", "/p/api/logout"); } catch { /* already gone */ }
  clearTimeout(pollTimer);
  state = null;
  leaveBtn.hidden = true;
  current = () => main.replaceChildren(h("h1", null, t("Disconnected")), h("p", null, t("To connect again, scan the QR code on the computer.")));
  current();
});

I18N.applyDOM(document);
renderLangBtn();
const m = /[#&]k=([A-Za-z0-9_-]+)/.exec(location.hash);
if (m) screenConnect(m[1]);
else poll();
