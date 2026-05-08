const params = new URLSearchParams(location.search);
const TOKEN = params.get("t") || "";
const initialFolder = params.get("folder") || "";
const initialFile = params.get("file") || "";
const RECENT_SECTION_STORAGE_KEY = "gloss-recent-section-open";

const state = {
  folderID: initialFolder,
  folderName: "",
  folderRoot: "",
  file: null,
  tree: null,
  openFolders: [],
  recent: [],
  highlights: [],
  cursor: 0,
  cursorList: [],
};

const $ = (sel) => document.querySelector(sel);
const $$ = (sel) => Array.from(document.querySelectorAll(sel));

function api(path, opts = {}) {
  const headers = Object.assign({ "X-Gloss-Token": TOKEN }, opts.headers || {});
  if (opts.body && !(opts.body instanceof FormData)) {
    headers["Content-Type"] = "application/json";
  }
  return fetch(path, Object.assign({}, opts, { headers })).then((r) => {
    if (!r.ok) throw new Error(`${r.status} ${r.statusText}`);
    return r.json().catch(() => null);
  });
}

function toast(msg) {
  const el = $("#toast");
  el.textContent = msg;
  el.classList.remove("hidden");
  setTimeout(() => el.classList.add("hidden"), 1600);
}

function setTheme(t) {
  document.documentElement.dataset.theme = t;
  localStorage.setItem("gloss-theme", t);
}

function initTheme() {
  const stored = localStorage.getItem("gloss-theme");
  if (stored) {
    setTheme(stored);
    return;
  }
  const dark = window.matchMedia && window.matchMedia("(prefers-color-scheme: dark)").matches;
  setTheme(dark ? "dark" : "light");
}

function initRecentSection() {
  const section = $("#recent-section");
  if (!section) return;
  const stored = localStorage.getItem(RECENT_SECTION_STORAGE_KEY);
  if (stored === "true") section.open = true;
  if (stored === "false") section.open = false;
  section.addEventListener("toggle", () => {
    localStorage.setItem(RECENT_SECTION_STORAGE_KEY, section.open ? "true" : "false");
  });
}

async function loadOpenFolders() {
  const data = await api("/api/folders");
  state.openFolders = data.folders || [];
  if (!state.folderID && state.openFolders.length > 0) {
    state.folderID = state.openFolders[0].id;
  }
  renderOpenFolders();
}

async function loadRecent() {
  try {
    const data = await api("/api/recent");
    state.recent = data.entries || [];
  } catch (e) {
    state.recent = [];
  }
  renderRecent();
}

async function loadHighlights() {
  try {
    const data = await api("/api/highlights");
    state.highlights = data.highlights || [];
    $("#hl-count").textContent = state.highlights.length;
  } catch (e) {
    state.highlights = [];
  }
}

async function loadTree() {
  if (!state.folderID) return;
  const data = await api(`/api/folders/${state.folderID}/tree`);
  state.tree = data.tree;
  state.folderName = data.name || "";
  state.folderRoot = data.root || "";
  $("#folder-name").textContent = state.folderRoot;
  renderTree();
}

async function openFile(rel) {
  if (!state.folderID || !rel) return;
  const data = await api(`/api/folders/${state.folderID}/file?path=${encodeURIComponent(rel)}`);
  state.file = data;
  $("#rendered").innerHTML = data.html;
  document.title = `${rel} · gloss`;
  highlightTreeNode(rel);
  const url = new URL(location.href);
  url.searchParams.set("folder", state.folderID);
  url.searchParams.set("file", rel);
  history.replaceState(null, "", url.toString());
  $(".content").scrollTo(0, 0);
}

function renderTree() {
  const root = $("#current-tree");
  root.innerHTML = "";
  if (!state.tree) return;
  const filter = $("#filter").value.trim().toLowerCase();
  const list = [];
  renderNode(root, state.tree, 0, filter, list);
  state.cursorList = list;
  if (state.cursor >= list.length) state.cursor = list.length - 1;
  if (state.cursor < 0) state.cursor = 0;
  applyCursor();
  if (state.file && state.file.path) highlightTreeNode(state.file.path);
}

function renderNode(parent, node, depth, filter, list) {
  if (!node) return;
  if (node.is_dir) {
    if (depth > 0) {
      const div = document.createElement("div");
      div.className = "tree-node tree-dir";
      div.textContent = "▾ " + node.name;
      div.style.paddingLeft = `${8 + depth * 10}px`;
      parent.appendChild(div);
    }
    const wrap = document.createElement("div");
    wrap.className = depth > 0 ? "tree-children" : "";
    parent.appendChild(wrap);
    for (const child of node.children || []) {
      renderNode(wrap, child, depth + 1, filter, list);
    }
    return;
  }
  if (filter && !node.name.toLowerCase().includes(filter)) return;
  const div = document.createElement("div");
  div.className = "tree-node tree-file";
  div.textContent = node.name;
  div.dataset.path = node.path;
  div.style.paddingLeft = `${8 + depth * 10}px`;
  div.addEventListener("click", () => {
    state.cursor = list.indexOf(div);
    openFile(node.path);
  });
  parent.appendChild(div);
  list.push(div);
}

