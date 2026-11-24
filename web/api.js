export async function request(path, { method = "GET", body, token } = {}) {
  const headers = new Headers();
  if (token) headers.set("X-Workspace", token);
  if (body !== undefined && !(body instanceof Blob)) {
    headers.set("Content-Type", "application/json");
    body = JSON.stringify(body);
  }
  let response;
  try {
    response = await fetch(path, { method, headers, body });
  } catch {
    throw new Error("无法连接本地程序，请确认工作手记仍在运行。未保存的编辑仍保留在页面中。");
  }
  if (response.status === 204) return null;
  const isJSON = response.headers.get("Content-Type")?.includes("application/json");
  const result = isJSON ? await response.json() : await response.text();
  if (!response.ok) {
    throw new Error(isJSON ? result.error : result || `请求失败（${response.status}）`);
  }
  return result;
}
