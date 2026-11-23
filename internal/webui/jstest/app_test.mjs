/* webui-live-contract unit harness (docs/webui-live-contract.md):
   exercises the app.js swap-guard + state re-apply over a minimal DOM stub
   — no browser, no external deps. Run via scripts/check-webui-js.sh.
   TQ_WEBUIJS_FAIL=1 forces one failing test (gate --self-test hook).

   Fidelity boundary — where this stub diverges from a real DOM. Harness
   work must know these before trusting a pass:

   - matches() is a hand-rolled approximator, not a selector engine: it
     accepts only simple comma-separated tokens (tag / #id / .class /
     [attr] / [attr=v]); no descendant combinators, no :has(). The one
     pseudo-class it knows, :hover, is answered by the el.hovered
     PROPERTY that tests set manually (2026-11-22 pause-on-hover work):
     a real browser derives hover from pointer position, applies it to
     ancestor chains, and updates it continuously — the stub pins one
     static answer.
   - removeChild() silently ignores a child it doesn't own (a real DOM
     throws NotFoundError) and still nulls the child's parentNode; it
     fires no mutation events. The feed badge removal and the feed cap
     trim (app.js) rely on the lenient form.
   - No layout engine: scrollTop is free-set state, scrollHeight/
     clientHeight are constants (1000/500), scrollWidth is undefined —
     overflow-sensitive behavior (board affordance, journal-scroll pin)
     only sees what a test explicitly pins. "atBottom" in the stub is
     scrollTop + clientHeight >= scrollHeight - 24 with those constants.
   - Events are synchronous and argument-less (_fire calls the listener
     with no Event object), never bubble, and there is no dispatch
     machinery: delegation tests assert listener registration, tests
     trigger handlers by calling el._fire(type) directly.
   - innerHTML parses a small HTML subset (no entity decoding, no
     implicit close, self-closing only for the VOID set); text nodes are
     stored as {text}. Fragment markup must stay inside that subset for
     a pass to mean anything. */
import { createRequire } from "node:module";
import { fileURLToPath } from "node:url";

/* ---------- minimal DOM stub ---------- */

const VOID = new Set(["input", "br", "img", "hr", "meta", "link"]);

class ClassList {
	constructor(el) {
		this.el = el;
	}
	_set() {
		return new Set((this.el.attributes.get("class") || "").split(/\s+/).filter(Boolean));
	}
	_store(s) {
		this.el.attributes.set("class", [...s].join(" "));
	}
	add(c) {
		const s = this._set();
		s.add(c);
		this._store(s);
	}
	remove(c) {
		const s = this._set();
		s.delete(c);
		this._store(s);
	}
	contains(c) {
		return this._set().has(c);
	}
	toggle(c, force) {
		const s = this._set();
		const has = s.has(c);
		const want = force === undefined ? !has : !!force;
		if (want) s.add(c);
		else s.delete(c);
		this._store(s);
		return want;
	}
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
		this._value = undefined;
		this.isContentEditable = false;
		this.style = {};
	}
	get id() {
		return this.attributes.get("id") || "";
	}
	set id(v) {
		this.attributes.set("id", v);
	}
	get name() {
		return this.attributes.get("name") || "";
	}
	get type() {
		return this.attributes.get("type") || "";
	}
	get value() {
		return this._value !== undefined ? this._value : this.getAttribute("value") || "";
	}
	set value(v) {
		this._value = v;
	}
	get open() {
		return this.attributes.get("open") !== undefined;
	}
	set open(v) {
		if (v) this.attributes.set("open", "");
		else this.attributes.delete("open");
	}
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
	set textContent(v) {
		this.childNodes = [{ text: v }];
	}
	appendChild(c) {
		c.parentNode = this;
		this.childNodes.push(c);
		return c;
	}
	insertBefore(c, ref) {
		c.parentNode = this;
		const i = this.childNodes.indexOf(ref);
		if (i < 0) this.childNodes.push(c);
		else this.childNodes.splice(i, 0, c);
		return c;
	}
	removeChild(c) {
		const i = this.childNodes.indexOf(c);
		if (i >= 0) this.childNodes.splice(i, 1);
		c.parentNode = null;
		return c;
	}
	get firstChild() {
		return this.childNodes[0] || null;
	}
	contains(n) {
		for (let p = n; p; p = p.parentNode) if (p === this) return true;
		return false;
	}
	addEventListener(type, fn) {
		this._listeners = this._listeners || {};
		(this._listeners[type] = this._listeners[type] || []).push(fn);
	}
	_fire(type) {
		for (const fn of (this._listeners && this._listeners[type]) || []) fn();
	}
	setAttribute(k, v) {
		this.attributes.set(k, String(v));
		if (k === "class") this.classList = new ClassList(this);
	}
	getAttribute(k) {
		const v = this.attributes.get(k);
		return v === undefined ? null : v;
	}
	focus() {
		document.activeElement = this;
		document.focusLog.push(this);
	}
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
	querySelector(sel) {
		return this.querySelectorAll(sel)[0] || null;
	}
	set innerHTML(html) {
		this.childNodes = [];
		parseHTML(html, this);
	}
	get innerHTML() {
		return "";
	}
}

