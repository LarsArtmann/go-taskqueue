/* webui-live-contract unit harness (docs/webui-live-contract.md):
   exercises the app.js swap-guard + state re-apply over a minimal DOM stub
   — no browser, no external deps. Run via scripts/check-webui-js.sh.
   TQ_WEBUIJS_FAIL=1 forces one failing test (gate --self-test hook). */
import { createRequire } from "node:module";
import { fileURLToPath } from "node:url";

/* ---------- minimal DOM stub ---------- */

const VOID = new Set(["input", "br", "img", "hr", "meta", "link"]);

class ClassList {
  constructor(el) { this.el = el; }
  _set() { return new Set((this.el.attributes.get("class") || "").split(/\s+/).filter(Boolean)); }
  _store(s) { this.el.attributes.set("class", [...s].join(" ")); }
  add(c) { const s = this._set(); s.add(c); this._store(s); }
  remove(c) { const s = this._set(); s.delete(c); this._store(s); }
  contains(c) { return this._set().has(c); }
}

class El {
  constructor(tag) {
    this.tagName = tag.toUpperCase();
    this.childNodes = [];
    this.parentNode = null;
    this.attributes = new Map();
    this.classList = new ClassList(this);
    this.scrollTop = 0;
    this.scrollHeight = 1000;
    this.clientHeight = 500;
    this.value = "";
    this.isContentEditable = false;
    this.style = {};
  }
  get id() { return this.attributes.get("id") || ""; }
  set id(v) { this.attributes.set("id", v); }
  get name() { return this.attributes.get("name") || ""; }
  get type() { return this.attributes.get("type") || ""; }
  get open() { return this.attributes.get("open") !== undefined; }
  set open(v) { if (v) this.attributes.set("open", ""); else this.attributes.delete("open"); }
  get textContent() {
    let out = "";
    const walk = (n) => {
      for (const c of n.childNodes) {
        if (c.text !== undefined) out += c.text;
        else walk(c);
      }
    };
    walk(this);
    return out;
  }
  set textContent(v) { this.childNodes = [{ text: v }]; }
  appendChild(c) { c.parentNode = this; this.childNodes.push(c); return c; }
  insertBefore(c, ref) {
    c.parentNode = this;
    const i = this.childNodes.indexOf(ref);
    if (i < 0) this.childNodes.push(c); else this.childNodes.splice(i, 0, c);
    return c;
  }
  get firstChild() { return this.childNodes[0] || null; }
  contains(n) {
    for (let p = n; p; p = p.parentNode) if (p === this) return true;
    return false;
  }
  setAttribute(k, v) {
    this.attributes.set(k, String(v));
    if (k === "class") this.classList = new ClassList(this);
  }
  getAttribute(k) { const v = this.attributes.get(k); return v === undefined ? null : v; }
  focus() { document.activeElement = this; document.focusLog.push(this); }
  closest(sel) {
    for (let p = this; p; p = p.parentNode) if (p.matches && p.matches(sel)) return p;
    return null;
  }
  matches(sel) {
    return sel.split(",").some((tok) => matchTok(this, tok.trim()));
  }
  querySelectorAll(sel) {
    const out = [];
    const walk = (n) => {
      for (const c of n.childNodes) {
        if (c.text === undefined) {
          if (c.matches(sel)) out.push(c);
          walk(c);
        }
      }
    };
    walk(this);
    return out;
  }
  querySelector(sel) { return this.querySelectorAll(sel)[0] || null; }
  set innerHTML(html) {
    this.childNodes = [];
    parseHTML(html, this);
  }
  get innerHTML() { return ""; }
}

