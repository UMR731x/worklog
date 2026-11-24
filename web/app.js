import { request } from "./api.js";
import { icon, textElement, showError, saveStatus } from "./ui.js";
import { ProgressEditor } from "./editor.js";
import { setupDialogs } from "./dialogs.js";
import { setupTheme } from "./theme.js";
import { setupUpdates } from "./updates.js";

const state = {
  token: null,
  directory: "",
  projects: [],
  project: null,
  entry: null,
  filters: new Set(["Doing", "Done", "Pending"]),
};
const entryOrder = new Intl.Collator("zh-CN", { numeric: true });
const editor = new ProgressEditor(entrySaved);
let navigation = Promise.resolve();

function api(path, options = {}) {
  return request(path, { token: state.token, ...options });
}

// Serialize navigation, waiting for uploads and saves before changing the editor.
async function navigate(action) {
  const previous = navigation;
  let finish;
  navigation = new Promise((resolve) => { finish = resolve; });
  await previous;
  document.body.classList.add("transitioning");
  editor.setReadOnly(true);
  try {
    await editor.flush();
    await action();
  } finally {
    document.body.classList.remove("transitioning");
    editor.setReadOnly(false);
    finish();
  }
}

function updateSummary(summary) {
  const index = state.projects.findIndex((project) => project.id === summary.id);
  if (index === -1) state.projects.unshift(summary);
  else state.projects[index] = summary;
  renderSidebar();
}

function entrySaved(summary, key, content) {
  updateSummary(summary);
  state.project.entries[key] = content;
  state.project.updatedAt = summary.updatedAt;
  renderEntries();
  renderHeader();
}

function applyWorkspace(snapshot) {
  state.token = snapshot.token;
  state.directory = snapshot.directory;
  state.projects = snapshot.projects;
  state.project = null;
  state.entry = null;
  editor.load(null);
  document.getElementById("project-search").value = "";
  document.getElementById("directory-label").textContent = snapshot.directory;
  document.getElementById("workspace-location").title = snapshot.directory;
  document.getElementById("workspace-warnings").hidden = snapshot.warnings.length === 0;
  document.getElementById("warning-list").replaceChildren(...snapshot.warnings.map((warning) => textElement("li", "", warning)));
  document.getElementById("welcome").hidden = false;
  document.getElementById("project-view").hidden = true;
  document.getElementById("welcome-create").lastChild.textContent = state.projects.length ? "新建项目" : "新建第一个项目";
  renderSidebar();
}

function renderSidebar() {
  const query = document.getElementById("project-search").value.trim().toLocaleLowerCase();
  const visible = state.projects.filter((project) => state.filters.has(project.status) && project.name.toLocaleLowerCase().includes(query));
  document.getElementById("project-count").textContent = state.projects.length;
  document.getElementById("total-projects").textContent = `${state.projects.length} 个项目`;
  document.getElementById("visible-count").textContent = `${visible.length} 个`;
  document.getElementById("delete-project").disabled = !state.project;
  const empty = document.getElementById("sidebar-empty");
  empty.hidden = visible.length > 0;
  empty.textContent = state.projects.length ? "没有符合筛选条件的项目" : "还没有项目，点击右上角 + 开始记录";
  document.getElementById("project-list").replaceChildren(...visible.map(projectRow));
}

function projectRow(project) {
  const button = document.createElement("button");
  button.className = "project-row";
  button.dataset.project = project.id;
  button.setAttribute("aria-current", String(project.id === state.project?.id));
  button.title = `${project.name} · ${project.status}`;
  const copy = document.createElement("span");
  copy.className = "project-copy";
  const modified = new Date(project.updatedAt).toLocaleDateString("zh-CN", { month: "2-digit", day: "2-digit" });
  copy.append(textElement("span", "project-name", project.name));
  copy.append(textElement("span", "project-meta", `${project.entryCount} 条记录 · ${modified}`));
  const status = textElement("span", `dot ${project.status.toLowerCase()}`, "");
  status.setAttribute("aria-label", project.status);
  button.append(icon("folder"), copy, status);
  button.addEventListener("click", () => navigate(async () => {
    const selected = await api(`/api/projects/${project.id}`);
    activateProject(selected);
    document.getElementById("layout").classList.remove("sidebar-open");
  }).catch(showError));
  return button;
}

function activateProject(project, key) {
  state.project = project;
  const keys = Object.keys(project.entries).sort((a, b) => entryOrder.compare(b, a));
  state.entry = key || keys[0] || null;
  updateSummary({ ...project, entryCount: keys.length });
  document.getElementById("welcome").hidden = true;
  document.getElementById("project-view").hidden = false;
  document.getElementById("entry-empty").hidden = keys.length > 0;
  document.getElementById("journal").hidden = keys.length === 0;
  renderHeader();
  renderEntries();
  loadEntry();
}

