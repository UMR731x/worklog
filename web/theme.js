import { request } from "./api.js";

export function setupTheme() {
  const root = document.documentElement;
  const choices = [...document.querySelectorAll("[data-theme-choice]")];
  function select(theme) {
    root.dataset.theme = theme;
    choices.forEach((button) => button.setAttribute("aria-pressed", String(button.dataset.themeChoice === theme)));
  }
  select(root.dataset.theme);
  choices.forEach((button) => button.addEventListener("click", async () => {
    const previous = root.dataset.theme;
    const theme = button.dataset.themeChoice;
    const errorText = document.getElementById("theme-error");
    errorText.textContent = "";
    select(theme);
    choices.forEach((choice) => { choice.disabled = true; });
    try {
      await request("/api/theme", { method: "PUT", body: { theme } });
    } catch (error) {
      select(previous);
      errorText.textContent = error.message;
    } finally {
      choices.forEach((choice) => { choice.disabled = false; });
    }
  }));
}