function matchTok(el, tok) {
  const m = tok.match(/^(?:([a-zA-Z][\w-]*))?(?:#([\w-]+))?(?:\.([\w-]+))?(?:\[([-\w]+)(?:=(["']?)([^\]]*)\5)?\])?$/);
  if (!m || tok === "") return false;
  const [, tag, id, cls, attr, , val] = m;
  if (tag && el.tagName.toLowerCase() !== tag.toLowerCase()) return false;
  if (id && el.id !== id) return false;
  if (cls && !el.classList.contains(cls)) return false;
  if (attr) {
    const v = el.getAttribute(attr);
    if (v === null) return false;
    if (val !== undefined && v !== val) return false;
  }
  return tag || id || cls || attr;
}

function parseAttr(line, el) {
  const re = /([-\w]+)(?:\s*=\s*("([^"]*)"|'([^']*)'|([^\s"'>]+)))?/g;
  let m;
  while ((m = re.exec(line))) {
    const v = m[3] !== undefined ? m[3] : m[4] !== undefined ? m[4] : m[2] !== undefined ? "" : "";
    if (m[2] === undefined) el.setAttribute(m[1], "");
    else el.setAttribute(m[1], v);
  }
}

function parseHTML(html, root) {
  const re = /<!--[\s\S]*?-->|<\/?[a-zA-Z][^>]*>|[^<]+/g;
  let m = null;
  const stack = [root];
  while ((m = re.exec(html))) {
    const tok = m[0];
    const top = stack[stack.length - 1];
    if (tok.startsWith("<!--")) continue;
    if (tok.startsWith("</")) {
      if (stack.length > 1) stack.pop();
      continue;
    }
    if (tok.startsWith("<")) {
      const nameMatch = tok.match(/^<([a-zA-Z][\w-]*)/);
      if (!nameMatch) continue;
      const el = new El(nameMatch[1]);
      const rest = tok.slice(1 + nameMatch[1].length).replace(/>$/, "");
      parseAttr(rest, el);
      top.appendChild(el);
      if (!VOID.has(el.tagName.toLowerCase())) stack.push(el);
      continue;
    }
    top.childNodes.push({ text: tok });
  }
}

/* ---------- document / window / EventSource stubs ---------- */

const focusLog = [];
const docListeners = [];
const doc = new El("#document");
doc.body = new El("body");
doc.appendChild(doc.body);
doc.focusLog = focusLog;
doc.getElementById = (id) => doc.querySelectorAll("#" + id)[0] || null;
doc.createElement = (tag) => new El(tag);
doc.addEventListener = (type, fn) => { docListeners.push([type, fn]); };

globalThis.document = doc;
globalThis.window = {
  location: { pathname: "/", search: "" },
  addEventListener: () => {},
};
globalThis.EventSource = class {
  constructor(url) { this.url = url; }
  addEventListener() {}
  close() {}
};
const realSetInterval = globalThis.setInterval;
globalThis.setInterval = () => 0;

const realDateNow = Date.now;
let fakeNow = realDateNow();
globalThis.Date.now = () => fakeNow;

const require = createRequire(import.meta.url);
const app = require(fileURLToPath(new URL("../static/app.js", import.meta.url)));

/* ---------- tiny test runner ---------- */

let ok = 0;
let failed = 0;
function test(name, fn) {
  try {
    fn();
    ok++;
    console.log("ok: " + name);
  } catch (e) {
    failed++;
    console.log("FAIL: " + name + ": " + (e && e.message));
  }
}
const apply = (o) => app.applyFragment(JSON.stringify(o));
function eq(a, b, msg) {
  if (a !== b) throw new Error((msg || "eq") + ": got " + JSON.stringify(a) + ", want " + JSON.stringify(b));
}

function fragEl(html, id) {
  const el = new El("div");
  el.id = id;
  doc.body.appendChild(el);
  el.innerHTML = html;
  return el;
}

/* ---------- tests ---------- */

test("module hook exposes the swap-guard surface", () => {
  eq(typeof app.applyFragment, "function");
  eq(typeof app.swapIn, "function");
  eq(typeof app.fragmentBusy, "function");
  eq(typeof app.flushPendingSwap, "function");
  eq(app.flushGraceMs, 2500);
});

test("document-level delegation registered for click/keydown (listener-drop invariant)", () => {
  const types = docListeners.map((p) => p[0]);
  for (const t of ["click", "keydown", "submit", "input", "toggle"]) {
    if (!types.includes(t)) throw new Error("missing delegated listener: " + t);
  }
});

test("applyFragment ignores invalid JSON and unknown fragment ids", () => {
  app.applyFragment("not json");
  app.applyFragment(JSON.stringify({ id: "frag-nope", html: "<p>x</p>" }));
  const el = fragEl("<p>old</p>", "frag-t0");
  app.applyFragment(JSON.stringify({ id: "frag-t0", html: "<p>new</p>" }));
  eq(el.textContent, "new", "valid swap still lands");
  doc.body.childNodes = doc.body.childNodes.filter((c) => c !== el);
});

test("plain swap replaces innerHTML when idle", () => {
  const el = fragEl("<p>old</p>", "frag-t1");
  apply({ id: "frag-t1", html: "<p>fresh</p>" });
  eq(el.textContent, "fresh");
});

test("details[open] makes fragment busy: latest payload queued, swapped after grace", () => {
  const el = fragEl('<details open data-state-key="cell-1"><summary>r</summary><p>x</p></details>', "frag-t2");
  fakeNow += 1000;
  apply({ id: "frag-t2", html: '<details data-state-key="cell-1"><summary>r</summary><p>v1</p></details>' });
  eq(el.textContent.includes("v1"), false, "not swapped while busy");
  fakeNow += 1000;
  apply({ id: "frag-t2", html: '<details data-state-key="cell-1"><summary>r</summary><p>v2</p></details>' });
  fakeNow += 1000;
  app.flushPendingSwap();
  eq(el.textContent.includes("v2"), false, "still inside grace window (age 2s < 2.5s)");
  fakeNow += 600;
  app.flushPendingSwap();
  eq(el.textContent.includes("v2"), true, "grace elapsed: LATEST payload lands");
  eq(el.querySelector('details[data-state-key="cell-1"]').open, true, "open state re-applied through the forced swap");
});

test("focused input makes fragment busy until grace flush", () => {
  const el = fragEl('<input name="q" value="a">', "frag-t3");
  el.querySelector("input").focus();
  fakeNow += 5000;
  apply({ id: "frag-t3", html: '<input name="q" value="b">' });
  eq(el.querySelector("input").value, "a", "swap deferred while focused");
  doc.activeElement = null;
  fakeNow += 3000;
  app.flushPendingSwap();
  eq(el.querySelector("input").value, "b", "swapped after grace once focus left");
});

test("listener-drop invariant: swapped-in data-error cell still expands via document delegation", () => {
  const el = fragEl("<p>old</p>", "frag-t4");
  apply({
    id: "frag-t4",
    html: '<span data-error="full message" data-short="short" data-expanded="0">short</span>',
  });
  const cell = el.querySelector("[data-error]");
  const click = docListeners.filter((p) => p[0] === "click");
  let prevented = false;
  click.forEach(([, fn]) => fn({ target: cell, preventDefault: () => { prevented = true; } }));
  eq(cell.getAttribute("data-expanded"), "1", "delegated handler reaches the fresh subtree");
  eq(cell.textContent, "full message", "cell expanded with the full error text");
});

test("data-state-key re-applies details open state even when the summary text drifts", () => {
  const el = fragEl('<details data-state-key="cancel-task-1" open><summary>Cancel (3)</summary></details>', "frag-t5");
  apply({
    id: "frag-t5",
    html: '<details data-state-key="cancel-task-1"><summary>Cancel (7)</summary></details>',
  });
  eq(el.querySelector("details").open, true, "matched by stable key, not summary text");
});

test("typed control values restored by name across the swap", () => {
  const el = fragEl('<input name="q" value="needle">', "frag-t6");
  el.querySelector("input").value = "needle-typed";
  apply({ id: "frag-t6", html: '<input name="q" value="">' });
  eq(el.querySelector("input").value, "needle-typed");
});

test("focus-after-swap: focus restored onto the fresh subtree", () => {
  const el = fragEl('<input name="q" value="">', "frag-t7");
  el.querySelector("input").focus();
  doc.focusLog.length = 0;
  apply({ id: "frag-t7", html: '<input name="q" value="">' });
  const ae = doc.activeElement;
  if (!ae) throw new Error("focus lost after swap");
  eq(ae.name, "q", "focus restored to matching control");
  eq(el.contains(ae), true, "restored focus lives in the FRESH subtree");
});

test("scroll containers keep scrollTop; bottom-pinned panes stay pinned", () => {
  const el = fragEl('<div class="journal-scroll"></div><div class="journal-scroll"></div>', "frag-t8");
  const scrolls = el.querySelectorAll(".journal-scroll");
  scrolls[0].scrollTop = 120;
  scrolls[1].scrollTop = 980;
  scrolls[1].clientHeight = 1000;
  apply({ id: "frag-t8", html: '<div class="journal-scroll"></div><div class="journal-scroll"></div>' });
  const fresh = el.querySelectorAll(".journal-scroll");
  eq(fresh[0].scrollTop, 120, "mid-scroll position preserved");
  eq(fresh[1].scrollTop, 1000, "at-bottom pane re-pinned to the new scrollHeight");
});

test("expanded error cells re-applied by data-short through the swap", () => {
  const el = fragEl('<span data-error="boom" data-short="err" data-expanded="1">boom</span>', "frag-t9");
  apply({ id: "frag-t9", html: '<span data-error="boom" data-short="err" data-expanded="0">err</span>' });
  const cell = el.querySelector("[data-error]");
  eq(cell.getAttribute("data-expanded"), "1");
  eq(cell.textContent, "boom");
  eq(cell.getAttribute("aria-expanded"), "true");
});

test("gateway: TQ_WEBUIJS_FAIL=1 forces a failing test (gate self-test hook)", () => {
  if (process.env.TQ_WEBUIJS_FAIL === "1") throw new Error("intentional failure for gate self-test");
});

if (failed > 0 || process.env.TQ_WEBUIJS_FAIL === "1") {
  if (process.env.TQ_WEBUIJS_FAIL !== "1") process.exitCode = 1;
  else process.exitCode = 1;
}
globalThis.setInterval = realSetInterval;
globalThis.Date.now = realDateNow;
console.log(`webui-js summary: ${ok} tests ok, ${failed} failed`);
if (process.env.TQ_WEBUIJS_FAIL === "1") process.exit(1);
