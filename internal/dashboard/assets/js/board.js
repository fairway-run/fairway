(() => {
  let prefix = "";

  function sortTable(table, column, descending) {
    const tbody = table.querySelector("tbody");
    if (!tbody) return;
    const rows = Array.from(tbody.querySelectorAll("tr"));
    rows.sort((a, b) => {
      const left = (a.children[column]?.innerText || "").trim().toLowerCase();
      const right = (b.children[column]?.innerText || "").trim().toLowerCase();
      return left.localeCompare(right, undefined, { numeric: true }) * (descending ? -1 : 1);
    });
    rows.forEach((row) => tbody.appendChild(row));
  }

  document.querySelectorAll("table[data-sortable]").forEach((table) => {
    table.querySelectorAll("thead button[data-sort]").forEach((button) => {
      button.addEventListener("click", () => {
        const index = button.closest("th").cellIndex;
        const next = button.getAttribute("aria-sort") !== "ascending" ? "ascending" : "descending";
        table.querySelectorAll("thead button[data-sort]").forEach((other) => other.removeAttribute("aria-sort"));
        button.setAttribute("aria-sort", next);
        sortTable(table, index, next === "descending");
      });
    });
  });

  document.querySelectorAll("[data-export-table]").forEach((button) => {
    button.addEventListener("click", () => {
      const table = document.querySelector(button.getAttribute("data-export-table"));
      if (!table) return;
      const rows = Array.from(table.querySelectorAll("tr"));
      const csv = rows.map((row) => Array.from(row.children).slice(1).map((cell) => {
        const value = (cell.innerText || "").replace(/\\s+/g, " ").trim().replace(/"/g, '""');
        return `"${value}"`;
      }).join(",")).join("\\n");
      const blob = new Blob([csv], { type: "text/csv;charset=utf-8" });
      const url = URL.createObjectURL(blob);
      const link = document.createElement("a");
      link.href = url;
      link.download = "fairway-board.csv";
      link.click();
      URL.revokeObjectURL(url);
    });
  });

  function isTextInput(el) {
    if (!el) return false;
    const tag = el.tagName ? el.tagName.toLowerCase() : "";
    return tag === "input" || tag === "textarea" || tag === "select" || el.isContentEditable;
  }

  window.addEventListener("keydown", (event) => {
    if (isTextInput(document.activeElement)) return;
    if (event.key === "g") {
      prefix = "g";
      return;
    }
    if (prefix === "g" && event.key === "w") {
      event.preventDefault();
      window.location.assign("/");
      return;
    }
    prefix = "";
  });
})();
