import { invoke } from "@tauri-apps/api/core";
import { message, open } from "@tauri-apps/plugin-dialog";
import "./styles.css";

const app = document.querySelector("#app");

const state = {
  workspaceRoot: "",
  workspaceDraft: "",
  workspace: null,
  selectedEnv: "",
  selectedRequest: "",
  snapshot: false,
  timeoutMs: 15000,
  loading: false,
  error: "",
  status: "Choose a workspace to start.",
  lastRun: null,
};

function requestEntries() {
  return state.workspace?.requests ?? [];
}

function selectedRequestEntry() {
  return requestEntries().find((entry) => entry.path === state.selectedRequest) ?? null;
}

function currentWorkspace() {
  return state.workspace?.root || state.workspaceRoot;
}

function render() {
  const workspace = state.workspace;
  const selected = selectedRequestEntry();
  const requestMarkup = requestEntries()
    .map((entry, index) => {
      const classes = [
        "request-card",
        entry.path === state.selectedRequest ? "request-card--active" : "",
        entry.loadError ? "request-card--invalid" : "",
      ].filter(Boolean).join(" ");
      const subtitle = entry.loadError ? "invalid request spec" : entry.path;

      return `
        <button class="${classes}" data-action="select-request" data-path="${escapeAttr(entry.path)}">
          <span class="request-card__index">${index + 1}</span>
          <span class="request-card__content">
            <span class="request-card__title">${escapeHtml(entry.name || entry.path)}</span>
            <span class="request-card__subtitle">${escapeHtml(subtitle)}</span>
          </span>
        </button>
      `;
    })
    .join("");

  const envMarkup = (workspace?.envs ?? [])
    .map((env) => `
      <button
        class="env-pill ${env === state.selectedEnv ? "env-pill--active" : ""}"
        data-action="select-env"
        data-env="${escapeAttr(env)}"
      >${escapeHtml(env)}</button>
    `)
    .join("");

  const selectedRun = state.lastRun;
  const outputMarkup = selectedRun
    ? renderRunDetails(selectedRun)
    : `<div class="empty-panel">Run a request or collection to see details here.</div>`;

  app.innerHTML = `
    <div class="shell">
      <header class="hero">
        <div>
          <p class="eyebrow">Desktop Shell</p>
          <h1>API Workbench</h1>
          <p class="hero__copy">
            Repo-native API checks, now with a desktop GUI for macOS and Windows.
          </p>
        </div>
        <div class="hero__actions">
          <button class="ghost-button" data-action="pick-workspace">Open Workspace</button>
          <button class="solid-button" data-action="reload-workspace" ${workspace ? "" : "disabled"}>Reload</button>
        </div>
      </header>

      <main class="grid">
        <section class="panel panel--sidebar">
          <div class="panel__header">
            <h2>Workspace</h2>
            <span class="status-chip ${state.loading ? "status-chip--loading" : ""}">
              ${state.loading ? "Loading" : "Ready"}
            </span>
          </div>

          <label class="field">
            <span>Workspace path</span>
            <div class="field__row">
              <input
                type="text"
                data-role="workspace-input"
                value="${escapeAttr(state.workspaceDraft || state.workspaceRoot)}"
                placeholder="/path/to/api-checks"
              />
              <button class="ghost-button" data-action="load-workspace">Load</button>
            </div>
          </label>

          <div class="meta-block">
            <div>
              <span class="meta-label">Current root</span>
              <strong>${escapeHtml(currentWorkspace() || "Not loaded")}</strong>
            </div>
            <div>
              <span class="meta-label">Collection</span>
              <strong>${escapeHtml(workspace?.collectionPath || "requests")}</strong>
            </div>
          </div>

          <div class="panel__section">
            <div class="panel__section-header">
              <h3>Environment</h3>
            </div>
            <div class="pill-row">
              ${envMarkup || `<div class="empty-inline">Load a workspace to see env files.</div>`}
            </div>
          </div>

          <div class="panel__section panel__section--compact">
            <label class="toggle">
              <input type="checkbox" data-role="snapshot-toggle" ${state.snapshot ? "checked" : ""} />
              <span>Write snapshots while running</span>
            </label>
          </div>

          <div class="panel__section panel__section--compact">
            <label class="field">
              <span>Timeout (ms)</span>
              <input
                type="number"
                min="1000"
                step="1000"
                data-role="timeout-input"
                value="${state.timeoutMs}"
              />
            </label>
          </div>

          <div class="action-stack">
            <button class="solid-button" data-action="run-selected" ${selected ? "" : "disabled"}>
              Run Selected
            </button>
            <button class="ghost-button" data-action="run-all" ${workspace ? "" : "disabled"}>
              Run Collection
            </button>
          </div>

          <div class="status-box ${state.error ? "status-box--error" : ""}">
            <span class="meta-label">Status</span>
            <p>${escapeHtml(state.error || state.status)}</p>
          </div>
        </section>

        <section class="panel panel--list">
          <div class="panel__header">
            <h2>Requests</h2>
            <span class="meta-count">${requestEntries().length}</span>
          </div>
          <div class="request-list">
            ${requestMarkup || `<div class="empty-panel">No request files loaded.</div>`}
          </div>
        </section>

        <section class="panel panel--detail">
          <div class="panel__header">
            <h2>Run Output</h2>
            <span class="meta-count">${selectedRun ? escapeHtml(selectedRun.modeLabel) : "Idle"}</span>
          </div>
          ${outputMarkup}
        </section>
      </main>
    </div>
  `;

  bindEvents();
}

