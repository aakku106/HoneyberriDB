/* HoneyberriDB function-call visualizer */
(function () {
  "use strict";

  const graph = window.CALL_GRAPH;
  if (!graph || !Array.isArray(graph.functions)) {
    console.error("Call graph data is missing. Expected call-graph-data.js to load first.");
    return;
  }

  const functions = graph.functions;
  const phases = graph.phases || {};
  const byId = new Map(functions.map((fn) => [fn.id, fn]));
  const callers = new Map(functions.map((fn) => [fn.id, []]));

  functions.forEach((fn) => {
    fn.calls.forEach((calleeId) => {
      const target = byId.get(calleeId);
      if (!target) {
        console.warn("Unknown callee in call graph:", fn.id, "→", calleeId);
        return;
      }
      callers.get(calleeId).push(fn.id);
    });
  });

  const escapeHTML = (value) =>
    String(value).replace(/[&<>"']/g, (character) => {
      const replacements = {
        "&": "&amp;",
        "<": "&lt;",
        ">": "&gt;",
        '"': "&quot;",
        "'": "&#39;"
      };
      return replacements[character];
    });

  const plural = (count, singular, pluralForm) => `${count} ${count === 1 ? singular : pluralForm}`;
  const phaseLabel = (phase) => phases[phase] || phase;
  const phaseStyle = (phase) => `--phase: var(--phase-${phase})`;
  const lineRange = (fn) => `${fn.lines[0]}–${fn.lines[1]}`;
  const displayReceiver = (fn) => (fn.receiver ? `${fn.receiver} method` : "function");

  const searchBlob = (fn) =>
    [
      fn.name,
      fn.receiver || "",
      fn.kind,
      fn.role,
      fn.phase,
      phaseLabel(fn.phase),
      fn.signature,
      fn.summary,
      fn.note,
      fn.tags.join(" "),
      fn.stdlib.join(" "),
      fn.flags.join(" ")
    ]
      .join(" ")
      .toLowerCase();

  const searchIndex = new Map(functions.map((fn) => [fn.id, searchBlob(fn)]));

  /* -----------------------------------------------------------------------
     Static triggers
     ----------------------------------------------------------------------- */

  document.querySelectorAll("[data-fn]").forEach((trigger) => {
    const fn = byId.get(trigger.dataset.fn);
    if (!fn) {
      return;
    }
    trigger.style.setProperty("--phase", `var(--phase-${fn.phase})`);
    if (trigger.matches("button")) {
      trigger.setAttribute("aria-haspopup", "dialog");
    }
  });

  /* -----------------------------------------------------------------------
     Tabs
     ----------------------------------------------------------------------- */

  const tabs = Array.from(document.querySelectorAll("[data-tab]"));
  const panels = {
    flow: document.getElementById("panel-flow"),
    tree: document.getElementById("panel-tree"),
    index: document.getElementById("panel-index")
  };
  let activeTab = "flow";

  function activateTab(name, options = {}) {
    const { focus = false, updateHash = true } = options;
    if (!panels[name]) {
      return;
    }

    activeTab = name;
    tabs.forEach((tab) => {
      const isActive = tab.dataset.tab === name;
      tab.classList.toggle("is-active", isActive);
      tab.setAttribute("aria-selected", String(isActive));
      tab.tabIndex = isActive ? 0 : -1;
      if (isActive && focus) {
        tab.focus();
      }
    });

    Object.entries(panels).forEach(([panelName, panel]) => {
      panel.hidden = panelName !== name;
    });

    if (updateHash && window.history && history.replaceState) {
      try {
        history.replaceState(null, "", `#${name}`);
      } catch (error) {
        /* Some file:// contexts disallow history updates; tabs still work. */
      }
    }
  }

  tabs.forEach((tab) => {
    tab.addEventListener("click", () => activateTab(tab.dataset.tab));
    tab.addEventListener("keydown", (event) => {
      const currentIndex = tabs.indexOf(tab);
      let nextIndex = currentIndex;

      if (event.key === "ArrowRight" || event.key === "ArrowDown") {
        nextIndex = (currentIndex + 1) % tabs.length;
      } else if (event.key === "ArrowLeft" || event.key === "ArrowUp") {
        nextIndex = (currentIndex - 1 + tabs.length) % tabs.length;
      } else if (event.key === "Home") {
        nextIndex = 0;
      } else if (event.key === "End") {
        nextIndex = tabs.length - 1;
      } else {
        return;
      }

      event.preventDefault();
      activateTab(tabs[nextIndex].dataset.tab, { focus: true });
    });
  });

  const initialTab = window.location.hash.replace(/^#/, "");
  if (panels[initialTab]) {
    activateTab(initialTab, { updateHash: false });
  }

  /* -----------------------------------------------------------------------
     Dialog
     ----------------------------------------------------------------------- */

  const dialog = document.getElementById("fnDialog");
  const dialogBody = document.getElementById("fnDialogBody");
  const dialogCard = dialog ? dialog.querySelector(".dialog-card") : null;

  function chipFor(id) {
    const fn = byId.get(id);
    if (!fn) {
      return "";
    }
    return `<button type="button" class="chip-link" data-fn="${escapeHTML(fn.id)}" style="${phaseStyle(fn.phase)}" aria-haspopup="dialog">${escapeHTML(fn.name)}</button>`;
  }

  function renderDialog(fn) {
    if (!dialogBody) {
      return;
    }

    const directCallers = callers.get(fn.id) || [];
    const callCount = fn.calls.length;
    const callerCount = directCallers.length;
    const isRoot = fn.id === "main";
    const flags = Array.from(new Set(fn.flags || []));
    const tags = Array.from(new Set(fn.tags || []));

    if (dialogCard) {
      dialogCard.style.setProperty("--phase", `var(--phase-${fn.phase})`);
      dialogCard.scrollTop = 0;
    }

    const meta = [
      `<span class="detail-meta__item">lines ${lineRange(fn)}</span>`,
      `<span class="detail-meta__item">${escapeHTML(displayReceiver(fn))}</span>`,
      `<span class="detail-meta__item">${cn(callCount, "callee", "callees")}</span>`,
      `<span class="detail-meta__item">${cn(callerCount, "caller", "callers")}</span>`
    ];

    if (flags.includes("unused")) {
      meta.push('<span class="detail-meta__item">unused by CLI</span>');
    }

    const callsMarkup = callCount
      ? `<div class="chip-list">${fn.calls.map(chipFor).join("")}</div>`
      : '<p class="detail-empty">No project-internal callees. Any standard-library calls are listed below.</p>';

    const callersMarkup = callerCount
      ? `<div class="chip-list">${directCallers.map(chipFor).join("")}</div>`
      : `<p class="detail-empty">${isRoot ? "Entry point: nothing in this program calls it." : "No project-internal caller recorded."}</p>`;

    const stdlibMarkup = fn.stdlib.length
      ? `<div class="stdlib-list">${fn.stdlib.map((call) => `<code>${escapeHTML(call)}</code>`).join("")}</div>`
      : '<p class="detail-empty">No standard-library calls recorded.</p>';

    const tagsMarkup = tags.length
      ? `<div class="detail-meta" aria-label="Tags">${tags.map((tag) => `<span class="detail-meta__item">#${escapeHTML(tag)}</span>`).join("")}</div>`
      : "";

    dialogBody.innerHTML = `
      <div class="detail-eyebrow">
        <span class="detail-eyebrow__phase" style="${phaseStyle(fn.phase)}">${escapeHTML(phaseLabel(fn.phase))}</span>
        <span class="detail-eyebrow__dot" aria-hidden="true"></span>
        <span>${escapeHTML(fn.role)}</span>
        <span class="detail-eyebrow__dot" aria-hidden="true"></span>
        <span>${escapeHTML(fn.kind)}</span>
      </div>

      <h2 class="detail-title" id="fnDialogTitle">
        <code>${escapeHTML(fn.name)}</code>
        ${fn.receiver ? `<span class="detail-receiver">${escapeHTML(fn.receiver)}</span>` : ""}
      </h2>

      <code class="detail-signature">${escapeHTML(fn.signature)}</code>
      <p class="detail-summary">${escapeHTML(fn.summary)}</p>
      <div class="detail-meta">${meta.join("")}</div>
      ${tagsMarkup}

      <section class="detail-section" aria-labelledby="detail-calls">
        <h4 id="detail-calls">Calls <span>${callCount ? `${callCount} internal` : "none internal"}</span></h4>
        ${callsMarkup}
      </section>

      <section class="detail-section" aria-labelledby="detail-callers">
        <h4 id="detail-callers">Called by <span>${callerCount ? `${callerCount} direct` : "no project caller"}</span></h4>
        ${callersMarkup}
      </section>

      <section class="detail-section" aria-labelledby="detail-stdlib">
        <h4 id="detail-stdlib">Standard library / OS</h4>
        ${stdlibMarkup}
      </section>

      ${fn.note ? `
        <section class="detail-section" aria-labelledby="detail-notes">
          <h4 id="detail-notes">Implementation note</h4>
          <p class="detail-note">${escapeHTML(fn.note)}</p>
        </section>
      ` : ""}

      <div class="detail-actions">
        <button class="primary-button" type="button" data-reveal="${escapeHTML(fn.id)}">
          Show in call tree <span aria-hidden="true">↘</span>
        </button>
        <a class="ghost-button" href="../../main.go" target="_blank" rel="noopener">Open main.go ↗</a>
      </div>
    `;

    const title = dialogBody.querySelector("#fnDialogTitle");
    if (title) {
      dialog.setAttribute("aria-labelledby", "fnDialogTitle");
    }
  }

  function cn(count, singular, pluralForm) {
    return `${count} ${count === 1 ? singular : pluralForm}`;
  }

  function highlightFunction(id) {
    document.querySelectorAll(".tree-node.is-selected").forEach((node) => {
      node.classList.remove("is-selected");
    });
    document.querySelectorAll(`.tree-node[data-fn="${id}"]`).forEach((node) => {
      node.classList.add("is-selected");
    });
  }

  function openFunction(id) {
    const fn = byId.get(id);
    if (!fn || !dialog) {
      return;
    }

    renderDialog(fn);
    highlightFunction(id);
    if (!dialog.open) {
      dialog.showModal();
    }
  }

  function closeDialog() {
    if (dialog && dialog.open) {
      dialog.close();
    }
  }

  if (dialog) {
    const closeButton = document.getElementById("fnDialogClose");
    if (closeButton) {
      closeButton.addEventListener("click", closeDialog);
    }

    dialog.addEventListener("click", (event) => {
      if (event.target === dialog) {
        closeDialog();
      }
    });

    dialog.addEventListener("close", () => {
      document.querySelectorAll(".tree-node.is-selected").forEach((node) => {
        node.classList.remove("is-selected");
      });
    });
  }

  if (dialogBody) {
    dialogBody.addEventListener("click", (event) => {
      const revealButton = event.target.closest("[data-reveal]");
      if (!revealButton) {
        return;
      }
      const id = revealButton.dataset.reveal;
      closeDialog();
      activateTab("tree");
      requestAnimationFrame(() => {
        requestAnimationFrame(() => {
          if (typeof window.revealInTree === "function") {
            window.revealInTree(id, { focus: true });
          }
        });
      });
    });
  }

  /* -----------------------------------------------------------------------
     Tree
     ----------------------------------------------------------------------- */

  const treeRoot = document.getElementById("callTree");
  const treeScroll = document.getElementById("treeScroll");
  const treeSearch = document.getElementById("treeSearch");
  const treeCount = document.getElementById("treeCount");
  const treeEmpty = document.getElementById("treeEmpty");

  function treeNodeMarkup(fn, path) {
    const childCallers = callers.get(fn.id).length;
    const hasChildren = fn.calls.length > 0;
    const isShared = childCallers > 1;
    const isUnused = (fn.flags || []).includes("unused");

    const twisty = hasChildren
      ? `<button class="twisty" type="button" data-toggle aria-expanded="true" aria-label="Collapse ${escapeHTML(fn.name)}"><span class="twisty__icon" aria-hidden="true">▼</span></button>`
      : '<span class="twisty-spacer" aria-hidden="true"></span>';

    const badges = [
      hasChildren ? `<span class="badge">${cn(fn.calls.length, "call", "calls")}</span>` : "",
      isShared ? '<span class="badge badge--shared">shared</span>' : "",
      isUnused ? '<span class="badge badge--unused">unused</span>' : "",
      `<span class="badge badge--phase">${escapeHTML(phaseLabel(fn.phase))}</span>`
    ]
      .filter(Boolean)
      .join("");

    const children = hasChildren
      ? `<ul class="call-tree">${fn.calls.map((calleeId) => treeNodeMarkup(byId.get(calleeId), `${path}/${calleeId}`)).join("")}</ul>`
      : "";

    return `
      <li class="tree-node" data-fn="${escapeHTML(fn.id)}" data-path="${escapeHTML(path)}" data-search="${escapeHTML(searchIndex.get(fn.id))}">
        <div class="node-row">
          ${twisty}
          <button class="node-card" type="button" data-fn="${escapeHTML(fn.id)}" style="${phaseStyle(fn.phase)}" aria-haspopup="dialog" aria-label="Inspect ${escapeHTML(fn.name)}, ${escapeHTML(fn.role)}">
            <span class="node-dot" aria-hidden="true"></span>
            <span class="node-ident">
              <span class="node-name">${escapeHTML(fn.name)}</span>
              ${fn.receiver ? `<span class="node-receiver">${escapeHTML(fn.receiver)}</span>` : ""}
            </span>
            <span class="node-role">${escapeHTML(fn.role)}</span>
            <span class="node-meta">${badges}</span>
          </button>
        </div>
        ${children}
      </li>
    `;
  }

  if (treeRoot) {
    treeRoot.innerHTML = treeNodeMarkup(byId.get("main"), "main");
    const allNodes = Array.from(treeRoot.querySelectorAll(".tree-node"));
    if (treeCount) {
      treeCount.textContent = `${functions.length} callables`;
    }

    function refreshTwisty(node) {
      const toggle = node.querySelector(":scope > .node-row [data-toggle]");
      if (!toggle) {
        return;
      }
      const collapsed = node.classList.contains("is-collapsed");
      toggle.setAttribute("aria-expanded", String(!collapsed));
      const fn = byId.get(node.dataset.fn);
      toggle.setAttribute("aria-label", `${collapsed ? "Expand" : "Collapse"} ${fn ? fn.name : "branch"}`);
    }

    function setAllCollapsed(collapsed) {
      allNodes.forEach((node) => {
        if (node === treeRoot.firstElementChild) {
          return;
        }
        const toggle = node.querySelector(":scope > .node-row [data-toggle]");
        if (!toggle) {
          return;
        }
        node.classList.toggle("is-collapsed", collapsed);
        refreshTwisty(node);
      });
    }

    treeRoot.addEventListener("click", (event) => {
      const toggle = event.target.closest("[data-toggle]");
      if (!toggle) {
        return;
      }
      const node = toggle.closest(".tree-node");
      node.classList.toggle("is-collapsed");
      refreshTwisty(node);
    });

    const expandAllButton = document.getElementById("expandAll");
    const collapseAllButton = document.getElementById("collapseAll");

    if (expandAllButton) {
      expandAllButton.addEventListener("click", () => {
        allNodes.forEach((node) => {
          node.classList.remove("is-collapsed");
          refreshTwisty(node);
        });
      });
    }

    if (collapseAllButton) {
      collapseAllButton.addEventListener("click", () => setAllCollapsed(true));
    }

    let filterActive = false;
    let savedCollapsed = new Set();

    function expandAllInternal() {
      allNodes.forEach((node) => {
        node.classList.remove("is-collapsed");
        refreshTwisty(node);
      });
    }

    function restoreCollapsed() {
      allNodes.forEach((node) => {
        node.classList.toggle("is-collapsed", savedCollapsed.has(node.dataset.path));
        refreshTwisty(node);
      });
    }

    function applyFilter(value) {
      const query = value.trim().toLowerCase();

      if (!query) {
        if (filterActive) {
          restoreCollapsed();
          filterActive = false;
        }
        allNodes.forEach((node) => {
          node.hidden = false;
          node.classList.remove("is-search-self");
        });
        if (treeEmpty) {
          treeEmpty.hidden = true;
        }
        if (treeCount) {
          treeCount.textContent = `${functions.length} callables`;
        }
        return;
      }

      if (!filterActive) {
        savedCollapsed = new Set(
          allNodes.filter((node) => node.classList.contains("is-collapsed")).map((node) => node.dataset.path)
        );
        expandAllInternal();
        filterActive = true;
      }

      const matches = new Set();

      function visit(node) {
        const selfMatch = searchIndex.get(node.dataset.fn).includes(query);
        if (selfMatch) {
          matches.add(node.dataset.fn);
        }

        const childLists = Array.from(node.children).filter((child) => child.classList.contains("call-tree"));
        let childMatch = false;
        childLists.forEach((list) => {
          Array.from(list.children).forEach((child) => {
            if (visit(child)) {
              childMatch = true;
            }
          });
        });

        const visible = selfMatch || childMatch;
        node.hidden = !visible;
        node.classList.toggle("is-search-self", selfMatch);
        return visible;
      }

      Array.from(treeRoot.children).forEach((node) => visit(node));

      if (treeCount) {
        treeCount.textContent = matches.size
          ? `${matches.size} of ${functions.length} callables`
          : "no matches";
      }
      if (treeEmpty) {
        treeEmpty.hidden = matches.size > 0;
      }
    }

    if (treeSearch) {
      treeSearch.addEventListener("input", () => applyFilter(treeSearch.value));
      treeSearch.addEventListener("search", () => applyFilter(treeSearch.value));
    }

    window.revealInTree = function revealInTree(id, options = {}) {
      const { focus = false } = options;
      const node = treeRoot.querySelector(`.tree-node[data-fn="${id}"]`);
      if (!node) {
        return;
      }

      if (treeSearch && treeSearch.value) {
        treeSearch.value = "";
        applyFilter("");
      }

      activateTab("tree");

      let parent = node.parentElement;
      while (parent && parent !== treeRoot) {
        if (parent.classList && parent.classList.contains("tree-node")) {
          parent.classList.remove("is-collapsed");
          refreshTwisty(parent);
        }
        parent = parent.parentElement;
      }

      highlightFunction(id);

      requestAnimationFrame(() => {
        if (treeScroll) {
          const scrollRect = treeScroll.getBoundingClientRect();
          const nodeRect = node.getBoundingClientRect();
          if (nodeRect.top < scrollRect.top || nodeRect.bottom > scrollRect.bottom) {
            treeScroll.scrollTop +=
              nodeRect.top - scrollRect.top - Math.max(20, (scrollRect.height - nodeRect.height) / 2);
          }
        }

        const card = node.querySelector(":scope > .node-row .node-card");
        if (card && focus) {
          card.focus({ preventScroll: true });
        }
      });
    };
  }

  /* -----------------------------------------------------------------------
     Index
     ----------------------------------------------------------------------- */

  const indexBody = document.getElementById("indexBody");
  const indexSearch = document.getElementById("indexSearch");
  const indexCount = document.getElementById("indexCount");
  let sortKey = "name";
  let sortDirection = 1;

  const sortAccessors = {
    name: (fn) => `${fn.name} ${fn.receiver || ""}`.toLowerCase(),
    phase: (fn) => phaseLabel(fn.phase),
    calls: (fn) => fn.calls.length,
    callers: (fn) => callers.get(fn.id).length,
    line: (fn) => fn.lines[0]
  };

  function flagMarkup(fn) {
    const flags = Array.from(new Set(fn.flags || []));
    if (callers.get(fn.id).length > 1) {
      flags.push("shared");
    }
    if (!flags.length) {
      return '<span class="count-cell is-zero">—</span>';
    }
    return `<div class="flag-list">${flags
      .map((flag) => {
        const modifier = flag === "unused" ? " badge--unused" : flag === "shared" ? " badge--shared" : "";
        return `<span class="badge${modifier}">${escapeHTML(flag)}</span>`;
      })
      .join("")}</div>`;
  }

  function renderIndex() {
    if (!indexBody) {
      return;
    }

    const query = indexSearch ? indexSearch.value.trim().toLowerCase() : "";
    const rows = functions.filter((fn) => !query || searchIndex.get(fn.id).includes(query));
    const accessor = sortAccessors[sortKey] || sortAccessors.name;

    rows.sort((a, b) => {
      const aValue = accessor(a);
      const bValue = accessor(b);
      let comparison;
      if (typeof aValue === "number" && typeof bValue === "number") {
        comparison = aValue - bValue;
      } else {
        comparison = String(aValue).localeCompare(String(bValue), undefined, { sensitivity: "base" });
      }
      if (comparison === 0) {
        comparison = a.name.localeCompare(b.name, undefined, { sensitivity: "base" });
      }
      return comparison * sortDirection;
    });

    if (!rows.length) {
      indexBody.innerHTML = '<tr><td class="table-empty" colspan="7">No functions match that filter.</td></tr>';
    } else {
      indexBody.innerHTML = rows
        .map((fn) => {
          const callerNames = (callers.get(fn.id) || []).map((id) => (byId.get(id) || {}).name).filter(Boolean);
          const callerCount = callerNames.length;
          const callCount = fn.calls.length;
          const phase = escapeHTML(phaseLabel(fn.phase));

          return `
            <tr data-row-fn="${escapeHTML(fn.id)}">
              <td>
                <button class="index-fn" type="button" data-fn="${escapeHTML(fn.id)}" aria-haspopup="dialog">
                  <span class="index-fn__name">${escapeHTML(fn.name)}</span>
                  <span class="index-fn__receiver">${fn.receiver ? escapeHTML(fn.receiver) : "package function"}</span>
                </button>
              </td>
              <td>${escapeHTML(fn.role)}</td>
              <td><span class="phase-badge" style="${phaseStyle(fn.phase)}">${phase}</span></td>
              <td class="is-numeric"><span class="count-cell${callCount ? "" : " is-zero"}"${callCount ? ` title="${escapeHTML(fn.calls.join(", "))}"` : ""}>${callCount ? `${callCount}<span class="count-cell__label">calls</span>` : "—"}</span></td>
              <td class="is-numeric"><span class="count-cell${callerCount ? "" : " is-zero"}"${callerCount ? ` title="${escapeHTML(callerNames.join(", "))}"` : ""}>${callerCount ? `${callerCount}<span class="count-cell__label">${callerCount === 1 ? "caller" : "callers"}</span>` : "—"}</span></td>
              <td class="is-numeric"><span class="line-cell">${lineRange(fn)}</span></td>
              <td>${flagMarkup(fn)}</td>
            </tr>
          `;
        })
        .join("");
    }

    if (indexCount) {
      indexCount.textContent = query
        ? `${rows.length} of ${functions.length} rows`
        : `${functions.length} rows`;
    }

    document.querySelectorAll("th[data-sort-key]").forEach((th) => {
      if (th.dataset.sortKey === sortKey) {
        th.setAttribute("aria-sort", sortDirection === 1 ? "ascending" : "descending");
      } else {
        th.setAttribute("aria-sort", "none");
      }
      const arrow = th.querySelector(".sort-arrow");
      if (arrow) {
        arrow.textContent = th.dataset.sortKey === sortKey ? (sortDirection === 1 ? "↑" : "↓") : "↕";
      }
    });
  }

  if (indexSearch) {
    indexSearch.addEventListener("input", renderIndex);
    indexSearch.addEventListener("search", renderIndex);
  }

  if (indexBody) {
    indexBody.addEventListener("click", (event) => {
      if (event.target.closest("[data-fn]")) {
        return;
      }
      const row = event.target.closest("[data-row-fn]");
      if (row) {
        openFunction(row.dataset.rowFn);
      }
    });
  }

  document.querySelectorAll("th[data-sort-key] .sort-button").forEach((button) => {
    button.addEventListener("click", () => {
      const key = button.closest("th").dataset.sortKey;
      const numeric = key === "calls" || key === "callers" || key === "line";
      if (sortKey === key) {
        sortDirection *= -1;
      } else {
        sortKey = key;
        sortDirection = numeric ? -1 : 1;
      }
      renderIndex();
    });
  });

  renderIndex();

  /* -----------------------------------------------------------------------
     Global function trigger + theme
     ----------------------------------------------------------------------- */

  document.addEventListener("click", (event) => {
    const trigger = event.target.closest("[data-fn]");
    if (!trigger || !trigger.matches("button")) {
      return;
    }
    event.preventDefault();
    openFunction(trigger.dataset.fn);
  });

  const themeToggle = document.getElementById("themeToggle");
  const root = document.documentElement;

  function applyTheme(theme) {
    root.dataset.theme = theme;
    if (themeToggle) {
      const isLight = theme === "light";
      themeToggle.setAttribute("aria-label", isLight ? "Switch to dark theme" : "Switch to light theme");
      themeToggle.setAttribute("title", isLight ? "Switch to dark theme" : "Switch to light theme");
      themeToggle.setAttribute("aria-pressed", String(isLight));
    }
  }

  let storedTheme = null;
  try {
    storedTheme = window.localStorage.getItem("honeyberri-callmap-theme");
  } catch (error) {
    storedTheme = null;
  }
  applyTheme(storedTheme === "light" ? "light" : "dark");

  if (themeToggle) {
    themeToggle.addEventListener("click", () => {
      const nextTheme = root.dataset.theme === "light" ? "dark" : "light";
      applyTheme(nextTheme);
      try {
        window.localStorage.setItem("honeyberri-callmap-theme", nextTheme);
      } catch (error) {
        /* Private mode or a blocked storage backend: keep the in-page theme. */
      }
    });
  }
})();
