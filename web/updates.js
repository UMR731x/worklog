import { request } from "./api.js";
import { saveStatus } from "./ui.js";

export function setupUpdates(navigate) {
  const dialog = document.getElementById("update-dialog");
  const open = document.getElementById("open-update");
  const label = document.getElementById("update-label");
  const check = document.getElementById("check-update");
  const install = document.getElementById("install-update");
  const message = document.getElementById("update-message");
  const errorText = document.getElementById("update-error");
  let installing = false;

  function display(status) {
    document.getElementById("update-version").textContent = `当前版本 ${status.current}`;
    open.dataset.available = String(status.available);
    label.textContent = status.available ? `发现 ${status.latest}` : status.error ? "检查失败" : "软件更新";
    errorText.textContent = status.error;
    message.textContent = status.available
      ? `新版本 ${status.latest} 已可下载。`
      : status.latest ? `暂无可用更新，最新正式版为 ${status.latest}。` : "暂未获取到版本信息。";
    install.hidden = !status.available;
    const link = document.getElementById("release-link");
    link.hidden = !status.url;
    if (status.url) link.href = status.url;
    open.title = status.checkedAt ? `上次检查：${new Date(status.checkedAt).toLocaleString("zh-CN")}` : "检查软件更新";
  }

  async function checkVersion(force) {
    check.disabled = true;
    install.disabled = true;
    label.textContent = "检查中…";
    message.textContent = "正在连接 GitHub…";
    errorText.textContent = "";
    try {
      display(await request(force ? "/api/update/check" : "/api/update", { method: force ? "POST" : "GET" }));
    } catch (error) {
      errorText.textContent = error.message;
      message.textContent = "检查失败，可以稍后重试。";
      label.textContent = "检查失败";
    } finally {
      check.disabled = false;
      install.disabled = false;
    }
  }

  open.addEventListener("click", () => dialog.showModal());
  check.addEventListener("click", () => checkVersion(true));
  dialog.addEventListener("cancel", (event) => {
    if (installing) event.preventDefault();
  });
  install.addEventListener("click", async () => {
    installing = true;
    check.disabled = true;
    install.disabled = true;
    dialog.querySelector("[data-close]").disabled = true;
    document.body.classList.add("updating");
    message.textContent = "正在保存编辑、下载并安装新版，请稍候…";
    errorText.textContent = "";
    try {
      await navigate(async () => {
        await request("/api/update/install", { method: "POST" });
        document.body.classList.add("stopped");
        saveStatus("saved", "更新已安装，正在打开新版");
      });
      message.textContent = "更新已安装，新版将自动打开，可以关闭此页面。";
      install.hidden = true;
      label.textContent = "已更新";
    } catch (error) {
      errorText.textContent = error.message;
      message.textContent = "更新未完成，当前版本仍可继续使用。";
      check.disabled = false;
      install.disabled = false;
    } finally {
      installing = false;
      dialog.querySelector("[data-close]").disabled = false;
      document.body.classList.remove("updating");
    }
  });
  checkVersion(false);
}