function renderHeader() {
  const project = state.project;
  document.getElementById("project-title").textContent = project.name;
  document.getElementById("breadcrumb-name").textContent = project.name;
  document.getElementById("project-status").value = project.status;
  document.getElementById("status-label").dataset.status = project.status;
  const modified = new Date(project.updatedAt).toLocaleString("zh-CN", { month: "long", day: "numeric", hour: "2-digit", minute: "2-digit", hour12: false });
  document.getElementById("project-subtitle").textContent = `${Object.keys(project.entries).length} 条进度记录 · 最近编辑于 ${modified}`;
}

function renderEntries() {
  const keys = Object.keys(state.project.entries).sort((a, b) => entryOrder.compare(b, a));
  document.getElementById("entry-count").textContent = String(keys.length).padStart(2, "0");
  document.getElementById("entry-list").replaceChildren(...keys.map((key) => {
    const button = document.createElement("button");
    button.className = "entry-row";
    button.dataset.entry = key;
    button.setAttribute("aria-current", String(state.entry === key));
    const copy = textElement("span", "entry-copy", "");
    const excerpt = state.project.entries[key].trim().split("\n")[0].replace(/^#+\s*/, "") || "还没有内容";
    copy.append(textElement("span", "entry-date", key), textElement("span", "entry-excerpt", excerpt));
    button.append(textElement("span", "dot", ""), copy);
    button.addEventListener("click", () => navigate(async () => {
      state.entry = key;
      renderEntries();
      loadEntry();
    }).catch(showError));
    return button;
  }));
}

function loadEntry() {
  document.getElementById("entry-title").textContent = state.entry || "";
  if (state.entry) {
    editor.load({ projectID: state.project.id, key: state.entry, token: state.token }, state.project.entries[state.entry]);
  } else {
    editor.load(null);
  }
}

async function addEntry() {
  await navigate(async () => {
    const now = new Date();
    const date = `${now.getFullYear()}-${String(now.getMonth() + 1).padStart(2, "0")}-${String(now.getDate()).padStart(2, "0")}`;
    const result = await api(`/api/projects/${state.project.id}/entries`, { method: "POST", body: { date } });
    activateProject(result.project, result.key);
    editor.setMode("edit");
    editor.textarea.focus();
  });
}

function setupFilters() {
  document.getElementById("project-search").addEventListener("input", renderSidebar);
  document.querySelectorAll("[data-filter]").forEach((button) => {
    button.addEventListener("click", () => {
      const status = button.dataset.filter;
      if (state.filters.has(status)) state.filters.delete(status);
      else state.filters.add(status);
      button.setAttribute("aria-pressed", String(state.filters.has(status)));
      renderSidebar();
    });
  });
}

function setupResize() {
  const handle = document.getElementById("resize-handle");
  function setWidth(width) {
    width = Math.round(Math.max(220, Math.min(480, width)));
    document.documentElement.style.setProperty("--sidebar-width", `${width}px`);
    handle.setAttribute("aria-valuenow", String(width));
  }
  handle.addEventListener("pointerdown", (event) => {
    handle.setPointerCapture(event.pointerId);
    handle.classList.add("dragging");
    document.body.classList.add("resizing");
  });
  handle.addEventListener("pointermove", (event) => {
    if (handle.hasPointerCapture(event.pointerId)) setWidth(event.clientX);
  });
  handle.addEventListener("lostpointercapture", () => {
    handle.classList.remove("dragging");
    document.body.classList.remove("resizing");
  });
  handle.addEventListener("keydown", (event) => {
    if (!["ArrowLeft", "ArrowRight"].includes(event.key)) return;
    event.preventDefault();
    setWidth(Number(handle.getAttribute("aria-valuenow")) + (event.key === "ArrowLeft" ? -10 : 10));
  });
}

function setupEvents() {
  setupTheme();
  setupUpdates(navigate);
  setupFilters();
  setupResize();
  setupDialogs({ state, api, navigate, applyWorkspace, activateProject });
  document.getElementById("today").textContent = new Date().toLocaleDateString("zh-CN", { year: "numeric", month: "long", day: "numeric", weekday: "long" });
  document.getElementById("add-entry").addEventListener("click", () => addEntry().catch(showError));
  document.getElementById("empty-add-entry").addEventListener("click", () => addEntry().catch(showError));
  document.getElementById("dismiss-notice").addEventListener("click", () => { document.getElementById("notice").hidden = true; });
  document.getElementById("toggle-sidebar").addEventListener("click", () => document.getElementById("layout").classList.toggle("sidebar-open"));
  document.getElementById("project-status").addEventListener("change", (event) => {
    const status = event.target.value;
    navigate(async () => {
      const project = await api(`/api/projects/${state.project.id}/status`, { method: "PUT", body: { status } });
      state.project.status = project.status;
      state.project.updatedAt = project.updatedAt;
      updateSummary({ ...project, entryCount: Object.keys(project.entries).length });
      renderHeader();
    }).catch((error) => { renderHeader(); showError(error); });
  });
  document.getElementById("confirm-quit").addEventListener("click", () => navigate(async () => {
    await api("/api/quit", { method: "POST" });
    document.getElementById("quit-dialog").close();
    document.body.classList.add("stopped");
    saveStatus("saved", "程序已退出，可以关闭此页面");
  }).catch(showError));
}

setupEvents();
request("/api/workspace").then(applyWorkspace).catch(showError);
