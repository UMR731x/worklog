import { icon, textElement } from "./ui.js";
import { entryName } from "./entries.js";

export function setupDialogs({ state, api, navigate, applyWorkspace, activateProject }) {
  document.querySelectorAll("[data-close]").forEach((button) => {
    button.addEventListener("click", () => button.closest("dialog").close());
  });
  const createDialog = document.getElementById("create-dialog");
  const settingsDialog = document.getElementById("settings-dialog");
  function openCreate() {
    document.getElementById("create-form").reset();
    document.getElementById("create-error").textContent = "";
    createDialog.showModal();
    document.getElementById("new-project-name").focus();
  }
  function openSettings() {
    document.getElementById("directory-input").value = state.directory;
    document.getElementById("directory-browser").hidden = true;
    document.getElementById("settings-error").textContent = "";
    settingsDialog.showModal();
  }
  document.getElementById("new-project").addEventListener("click", openCreate);
  document.getElementById("welcome-create").addEventListener("click", openCreate);
  document.getElementById("open-settings").addEventListener("click", openSettings);
  document.getElementById("workspace-location").addEventListener("click", openSettings);
  document.getElementById("delete-project").addEventListener("click", () => {
    document.getElementById("delete-project-name").textContent = state.project.name;
    document.getElementById("delete-error").textContent = "";
    document.getElementById("delete-dialog").showModal();
  });
  document.getElementById("quit").addEventListener("click", () => document.getElementById("quit-dialog").showModal());

  bindForm("create", navigate, async () => {
    const name = document.getElementById("new-project-name").value;
    const status = document.getElementById("new-project-status").value;
    const project = await api("/api/projects", { method: "POST", body: { name, status } });
    document.getElementById("project-search").value = "";
    state.filters.add(status);
    document.querySelector(`[data-filter="${status}"]`).setAttribute("aria-pressed", "true");
    activateProject(project);
    document.getElementById("layout").classList.remove("sidebar-open");
  });
  bindForm("delete", navigate, async () => {
    await api(`/api/projects/${state.project.id}`, { method: "DELETE" });
    const snapshot = await api("/api/workspace");
    applyWorkspace(snapshot);
  });
  bindForm("settings", navigate, async () => {
    const directory = document.getElementById("directory-input").value;
    const snapshot = await api("/api/workspace", { method: "POST", body: { directory } });
    applyWorkspace(snapshot);
    document.getElementById("layout").classList.remove("sidebar-open");
  });
  setupDirectoryBrowser(api);
  setupEntryDialogs({ state, api, navigate, activateProject });
}

function setupEntryDialogs({ state, api, navigate, activateProject }) {
  document.getElementById("rename-entry").addEventListener("click", () => {
    const input = document.getElementById("entry-name-input");
    input.value = entryName(state.project, state.entry);
    document.getElementById("rename-entry-error").textContent = "";
    document.getElementById("rename-entry-dialog").showModal();
    input.focus();
    input.select();
  });
  document.getElementById("delete-entry").addEventListener("click", () => {
    document.getElementById("delete-entry-name").textContent = entryName(state.project, state.entry);
    document.getElementById("delete-entry-error").textContent = "";
    document.getElementById("delete-entry-dialog").showModal();
  });
  bindForm("rename-entry", navigate, async () => {
    const name = document.getElementById("entry-name-input").value;
    const path = `/api/projects/${state.project.id}/entries/${encodeURIComponent(state.entry)}/name`;
    const project = await api(path, { method: "PUT", body: { name } });
    activateProject(project, state.entry);
  });
  bindForm("delete-entry", navigate, async () => {
    const path = `/api/projects/${state.project.id}/entries/${encodeURIComponent(state.entry)}`;
    const project = await api(path, { method: "DELETE" });
    activateProject(project);
  });
}

function bindForm(name, navigate, action) {
  const form = document.getElementById(`${name}-form`);
  form.addEventListener("submit", async (event) => {
    event.preventDefault();
    const button = event.submitter;
    const errorText = document.getElementById(`${name}-error`);
    errorText.textContent = "";
    button.disabled = true;
    try {
      await navigate(action);
      form.closest("dialog").close();
    } catch (error) {
      errorText.textContent = error.message;
    } finally {
      button.disabled = false;
    }
  });
}

function setupDirectoryBrowser(api) {
  const input = document.getElementById("directory-input");
  const errorText = document.getElementById("settings-error");
  const parentButton = document.getElementById("directory-parent");
  let parent;
  let browseVersion = 0;
  async function browse(path) {
    const version = ++browseVersion;
    errorText.textContent = "";
    try {
      const listing = await api(`/api/directories?path=${encodeURIComponent(path)}`);
      if (version !== browseVersion) return;
      input.value = listing.path;
      parent = listing.parent;
      parentButton.disabled = listing.parent === listing.path;
      document.getElementById("browsing-path").textContent = listing.path;
      document.getElementById("browsing-path").title = listing.path;
      document.getElementById("directory-browser").hidden = false;
      const separator = listing.path.includes("\\") ? "\\" : "/";
      const prefix = listing.path.endsWith(separator) ? listing.path : listing.path + separator;
      const rows = listing.directories.map((name) => {
        const button = document.createElement("button");
        button.type = "button";
        button.className = "directory-row";
        button.append(icon("folder"), textElement("span", "", name), icon("chevron"));
        button.addEventListener("click", () => browse(prefix + name));
        return button;
      });
      if (!rows.length) rows.push(textElement("p", "directory-empty", "当前目录没有子文件夹，可直接选择此目录"));
      document.getElementById("directory-list").replaceChildren(...rows);
    } catch (error) {
      errorText.textContent = error.message;
    }
  }
  parentButton.addEventListener("click", () => browse(parent));
  document.getElementById("browse-directory").addEventListener("click", () => browse(input.value));
}
