import { invoke } from "@tauri-apps/api/core";
import { message, open } from "@tauri-apps/plugin-dialog";
import "./styles.css";

const app = document.querySelector("#app");
const REQUEST_TABS = ["params", "headers", "body", "tests"];
const RESPONSE_TABS = ["body", "headers", "tests", "collection"];

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
  requestFilter: "",
  activeRequestTab: "params",
  activeResponseTab: "body",
};

function allRequestEntries() {
  return state.workspace?.requests ?? [];
}

function filteredRequestEntries() {
  const filter = state.requestFilter.trim().toLowerCase();
  if (!filter) {
    return allRequestEntries();
  }

  return allRequestEntries().filter((entry) =>
    [entry.name, entry.path, entry.url, entry.method]
      .filter(Boolean)
      .some((value) => String(value).toLowerCase().includes(filter)),
  );
}

function selectedRequestEntry() {
  return allRequestEntries().find((entry) => entry.path === state.selectedRequest) ?? null;
}

function selectedResponseRun() {
  if (!state.lastRun) {
    return null;
  }

  if (state.lastRun.mode === "collection") {
    return state.lastRun.runs.find((entry) => entry.requestPath === state.selectedRequest) ?? null;
  }

  if (!state.selectedRequest || state.lastRun.requestPath === state.selectedRequest) {
    return state.lastRun;
  }

  return null;
}

function currentWorkspace() {
  return state.workspace?.root || state.workspaceRoot;
}