function renderRunDetails(run) {
  if (run.mode === "collection") {
    const runs = run.runs
      .map(
        (entry) => `
          <div class="run-row">
            <div>
              <strong>${escapeHtml(entry.requestName || entry.requestPath)}</strong>
              <div class="run-row__path">${escapeHtml(entry.requestPath)}</div>
            </div>
            <div class="run-row__metrics">
              <span class="run-row__badge run-row__badge--${statusClass(entry.exitCode)}">${statusLabel(entry.exitCode)}</span>
              <span>${entry.result?.statusCode ?? "n/a"}</span>
            </div>
          </div>
        `,
      )
      .join("");

    return `
      <div class="summary-grid">
        <div><span>Total</span><strong>${run.summary.total}</strong></div>
        <div><span>Passed</span><strong>${run.summary.passed}</strong></div>
        <div><span>Failed</span><strong>${run.summary.failed}</strong></div>
        <div><span>Transport</span><strong>${run.summary.transport}</strong></div>
      </div>
      <div class="detail-section">
        <h3>Collection Runs</h3>
        <div class="run-rows">${runs}</div>
      </div>
      ${run.error ? `<div class="detail-section detail-section--error"><h3>Error</h3><pre>${escapeHtml(run.error)}</pre></div>` : ""}
    `;
  }

  const result = run.result ?? {};
  const headers = result.headers
    ? Object.entries(result.headers)
        .map(([key, value]) => `<li><span>${escapeHtml(key)}</span><strong>${escapeHtml(String(value))}</strong></li>`)
        .join("")
    : "";

  return `
    <div class="summary-grid">
      <div><span>Exit</span><strong>${statusLabel(run.exitCode)}</strong></div>
      <div><span>Status</span><strong>${result.statusCode ?? "n/a"}</strong></div>
      <div><span>Duration</span><strong>${result.durationMs ?? 0} ms</strong></div>
      <div><span>Snapshot</span><strong>${escapeHtml(run.snapshotPath || "off")}</strong></div>
    </div>
    <div class="detail-section">
      <h3>${escapeHtml(run.requestName || run.requestPath)}</h3>
      <p class="detail-subtitle">${escapeHtml(run.requestPath)}</p>
      <p class="detail-url">${escapeHtml(result.url || "")}</p>
    </div>
    ${run.error ? `<div class="detail-section detail-section--error"><h3>Error</h3><pre>${escapeHtml(run.error)}</pre></div>` : ""}
    <div class="detail-section">
      <h3>Headers</h3>
      ${headers ? `<ul class="header-list">${headers}</ul>` : `<div class="empty-inline">No headers captured.</div>`}
    </div>
    <div class="detail-section">
      <h3>Body</h3>
      <pre>${escapeHtml(result.body || "")}</pre>
    </div>
  `;
}

function bindEvents() {
  app.querySelectorAll("[data-action='select-request']").forEach((button) => {
    button.addEventListener("click", () => {
      state.selectedRequest = button.dataset.path;
      state.status = `Selected ${button.dataset.path}`;
      state.error = "";
      render();
    });
  });

  app.querySelectorAll("[data-action='select-env']").forEach((button) => {
    button.addEventListener("click", () => {
      state.selectedEnv = button.dataset.env;
      state.status = `Selected env ${button.dataset.env}`;
      state.error = "";
      render();
    });
  });

  app.querySelector("[data-action='pick-workspace']")?.addEventListener("click", pickWorkspace);
  app.querySelector("[data-action='load-workspace']")?.addEventListener("click", () => loadWorkspace(state.workspaceDraft));
  app.querySelector("[data-action='reload-workspace']")?.addEventListener("click", () => loadWorkspace(currentWorkspace()));
  app.querySelector("[data-action='run-selected']")?.addEventListener("click", runSelected);
  app.querySelector("[data-action='run-all']")?.addEventListener("click", runAll);

  app.querySelector("[data-role='workspace-input']")?.addEventListener("input", (event) => {
    state.workspaceDraft = event.target.value;
  });
  app.querySelector("[data-role='timeout-input']")?.addEventListener("input", (event) => {
    state.timeoutMs = Number(event.target.value || 15000);
  });
  app.querySelector("[data-role='snapshot-toggle']")?.addEventListener("change", (event) => {
    state.snapshot = Boolean(event.target.checked);
  });
}