function highlightTreeNode(rel) {
  $$(".tree-node.active").forEach((n) => n.classList.remove("active"));
  const node = $(`.tree-node[data-path="${CSS.escape(rel)}"]`);
  if (node) {
    node.classList.add("active");
    state.cursor = state.cursorList.indexOf(node);
    applyCursor();
  }
}

function applyCursor() {
  $$(".tree-node.cursor").forEach((n) => n.classList.remove("cursor"));
  const el = state.cursorList[state.cursor];
  if (el) {
    el.classList.add("cursor");
    el.scrollIntoView({ block: "nearest" });
  }
}

function renderOpenFolders() {
  const root = $("#open-folders");
  root.innerHTML = "";
  if (state.openFolders.length === 0) {
    root.innerHTML = '<div class="empty-state">none</div>';
    return;
  }
  for (const f of state.openFolders) {
    const div = document.createElement("div");
    div.className = "flat-item";
    if (f.id === state.folderID) div.classList.add("active");
    div.textContent = f.name;
    div.title = f.root;
    div.addEventListener("click", () => switchFolder(f.id));
    root.appendChild(div);
  }
}

function renderRecent() {
  const root = $("#recent-files");
  root.innerHTML = "";
  if (state.recent.length === 0) {
    root.innerHTML = '<div class="empty-state">no recent .md files</div>';
    return;
  }
  for (const e of state.recent) {
    const div = document.createElement("div");
    div.className = "flat-item";
    const name = e.path.split("/").pop();
    div.innerHTML = `${escapeHTML(name)}<span class="meta">${escapeHTML(e.relative)}</span>`;
    const dir = e.path.substring(0, e.path.length - name.length - 1);
    div.title = e.path;
    const dirEl = document.createElement("span");
    dirEl.className = "path-dir";
    dirEl.textContent = dir;
    div.appendChild(dirEl);
    div.addEventListener("click", () => openExternalFile(e.path));
    root.appendChild(div);
  }
}

async function openExternalFile(absPath) {
  const data = await api("/api/open", {
    method: "POST",
    body: JSON.stringify({ folder: dirname(absPath), file: absPath }),
  });
  state.folderID = data.folder_id;
  await loadOpenFolders();
  await loadTree();
  await openFile(data.file_rel);
}

function dirname(p) {
  const i = p.lastIndexOf("/");
  return i < 0 ? p : p.substring(0, i);
}

async function switchFolder(id) {
  state.folderID = id;
  state.file = null;
  await loadTree();
}

function escapeHTML(s) {
  return s.replace(/[&<>"']/g, (c) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" }[c]));
}

function selectionRange() {
  const sel = window.getSelection();
  if (!sel || sel.rangeCount === 0 || sel.isCollapsed) return null;
  const text = sel.toString();
  if (!text.trim()) return null;
  const blockOf = (n) => {
    while (n && n.nodeType === Node.TEXT_NODE) n = n.parentNode;
    while (n && !(n.dataset && n.dataset.lineStart)) n = n.parentNode;
    return n;
  };
  const a = blockOf(sel.anchorNode);
  const f = blockOf(sel.focusNode);
  if (!a || !f) return null;
  const starts = [+a.dataset.lineStart, +f.dataset.lineStart];
  const ends = [+a.dataset.lineEnd, +f.dataset.lineEnd];
  return {
    lineStart: Math.min(...starts),
    lineEnd: Math.max(...ends),
    text,
  };
}

function formatPath(absPath, style) {
  if (style === "absolute") return absPath;
  if (style === "tilde") {
    const home = state.file && state.file.home;
    if (home && absPath.startsWith(home)) return "~" + absPath.substring(home.length);
    return absPath;
  }
  if (style === "relative" && state.folderRoot && absPath.startsWith(state.folderRoot)) {
    return absPath.substring(state.folderRoot.length + 1);
  }
  return absPath;
}

async function copyWithRef() {
  const r = selectionRange();
  if (!r || !state.file) return;
  const path = formatPath(state.file.abs_path, state.file.copy_style || "tilde");
  const body =
    `${path}:${r.lineStart}-${r.lineEnd}\n\n` +
    r.text.split("\n").map((l) => `> ${l}`).join("\n");
  await navigator.clipboard.writeText(body);
  toast(`Copied ${r.lineStart}-${r.lineEnd}`);
  hideSelectionBar();
}

async function copyPlain() {
  const r = selectionRange();
  if (!r) return;
  await navigator.clipboard.writeText(r.text);
  toast("Copied");
  hideSelectionBar();
}

