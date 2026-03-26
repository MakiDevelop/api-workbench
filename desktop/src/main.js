import { invoke } from "@tauri-apps/api/core";
import { message, open } from "@tauri-apps/plugin-dialog";
import "./styles.css";
import { t, getLang, setLang, LANGUAGES } from "./i18n.js";
import { getTheme, setTheme, applyTheme, THEMES } from "./themes.js";

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
  status: "",
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
            <span class="brand__eyebrow">${t("brandSub")}</span>
            <strong>${t("brand")}</strong>
          </div>
        </div>
        <div class="topbar__meta">
          <span class="topbar__chip">${workspace ? `${workspace.requests.length} ${t("requests").toLowerCase()}` : t("noWorkspace")}</span>
          <span class="topbar__chip">${state.selectedEnv ? `Env: ${escapeHtml(state.selectedEnv)}` : t("chooseEnv")}</span>
        </div>
        <div class="topbar__actions">
          <select class="settings-select" data-role="lang-select">${LANGUAGES.map((l) => `<option value="${l.code}" ${l.code === getLang() ? "selected" : ""}>${l.label}</option>`).join("")}</select>
          <select class="settings-select" data-role="theme-select">${THEMES.map((th) => `<option value="${th.id}" ${th.id === getTheme() ? "selected" : ""}>${th.label}</option>`).join("")}</select>
          <button class="chrome-button" data-action="pick-workspace">${t("openWorkspace")}</button>
          <button class="chrome-button chrome-button--accent" data-action="reload-workspace" ${workspace ? "" : "disabled"}>${t("reload")}</button>
        </div>
      </header>

      <main class="workspace-layout">
        <aside class="sidebar">
          <section class="sidebar-panel">
            <div class="sidebar-panel__header">
              <h2>${t("workspace")}</h2>
              <span class="panel-chip ${state.loading ? "panel-chip--loading" : ""}">
                ${state.loading ? t("busy") : t("ready")}
              </span>
            </div>

            <label class="field field--stacked">
              <span>${t("rootPath")}</span>
              <input
                type="text"
                data-role="workspace-input"
                value="${escapeAttr(state.workspaceDraft || state.workspaceRoot)}"
                placeholder="${t("rootPlaceholder")}"
              />
            </label>

            <div class="button-row">
              <button class="ghost-button" data-action="load-workspace">${t("load")}</button>
              <button class="ghost-button" data-action="pick-workspace">${t("browse")}</button>
            </div>

            <div class="summary-card">
              <div>
                <span class="summary-card__label">${t("currentRoot")}</span>
                <strong>${escapeHtml(currentWorkspace() || t("notLoaded"))}</strong>
              </div>
              <div>
                <span class="summary-card__label">${t("collection")}</span>
                <strong>${escapeHtml(workspace?.collectionPath || "requests")}</strong>
              </div>
            </div>
          </section>

          <section class="sidebar-panel">
            <div class="sidebar-panel__header">
              <h2>${t("collections")}</h2>
              <span class="panel-chip">${workspace?.requests.length ?? 0}</span>
            </div>
            <div class="tree-card ${workspace ? "tree-card--active" : ""}">
              <span class="tree-card__label">${t("requests")}</span>
              <strong>${escapeHtml(workspace?.collectionPath || "requests")}</strong>
              <span class="tree-card__meta">${workspace ? `${workspace.requests.length} ${t("specsLoaded")}` : t("loadWorkspaceHint")}</span>
            </div>
          </section>

          <section class="sidebar-panel">
            <div class="sidebar-panel__header">
              <h2>${t("environments")}</h2>
            </div>
            <div class="env-stack">
              ${renderEnvButtons(workspace)}
            </div>
          </section>

          <section class="sidebar-panel">
            <div class="sidebar-panel__header">
              <h2>${t("runner")}</h2>
            </div>
            <label class="toggle">
              <input type="checkbox" data-role="snapshot-toggle" ${state.snapshot ? "checked" : ""} />
              <span>${t("saveSnapshots")}</span>
            </label>
            <label class="field field--stacked">
              <span>${t("timeoutMs")}</span>
              <input type="number" min="1000" step="1000" data-role="timeout-input" value="${state.timeoutMs}" />
            </label>
            <div class="button-stack">
              <button class="send-button" data-action="run-selected" ${canRunSelected ? "" : "disabled"}>${t("send")}</button>
              <button class="ghost-button" data-action="run-all" ${workspace ? "" : "disabled"}>${t("runCollection")}</button>
            </div>
          </section>
        </aside>

        <section class="explorer">
          <div class="panel-topline">
            <div>
              <span class="panel-caption">${t("explorer")}</span>
              <h2>${t("requests")}</h2>
            </div>
            <span class="panel-chip">${requests.length}/${allRequestEntries().length}</span>
          </div>

          <label class="search-field">
            <input
              type="search"
              data-role="request-filter"
              value="${escapeAttr(state.requestFilter)}"
              placeholder="${t("filterPlaceholder")}"
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
                <span class="panel-caption">${t("request")}</span>
                <h2>${escapeHtml(selected?.name || t("chooseRequest"))}</h2>
              </div>
              <div class="request-actions">
                <button class="ghost-button" data-action="run-all" ${workspace ? "" : "disabled"}>${t("runner")}</button>
                <button class="send-button" data-action="run-selected" ${canRunSelected ? "" : "disabled"}>${t("send")}</button>
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
                <span class="panel-caption">${t("response")}</span>
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
        <span class="statusbar__label">${state.error ? t("error") : t("status")}</span>
        <p>${escapeHtml(state.error || state.status)}</p>
      </footer>
    </div>
  `;

  bindEvents();
}

function renderEnvButtons(workspace) {
  if (!workspace?.envs?.length) {
    return `<div class="empty-state empty-state--compact">${t("loadEnvHint")}</div>`;
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
    return `<div class="empty-state">${t("chooseRequestHint")}</div>`;
  }

  return requests
    .map((entry) => {
      const classes = [
        "request-row",
        entry.path === state.selectedRequest ? "request-row--active" : "",
        entry.loadError ? "request-row--invalid" : "",
      ].filter(Boolean).join(" ");
      const secondaryLine = entry.loadError ? t("invalidSpec") : entry.path;

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
    return `<div class="empty-state">${t("chooseRequestHint")}</div>`;
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
        t("queryParam"),
        t("value"),
        t("noQueryParams"),
      );
    case "headers":
      return renderKeyValueTable(
        sortedEntries(entry.headers),
        t("header"),
        t("value"),
        t("noHeaders"),
      );
    case "body":
      return renderBodyPanel(entry.body, t("noBody"));
    case "tests":
      return renderAssertionPanel(entry.assertions);
    default:
      return `<div class="empty-state">${t("unknownTab")}</div>`;
  }
}

function renderResponseTabContent(run, requestEntry, tab) {
  if (tab === "collection") {
    return renderCollectionRuns();
  }

  if (!run) {
    return `<div class="empty-state">${t("runHint")}</div>`;
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
        t("header"),
        t("value"),
        run.error || t("noResponseHeaders"),
      );
    case "tests":
      return renderResponseTests(run, requestEntry);
    default:
      return `<div class="empty-state">${t("unknownTab")}</div>`;
  }
}

function renderCollectionSummaryBanner() {
  if (!state.lastRun || state.lastRun.mode !== "collection") {
    return "";
  }

  return `
    <div class="collection-banner">
      <div><span>${t("total")}</span><strong>${state.lastRun.summary.total}</strong></div>
      <div><span>${t("passed")}</span><strong>${state.lastRun.summary.passed}</strong></div>
      <div><span>${t("failed")}</span><strong>${state.lastRun.summary.failed}</strong></div>
      <div><span>${t("transport")}</span><strong>${state.lastRun.summary.transport}</strong></div>
      <div><span>${t("invalid")}</span><strong>${state.lastRun.summary.invalid}</strong></div>
    </div>
  `;
}

function renderCollectionRuns() {
  if (!state.lastRun || state.lastRun.mode !== "collection") {
    return `<div class="empty-state">${t("collectionHint")}</div>`;
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
    return `<div class="empty-state">${t("noAssertions")}</div>`;
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
            <span class="test-block__label">${t("failedAssertions")}</span>
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
        <span class="test-block__label">${t("configuredAssertions")}</span>
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
    return `<span class="result-pill">${t("idle")}</span>`;
  }

  return [
    `<span class="result-pill result-pill--${statusClass(run.exitCode)}">${escapeHtml(statusLabel(run.exitCode))}</span>`,
    `<span class="result-pill">HTTP ${run.result?.statusCode ?? "n/a"}</span>`,
    `<span class="result-pill">${run.result?.durationMs ?? 0} ms</span>`,
    `<span class="result-pill">${escapeHtml(run.snapshotPath || t("snapshotOff"))}</span>`,
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
    return `${failureCount} ${t("assertionFailures")}`;
  }
  if (configuredCount === 0) {
    return run.error || t("noAssertionsConfigured");
  }
  if (run.exitCode === 0) {
    return `${configuredCount} ${t("assertionsPassed")}`;
  }
  return statusLabel(run.exitCode);
}

function testDetail(run, configuredCount, failureCount) {
  if (failureCount > 0) {
    return t("assertionDetail_fail");
  }
  if (run.error) {
    return run.error;
  }
  if (configuredCount === 0) {
    return t("assertionDetail_none");
  }
  return t("assertionDetail_pass");
}

function formatAssertion(assertion) {
  switch (assertion.type) {
    case "status":
      return `${t("statusEquals")} ${assertion.equals}`;
    case "body_contains":
      return `${t("bodyContains")} "${assertion.contains}"`;
    case "header_equals":
      return `${t("headerEquals", { key: assertion.key })} "${assertion.value}"`;
    case "json_path":
      return `${t("jsonPath", { path: assertion.path })} "${assertion.expected}"`;
    default:
      return assertion.type;
  }
}

function tabLabel(tab) {
  return t(tab) || tab;
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
    const raw = parseInt(event.target.value, 10);
    state.timeoutMs = (Number.isFinite(raw) && raw >= 100) ? Math.min(raw, 300000) : 15000;
  });
  app.querySelector("[data-role='snapshot-toggle']")?.addEventListener("change", (event) => {
    state.snapshot = Boolean(event.target.checked);
  });
  app.querySelector("[data-role='lang-select']")?.addEventListener("change", (event) => {
    setLang(event.target.value);
    render();
  });
  app.querySelector("[data-role='theme-select']")?.addEventListener("change", (event) => {
    setTheme(event.target.value);
    render();
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
    state.error = t("chooseWorkspaceFolder");
    render();
    return;
  }

  state.loading = true;
  state.error = "";
  state.status = t("loadingWorkspace");
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
    state.status = `${t("loaded")} ${workspace.requests.length} ${t("requests").toLowerCase()} ${t("from")} ${workspace.collectionPath}`;
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
    state.error = t("selectValidRequest");
    render();
    return;
  }

  state.loading = true;
  state.error = "";
  state.status = `${t("running")} ${state.selectedRequest}...`;
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
    await message(state.error, { title: t("runFailed"), kind: "error" });
  } finally {
    state.loading = false;
    render();
  }
}

async function runAll() {
  if (!state.workspace) {
    state.error = t("loadFirst");
    render();
    return;
  }

  state.loading = true;
  state.error = "";
  state.status = `${t("running")} ${state.workspace.collectionPath}...`;
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
    await message(state.error, { title: t("collectionFailed"), kind: "error" });
  } finally {
    state.loading = false;
    render();
  }
}

function statusLabel(code) {
  switch (code) {
    case 0:
      return t("passed");
    case 1:
      return t("invalid");
    case 2:
      return t("transport");
    case 3:
      return t("failed");
    default:
      return t("unknown");
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
  applyTheme();
  state.status = t("chooseWorkspaceFirst");
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