function render() {
  const workspace = state.workspace;
  const requests = filteredRequestEntries();
  const selected = selectedRequestEntry();
  const selectedRun = selectedResponseRun();
  const canRunSelected = Boolean(selected && !selected.loadError);
  const activeRequestTab = REQUEST_TABS.includes(state.activeRequestTab) ? state.activeRequestTab : "params";
  const activeResponseTab = responseTabs().includes(state.activeResponseTab) ? state.activeResponseTab : "body";

  app.innerHTML = `
    <div class="postman-shell">
      <header class="topbar">
        <div class="brand">
          <div class="brand__mark">AW</div>
          <div>
            <span class="brand__eyebrow">Desktop Client</span>
            <strong>API Workbench</strong>
          </div>
        </div>
        <div class="topbar__meta">
          <span class="topbar__chip">${workspace ? `${workspace.requests.length} requests` : "No workspace"}</span>
          <span class="topbar__chip">${state.selectedEnv ? `Env: ${escapeHtml(state.selectedEnv)}` : "Choose env"}</span>
        </div>
        <div class="topbar__actions">
          <button class="chrome-button" data-action="pick-workspace">Open Workspace</button>
          <button class="chrome-button chrome-button--accent" data-action="reload-workspace" ${workspace ? "" : "disabled"}>Reload</button>
        </div>
      </header>

      <main class="workspace-layout">
        <aside class="sidebar">
          <section class="sidebar-panel">
            <div class="sidebar-panel__header">
              <h2>Workspace</h2>
              <span class="panel-chip ${state.loading ? "panel-chip--loading" : ""}">
                ${state.loading ? "Busy" : "Ready"}
              </span>
            </div>

            <label class="field field--stacked">
              <span>Root Path</span>
              <input
                type="text"
                data-role="workspace-input"
                value="${escapeAttr(state.workspaceDraft || state.workspaceRoot)}"
                placeholder="/path/to/api-workspace"
              />
            </label>

            <div class="button-row">
              <button class="ghost-button" data-action="load-workspace">Load</button>
              <button class="ghost-button" data-action="pick-workspace">Browse</button>
            </div>

            <div class="summary-card">
              <div>
                <span class="summary-card__label">Current Root</span>
                <strong>${escapeHtml(currentWorkspace() || "Not loaded")}</strong>
              </div>
              <div>
                <span class="summary-card__label">Collection</span>
                <strong>${escapeHtml(workspace?.collectionPath || "requests")}</strong>
              </div>
            </div>
          </section>

          <section class="sidebar-panel">
            <div class="sidebar-panel__header">
              <h2>Collections</h2>
              <span class="panel-chip">${workspace?.requests.length ?? 0}</span>
            </div>
            <div class="tree-card ${workspace ? "tree-card--active" : ""}">
              <span class="tree-card__label">Requests</span>
              <strong>${escapeHtml(workspace?.collectionPath || "requests")}</strong>
              <span class="tree-card__meta">${workspace ? `${workspace.requests.length} specs loaded` : "Load a workspace to inspect requests."}</span>
            </div>
          </section>

          <section class="sidebar-panel">
            <div class="sidebar-panel__header">
              <h2>Environments</h2>
            </div>
            <div class="env-stack">
              ${renderEnvButtons(workspace)}
            </div>
          </section>

          <section class="sidebar-panel">
            <div class="sidebar-panel__header">
              <h2>Runner</h2>
            </div>
            <label class="toggle">
              <input type="checkbox" data-role="snapshot-toggle" ${state.snapshot ? "checked" : ""} />
              <span>Save snapshots while sending</span>
            </label>
            <label class="field field--stacked">
              <span>Timeout (ms)</span>
              <input type="number" min="1000" step="1000" data-role="timeout-input" value="${state.timeoutMs}" />
            </label>
            <div class="button-stack">
              <button class="send-button" data-action="run-selected" ${canRunSelected ? "" : "disabled"}>Send</button>
              <button class="ghost-button" data-action="run-all" ${workspace ? "" : "disabled"}>Run Collection</button>
            </div>
          </section>
        </aside>

        <section class="explorer">
          <div class="panel-topline">
            <div>
              <span class="panel-caption">Explorer</span>
              <h2>Requests</h2>
            </div>
            <span class="panel-chip">${requests.length}/${allRequestEntries().length}</span>
          </div>

          <label class="search-field">
            <input
              type="search"
              data-role="request-filter"
              value="${escapeAttr(state.requestFilter)}"
              placeholder="Filter by name, path, method, or URL"
            />
          </label>

          <div class="request-list">
            ${renderRequestList(requests)}
          </div>
        </section>

        <section class="editor-stack">
          <section class="editor-panel">
            <div class="panel-topline panel-topline--editor">
              <div>
                <span class="panel-caption">Request</span>
                <h2>${escapeHtml(selected?.name || "Choose a request")}</h2>
              </div>
              <div class="request-actions">
                <button class="ghost-button" data-action="run-all" ${workspace ? "" : "disabled"}>Runner</button>
                <button class="send-button" data-action="run-selected" ${canRunSelected ? "" : "disabled"}>Send</button>
              </div>
            </div>

            <div class="request-builder ${selected?.loadError ? "request-builder--invalid" : ""}">
              <div class="request-builder__bar">
                <span class="method-badge method-badge--${methodClass(selected?.method)}">${escapeHtml(selected?.method || "GET")}</span>
                <div class="url-field">${escapeHtml(selected?.url || "Select a request from the collection explorer.")}</div>
                <span class="inline-chip">${escapeHtml(state.selectedEnv || "local")}</span>
              </div>
              <div class="tab-row">
                ${renderTabs(REQUEST_TABS, activeRequestTab, "select-request-tab")}
              </div>
              <div class="tab-panel">
                ${renderRequestTabContent(selected, activeRequestTab)}
              </div>
            </div>
          </section>

          <section class="editor-panel editor-panel--response">
            <div class="panel-topline panel-topline--editor">
              <div>
                <span class="panel-caption">Response</span>
                <h2>${escapeHtml(responseTitle(selectedRun, selected))}</h2>
              </div>
              <div class="response-metrics">
                ${renderResponseMetrics(selectedRun)}
              </div>
            </div>

            ${renderCollectionSummaryBanner()}

            <div class="tab-row">
              ${renderTabs(responseTabs(), activeResponseTab, "select-response-tab")}
            </div>
            <div class="tab-panel tab-panel--response">
              ${renderResponseTabContent(selectedRun, selected, activeResponseTab)}
            </div>
          </section>
        </section>
      </main>

      <footer class="statusbar ${state.error ? "statusbar--error" : ""}">
        <span class="statusbar__label">${state.error ? "Error" : "Status"}</span>
        <p>${escapeHtml(state.error || state.status)}</p>
      </footer>
    </div>
  `;

  bindEvents();
}