function matchTok(el, tok) {
	if (tok === ":hover") return !!el.hovered;
	const m = tok.match(
		/^(?:([a-zA-Z][\w-]*))?(?:#([\w-]+))?(?:\.([\w-]+))?(?:\[([-\w]+)(?:=(["']?)([^\]]*)\5)?\])?$/,
	);
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
doc.addEventListener = (type, fn) => {
	docListeners.push([type, fn]);
};

globalThis.document = doc;
globalThis.window = {
	location: { pathname: "/", search: "" },
	addEventListener: () => {},
};
globalThis.EventSource = class {
	constructor(url) {
		this.url = url;
	}
	addEventListener() {}
	close() {}
};
const realSetInterval = globalThis.setInterval;
const realSetTimeout = globalThis.setTimeout;
globalThis.setInterval = () => 0;
globalThis.setTimeout = () => 0;

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
	if (a !== b)
		throw new Error((msg || "eq") + ": got " + JSON.stringify(a) + ", want " + JSON.stringify(b));
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
	const el = fragEl(
		'<details open data-state-key="cell-1"><summary>r</summary><p>x</p></details>',
		"frag-t2",
	);
	fakeNow += 1000;
	apply({
		id: "frag-t2",
		html: '<details data-state-key="cell-1"><summary>r</summary><p>v1</p></details>',
	});
	eq(el.textContent.includes("v1"), false, "not swapped while busy");
	fakeNow += 1000;
	apply({
		id: "frag-t2",
		html: '<details data-state-key="cell-1"><summary>r</summary><p>v2</p></details>',
	});
	fakeNow += 1000;
	app.flushPendingSwap();
	eq(el.textContent.includes("v2"), false, "still inside grace window (v2 age 1s < 2.5s)");
	fakeNow += 1600;
	app.flushPendingSwap();
	eq(el.textContent.includes("v2"), true, "grace elapsed: LATEST payload lands");
	eq(
		el.querySelector('details[data-state-key="cell-1"]').open,
		true,
		"open state re-applied through the forced swap",
	);
});

test("focused input makes fragment busy until grace flush", () => {
	const el = fragEl('<input name="q" value="a">', "frag-t3");
	el.querySelector("input").focus();
	fakeNow += 5000;
	apply({ id: "frag-t3", html: '<input name="q" value="b"><i id="swapped-in"></i>' });
	eq(el.querySelector("#swapped-in"), null, "swap deferred while focused");
	doc.activeElement = null;
	fakeNow += 3000;
	app.flushPendingSwap();
	if (!el.querySelector("#swapped-in")) throw new Error("swap did not land after grace");
	eq(
		el.querySelector("input").value,
		"a",
		"operator-typed value survives the deferred swap (re-apply)",
	);
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
	click.forEach(([, fn]) =>
		fn({
			target: cell,
			preventDefault: () => {
				prevented = true;
			},
		}),
	);
	eq(cell.getAttribute("data-expanded"), "1", "delegated handler reaches the fresh subtree");
	eq(cell.textContent, "full message", "cell expanded with the full error text");
});

