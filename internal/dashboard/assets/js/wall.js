(() => {
  const toggles = Array.from(document.querySelectorAll("[data-lane-toggle]"));
  const panels = Array.from(document.querySelectorAll("[data-lane-panel]"));
  const wall = document.querySelector(".wall-lanes");
  const ticker = document.querySelector("[data-wall-ticker]");
  const handoffCount = document.querySelector("[data-handoff-count]");
  const relativeTimes = new Set();

  function setExpanded(role) {
    panels.forEach((panel) => {
      panel.hidden = panel.getAttribute("data-lane-panel") !== role;
    });
    toggles.forEach((toggle) => {
      toggle.setAttribute("aria-expanded", String(toggle.getAttribute("data-lane-toggle") === role));
    });
  }

  toggles.forEach((toggle) => {
    toggle.addEventListener("click", () => {
      const role = toggle.getAttribute("data-lane-toggle");
      const expanded = toggle.getAttribute("aria-expanded") === "true";
      setExpanded(expanded ? "" : role);
    });
  });

  function ensureArcLayer() {
    if (!wall) return null;
    let svg = wall.querySelector("svg.handoff-arc-layer");
    if (svg) return svg;
    svg = document.createElementNS("http://www.w3.org/2000/svg", "svg");
    svg.classList.add("handoff-arc-layer");
    svg.setAttribute("aria-hidden", "true");
    wall.prepend(svg);
    return svg;
  }

  function laneCenter(role) {
    const row = document.querySelector(`[data-wall-role="${escapeIdent(role || "")}"]`);
    const svg = wall?.querySelector("svg.handoff-arc-layer");
    if (!row || !svg) return null;
    const rowRect = row.getBoundingClientRect();
    const svgRect = svg.getBoundingClientRect();
    return {
      x: rowRect.left - svgRect.left + rowRect.width / 2,
      y: rowRect.top - svgRect.top + rowRect.height / 2,
    };
  }

  function drawHandoffArc(event) {
    const from = event.from_role || event.role || event.actor || "";
    const to = event.to_role || "";
    if (!from || !to) return;
    const svg = ensureArcLayer();
    if (!svg) return;
    const fromPoint = laneCenter(from);
    const toPoint = laneCenter(to);
    if (!fromPoint || !toPoint) return;
    const width = Math.max(wall.scrollWidth, wall.clientWidth);
    const height = Math.max(wall.scrollHeight, wall.clientHeight);
    svg.setAttribute("viewBox", `0 0 ${width} ${height}`);
    const path = document.createElementNS("http://www.w3.org/2000/svg", "path");
    const midX = (fromPoint.x + toPoint.x) / 2;
    const lift = Math.max(32, Math.abs(toPoint.y - fromPoint.y) / 2);
    path.setAttribute("d", `M ${fromPoint.x} ${fromPoint.y} Q ${midX} ${Math.min(fromPoint.y, toPoint.y) - lift} ${toPoint.x} ${toPoint.y}`);
    path.classList.add("handoff-arc");
    if (event.provider) path.classList.add(`provider-${safeClass(event.provider)}`);
    svg.appendChild(path);
    window.setTimeout(() => path.remove(), 2200);
    if (handoffCount) {
      const next = Number(handoffCount.textContent || "0") + 1;
      handoffCount.textContent = String(next);
    }
  }

  function safeClass(value) {
    return String(value || "").toLowerCase().replace(/[^a-z0-9_-]+/g, "-").replace(/^-|-$/g, "") || "default";
  }

  function escapeIdent(value) {
    if (window.CSS && typeof window.CSS.escape === "function") return window.CSS.escape(String(value));
    return String(value).replace(/["\\]/g, "\\$&");
  }

  function eventPhrase(name, event) {
    const provider = event.provider || event.actor || event.role || "Fairway";
    const task = event.task_id ? ` ${event.task_id}` : "";
    switch (name) {
      case "claim":
        return `${provider} claimed${task}`;
      case "status_change":
        return `${provider} moved${task} ${event.from || "unknown"} -> ${event.to || "unknown"}`;
      case "handoff":
        return `${provider} handed off${task} ${event.from_role || "unknown"} -> ${event.to_role || "unknown"}`;
      case "evidence":
        return `${provider} recorded ${event.kind || "evidence"} evidence${task}`;
      case "review_verdict":
        return `${provider} reviewed${task}: ${event.verdict || "verdict"}`;
      case "done":
        return `${provider} completed${task}`;
      case "session_attach":
        return `${provider} attached ${event.session_id || "session"}${task}`;
      case "session_heartbeat":
        return `${provider} heartbeat ${event.session_id || "session"}${task}`;
      case "session_detach":
        return `${provider} detached ${event.session_id || "session"}`;
      case "gate_change":
        return `${provider} updated ${event.profile || "profile"} / ${event.gate || "gate"}`;
      default:
        return `${provider} emitted ${name}${task}`;
    }
  }

  function addTickerEntry(name, event) {
    if (!ticker) return;
    const article = document.createElement("article");
    article.className = "ticker-entry live";
    const who = document.createElement("b");
    who.className = `who provider-${safeClass(event.provider || event.actor || event.role || name)}`;
    who.textContent = event.provider || event.actor || event.role || name;
    const what = document.createElement("span");
    what.className = "what";
    what.textContent = eventPhrase(name, event);
    const time = document.createElement("time");
    time.dateTime = event.at || new Date().toISOString();
    time.dataset.relativeTime = time.dateTime;
    time.textContent = relativeTime(time.dateTime);
    relativeTimes.add(time);
    article.append(who, what, time);
    ticker.prepend(article);
    Array.from(ticker.querySelectorAll(".ticker-entry")).slice(6).forEach((entry) => entry.remove());
  }

  function relativeTime(value) {
    const then = Date.parse(value);
    if (!Number.isFinite(then)) return value || "now";
    const seconds = Math.max(0, Math.floor((Date.now() - then) / 1000));
    if (seconds < 5) return "now";
    if (seconds < 60) return `${seconds}s ago`;
    const minutes = Math.floor(seconds / 60);
    if (minutes < 60) return `${minutes}m ago`;
    const hours = Math.floor(minutes / 60);
    return `${hours}h ago`;
  }

  function refreshRelativeTimes() {
    document.querySelectorAll("time[data-relative-time]").forEach((time) => relativeTimes.add(time));
    relativeTimes.forEach((time) => {
      if (!time.isConnected) {
        relativeTimes.delete(time);
        return;
      }
      time.textContent = relativeTime(time.dataset.relativeTime || time.dateTime);
    });
  }

  function markHeartbeat(event) {
    const selector = event.task_id ? `[data-task-id="${escapeIdent(event.task_id)}"]` : `[data-task-role="${escapeIdent(event.role || "")}"].working`;
    const pill = document.querySelector(selector);
    if (!pill) return;
    pill.dataset.heartbeatAt = event.at || new Date().toISOString();
    updateHeartbeatPill(pill);
  }

  function updateHeartbeatPill(pill) {
    const then = Date.parse(pill.dataset.heartbeatAt || "");
    pill.classList.remove("heartbeat-fresh", "heartbeat-warm", "heartbeat-stale");
    if (!Number.isFinite(then)) return;
    const age = Date.now() - then;
    if (age < 60_000) {
      pill.classList.add("heartbeat-fresh");
    } else if (age < 300_000) {
      pill.classList.add("heartbeat-warm");
    } else {
      pill.classList.add("heartbeat-stale");
    }
  }

  function refreshHeartbeatPills() {
    document.querySelectorAll("[data-heartbeat-at]").forEach(updateHeartbeatPill);
  }

  function handleEvent(name, event) {
    if (name === "handoff") drawHandoffArc(event);
    if (name === "session_heartbeat") markHeartbeat(event);
    addTickerEntry(name, event);
  }

  if (window.EventSource) {
    const source = new EventSource("/events");
    ["claim", "status_change", "handoff", "evidence", "review_verdict", "done", "session_attach", "session_heartbeat", "session_detach", "gate_change"].forEach((name) => {
      source.addEventListener(name, (message) => {
        try {
          handleEvent(name, JSON.parse(message.data || "{}"));
        } catch (_err) {
          // Ignore malformed event payloads; the legacy refresh fallback remains server-side.
        }
      });
    });
  }

  refreshRelativeTimes();
  refreshHeartbeatPills();
  window.setInterval(refreshRelativeTimes, 30_000);
  window.setInterval(refreshHeartbeatPills, 15_000);
})();