function renderEnvButtons(workspace) {
  if (!workspace?.envs?.length) {
    return `<div class="empty-state empty-state--compact">Load a workspace to see env files.</div>`;
  }

  return workspace.envs
    .map((env) => `
      <button
        class="env-button ${env === state.selectedEnv ? "env-button--active" : ""}"
        data-action="select-env"
        data-env="${escapeAttr(env)}"
      >${escapeHtml(env)}</button>
    `)
    .join("");
}

function renderRequestList(requests) {
  if (!requests.length) {
    return `<div class="empty-state">No requests match the current workspace or filter.</div>`;
  }

  return requests
    .map((entry) => {
      const classes = [
        "request-row",
        entry.path === state.selectedRequest ? "request-row--active" : "",
        entry.loadError ? "request-row--invalid" : "",
      ].filter(Boolean).join(" ");
      const secondaryLine = entry.loadError ? "Invalid request spec" : entry.path;

      return `
        <button class="${classes}" data-action="select-request" data-path="${escapeAttr(entry.path)}">
          <span class="method-badge method-badge--${methodClass(entry.method)}">${escapeHtml(entry.method || "ERR")}</span>
          <span class="request-row__content">
            <strong>${escapeHtml(entry.name || entry.path)}</strong>
            <span class="request-row__path">${escapeHtml(secondaryLine)}</span>
            <span class="request-row__url">${escapeHtml(entry.url || "No URL metadata")}</span>
          </span>
        </button>
      `;
    })
    .join("");
}

function renderTabs(tabs, activeTab, action) {
  return tabs
    .map((tab) => `
      <button
        class="tab-button ${tab === activeTab ? "tab-button--active" : ""}"
        data-action="${action}"
        data-tab="${escapeAttr(tab)}"
      >${escapeHtml(tabLabel(tab))}</button>
    `)
    .join("");
}

function renderRequestTabContent(entry, tab) {
  if (!entry) {
    return `<div class="empty-state">Choose a request from the collection explorer to inspect it.</div>`;
  }

  if (entry.loadError) {
    return `
      <div class="callout callout--error">
        <strong>${escapeHtml(entry.name || entry.path)}</strong>
        <p>${escapeHtml(entry.loadError)}</p>
      </div>
    `;
  }

  switch (tab) {
    case "params":
      return renderKeyValueTable(
        sortedEntries(entry.query),
        "Query parameter",
        "Value",
        "No query parameters defined for this request.",
      );
    case "headers":
      return renderKeyValueTable(
        sortedEntries(entry.headers),
        "Header",
        "Value",
        "No custom headers defined for this request.",
      );
    case "body":
      return renderBodyPanel(entry.body, "This request does not send a body.");
    case "tests":
      return renderAssertionPanel(entry.assertions);
    default:
      return `<div class="empty-state">Unknown request tab.</div>`;
  }
}

function renderResponseTabContent(run, requestEntry, tab) {
  if (tab === "collection") {
    return renderCollectionRuns();
  }

  if (!run) {
    return `<div class="empty-state">Run the selected request to see response details.</div>`;
  }

  switch (tab) {
    case "body":
      return renderBodyPanel(
        run.result ? { type: "response", content: run.result.body || "" } : null,
        run.error || "No response body captured.",
      );
    case "headers":
      return renderKeyValueTable(
        sortedEntries(run.result?.headers),
        "Header",
        "Value",
        run.error || "No response headers captured.",
      );
    case "tests":
      return renderResponseTests(run, requestEntry);
    default:
      return `<div class="empty-state">Unknown response tab.</div>`;
  }
}

function renderCollectionSummaryBanner() {
  if (!state.lastRun || state.lastRun.mode !== "collection") {
    return "";
  }

  return `
    <div class="collection-banner">
      <div><span>Total</span><strong>${state.lastRun.summary.total}</strong></div>
      <div><span>Passed</span><strong>${state.lastRun.summary.passed}</strong></div>
      <div><span>Failed</span><strong>${state.lastRun.summary.failed}</strong></div>
      <div><span>Transport</span><strong>${state.lastRun.summary.transport}</strong></div>
      <div><span>Invalid</span><strong>${state.lastRun.summary.invalid}</strong></div>
    </div>
  `;
}

