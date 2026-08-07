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

  function initSortableTables(root = document) {
    root.querySelectorAll("table[data-sortable]").forEach((table) => {
      if (table.dataset.sortableReady === "true") return;
      table.dataset.sortableReady = "true";
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
  }

  initSortableTables();

  document.querySelectorAll("[data-lazy-panel]").forEach((panel) => {
    const src = panel.getAttribute("data-src");
    if (!src) return;
    fetch(src, { headers: { "Accept": "text/html" } })
      .then((response) => {
        if (!response.ok) throw new Error(`HTTP ${response.status}`);
        return response.text();
      })
      .then((html) => {
        panel.innerHTML = html;
        initSortableTables(panel);
      })
      .catch((error) => {
        const status = panel.querySelector(".lazy-panel-status");
        if (status) status.textContent = `Diagnostics unavailable: ${error.message}`;
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

  document.querySelectorAll("form[data-saved-view-form]").forEach((form) => {
    form.addEventListener("submit", () => {
      const query = form.querySelector('input[name="query"]');
      const returnTo = form.querySelector('input[name="return_to"]');
      const url = new URL(window.location.href);
      url.searchParams.delete("page");
      if (query) query.value = url.searchParams.toString();
      if (returnTo) returnTo.value = `/board${url.search ? url.search : ""}`;
    });
  });

  window.addEventListener("keydown", (event) => {
    if (!(event.metaKey || event.ctrlKey) || event.altKey || event.shiftKey) return;
    const digit = Number(event.key);
    if (!Number.isInteger(digit) || digit < 1 || digit > 9) return;
    const link = document.querySelector(`[data-saved-view-shortcut="${digit}"]`);
    if (!link) return;
    event.preventDefault();
    window.location.assign(link.href);
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

  const boardTable = document.querySelector(".board-table");
  const boardRows = Array.from(boardTable?.querySelectorAll("tbody tr[data-board-row]") || []);
  let cursorIndex = boardRows.length ? 0 : -1;

  function isTextInput(el) {
    if (!el) return false;
    const tag = el.tagName ? el.tagName.toLowerCase() : "";
    return tag === "input" || tag === "textarea" || tag === "select" || el.isContentEditable;
  }

  function setCursor(index) {
    if (!boardRows.length) return;
    cursorIndex = Math.max(0, Math.min(index, boardRows.length - 1));
    boardRows.forEach((row, rowIndex) => {
      row.classList.toggle("cursor", rowIndex === cursorIndex);
      if (rowIndex === cursorIndex) row.scrollIntoView({ block: "nearest" });
    });
  }

  function cursorRow() {
    if (cursorIndex < 0) setCursor(0);
    return boardRows[cursorIndex] || null;
  }

  function closeMenusAndDialogs() {
    document.querySelectorAll("details[open]").forEach((details) => {
      details.open = false;
    });
    document.querySelectorAll("dialog[open]").forEach((dialog) => {
      if (typeof dialog.close === "function") dialog.close();
    });
  }

  function toggleDetails(selector) {
    const details = document.querySelector(selector);
    if (!details) return;
    details.open = !details.open;
  }

  function openCursorDialog(dialogID) {
    const row = cursorRow();
    if (!row) return;
    const checkbox = row.querySelector('input[name="task_id"]');
    if (!checkbox) return;
    checkbox.checked = true;
    checkbox.dispatchEvent(new Event("change", { bubbles: true }));
    document.querySelector(`[data-bulk-open="${dialogID}"]`)?.click();
  }

  function toggleCursorSelection() {
    const row = cursorRow();
    const checkbox = row?.querySelector('input[name="task_id"]');
    if (!checkbox) return;
    checkbox.checked = !checkbox.checked;
    checkbox.dispatchEvent(new Event("change", { bubbles: true }));
  }

  function toggleKeyboardHelp() {
    const dialog = document.getElementById("keyboard-help-dialog");
    if (!dialog) return;
    if (dialog.open) {
      dialog.close();
      return;
    }
    if (typeof dialog.showModal === "function") dialog.showModal();
  }

  document.querySelector("[data-keyboard-help-open]")?.addEventListener("click", toggleKeyboardHelp);
  setCursor(0);

  window.addEventListener("keydown", (event) => {
    if (isTextInput(document.activeElement)) return;
    if (event.key === "Escape") {
      closeMenusAndDialogs();
      prefix = "";
      return;
    }
    if (event.key === "g") {
      prefix = "g";
      return;
    }
    if (prefix === "g" && event.key === "w") {
      event.preventDefault();
      window.location.assign(document.body.dataset.wallHref || "/wall");
      return;
    }
    prefix = "";
    switch (event.key) {
      case "j":
        event.preventDefault();
        setCursor(cursorIndex + 1);
        break;
      case "k":
        event.preventDefault();
        setCursor(cursorIndex - 1);
        break;
      case "Enter": {
        const href = cursorRow()?.getAttribute("data-task-href");
        if (href) {
          event.preventDefault();
          window.location.assign(href);
        }
        break;
      }
      case "/":
        event.preventDefault();
        document.querySelector('input[data-board-search]')?.focus();
        break;
      case "c":
        event.preventDefault();
        toggleDetails('[data-keyboard-panel="columns"]');
        break;
      case "v":
        event.preventDefault();
        toggleDetails('[data-keyboard-panel="views"]');
        break;
      case "x":
        event.preventDefault();
        toggleCursorSelection();
        break;
      case "s":
        event.preventDefault();
        openCursorDialog("bulk-status-dialog");
        break;
      case "h":
        event.preventDefault();
        openCursorDialog("bulk-handoff-dialog");
        break;
      case "t":
        event.preventDefault();
        document.querySelector("[data-theme-toggle]")?.click();
        break;
      case "?":
        event.preventDefault();
        toggleKeyboardHelp();
        break;
    }
  });
})();
