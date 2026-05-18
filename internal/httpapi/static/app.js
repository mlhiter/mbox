const state = {
  projects: [],
  templates: [],
  sandboxes: [],
  selected: null,
};

const els = {
  apiDot: document.querySelector("#api-dot"),
  apiState: document.querySelector("#api-state"),
  projectCount: document.querySelector("#project-count"),
  templateCount: document.querySelector("#template-count"),
  sandboxCount: document.querySelector("#sandbox-count"),
  runningCount: document.querySelector("#running-count"),
  projectsBody: document.querySelector("#projects-body"),
  templatesBody: document.querySelector("#templates-body"),
  sandboxesBody: document.querySelector("#sandboxes-body"),
  detailTitle: document.querySelector("#detail-title"),
  detailContent: document.querySelector("#detail-content"),
  toast: document.querySelector("#toast"),
};

document.querySelector("#refresh-button").addEventListener("click", () => loadAll());

document.querySelectorAll("[data-dialog]").forEach((button) => {
  button.addEventListener("click", () => {
    fillSelects();
    document.querySelector(`#${button.dataset.dialog}`).showModal();
  });
});

document.querySelectorAll("[data-close]").forEach((button) => {
  button.addEventListener("click", () => button.closest("dialog").close());
});

document.querySelector("#project-form").addEventListener("submit", async (event) => {
  event.preventDefault();
  const form = event.currentTarget;
  const data = formData(form);
  await request("/v1/projects", {
    method: "POST",
    body: JSON.stringify(data),
  });
  form.closest("dialog").close();
  form.reset();
  await loadAll();
  showToast("Project created");
});

document.querySelector("#template-form").addEventListener("submit", async (event) => {
  event.preventDefault();
  const form = event.currentTarget;
  const data = formData(form);
  const setDefault = data.setDefault === "on";
  delete data.setDefault;
  data.startupCommand = parseCommand(data.startupCommand);
  if (!data.projectId) {
    delete data.projectId;
  }
  const template = await request("/v1/templates", {
    method: "POST",
    body: JSON.stringify(data),
  });
  if (setDefault && data.projectId) {
    await request(`/v1/projects/${data.projectId}`, {
      method: "PATCH",
      body: JSON.stringify({ defaultTemplateId: template.id }),
    });
  }
  form.closest("dialog").close();
  form.reset();
  await loadAll();
  showToast("Template created");
});

document.querySelector("#sandbox-form").addEventListener("submit", async (event) => {
  event.preventDefault();
  const form = event.currentTarget;
  const data = formData(form);
  if (!data.templateId) {
    delete data.templateId;
  }
  if (!data.namespace) {
    delete data.namespace;
  }
  if (!data.serviceAccountName) {
    delete data.serviceAccountName;
  }
  const sandbox = await request("/v1/sandboxes", {
    method: "POST",
    body: JSON.stringify(data),
  });
  form.closest("dialog").close();
  form.reset();
  await loadAll();
  selectResource("sandbox", sandbox.id);
  showToast("Sandbox launched");
});

async function loadAll() {
  setAPIState("checking", "Checking API");
  try {
    const [health, projects, templates, sandboxes] = await Promise.all([
      request("/healthz"),
      request("/v1/projects"),
      request("/v1/templates"),
      request("/v1/sandboxes"),
    ]);
    state.projects = projects.items || [];
    state.templates = templates.items || [];
    state.sandboxes = sandboxes.items || [];
    setAPIState(health.status === "ok" ? "ok" : "bad", health.status || "Unknown");
    render();
  } catch (error) {
    setAPIState("bad", "API unavailable");
    showToast(error.message || "Request failed");
  }
}

async function request(path, options = {}) {
  const response = await fetch(path, {
    headers: { "content-type": "application/json" },
    ...options,
  });
  if (!response.ok) {
    let message = `${response.status} ${response.statusText}`;
    try {
      const body = await response.json();
      message = body.error || message;
    } catch {
      // Keep the HTTP status message.
    }
    throw new Error(message);
  }
  if (response.status === 204) {
    return null;
  }
  return response.json();
}