async function pickWorkspace() {
  const selected = await open({
    directory: true,
    multiple: false,
  });

  if (!selected || Array.isArray(selected)) {
    return;
  }

  state.workspaceDraft = selected;
  await loadWorkspace(selected);
}

async function loadWorkspace(root) {
  if (!root) {
    state.error = "Choose a workspace folder first.";
    render();
    return;
  }

  state.loading = true;
  state.error = "";
  state.status = "Loading workspace...";
  render();

  try {
    const workspace = await invoke("load_workspace", { root });
    state.workspace = workspace;
    state.workspaceRoot = workspace.root;
    state.workspaceDraft = workspace.root;
    state.selectedEnv = workspace.envs.includes(state.selectedEnv) ? state.selectedEnv : workspace.envs[0] || "";
    state.selectedRequest = workspace.requests.some((entry) => entry.path === state.selectedRequest)
      ? state.selectedRequest
      : workspace.requests.find((entry) => !entry.loadError)?.path || workspace.requests[0]?.path || "";
    state.status = `Loaded ${workspace.requests.length} requests from ${workspace.collectionPath}.`;
    localStorage.setItem("apiw.workspaceRoot", workspace.root);
    localStorage.setItem("apiw.selectedEnv", state.selectedEnv);
  } catch (error) {
    state.workspace = null;
    state.error = String(error);
  } finally {
    state.loading = false;
    render();
  }
}

async function runSelected() {
  if (!state.workspace || !state.selectedRequest) {
    state.error = "Select a request first.";
    render();
    return;
  }

  state.loading = true;
  state.error = "";
  state.status = `Running ${state.selectedRequest}...`;
  render();

  try {
    const result = await invoke("run_request_gui", {
      root: state.workspace.root,
      requestPath: state.selectedRequest,
      envName: state.selectedEnv,
      timeoutMs: state.timeoutMs,
      snapshot: state.snapshot,
    });
    state.lastRun = {
      ...result,
      mode: "request",
      modeLabel: "Single Request",
    };
    state.status = `${statusLabel(result.exitCode)}: ${result.requestPath}`;
  } catch (error) {
    state.error = String(error);
    await message(state.error, { title: "Run failed", kind: "error" });
  } finally {
    state.loading = false;
    render();
  }
}

async function runAll() {
  if (!state.workspace) {
    state.error = "Load a workspace first.";
    render();
    return;
  }

  state.loading = true;
  state.error = "";
  state.status = `Running ${state.workspace.collectionPath}...`;
  render();

  try {
    const result = await invoke("run_collection_gui", {
      root: state.workspace.root,
      collectionPath: state.workspace.collectionPath,
      envName: state.selectedEnv,
      timeoutMs: state.timeoutMs,
      snapshot: state.snapshot,
    });
    state.lastRun = {
      ...result,
      mode: "collection",
      modeLabel: "Collection Run",
    };
    state.status = `${statusLabel(result.exitCode)}: ${result.summary.passed}/${result.summary.total} passed`;
  } catch (error) {
    state.error = String(error);
    await message(state.error, { title: "Collection failed", kind: "error" });
  } finally {
    state.loading = false;
    render();
  }
}

function statusLabel(code) {
  switch (code) {
    case 0:
      return "passed";
    case 1:
      return "invalid";
    case 2:
      return "transport";
    case 3:
      return "failed";
    default:
      return "unknown";
  }
}

function statusClass(code) {
  switch (code) {
    case 0:
      return "passed";
    case 1:
      return "invalid";
    case 2:
      return "transport";
    case 3:
      return "failed";
    default:
      return "idle";
  }
}

function escapeHtml(value) {
  return String(value)
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;")
    .replaceAll("'", "&#39;");
}

function escapeAttr(value) {
  return escapeHtml(value);
}

async function bootstrap() {
  render();
  const rememberedRoot = localStorage.getItem("apiw.workspaceRoot");
  const rememberedEnv = localStorage.getItem("apiw.selectedEnv");
  if (rememberedEnv) {
    state.selectedEnv = rememberedEnv;
  }
  if (rememberedRoot) {
    state.workspaceDraft = rememberedRoot;
    await loadWorkspace(rememberedRoot);
  }
}

bootstrap();
