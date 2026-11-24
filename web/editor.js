import { request } from "./api.js";
import { saveStatus, showError, textElement } from "./ui.js";

export class ProgressEditor {
  constructor(onSaved) {
    this.onSaved = onSaved;
    this.context = null;
    this.savedContent = "";
    this.saving = null;
    this.uploadTask = null;
    this.previewVersion = 0;
    this.mode = "edit";
    this.textarea = document.getElementById("markdown-editor");
    this.preview = document.getElementById("markdown-preview");
    this.panel = document.getElementById("editor-panel");
    this.fileInput = document.getElementById("image-input");
    this.bindEvents();
  }

  bindEvents() {
    this.textarea.addEventListener("input", () => this.changed());
    this.textarea.addEventListener("paste", (event) => {
      const images = Array.from(event.clipboardData.files).filter((file) => file.type.startsWith("image/"));
      if (!images.length) return;
      event.preventDefault();
      this.upload(images).catch(showError);
    });
    this.textarea.addEventListener("dragover", (event) => event.preventDefault());
    this.textarea.addEventListener("drop", (event) => {
      event.preventDefault();
      const images = Array.from(event.dataTransfer.files).filter((file) => file.type.startsWith("image/"));
      this.upload(images).catch(showError);
    });
    document.getElementById("insert-image").addEventListener("click", () => this.fileInput.click());
    this.fileInput.addEventListener("change", () => {
      this.upload(Array.from(this.fileInput.files)).catch(showError);
      this.fileInput.value = "";
    });
    document.getElementById("save-entry").addEventListener("click", () => this.flush().catch(showError));
    document.querySelectorAll("[data-format]").forEach((button) => {
      button.addEventListener("click", () => this.format(button.dataset.format));
    });
    document.querySelectorAll(".editor-modes button").forEach((button) => {
      button.addEventListener("click", () => this.setMode(button.dataset.mode));
    });
    window.addEventListener("beforeunload", (event) => {
      if (this.isDirty()) {
        event.preventDefault();
        event.returnValue = "";
      }
    });
    document.addEventListener("keydown", (event) => {
      if ((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === "s") {
        event.preventDefault();
        this.flush().catch(showError);
      }
    });
  }

  load(context, content = "") {
    clearTimeout(this.saveTimer);
    clearTimeout(this.previewTimer);
    this.context = context;
    this.savedContent = content;
    this.textarea.value = content;
    this.textarea.readOnly = !context;
    this.textarea.scrollTop = 0;
    this.previewVersion++;
    this.preview.replaceChildren();
    this.updateCount();
    saveStatus("saved", context ? "所有更改已保存" : "准备就绪");
    if (this.mode !== "edit" && context) this.renderPreview().catch(showError);
  }

  isDirty() {
    return Boolean(this.context && (this.textarea.value !== this.savedContent || this.uploadTask));
  }

  setReadOnly(readOnly) {
    this.textarea.readOnly = readOnly || !this.context;
  }

  updateCount() {
    document.getElementById("character-count").textContent = `${Array.from(this.textarea.value).length.toLocaleString()} 字符`;
  }

  changed() {
    this.updateCount();
    saveStatus("dirty", "有未保存的更改");
    clearTimeout(this.saveTimer);
    clearTimeout(this.previewTimer);
    this.previewVersion++;
    this.saveTimer = setTimeout(() => this.flush().catch(showError), 600);
    if (this.mode !== "edit") {
      this.previewTimer = setTimeout(() => this.renderPreview().catch(showError), 250);
    }
  }

  async flush() {
    clearTimeout(this.saveTimer);
    if (this.uploadTask) await this.uploadTask;
    if (this.saving) await this.saving;
    while (this.context && this.textarea.value !== this.savedContent) {
      if (this.saving) {
        await this.saving;
        continue;
      }
      this.saving = this.saveCurrent();
      try {
        await this.saving;
      } finally {
        this.saving = null;
      }
    }
  }

  async saveCurrent() {
    const { projectID, key, token } = this.context;
    const content = this.textarea.value;
    saveStatus("saving", "正在保存…");
    try {
      const summary = await request(`/api/projects/${projectID}/entries/${encodeURIComponent(key)}`, {
        method: "PUT", token, body: { content },
      });
      this.savedContent = content;
      this.onSaved(summary, key, content);
      const dirty = this.isDirty();
      saveStatus(dirty ? "dirty" : "saved", dirty ? "有未保存的更改" : "所有更改已保存");
    } catch (error) {
      saveStatus("error", "保存失败 · 内容仍在编辑区");
      throw error;
    }
  }

  setMode(mode) {
    this.mode = mode;
    this.panel.dataset.mode = mode;
    document.querySelectorAll(".editor-modes button").forEach((button) => {
      button.setAttribute("aria-pressed", String(button.dataset.mode === mode));
    });
    if (mode !== "edit" && this.context) this.renderPreview().catch(showError);
  }

  async renderPreview() {
    const version = ++this.previewVersion;
    const { projectID, token } = this.context;
    const content = this.textarea.value;
    if (!content.trim()) {
      this.preview.replaceChildren(textElement("p", "preview-placeholder", "写下第一行，预览会在这里出现。"));
      return;
    }
    const result = await request("/api/preview", { method: "POST", body: { content } });
    if (version !== this.previewVersion) return;
    // The backend renders with goldmark's default raw-HTML and unsafe-link policy.
    this.preview.innerHTML = result.html;
    this.preview.querySelectorAll("img").forEach((image) => {
      const source = image.getAttribute("src");
      if (source?.startsWith("sources/")) {
        image.src = `/images/${token}/${projectID}/${encodeURIComponent(source.slice(8))}`;
      }
    });
    this.preview.querySelectorAll("a").forEach((link) => {
      if (!link.getAttribute("href")?.startsWith("#")) {
        link.target = "_blank";
        link.rel = "noopener noreferrer";
      }
    });
  }

  format(kind) {
    if (!this.context || this.textarea.readOnly) return;
    const formats = {
      heading: ["## ", "", "标题"],
      bold: ["**", "**", "粗体文字"],
      italic: ["*", "*", "斜体文字"],
      list: ["- ", "", "工作事项"],
      task: ["- [ ] ", "", "待办事项"],
      code: ["```\n", "\n```", "代码"],
      link: ["[", "](https://)", "链接文字"],
    };
    let [prefix, suffix, placeholder] = formats[kind];
    const { selectionStart: start, selectionEnd: end, value } = this.textarea;
    const selected = value.slice(start, end) || placeholder;
    if (["heading", "list", "task", "code"].includes(kind) && start > 0 && value[start - 1] !== "\n") {
      prefix = "\n" + prefix;
    }
    this.textarea.setRangeText(prefix + selected + suffix, start, end, "end");
    this.textarea.focus();
    this.textarea.setSelectionRange(start + prefix.length, start + prefix.length + selected.length);
    this.changed();
  }

  async upload(files) {
    if (!this.context || !files.length) return;
    const previous = this.uploadTask;
    const task = this.uploadFiles(files, previous);
    this.uploadTask = task;
    try {
      await task;
    } catch (error) {
      saveStatus("error", "图片保存失败 · 编辑内容已保留");
      throw error;
    } finally {
      if (this.uploadTask === task) this.uploadTask = null;
    }
  }

  async uploadFiles(files, previous) {
    if (previous) await previous;
    const { projectID, token } = this.context;
    for (const file of files) {
      saveStatus("saving", "正在保存图片…");
      const result = await request(`/api/projects/${projectID}/images`, { method: "POST", token, body: file });
      const markdown = `\n![图片](${result.path})\n`;
      this.textarea.setRangeText(markdown, this.textarea.selectionStart, this.textarea.selectionEnd, "end");
      this.changed();
    }
  }
}