function render() {
  els.projectCount.textContent = state.projects.length;
  els.templateCount.textContent = state.templates.length;
  els.sandboxCount.textContent = state.sandboxes.length;
  els.runningCount.textContent = state.sandboxes.filter((sandbox) => sandbox.status === "running").length;
  renderProjects();
  renderTemplates();
  renderSandboxes();
  fillSelects();
  renderSelection();
}

function renderProjects() {
  if (state.projects.length === 0) {
    renderEmpty(els.projectsBody, 4, "No projects yet");
    return;
  }
  els.projectsBody.innerHTML = state.projects.map((project) => `
    <tr>
      <td>${titleCell(project.name, project.slug)}</td>
      <td class="mono">${escapeHTML(project.defaultNamespace)}</td>
      <td>${templateName(project.defaultTemplateId)}</td>
      <td><button class="button ghost" data-select="project:${project.id}" type="button">Inspect</button></td>
    </tr>
  `).join("");
  bindSelectionButtons();
}

function renderTemplates() {
  if (state.templates.length === 0) {
    renderEmpty(els.templatesBody, 4, "No templates yet");
    return;
  }
  els.templatesBody.innerHTML = state.templates.map((template) => `
    <tr>
      <td>${titleCell(template.name, template.slug)}</td>
      <td class="mono">${escapeHTML(template.image)}</td>
      <td>${resourceText(template)}</td>
      <td><button class="button ghost" data-select="template:${template.id}" type="button">Inspect</button></td>
    </tr>
  `).join("");
  bindSelectionButtons();
}

function renderSandboxes() {
  if (state.sandboxes.length === 0) {
    renderEmpty(els.sandboxesBody, 6, "No sandboxes yet");
    return;
  }
  els.sandboxesBody.innerHTML = state.sandboxes.map((sandbox) => `
    <tr>
      <td>${titleCell(sandbox.name, sandbox.slug)}</td>
      <td>${statusBadge(sandbox.status)}</td>
      <td>${projectName(sandbox.projectId)}</td>
      <td class="mono">${escapeHTML(sandbox.namespace)}</td>
      <td>${runtimeText(sandbox.runtimeRef)}</td>
      <td>
        <button class="button ghost" data-select="sandbox:${sandbox.id}" type="button">Inspect</button>
        <button class="button danger" data-delete-sandbox="${sandbox.id}" type="button">Delete</button>
      </td>
    </tr>
  `).join("");
  bindSelectionButtons();
  document.querySelectorAll("[data-delete-sandbox]").forEach((button) => {
    button.addEventListener("click", () => deleteSandbox(button.dataset.deleteSandbox));
  });
}

function renderEmpty(tbody, colspan, text) {
  tbody.innerHTML = `<tr><td colspan="${colspan}" class="empty">${text}</td></tr>`;
}

function fillSelects() {
  document.querySelectorAll("[data-project-select]").forEach((select) => {
    const previousValue = select.value;
    const optional = select.hasAttribute("data-optional-project");
    select.innerHTML = `${optional ? '<option value="">Global template</option>' : ""}${state.projects.map((project) => `<option value="${project.id}">${escapeHTML(project.name)}</option>`).join("")}`;
    if (previousValue && state.projects.some((project) => project.id === previousValue)) {
      select.value = previousValue;
    } else if (optional && state.projects.length > 0) {
      select.value = state.projects[0].id;
    }
  });
  document.querySelectorAll("[data-template-select]").forEach((select) => {
    select.innerHTML = `<option value="">Project default</option>${state.templates.map((template) => `<option value="${template.id}">${escapeHTML(template.name)}</option>`).join("")}`;
  });
}

function bindSelectionButtons() {
  document.querySelectorAll("[data-select]").forEach((button) => {
    button.addEventListener("click", () => {
      const [kind, id] = button.dataset.select.split(":");
      selectResource(kind, id);
    });
  });
}

function selectResource(kind, id) {
  state.selected = { kind, id };
  renderSelection();
}

