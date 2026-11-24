export function icon(name) {
  const svg = document.createElementNS("http://www.w3.org/2000/svg", "svg");
  svg.classList.add("icon");
  svg.setAttribute("aria-hidden", "true");
  const use = document.createElementNS("http://www.w3.org/2000/svg", "use");
  use.setAttribute("href", `#i-${name}`);
  svg.append(use);
  return svg;
}

export function textElement(tag, className, text) {
  const element = document.createElement(tag);
  element.className = className;
  element.textContent = text;
  return element;
}

export function showError(error) {
  document.getElementById("notice-text").textContent = error.message;
  document.getElementById("notice").hidden = false;
}

export function saveStatus(state, message) {
  document.getElementById("save-status").dataset.state = state;
  document.getElementById("save-status-text").textContent = message;
}