test("data-state-key re-applies details open state even when the summary text drifts", () => {
	const el = fragEl(
		'<details data-state-key="cancel-task-1" open><summary>Cancel (3)</summary></details>',
		"frag-t5",
	);
	app.swapIn(el, {
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
	app.swapIn(el, { html: '<input name="q" value="">' });
	const ae = doc.activeElement;
	if (!ae) throw new Error("focus lost after swap");
	eq(ae.name, "q", "focus restored to matching control");
	eq(el.contains(ae), true, "restored focus lives in the FRESH subtree");
});

test("scroll containers keep scrollTop; bottom-pinned panes stay pinned", () => {
	const el = fragEl(
		'<div class="journal-scroll"></div><div class="journal-scroll"></div>',
		"frag-t8",
	);
	const scrolls = el.querySelectorAll(".journal-scroll");
	scrolls[0].scrollTop = 120;
	scrolls[1].scrollTop = 980;
	scrolls[1].clientHeight = 1000;
	apply({
		id: "frag-t8",
		html: '<div class="journal-scroll"></div><div class="journal-scroll"></div>',
	});
	const fresh = el.querySelectorAll(".journal-scroll");
	eq(fresh[0].scrollTop, 120, "mid-scroll position preserved");
	eq(fresh[1].scrollTop, 1000, "at-bottom pane re-pinned to the new scrollHeight");
});

test("expanded error cells re-applied by data-short through the swap", () => {
	const el = fragEl(
		'<span data-error="boom" data-short="err" data-expanded="1">boom</span>',
		"frag-t9",
	);
	app.swapIn(el, { html: '<span data-error="boom" data-short="err" data-expanded="0">err</span>' });
	const cell = el.querySelector("[data-error]");
	eq(cell.getAttribute("data-expanded"), "1");
	eq(cell.textContent, "boom");
	eq(cell.getAttribute("aria-expanded"), "true");
});

test("gateway: TQ_WEBUIJS_FAIL=1 forces a failing test (gate self-test hook)", () => {
	if (process.env.TQ_WEBUIJS_FAIL === "1")
		throw new Error("intentional failure for gate self-test");
});

test("feed filter hides non-matching lines and flags the active chip", () => {
	const el = fragEl(
		'<div class="tq-feed-chips">' +
			'<button type="button" class="tq-feed-chip" data-feed-type="" aria-pressed="false">all</button>' +
			'<button type="button" class="tq-feed-chip" data-feed-type="task.claimed" aria-pressed="false">task.claimed</button>' +
			'<button type="button" class="tq-feed-chip" data-feed-type="task.completed" aria-pressed="false">task.completed</button>' +
			"</div>" +
			'<div class="journal-scroll"><div data-fact-type="task.claimed">c</div><div data-fact-type="task.completed">d</div></div>',
		"frag-feed",
	);
	app.setFeedFilter("task.claimed");
	eq(
		el.querySelector('[data-fact-type="task.claimed"]').hidden,
		false,
		"matching line stays visible",
	);
	eq(el.querySelector('[data-fact-type="task.completed"]').hidden, true, "non-matching line hides");
	eq(
		el.querySelector('[data-feed-type="task.claimed"]').classList.contains("tq-feed-chip-active"),
		true,
		"active chip flagged",
	);
	eq(
		el.querySelector('[data-feed-type="task.claimed"]').getAttribute("aria-pressed"),
		"true",
		"aria-pressed follows",
	);
	/* reset through the "all" chip */
	app.setFeedFilter("");
	eq(el.querySelector('[data-fact-type="task.completed"]').hidden, false, "reset shows every line");
	eq(
		el.querySelector('[data-feed-type=""]').classList.contains("tq-feed-chip-active"),
		true,
		"all chip owns the reset",
	);
	app.setFeedFilter("");
	doc.body.childNodes = doc.body.childNodes.filter((c) => c !== el);
});

test("feed filter re-applies after an SSE swap of frag-feed", () => {
	const el = fragEl(
		'<div class="journal-scroll"><div data-fact-type="task.claimed">c</div></div>',
		"frag-feed",
	);
	app.setFeedFilter("task.completed");
	apply({
		id: "frag-feed",
		html:
			'<div class="tq-feed-chips"><button type="button" class="tq-feed-chip" data-feed-type="" aria-pressed="false">all</button><button type="button" class="tq-feed-chip" data-feed-type="task.completed" aria-pressed="false">task.completed</button></div>' +
			'<div class="journal-scroll"><div data-fact-type="task.claimed">c</div><div data-fact-type="task.completed">d</div></div>',
	});
	eq(
		el.querySelector('[data-fact-type="task.claimed"]').hidden,
		true,
		"fresh line filtered by persisted selection",
	);
	eq(
		el.querySelector('[data-feed-type="task.completed"]').classList.contains("tq-feed-chip-active"),
		true,
		"fresh chip re-flagged",
	);
	app.setFeedFilter("");
	doc.body.childNodes = doc.body.childNodes.filter((c) => c !== el);
});

test("feed filter tolerates a missing feed and dead sessionStorage", () => {
	app.applyFeedFilter(); /* no frag-feed on the stub page: no throw */
	app.setFeedFilter("task.claimed");
	app.setFeedFilter("");
});

function feedHTML(types, seqBase = 0) {
	return (
		'<div class="tq-feed-chips"><button type="button" class="tq-feed-chip" data-feed-type="">all</button></div>' +
		'<div class="journal-scroll">' +
		types
			.map((t, i) => `<div data-fact-type="${t}" data-seq="${seqBase + i + 1}">${t} tail</div>`)
			.join("") +
		"</div>"
	);
}

function feedFixture(types, seqBase = 0) {
	const el = fragEl(feedHTML(types, seqBase), "frag-feed");
	return el;
}

test("feed burst badge counts lines added since the pre-swap last line", () => {
	const el = feedFixture(["a", "b"]);
	app.swapIn(el, { html: feedHTML(["a", "b", "c", "d"], 0) });
	const badge = el.querySelector(".tq-feed-burst");
	eq(badge.textContent, "2 new facts", "burst badge counts new lines");
	eq(
		badge.getAttribute("class"),
		"tq-feed-burst",
		"badge carries its own ephemeral class, not the chip pill",
	);
	eq(badge.getAttribute("aria-live"), "polite", "badge announces politely");
	doc.body.childNodes = doc.body.childNodes.filter((c) => c !== el);
});

test("feed burst count is exact across a full window shift via data-seq", () => {
	const el = feedFixture(["a", "b"]);
	/* prev tail seq 2; the new window shows seqs 10..11 but 9 facts
	   streamed in (seq gap 9): the exact count is 9, not the window-size
	   bound 2. */
	app.swapIn(el, { html: feedHTML(["x", "y"], 9) });
	const badge = el.querySelector(".tq-feed-burst");
	eq(badge.textContent, "9 new facts", "seq gap counts every missed fact");
	doc.body.childNodes = doc.body.childNodes.filter((c) => c !== el);
});

test("feed burst badge clears on a zero-new swap", () => {
	const el = feedFixture(["a", "b"]);
	app.swapIn(el, { html: feedHTML(["x", "y"], 8) });
	const badge = el.querySelector(".tq-feed-burst");
	eq(badge.textContent, "8 new facts", "window shifted: seq gap counts both");
	app.swapIn(el, { html: feedHTML(["x", "y"], 8) });
	eq(el.querySelector(".tq-feed-burst"), null, "zero-new swap clears the badge");
	doc.body.childNodes = doc.body.childNodes.filter((c) => c !== el);
});

test("pause-on-hover: a hovered feed is not re-pinned by a burst", () => {
	const el = feedFixture(["a", "b"]);
	const pane = el.querySelector(".journal-scroll");
	pane.scrollTop = 600; /* atBottom per stub geometry (1000-500-24) */
	el.hovered = true;
	app.swapIn(el, { html: feedHTML(["a", "b", "c"]) });
	eq(
		el.querySelector(".journal-scroll").scrollTop,
		600,
		"hovered burst restores position, withholds the pin",
	);
	el.hovered = false;
	app.swapIn(el, { html: feedHTML(["a", "b", "c"], 2) });
	eq(el.querySelector(".journal-scroll").scrollTop, 1000, "unhovered burst resumes the tail pin");
	doc.body.childNodes = doc.body.childNodes.filter((c) => c !== el);
});

test("pause-on-hover: hovered bursts accumulate and resurface on un-hover", () => {
	const el = feedFixture(["a", "b"]);
	el.hovered = true;
	app.swapIn(el, { html: feedHTML(["a", "b", "c"], 0) });
	let badge = el.querySelector(".tq-feed-burst");
	eq(badge.textContent, "1 new facts", "hovered burst still shows its own count");
	app.swapIn(el, { html: feedHTML(["a", "b", "c", "d", "e", "f", "g", "h", "i", "j"], 3) });
	el._fire("mouseleave");
	badge = el.querySelector(".tq-feed-burst");
	eq(badge.textContent, "12 new while paused", "un-hover resurfaces the accumulated total");
	el.hovered = false;
	app.swapIn(el, { html: feedHTML(["a", "b", "c", "d", "e", "f", "g", "h", "i", "j"], 3) });
	eq(el.querySelector(".tq-feed-burst"), null, "next unhovered zero-new swap clears the badge");
	doc.body.childNodes = doc.body.childNodes.filter((c) => c !== el);
});

test("pause-on-hover: un-hover without hovered bursts leaves no badge", () => {
	const el = feedFixture(["a", "b"]);
	app.swapIn(el, { html: feedHTML(["a", "b"], 0) });
	el._fire("mouseleave");
	eq(el.querySelector(".tq-feed-burst"), null, "no accumulation, no badge");
	doc.body.childNodes = doc.body.childNodes.filter((c) => c !== el);
});

if (failed > 0) process.exitCode = 1;
globalThis.setInterval = realSetInterval;
globalThis.setTimeout = realSetTimeout;
globalThis.Date.now = realDateNow;
console.log(`webui-js summary: ${ok} tests ok, ${failed} failed`);
if (process.exitCode) process.exit(process.exitCode);
