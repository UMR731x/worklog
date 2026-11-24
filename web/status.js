import { showError } from "./ui.js";

export function renderStatusPicker(status) {
  const trigger = document.getElementById("project-status");
  trigger.dataset.status = status;
  trigger.setAttribute("aria-label", `项目状态：${status}`);
  document.getElementById("project-status-name").textContent = status;
  document.querySelectorAll("#status-options button").forEach((button) => {
    button.setAttribute("aria-checked", String(button.dataset.status === status));
  });
}

export function setupStatusPicker(onChange) {
  const picker = document.getElementById("status-picker");
  const trigger = document.getElementById("project-status");
  const menu = document.getElementById("status-options");
  const options = Array.from(menu.querySelectorAll("button"));

  function close(returnFocus = false) {
    menu.hidden = true;
    trigger.setAttribute("aria-expanded", "false");
    if (returnFocus) trigger.focus();
  }
  function open() {
    menu.hidden = false;
    trigger.setAttribute("aria-expanded", "true");
    menu.querySelector('[aria-checked="true"]').focus();
  }
  trigger.addEventListener("click", () => menu.hidden ? open() : close());
  trigger.addEventListener("keydown", (event) => {
    if (event.key === "ArrowDown" || event.key === "ArrowUp") {
      event.preventDefault();
      open();
    }
  });
  menu.addEventListener("keydown", (event) => {
    const index = options.indexOf(document.activeElement);
    let next;
    if (event.key === "ArrowDown") next = (index + 1) % options.length;
    else if (event.key === "ArrowUp") next = (index + options.length - 1) % options.length;
    else if (event.key === "Home") next = 0;
    else if (event.key === "End") next = options.length - 1;
    if (next !== undefined) {
      event.preventDefault();
      options[next].focus();
    }
  });
  picker.addEventListener("keydown", (event) => {
    if (event.key === "Escape" && !menu.hidden) {
      event.preventDefault();
      close(true);
    }
  });
  picker.addEventListener("focusout", (event) => {
    if (!picker.contains(event.relatedTarget)) close();
  });
  document.addEventListener("pointerdown", (event) => {
    if (!picker.contains(event.target)) close();
  });
  options.forEach((button) => button.addEventListener("click", async () => {
    close(true);
    if (button.getAttribute("aria-checked") === "true") return;
    trigger.disabled = true;
    try {
      await onChange(button.dataset.status);
    } catch (error) {
      showError(error);
    } finally {
      trigger.disabled = false;
      trigger.focus();
    }
  }));
}
