(() => {
  const key = "fairway.dashboard.theme";
  const root = document.documentElement;
  const toggle = document.querySelector("[data-theme-toggle]");

  function apply(theme) {
    const next = theme === "dark" ? "dark" : "light";
    root.dataset.theme = next;
    if (toggle) {
      toggle.textContent = next;
      toggle.setAttribute("aria-label", `Switch to ${next === "dark" ? "light" : "dark"} theme`);
    }
  }

  apply(localStorage.getItem(key) || root.dataset.theme || "light");

  if (toggle) {
    toggle.addEventListener("click", () => {
      const next = root.dataset.theme === "dark" ? "light" : "dark";
      localStorage.setItem(key, next);
      apply(next);
    });
  }
})();
