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

  document.querySelectorAll(".board-table thead a[data-sort-key]").forEach((link) => {
    link.addEventListener("click", (event) => {
      if (!event.shiftKey) return;
      event.preventDefault();
      const key = link.getAttribute("data-sort-key");
      if (!key) return;
      const url = new URL(window.location.href);
      const existing = (url.searchParams.get("sort") || "").split(",").map((part) => part.trim()).filter(Boolean);
      const withoutKey = existing.filter((part) => part.replace(/^-/, "") !== key);
      const current = existing.find((part) => part.replace(/^-/, "") === key);
      let next = key;
      if (current === key) next = `-${key}`;
      if (current === `-${key}`) next = "";
      const parts = next ? [...withoutKey, next] : withoutKey;
      if (parts.length) {
        url.searchParams.set("sort", parts.join(","));
      } else {
        url.searchParams.delete("sort");
      }
      url.searchParams.set("page", "1");
      window.location.assign(url.toString());
    });
  });

  document.querySelectorAll('input[data-board-search]').forEach((input) => {
    const selector = input.getAttribute("data-board-search");
    const table = selector ? document.querySelector(selector) : null;
    let timer = null;

    function applyClientSearch() {
      if (!table) return;
      const needle = input.value.trim().toLowerCase();
      table.querySelectorAll("tbody tr").forEach((row) => {
        row.hidden = needle !== "" && !(row.innerText || "").toLowerCase().includes(needle);
      });
    }

    input.addEventListener("input", () => {
      applyClientSearch();
      if (timer) window.clearTimeout(timer);
      timer = window.setTimeout(() => {
        const url = new URL(window.location.href);
        const value = input.value.trim();
        if (value) {
          url.searchParams.set("q", value);
        } else {
          url.searchParams.delete("q");
        }
        url.searchParams.set("page", "1");
        window.history.replaceState({}, "", url.toString());
      }, 250);
    });
  });

  document.querySelectorAll(".board-table[data-virtual-window]").forEach((table) => {
    const tbody = table.querySelector("tbody");
    const rows = Array.from(tbody?.querySelectorAll("tr") || []);
    const windowSize = Number(table.getAttribute("data-virtual-window")) || 200;
    const rowHeight = 42;
    const summary = table.closest(".task-table-section")?.querySelector("[data-virtual-summary]");
    if (!tbody || rows.length <= windowSize) return;
    table.classList.add("virtualized");
    tbody.style.position = "relative";
    tbody.style.display = "block";
    tbody.style.height = `${rows.length * rowHeight}px`;
    rows.forEach((row) => {
      row.style.position = "absolute";
      row.style.left = "0";
      row.style.right = "0";
      row.style.height = `${rowHeight}px`;
    });

    function renderWindow() {
      const tableRect = table.getBoundingClientRect();
      const viewportTop = -tableRect.top;
      const start = Math.max(0, Math.floor(viewportTop / rowHeight) - 10);
      const end = Math.min(rows.length, start + windowSize);
      rows.forEach((row, index) => {
        const visible = index >= start && index < end;
        row.hidden = !visible;
        if (visible) row.style.transform = `translateY(${index * rowHeight}px)`;
      });
      if (summary) {
        summary.textContent = `showing ${start + 1}-${end} of ${rows.length} filtered tasks`;
      }
    }

    renderWindow();
    window.addEventListener("scroll", renderWindow, { passive: true });
    window.addEventListener("resize", renderWindow);
  });

  document.querySelectorAll(".board-table").forEach((table) => {
    const selectAll = table.querySelector('thead input[type="checkbox"]');
    const rowChecks = Array.from(table.querySelectorAll('tbody input[type="checkbox"]'));
    const selectionBar = table.closest(".task-table-section")?.querySelector(".selection-bar");
    const selectionCount = selectionBar?.querySelector("span");
    const clearButton = selectionBar?.querySelector("[data-selection-clear]");

    function selectedTaskIDs() {
      return rowChecks.filter((check) => check.checked).map((check) => check.value).filter(Boolean);
    }

    function updateSelection() {
      const selected = selectedTaskIDs().length;
      if (selectionBar) selectionBar.hidden = selected === 0;
      if (selectionCount) selectionCount.textContent = `${selected} selected`;
      if (selectAll) {
        selectAll.checked = selected > 0 && selected === rowChecks.length;
        selectAll.indeterminate = selected > 0 && selected < rowChecks.length;
      }
    }

    if (selectAll) {
      selectAll.addEventListener("change", () => {
        rowChecks.forEach((check) => {
          check.checked = selectAll.checked;
        });
        updateSelection();
      });
    }
    rowChecks.forEach((check) => check.addEventListener("change", updateSelection));
    if (clearButton) {
      clearButton.addEventListener("click", () => {
        rowChecks.forEach((check) => {
          check.checked = false;
        });
        updateSelection();
      });
    }
    selectionBar?.querySelectorAll("[data-bulk-open]").forEach((button) => {
      button.addEventListener("click", () => {
        const ids = selectedTaskIDs();
        if (ids.length === 0) return;
        const dialog = document.getElementById(button.getAttribute("data-bulk-open"));
        const form = dialog?.querySelector("form[data-bulk-form]");
        if (!dialog || !form) return;
        form.querySelectorAll('input[name="task_id"]').forEach((input) => input.remove());
        ids.forEach((id) => {
          const input = document.createElement("input");
          input.type = "hidden";
          input.name = "task_id";
          input.value = id;
          form.appendChild(input);
        });
        const returnTo = form.querySelector('input[name="return_to"]');
        if (returnTo) returnTo.value = `${window.location.pathname}${window.location.search}`;
        const list = form.querySelector("[data-selected-task-list]");
        if (list) list.textContent = ids.join(", ");
        if (typeof dialog.showModal === "function") {
          dialog.showModal();
        }
      });
    });
    updateSelection();
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