function renderCollectionRuns() {
  if (!state.lastRun || state.lastRun.mode !== "collection") {
    return `<div class="empty-state">Collection results appear here after you run the collection.</div>`;
  }

  const runs = state.lastRun.runs
    .map((entry) => `
      <div class="collection-run ${entry.requestPath === state.selectedRequest ? "collection-run--active" : ""}">
        <div class="collection-run__info">
          <strong>${escapeHtml(entry.requestName || entry.requestPath)}</strong>
          <span>${escapeHtml(entry.requestPath)}</span>
        </div>
        <div class="collection-run__metrics">
          <span class="result-pill result-pill--${statusClass(entry.exitCode)}">${escapeHtml(statusLabel(entry.exitCode))}</span>
          <span>${entry.result?.statusCode ?? "n/a"}</span>
        </div>
      </div>
    `)
    .join("");

  return `
    <div class="collection-runs">
      ${runs}
      ${state.lastRun.error ? `<div class="callout callout--error"><p>${escapeHtml(state.lastRun.error)}</p></div>` : ""}
    </div>
  `;
}

function renderBodyPanel(body, emptyText) {
  if (!body || !body.content) {
    return `<div class="empty-state">${escapeHtml(emptyText)}</div>`;
  }

  return `
    <div class="body-panel">
      <span class="body-panel__meta">${escapeHtml(body.type || "raw")}</span>
      <pre>${escapeHtml(body.content)}</pre>
    </div>
  `;
}

function renderAssertionPanel(assertions) {
  if (!assertions?.length) {
    return `<div class="empty-state">No assertions configured for this request.</div>`;
  }

  return `
    <div class="assertion-list">
      ${assertions
        .map((assertion) => `
          <div class="assertion-row">
            <span class="inline-chip">${escapeHtml(assertion.type)}</span>
            <strong>${escapeHtml(formatAssertion(assertion))}</strong>
          </div>
        `)
        .join("")}
    </div>
  `;
}

function renderResponseTests(run, requestEntry) {
  const configuredAssertions = requestEntry?.assertions ?? [];
  const failures = run.result?.assertions ?? [];
  const success = run.exitCode === 0 && configuredAssertions.length > 0;

  return `
    <div class="test-panel">
      <div class="callout ${success ? "callout--success" : failures.length > 0 || run.error ? "callout--error" : ""}">
        <strong>${escapeHtml(testHeadline(run, configuredAssertions.length, failures.length))}</strong>
        <p>${escapeHtml(testDetail(run, configuredAssertions.length, failures.length))}</p>
      </div>
      ${failures.length
        ? `
          <div class="test-block">
            <span class="test-block__label">Failed Assertions</span>
            <div class="assertion-list">
              ${failures
                .map((message) => `
                  <div class="assertion-row assertion-row--error">
                    <span class="inline-chip inline-chip--error">failed</span>
                    <strong>${escapeHtml(message)}</strong>
                  </div>
                `)
                .join("")}
            </div>
          </div>
        `
        : ""}
      <div class="test-block">
        <span class="test-block__label">Configured Assertions</span>
        ${renderAssertionPanel(configuredAssertions)}
      </div>
    </div>
  `;
}

function renderKeyValueTable(rows, keyLabel, valueLabel, emptyText) {
  if (!rows.length) {
    return `<div class="empty-state">${escapeHtml(emptyText)}</div>`;
  }

  return `
    <div class="kv-table">
      <div class="kv-table__header">
        <span>${escapeHtml(keyLabel)}</span>
        <span>${escapeHtml(valueLabel)}</span>
      </div>
      ${rows
        .map(([key, value]) => `
          <div class="kv-table__row">
            <strong>${escapeHtml(key)}</strong>
            <span>${escapeHtml(String(value))}</span>
          </div>
        `)
        .join("")}
    </div>
  `;
}

function renderResponseMetrics(run) {
  if (!run) {
    return `<span class="result-pill">Idle</span>`;
  }

  return [
    `<span class="result-pill result-pill--${statusClass(run.exitCode)}">${escapeHtml(statusLabel(run.exitCode))}</span>`,
    `<span class="result-pill">HTTP ${run.result?.statusCode ?? "n/a"}</span>`,
    `<span class="result-pill">${run.result?.durationMs ?? 0} ms</span>`,
    `<span class="result-pill">${escapeHtml(run.snapshotPath || "snapshot off")}</span>`,
  ].join("");
}

function responseTitle(run, requestEntry) {
  if (!run) {
    return requestEntry?.name ? `${requestEntry.name} response` : "Run output";
  }
  return run.requestName || requestEntry?.name || run.requestPath || "Run output";
}