function renderSelection() {
  if (!state.selected) {
    els.detailTitle.textContent = "No resource selected";
    els.detailContent.innerHTML = `<p class="empty">Inspect a project, template, or sandbox to see IDs, runtime state, and configuration.</p>`;
    return;
  }
  const item = collectionFor(state.selected.kind).find((entry) => entry.id === state.selected.id);
  if (!item) {
    state.selected = null;
    renderSelection();
    return;
  }
  els.detailTitle.textContent = item.name || item.slug || item.id;
  els.detailContent.innerHTML = detailHTML(state.selected.kind, item);
}

function detailHTML(kind, item) {
  const rows = [];
  rows.push(["ID", item.id]);
  rows.push(["Slug", item.slug]);
  if (kind === "project") {
    rows.push(["Namespace", item.defaultNamespace]);
    rows.push(["Repository", item.repositoryUrl || ""]);
    rows.push(["Default template", templateName(item.defaultTemplateId)]);
  }
  if (kind === "template") {
    rows.push(["Image", item.image]);
    rows.push(["Working dir", item.workingDir]);
    rows.push(["Resources", resourceText(item)]);
    rows.push(["Project", item.projectId ? projectName(item.projectId) : "Global"]);
  }
  if (kind === "sandbox") {
    rows.push(["Status", item.status]);
    rows.push(["Project", projectName(item.projectId)]);
    rows.push(["Template", templateName(item.templateId)]);
    rows.push(["Namespace", item.namespace]);
    rows.push(["ServiceAccount", item.serviceAccountName]);
    rows.push(["Runtime", runtimeText(item.runtimeRef)]);
  }
  return `<dl class="kv">${rows.map(([key, value]) => `<div><dt>${escapeHTML(key)}</dt><dd>${escapeHTML(String(value || "-"))}</dd></div>`).join("")}</dl>`;
}

async function deleteSandbox(id) {
  await request(`/v1/sandboxes/${id}`, { method: "DELETE" });
  if (state.selected?.kind === "sandbox" && state.selected.id === id) {
    state.selected = null;
  }
  await loadAll();
  showToast("Sandbox deleted");
}

function setAPIState(stateName, label) {
  els.apiDot.className = `status-dot ${stateName === "ok" ? "ok" : stateName === "bad" ? "bad" : ""}`;
  els.apiState.textContent = label;
}

function formData(form) {
  return Object.fromEntries(new FormData(form).entries());
}

function parseCommand(value) {
  const trimmed = String(value || "").trim();
  if (!trimmed) {
    return [];
  }
  if (trimmed.startsWith("[") && trimmed.endsWith("]")) {
    try {
      const parsed = JSON.parse(trimmed);
      return Array.isArray(parsed) ? parsed.map(String) : [trimmed];
    } catch {
      return [trimmed];
    }
  }
  if (trimmed.startsWith("sh -c ")) {
    return ["sh", "-c", trimmed.slice(6).replace(/^['"]|['"]$/g, "")];
  }
  return trimmed.split(/\s+/);
}

function collectionFor(kind) {
  if (kind === "project") {
    return state.projects;
  }
  if (kind === "template") {
    return state.templates;
  }
  return state.sandboxes;
}

function projectName(id) {
  return state.projects.find((project) => project.id === id)?.name || shortID(id);
}

function templateName(id) {
  if (!id) {
    return "-";
  }
  return state.templates.find((template) => template.id === id)?.name || shortID(id);
}

function resourceText(template) {
  return [template.cpuRequest, template.memoryRequest, template.storageRequest].filter(Boolean).join(" / ") || "-";
}

function runtimeText(ref) {
  if (!ref) {
    return "-";
  }
  return `${ref.kind} ${ref.namespace}/${ref.name}`;
}

function statusBadge(status) {
  return `<span class="badge ${escapeHTML(status)}">${escapeHTML(status)}</span>`;
}

function titleCell(name, slug) {
  return `<span class="cell-title"><strong>${escapeHTML(name)}</strong><span>${escapeHTML(slug)}</span></span>`;
}

function shortID(id) {
  return id ? `${id.slice(0, 8)}...` : "-";
}

function showToast(message) {
  els.toast.textContent = message;
  els.toast.classList.add("show");
  setTimeout(() => els.toast.classList.remove("show"), 2200);
}

function escapeHTML(value) {
  return String(value ?? "")
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;")
    .replaceAll("'", "&#039;");
}

loadAll();
