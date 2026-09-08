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

  function applyFragment(raw) {
    var frag;
    try {
      frag = JSON.parse(raw);
    } catch (e) {
      return;
    }
    var el = document.getElementById(frag.id);
    if (el) el.innerHTML = frag.html;
  }

  function connect() {
    /* Token-authenticated serves accept the token as a query param
       (EventSource cannot set headers); forward it when present. */
    var url = "/api/events";
    var token = new URLSearchParams(window.location.search).get("token");
    if (token) url += "?token=" + encodeURIComponent(token);
    var es = new EventSource(url);

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
  });
})();