function responseTabs() {
  if (state.lastRun?.mode === "collection") {
    return RESPONSE_TABS;
  }
  return RESPONSE_TABS.filter((tab) => tab !== "collection");
}

function testHeadline(run, configuredCount, failureCount) {
  if (failureCount > 0) {
    return `${failureCount} assertion failures`;
  }
  if (configuredCount === 0) {
    return run.error || "No assertions configured";
  }
  if (run.exitCode === 0) {
    return `${configuredCount} assertions passed`;
  }
  return statusLabel(run.exitCode);
}

function testDetail(run, configuredCount, failureCount) {
  if (failureCount > 0) {
    return "The response returned data, but one or more expectations failed.";
  }
  if (run.error) {
    return run.error;
  }
  if (configuredCount === 0) {
    return "Add assertions to turn this request into a repeatable API check.";
  }
  return "The current response matched every configured check.";
}

function formatAssertion(assertion) {
  switch (assertion.type) {
    case "status":
      return `Status equals ${assertion.equals}`;
    case "body_contains":
      return `Body contains "${assertion.contains}"`;
    case "header_equals":
      return `Header ${assertion.key} equals "${assertion.value}"`;
    default:
      return assertion.type;
  }
}

function tabLabel(tab) {
  switch (tab) {
    case "params":
      return "Params";
    case "headers":
      return "Headers";
    case "body":
      return "Body";
    case "tests":
      return "Tests";
    case "collection":
      return "Collection";
    default:
      return tab;
  }
}

function methodClass(method) {
  return (method || "unknown").trim().toLowerCase() || "unknown";
}

function sortedEntries(record) {
  return Object.entries(record ?? {}).sort(([left], [right]) => left.localeCompare(right));
}

function bindEvents() {
  app.querySelectorAll("[data-action='select-request']").forEach((button) => {
    button.addEventListener("click", () => {
      state.selectedRequest = button.dataset.path;
      state.error = "";
      state.status = `Selected ${button.dataset.path}`;
      render();
    });
  });

  app.querySelectorAll("[data-action='select-env']").forEach((button) => {
    button.addEventListener("click", () => {
      state.selectedEnv = button.dataset.env;
      state.error = "";
      state.status = `Selected env ${button.dataset.env}`;
      localStorage.setItem("apiw.selectedEnv", state.selectedEnv);
      render();
    });
  });

  app.querySelectorAll("[data-action='select-request-tab']").forEach((button) => {
    button.addEventListener("click", () => {
      state.activeRequestTab = button.dataset.tab;
      render();
    });
  });

  app.querySelectorAll("[data-action='select-response-tab']").forEach((button) => {
    button.addEventListener("click", () => {
      state.activeResponseTab = button.dataset.tab;
      render();
    });
  });

  app.querySelectorAll("[data-action='pick-workspace']").forEach((button) => {
    button.addEventListener("click", pickWorkspace);
  });
  app.querySelectorAll("[data-action='load-workspace']").forEach((button) => {
    button.addEventListener("click", () => loadWorkspace(state.workspaceDraft));
  });
  app.querySelectorAll("[data-action='reload-workspace']").forEach((button) => {
    button.addEventListener("click", () => loadWorkspace(currentWorkspace()));
  });
  app.querySelectorAll("[data-action='run-selected']").forEach((button) => {
    button.addEventListener("click", runSelected);
  });
  app.querySelectorAll("[data-action='run-all']").forEach((button) => {
    button.addEventListener("click", runAll);
  });

  app.querySelector("[data-role='workspace-input']")?.addEventListener("input", (event) => {
    state.workspaceDraft = event.target.value;
  });
  app.querySelector("[data-role='workspace-input']")?.addEventListener("keydown", (event) => {
    if (event.key === "Enter") {
      loadWorkspace(state.workspaceDraft);
    }
  });
  app.querySelector("[data-role='request-filter']")?.addEventListener("input", (event) => {
    state.requestFilter = event.target.value;
    render();
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
  const selected = selectedRequestEntry();
  if (!state.workspace || !selected || selected.loadError) {
    state.error = "Select a valid request first.";
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
    };
    state.activeResponseTab = "body";
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
    };
    state.activeResponseTab = "collection";
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
