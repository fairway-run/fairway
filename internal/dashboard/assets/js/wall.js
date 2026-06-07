(() => {
  const toggles = Array.from(document.querySelectorAll("[data-lane-toggle]"));
  const panels = Array.from(document.querySelectorAll("[data-lane-panel]"));

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
})();