async function saveHighlight() {
  const r = selectionRange();
  if (!r || !state.file) return;
  await api("/api/highlights", {
    method: "POST",
    body: JSON.stringify({
      abs_path: state.file.abs_path,
      line_start: r.lineStart,
      line_end: r.lineEnd,
      text: r.text,
    }),
  });
  await loadHighlights();
  toast("Highlight saved");
  hideSelectionBar();
}

function showSelectionBar() {
  const sel = window.getSelection();
  if (!sel || sel.rangeCount === 0 || sel.isCollapsed) {
    hideSelectionBar();
    return;
  }
  if (!sel.toString().trim()) {
    hideSelectionBar();
    return;
  }
  const range = sel.getRangeAt(0);
  const rect = range.getBoundingClientRect();
  const bar = $("#selection-bar");
  bar.classList.remove("hidden");
  bar.style.left = `${rect.left + rect.width / 2}px`;
  bar.style.top = `${rect.top + window.scrollY}px`;
}

function hideSelectionBar() {
  $("#selection-bar").classList.add("hidden");
}

function bindSelectionBar() {
  document.addEventListener("selectionchange", () => {
    if (document.activeElement && document.activeElement.tagName === "INPUT") return;
    showSelectionBar();
  });
  $("#selection-bar").addEventListener("mousedown", (e) => e.preventDefault());
  $$("#selection-bar button").forEach((b) => {
    b.addEventListener("click", () => {
      const a = b.dataset.action;
      if (a === "copy-ref") copyWithRef();
      if (a === "copy-plain") copyPlain();
      if (a === "save-highlight") saveHighlight();
    });
  });
  document.addEventListener("scroll", hideSelectionBar, true);
}

function bindKeys() {
  document.addEventListener("keydown", (e) => {
    if (e.target.tagName === "INPUT") {
      if (e.key === "Escape") e.target.blur();
      return;
    }
    if (e.key === "j") {
      e.preventDefault();
      if (state.cursor < state.cursorList.length - 1) state.cursor++;
      applyCursor();
    } else if (e.key === "k") {
      e.preventDefault();
      if (state.cursor > 0) state.cursor--;
      applyCursor();
    } else if (e.key === "Enter") {
      e.preventDefault();
      const el = state.cursorList[state.cursor];
      if (el) openFile(el.dataset.path);
    } else if (e.key === "/") {
      e.preventDefault();
      $("#filter").focus();
    } else if (e.key === "g") {
      $(".content").scrollTo(0, 0);
    } else if (e.key === "G") {
      const c = $(".content");
      c.scrollTo(0, c.scrollHeight);
    } else if (e.key === "y") {
      copyWithRef();
    } else if (e.key === "Y") {
      copyPlain();
    } else if (e.key === "*") {
      saveHighlight();
    } else if (e.key === "?") {
      $("#help-dialog").showModal();
    }
  });
}

function bindButtons() {
  $("#btn-theme").addEventListener("click", () => {
    setTheme(document.documentElement.dataset.theme === "dark" ? "light" : "dark");
  });
  $("#btn-help").addEventListener("click", () => $("#help-dialog").showModal());
  $("#btn-highlights").addEventListener("click", showHighlightsView);
  $("#filter").addEventListener("input", renderTree);
}

function showHighlightsView() {
  if (state.highlights.length === 0) {
    toast("No highlights yet");
    return;
  }
  const groups = {};
  for (const h of state.highlights) {
    (groups[h.abs_path] ||= []).push(h);
  }
  const html = Object.entries(groups)
    .map(([p, list]) => {
      const items = list
        .map(
          (h) =>
            `<blockquote data-line-start="${h.line_start}" data-line-end="${h.line_end}"><strong>${escapeHTML(p)}:${h.line_start}-${h.line_end}</strong><br>${escapeHTML(h.text)}</blockquote>`
        )
        .join("\n");
      return `<h2>${escapeHTML(p)}</h2>${items}`;
    })
    .join("\n");
  $("#rendered").innerHTML = `<h1>★ Highlights</h1>${html}`;
  document.title = "Highlights · gloss";
}

function bindSSE() {
  const es = new EventSource(`/api/events?t=${encodeURIComponent(TOKEN)}`);
  es.addEventListener("file-changed", (ev) => {
    const data = JSON.parse(ev.data);
    if (state.file && state.file.path === data.path && state.folderID === data.folder_id) {
      openFile(data.path);
    }
  });
  es.addEventListener("tree-changed", (ev) => {
    const data = JSON.parse(ev.data);
    if (state.folderID === data.folder_id) {
      loadTree();
    }
  });
}

async function bootstrap() {
  initTheme();
  initRecentSection();
  bindButtons();
  bindKeys();
  bindSelectionBar();
  bindSSE();
  await loadOpenFolders();
  await loadHighlights();
  await loadRecent();
  if (state.folderID) {
    await loadTree();
    if (initialFile) await openFile(initialFile);
  }
}

bootstrap().catch((e) => {
  console.error(e);
  $("#rendered").innerHTML = `<div class="render-error">Failed to load: ${e.message}</div>`;
});
