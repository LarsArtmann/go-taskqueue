/* tq live dashboard client: SSE-driven fragment swapping. */
(function () {
  "use strict";

  var conn = document.getElementById("conn");
  var staleTimer = null;

  function setConn(state) {
    if (!conn) return;
    conn.textContent = state;
    conn.className = "conn conn-" + state;
  }

  function markFresh() {
    setConn("live");
    if (staleTimer) clearTimeout(staleTimer);
    /* no update for 3x the heartbeat interval => connection is stale */
    staleTimer = setTimeout(function () {
      setConn("stale");
    }, 45000);
  }

  /* Swap-guard: a fragment the operator is interacting with (text entry,
     open cancel/reason form, expanded error cell) must not be replaced
     mid-interaction. Skipped ids queue their LATEST payload and flush on
     the next tick after a short grace window — data never goes stale by
     more than ~2 ticks; the re-apply below preserves the open state when
     the grace period forces the swap through. */
  var pendingSwap = {};
  var FLUSH_GRACE_MS = 2500;

  function fragmentBusy(el) {
    var ae = document.activeElement;
    if (ae && el.contains(ae)) {
      var tag = (ae.tagName || "").toLowerCase();
      if (tag === "input" || tag === "select" || tag === "textarea" || ae.isContentEditable)
        return true;
    }
    if (el.querySelector("details[open]")) return true;
    if (el.querySelector('[data-expanded="1"]')) return true;
    return false;
  }

  /* State re-apply: capture the bits the swap would reset — per-<details>
     open state (matched by data-state-key or summary text, stable across
     re-renders), expanded error cells (matched by their short text),
     form-control values the operator typed (matched by name/id), and
     keyboard focus — then put them back on the fresh subtree. Scroll
     containers keep their scrollTop. */
  function controlKey(c) {
    return c.getAttribute("data-state-key") || c.name || c.id || "";
  }

  function captureState(el) {
    var st = { details: {}, expanded: {}, scrolls: [], values: {}, focus: "" };
    el.querySelectorAll("details").forEach(function (d) {
      var key = d.getAttribute("data-state-key");
      if (!key) {
        var s = d.querySelector("summary");
        key = s ? s.textContent.trim() : d.id || d.className;
      }
      st.details[key] = d.open;
    });
    el.querySelectorAll('[data-expanded="1"]').forEach(function (c) {
      st.expanded[c.getAttribute("data-short") || c.textContent] = true;
    });
    el.querySelectorAll("input, select, textarea").forEach(function (c) {
      var t = (c.type || "").toLowerCase();
      if (t === "hidden" || t === "submit" || t === "button") return;
      var key = controlKey(c);
      if (key) st.values[key] = c.value;
    });
    var ae = document.activeElement;
    if (ae && el.contains(ae)) st.focus = controlKey(ae) || ae.tagName;
    el.querySelectorAll(".journal-scroll").forEach(function (s) {
      st.scrolls.push(s.scrollTop);
    });
    return st;
  }

  function restoreState(el, st) {
    el.querySelectorAll("details").forEach(function (d) {
      var key = d.getAttribute("data-state-key");
      if (!key) {
        var s = d.querySelector("summary");
        key = s ? s.textContent.trim() : d.id || d.className;
      }
      if (st.details[key]) d.open = true;
    });
    el.querySelectorAll("[data-error]").forEach(function (c) {
      var short = c.getAttribute("data-short") || "";
      if (st.expanded[short] && c.getAttribute("data-expanded") !== "1") {
        c.textContent = c.getAttribute("data-error") || "";
        c.setAttribute("data-expanded", "1");
        c.setAttribute("aria-expanded", "true");
      }
    });
    el.querySelectorAll("input, select, textarea").forEach(function (c) {
      var t = (c.type || "").toLowerCase();
      if (t === "hidden" || t === "submit" || t === "button") return;
      var key = controlKey(c);
      if (key && Object.prototype.hasOwnProperty.call(st.values, key)) c.value = st.values[key];
    });
    var scrolls = el.querySelectorAll(".journal-scroll");
    for (var i = 0; i < scrolls.length && i < st.scrolls.length; i++) {
      scrolls[i].scrollTop = st.scrolls[i];
    }
    if (st.focus) {
      try {
        var target = el.querySelector(
          '[data-state-key="' + st.focus + '"], [name="' + st.focus + '"], #' + st.focus,
        );
        if (target) target.focus();
      } catch (e) {
        /* malformed key: skip focus restore */
      }
    }
  }

  function swapIn(el, frag) {
    var st = captureState(el);
    el.innerHTML = frag.html;
    restoreState(el, st);
    updateBoardAffordance();
  }

  /* Board scroll affordance: the edge fade (theme.css .board-overflow) is
     honest — it only appears when the lane strip actually overflows. */
  function updateBoardAffordance() {
    var boards = document.querySelectorAll(".board");
    for (var i = 0; i < boards.length; i++) {
      var b = boards[i];
      if (b.scrollWidth > b.clientWidth + 4) b.classList.add("board-overflow");
      else b.classList.remove("board-overflow");
    }
  }
  window.addEventListener("resize", updateBoardAffordance);
  document.addEventListener("DOMContentLoaded", updateBoardAffordance);
  updateBoardAffordance();

  function flushPendingSwap() {
    var now = Date.now();
    var youngest = Infinity;
    for (var id in pendingSwap) {
      var entry = pendingSwap[id];
      var age = now - entry.at;
      if (age < FLUSH_GRACE_MS) {
        youngest = Math.min(youngest, FLUSH_GRACE_MS - age);
        continue;
      }
      var el = document.getElementById(id);
      if (!el) {
        delete pendingSwap[id];
        continue;
      }
      swapIn(el, entry.frag);
      delete pendingSwap[id];
    }
    pendingTimer = youngest < Infinity ? setTimeout(flushPendingSwap, youngest + 50) : null;
  }
  var pendingTimer = null;

  function applyFragment(raw) {
    var frag;
    try {
      frag = JSON.parse(raw);
    } catch (e) {
      return;
    }
    var el = document.getElementById(frag.id);
    if (!el) return;
    if (fragmentBusy(el)) {
      pendingSwap[frag.id] = { frag: frag, at: Date.now() };
      if (!pendingTimer) pendingTimer = setTimeout(flushPendingSwap, FLUSH_GRACE_MS);
      return;
    }
    swapIn(el, frag);
  }

  /* Live relative ages between SSE bursts: cells carry their wall time in
     data-age (ms epoch); a 30s ticker re-renders "3m" -> "4m" without a
     server round-trip. Reduced-motion users still get correct text. */
  function fmtAge(ms) {
    var s = Math.max(0, Math.floor((Date.now() - ms) / 1000));
    if (s < 60) return s + "s";
    if (s < 3600) return Math.floor(s / 60) + "m";
    if (s < 86400) return Math.floor(s / 3600) + "h";
    return Math.floor(s / 86400) + "d";
  }

  function tickAges() {
    var cells = document.querySelectorAll("[data-age]");
    for (var i = 0; i < cells.length; i++) {
      var ms = parseInt(cells[i].getAttribute("data-age"), 10);
      if (!isNaN(ms)) cells[i].textContent = fmtAge(ms);
    }
  }

  setInterval(tickAges, 30000);

  /* Error cells expand on click (or Enter/Space when focused): the full
     message lives in title=/data-error, the toggle swaps it into view.
     aria-expanded tracks the state for assistive tech. */
  function toggleErrorCell(cell) {
    var full = cell.getAttribute("data-error") || "";
    var short = cell.getAttribute("data-short") || cell.textContent;
    var expanded = cell.getAttribute("data-expanded") === "1";
    if (expanded) {
      cell.textContent = short;
      cell.setAttribute("data-expanded", "0");
      cell.setAttribute("aria-expanded", "false");
    } else {
      cell.textContent = full;
      cell.setAttribute("data-expanded", "1");
      cell.setAttribute("aria-expanded", "true");
    }
  }

  document.addEventListener("click", function (e) {
    var cell = e.target.closest ? e.target.closest("[data-error]") : null;
    if (!cell) return;
    toggleErrorCell(cell);
  });

  document.addEventListener("keydown", function (e) {
    if (e.key !== "Enter" && e.key !== " ") return;
    var cell = e.target.closest ? e.target.closest("[data-error]") : null;
    if (!cell) return;
    e.preventDefault();
    toggleErrorCell(cell);
  });

  /* "?" toggles the keyboard-shortcut overlay. */
  var overlay = null;

  function shortcutOverlay() {
    if (overlay) return overlay;
    overlay = document.createElement("div");
    overlay.id = "shortcut-overlay";
    overlay.setAttribute("role", "dialog");
    overlay.setAttribute("aria-label", "keyboard shortcuts");
    overlay.style.cssText =
      "position:fixed;inset:0;z-index:50;display:none;align-items:center;justify-content:center;background:rgba(0,0,0,0.4)";
    overlay.addEventListener("click", function () {
      overlay.style.display = "none";
    });

    /* Built via the CSSOM, not innerHTML: the strict CSP blocks inline
       style attributes, but el.style assignments are always allowed. */
    var box = document.createElement("div");
    box.style.cssText =
      "max-width:22rem;padding:1.25rem;border-radius:0.5rem;background:#fff;color:#111;font-size:0.875rem";
    var list = document.createElement("table");
    var tbody = document.createElement("tbody");
    [
      ["1-4", "jump to section"],
      ["/", "focus search"],
      ["?", "this overlay"],
      ["Esc", "close"],
    ].forEach(function (row) {
      var tr = document.createElement("tr");
      var kbd = document.createElement("td");
      kbd.textContent = row[0];
      kbd.style.cssText = "font-family:monospace;padding-right:1rem";
      var desc = document.createElement("td");
      desc.textContent = row[1];
      tr.appendChild(kbd);
      tr.appendChild(desc);
      tbody.appendChild(tr);
    });
    list.appendChild(tbody);
    box.appendChild(list);
    overlay.appendChild(box);
    document.body.appendChild(overlay);
    return overlay;
  }

  /* The only page params /api/events understands: the view filter the
     server re-applies to every snapshot plus the token, which EventSource
     cannot send as a header. "view" keeps live ticks rendering whichever
     task projection (table or board) is on screen. */
  var STREAM_PARAMS = ["project", "status", "q", "page", "sort", "view", "token"];

  function streamURL() {
    var pageQuery = new URLSearchParams(window.location.search);

    /* Task detail pages stream their own two fragments from a task-scoped
       endpoint; the token still rides the query for EventSource. */
    var task = window.location.pathname.match(/^\/task\/([^/]+)\/?$/);
    if (task) {
      var tok = pageQuery.get("token");
      return "/task/" + task[1] + "/events" + (tok ? "?token=" + encodeURIComponent(tok) : "");
    }

    var params = new URLSearchParams();
    STREAM_PARAMS.forEach(function (key) {
      var value = pageQuery.get(key);
      if (value !== null) params.set(key, value);
    });
    var query = params.toString();
    return "/api/events" + (query ? "?" + query : "");
  }

  var es = null;

  function connect() {
    /* Forwarding the page's filter params keeps live ticks and reconnects
       scoped to the view on screen: the browser reuses this exact URL when
       reconnecting, so resume restores the same filtered projection
       instead of clobbering it with the unfiltered table. */
    es = new EventSource(streamURL());

    es.addEventListener("frag", function (e) {
      applyFragment(e.data);
      markFresh();
    });

    es.addEventListener("title", function (e) {
      document.title = e.data;
    });

    es.onopen = function () {
      markFresh();
    };
    es.onerror = function () {
      setConn("reconnecting");
      /* EventSource retries automatically; nothing else to do. */
    };
  }

  function reconnectStream() {
    if (es) es.close();
    connect();
  }

  /* Filter auto-submit (G1): the filter form applies via fetch + fragment
     swap + pushState — no full reload, the SSE stream re-scopes itself to
     the new filter from the pushed location. The swap-guard and state
     re-apply above apply to these swaps too. A seq counter drops stale
     responses when a debounce fires after an Enter submit. */
  var FRAG_IDS = ["frag-filters", "frag-stats", "frag-table", "frag-dlq"];
  var applySeq = 0;
  var searchDebounce = null;

  function formURL(form) {
    var params = new URLSearchParams();
    new FormData(form).forEach(function (value, key) {
      if (value !== "") params.set(key, value);
    });
    var qs = params.toString();
    return window.location.pathname + (qs ? "?" + qs : "");
  }

  function applyFilterURL(url, push) {
    var seq = ++applySeq;
    var btn = document.querySelector("#frag-filters button[type='submit']");
    if (btn) btn.classList.add("tq-busy");

    fetch(url, { headers: { "X-TQ-Fragment": "1" } })
      .then(function (r) {
        if (!r.ok) throw new Error("HTTP " + r.status);
        return r.text();
      })
      .then(function (html) {
        if (seq !== applySeq) return;
        var doc = new DOMParser().parseFromString(html, "text/html");
        FRAG_IDS.forEach(function (id) {
          var remote = doc.getElementById(id);
          var local = document.getElementById(id);
          if (remote && local) swapIn(local, { html: remote.innerHTML });
        });
        if (push) history.pushState({}, "", url);
        reconnectStream();
      })
      .catch(function () {
        /* Network or server hiccup: fall back to a full navigation so the
           operator's action still lands. */
        if (seq === applySeq) window.location.href = url;
      })
      .finally(function () {
        if (btn) btn.classList.remove("tq-busy");
      });
  }

  document.addEventListener("submit", function (e) {
    var form = e.target;
    if (!(form instanceof HTMLFormElement)) return;
    if ((form.getAttribute("method") || "get").toLowerCase() !== "get") return;
    if (!form.querySelector("#search-box, select[name='status'], select[name='band']")) return;
    e.preventDefault();
    if (searchDebounce) clearTimeout(searchDebounce);
    applyFilterURL(formURL(form), true);
  });

  document.addEventListener("input", function (e) {
    if (!e.target || e.target.id !== "search-box") return;
    if (searchDebounce) clearTimeout(searchDebounce);
    searchDebounce = setTimeout(function () {
      var form = e.target.closest("form");
      if (form) applyFilterURL(formURL(form), true);
    }, 300);
  });

  window.addEventListener("popstate", function () {
    applyFilterURL(window.location.pathname + window.location.search, false);
  });

  /* Full-journal browser: pages forward through history via the
     /api/facts cursor (the SSE feed above only carries the tail). Opens
     lazily on first toggle; "load older" pages BACKWARD from the live
     end (after=-N is the tail window; older pages are prepended). */
  var oldestLoaded = 0;
  var browserBusy = false;

  function factRow(f) {
    var div = document.createElement("div");
    div.className = "flex gap-2 py-0.5";
    var ts = document.createElement("span");
    ts.className = "text-gray-400 dark:text-gray-500";
    ts.textContent = (f.time || "").replace("T", " ").slice(0, 19);
    var tag = document.createElement("span");
    tag.className = "text-gray-600 dark:text-gray-300";
    tag.textContent = f.type;
    var id = document.createElement("a");
    id.className = "text-blue-600 hover:underline dark:text-blue-400";
    id.href = "/task/" + f.taskId;
    id.textContent = (f.taskId || "").slice(-8);
    div.appendChild(ts);
    div.appendChild(tag);
    div.appendChild(id);
    return div;
  }

  function loadJournalFacts() {
    var rows = document.getElementById("journal-browser-rows");
    var more = document.getElementById("journal-browser-more");
    if (!rows || browserBusy) return;
    browserBusy = true;
    var prepend = oldestLoaded > 0;
    var cursor = prepend ? Math.max(0, oldestLoaded - 100) : -100;
    fetch("/api/facts?after=" + cursor + "&limit=100")
      .then(function (r) {
        return r.json();
      })
      .then(function (page) {
        var facts = page.facts || [];
        facts.forEach(function (f) {
          if (prepend) rows.insertBefore(factRow(f), rows.firstChild);
          else rows.appendChild(factRow(f));
        });
        if (facts.length > 0) oldestLoaded = facts[0].seq;
        if (more) more.hidden = facts.length === 0 || oldestLoaded <= 1;
        browserBusy = false;
      })
      .catch(function () {
        browserBusy = false;
      });
  }

  document.addEventListener("toggle", function (e) {
    var det = e.target;
    if (det && det.id === "journal-browser" && det.open && !det.getAttribute("data-loaded")) {
      det.setAttribute("data-loaded", "1");
      loadJournalFacts();
    }
  });

  document.addEventListener("click", function (e) {
    if (e.target && e.target.id === "journal-browser-more") loadJournalFacts();
  });

  connect();

  /* keyboard navigation: 1-4 jump between sections, / focuses search. */
  document.addEventListener("keydown", function (e) {
    var tag = (e.target.tagName || "").toLowerCase();
    if (tag === "input" || tag === "select" || tag === "textarea") return;

    var sections = ["sec-overview", "sec-tasks", "sec-dlq", "sec-feed"];
    var idx = "1234".indexOf(e.key);

    if (idx >= 0 && sections[idx]) {
      var el = document.getElementById(sections[idx]);
      if (el) el.scrollIntoView({ behavior: "smooth" });

      return;
    }

    if (e.key === "/") {
      e.preventDefault();
      var box = document.getElementById("search-box");
      if (box) box.focus();
    }

    if (e.key === "?") {
      var ov = shortcutOverlay();
      ov.style.display = ov.style.display === "flex" ? "none" : "flex";
    }

    if (e.key === "Escape" && overlay) {
      overlay.style.display = "none";
    }
  });
})();
