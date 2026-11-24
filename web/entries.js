import { textElement } from "./ui.js";

export function entryName(project, key) {
  return project.entryMetadata?.[key]?.name || key;
}

function formatTime(value) {
  if (!value) return "未记录";
  const date = new Date(value);
  const pad = (number) => String(number).padStart(2, "0");
  const day = `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}`;
  return `${day} ${pad(date.getHours())}:${pad(date.getMinutes())}`;
}

export function entryTimes(project, key) {
  const metadata = project.entryMetadata?.[key];
  const times = textElement("span", "entry-times", "");
  times.append(
    textElement("span", "", `创建：${formatTime(metadata?.createdAt)}`),
    textElement("span", "", `最后修改：${formatTime(metadata?.updatedAt)}`),
  );
  return times;
}
