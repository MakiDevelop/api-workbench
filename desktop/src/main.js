import { invoke } from "@tauri-apps/api/core";
import { message, open } from "@tauri-apps/plugin-dialog";
import "./styles.css";
import { t, getLang, setLang, LANGUAGES } from "./i18n.js";
import { getTheme, setTheme, applyTheme, THEMES } from "./themes.js";

const app = document.querySelector("#app");
const REQUEST_TABS = ["params", "headers", "auth", "body", "tests"];
const RESPONSE_TABS = ["body", "headers", "tests", "collection", "snapshots"];

const METHODS = ["GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"];

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
  editing: false,
  editDraft: null,
  showImportModal: false,
  importCurlText: "",
  snapshots: [],
  diffResult: null,
  diffLeft: "",
  diffRight: "",
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
          <button class="chrome-button" data-action="import-curl" ${workspace ? "" : "disabled"}>${t("importCurl")}</button>
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
                <span class="panel-caption">${t("request")} ${state.editing ? `<span class="inline-chip inline-chip--edit">${t("editing")}</span>` : ""}</span>
                <h2>${escapeHtml(state.editing ? (state.editDraft?.name || t("newRequest")) : (selected?.name || t("chooseRequest")))}</h2>
              </div>
              <div class="request-actions">
                ${state.editing
                  ? `<button class="ghost-button" data-action="cancel-edit">${t("cancel")}</button>
                     <button class="send-button" data-action="save-request">${t("save")}</button>`
                  : `<button class="ghost-button" data-action="edit-request" ${canRunSelected ? "" : "disabled"}>${"Edit"}</button>
                     <button class="ghost-button" data-action="run-all" ${workspace ? "" : "disabled"}>${t("runner")}</button>
                     <button class="send-button" data-action="run-selected" ${canRunSelected ? "" : "disabled"}>${t("send")}</button>`
                }
              </div>
            </div>

            <div class="request-builder ${selected?.loadError && !state.editing ? "request-builder--invalid" : ""}">
              <div class="request-builder__bar">
                ${state.editing
                  ? `<select class="method-select" data-role="edit-method">${METHODS.map(m => `<option value="${m}" ${m === state.editDraft?.method ? "selected" : ""}>${m}</option>`).join("")}</select>
                     <input type="text" class="url-input" data-role="edit-url" value="${escapeAttr(state.editDraft?.url || "")}" placeholder="https://api.example.com/endpoint" />`
                  : `<span class="method-badge method-badge--${methodClass(selected?.method)}">${escapeHtml(selected?.method || "GET")}</span>
                     <div class="url-field">${escapeHtml(selected?.url || "Select a request from the collection explorer.")}</div>`
                }
                <span class="inline-chip">${escapeHtml(state.selectedEnv || "local")}</span>
              </div>
              <div class="tab-row">
                ${renderTabs(REQUEST_TABS, activeRequestTab, "select-request-tab")}
              </div>
              <div class="tab-panel">
                ${state.editing ? renderEditableTabContent(state.editDraft, activeRequestTab) : renderRequestTabContent(selected, activeRequestTab)}
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

      ${state.showImportModal ? renderImportModal() : ""}

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
    case "auth":
      return renderAuthPanel(entry.auth, false);
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
  if (tab === "snapshots") {
    return renderSnapshotsTab();
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

function renderSnapshotsTab() {
  if (!state.workspace) {
    return `<div class="empty-state">${t("loadFirst")}</div>`;
  }

  const snapList = state.snapshots.length > 0
    ? state.snapshots.map((snap) => `
        <div class="snapshot-row ${snap.isLatest ? "snapshot-row--latest" : ""}">
          <div class="snapshot-row__info">
            <strong>${escapeHtml(snap.name)}</strong>
            <span class="snapshot-row__time">${escapeHtml(snap.capturedAt || "unknown")}</span>
          </div>
          <div class="snapshot-row__actions">
            <label><input type="radio" name="diff-left" value="${escapeAttr(snap.path)}" ${state.diffLeft === snap.path ? "checked" : ""} data-role="diff-left-radio" /> A</label>
            <label><input type="radio" name="diff-right" value="${escapeAttr(snap.path)}" ${state.diffRight === snap.path ? "checked" : ""} data-role="diff-right-radio" /> B</label>
          </div>
        </div>
      `).join("")
    : `<div class="empty-state">${t("noSnapshots")}</div>`;

  const diffSection = state.diffResult
    ? renderDiffResult(state.diffResult)
    : "";

  return `
    <div class="snapshots-tab">
      <div class="snapshots-toolbar">
        <button class="ghost-button" data-action="load-snapshots">${t("refreshSnapshots")}</button>
        <button class="send-button" data-action="run-diff" ${state.diffLeft && state.diffRight && state.diffLeft !== state.diffRight ? "" : "disabled"}>${t("compareDiff")}</button>
      </div>
      <div class="snapshot-list">${snapList}</div>
      ${diffSection}
    </div>
  `;
}

function renderDiffResult(diff) {
  if (diff.same) {
    return `<div class="callout callout--success"><strong>${t("noDifferences")}</strong></div>`;
  }

  const rows = diff.changes.map((c) => {
    const typeClass = c.type === "added" ? "diff-added" : c.type === "removed" ? "diff-removed" : "diff-changed";
    return `
      <div class="diff-row ${typeClass}">
        <div class="diff-row__field">
          <span class="inline-chip inline-chip--${c.type}">${escapeHtml(c.type)}</span>
          <strong>${escapeHtml(c.field)}</strong>
        </div>
        ${c.left ? `<div class="diff-row__value diff-row__value--left"><span class="diff-label">A</span><pre>${escapeHtml(c.left)}</pre></div>` : ""}
        ${c.right ? `<div class="diff-row__value diff-row__value--right"><span class="diff-label">B</span><pre>${escapeHtml(c.right)}</pre></div>` : ""}
      </div>
    `;
  }).join("");

  return `
    <div class="diff-result">
      <div class="diff-header">
        <span>A: ${escapeHtml(diff.leftTime)}</span>
        <span>B: ${escapeHtml(diff.rightTime)}</span>
        <span>${diff.changes.length} change(s)</span>
      </div>
      ${rows}
    </div>
  `;
}

function renderAuthPanel(auth, editable) {
  const authType = auth?.type || "none";

  if (!editable) {
    if (!auth || authType === "none") {
      return `<div class="empty-state">No authentication configured.</div>`;
    }
    return `
      <div class="auth-panel">
        <div class="auth-info">
          <span class="inline-chip">${escapeHtml(authType)}</span>
          ${authType === "bearer" ? `<span>Token: <code>${escapeHtml(auth.token || "")}</code></span>` : ""}
          ${authType === "basic" ? `<span>User: <code>${escapeHtml(auth.user || "")}</code></span>` : ""}
          ${authType === "api-key" ? `<span>Key: <code>${escapeHtml(auth.key || "X-API-Key")}</code> in ${escapeHtml(auth.in || "header")}</span>` : ""}
        </div>
      </div>
    `;
  }

  return `
    <div class="auth-panel auth-panel--editable">
      <div class="auth-type-row">
        <label>Type</label>
        <select class="settings-select" data-role="edit-auth-type">
          <option value="none" ${authType === "none" ? "selected" : ""}>None</option>
          <option value="bearer" ${authType === "bearer" ? "selected" : ""}>Bearer Token</option>
          <option value="basic" ${authType === "basic" ? "selected" : ""}>Basic Auth</option>
          <option value="api-key" ${authType === "api-key" ? "selected" : ""}>API Key</option>
        </select>
      </div>
      ${authType === "bearer" ? `
        <label class="field field--stacked">
          <span>Token</span>
          <input type="text" class="kv-input" data-role="edit-auth-token" value="${escapeAttr(auth?.token || "")}" placeholder="$API_TOKEN or paste token" />
        </label>
      ` : ""}
      ${authType === "basic" ? `
        <label class="field field--stacked">
          <span>Username</span>
          <input type="text" class="kv-input" data-role="edit-auth-user" value="${escapeAttr(auth?.user || "")}" placeholder="username" />
        </label>
        <label class="field field--stacked">
          <span>Password</span>
          <input type="password" class="kv-input" data-role="edit-auth-pass" value="${escapeAttr(auth?.pass || "")}" placeholder="password" />
        </label>
      ` : ""}
      ${authType === "api-key" ? `
        <label class="field field--stacked">
          <span>Header Name</span>
          <input type="text" class="kv-input" data-role="edit-auth-key" value="${escapeAttr(auth?.key || "X-API-Key")}" placeholder="X-API-Key" />
        </label>
        <label class="field field--stacked">
          <span>Value</span>
          <input type="text" class="kv-input" data-role="edit-auth-value" value="${escapeAttr(auth?.value || "")}" placeholder="$API_KEY or paste key" />
        </label>
        <div class="auth-type-row">
          <label>Send in</label>
          <select class="settings-select" data-role="edit-auth-in">
            <option value="header" ${(auth?.in || "header") === "header" ? "selected" : ""}>Header</option>
            <option value="query" ${auth?.in === "query" ? "selected" : ""}>Query Parameter</option>
          </select>
        </div>
      ` : ""}
    </div>
  `;
}

function renderImportModal() {
  return `
    <div class="modal-overlay" data-action="close-import">
      <div class="modal-box" onclick="event.stopPropagation()">
        <h3>${t("importCurl")}</h3>
        <textarea class="import-textarea" data-role="import-curl-input" rows="8" placeholder="${escapeAttr(t("importCurlPlaceholder"))}">${escapeHtml(state.importCurlText)}</textarea>
        <div class="modal-actions">
          <button class="ghost-button" data-action="close-import">${t("cancel")}</button>
          <button class="send-button" data-action="do-import-curl">${t("importBtn")}</button>
        </div>
      </div>
    </div>
  `;
}

function renderEditableTabContent(draft, tab) {
  if (!draft) return `<div class="empty-state">${t("chooseRequestHint")}</div>`;

  switch (tab) {
    case "params":
      return renderEditableKVTable(draft.query || {}, "query", t("queryParam"), t("value"));
    case "headers":
      return renderEditableKVTable(draft.headers || {}, "headers", t("header"), t("value"));
    case "auth":
      return renderAuthPanel(draft.auth, true);
    case "body":
      return renderEditableBody(draft);
    case "tests":
      return renderAssertionPanel(draft.assertions);
    default:
      return `<div class="empty-state">${t("unknownTab")}</div>`;
  }
}

function renderEditableKVTable(record, fieldName, keyLabel, valueLabel) {
  const entries = Object.entries(record);

  const rows = entries.map(([key, value], idx) => `
    <div class="kv-table__row kv-table__row--editable">
      <input type="text" class="kv-input" data-role="edit-kv-key" data-field="${fieldName}" data-index="${idx}" value="${escapeAttr(key)}" placeholder="${escapeAttr(keyLabel)}" />
      <input type="text" class="kv-input" data-role="edit-kv-value" data-field="${fieldName}" data-index="${idx}" value="${escapeAttr(value)}" placeholder="${escapeAttr(valueLabel)}" />
      <button class="ghost-button ghost-button--small" data-action="remove-kv-row" data-field="${fieldName}" data-index="${idx}">&times;</button>
    </div>
  `).join("");

  return `
    <div class="kv-table kv-table--editable">
      <div class="kv-table__header">
        <span>${escapeHtml(keyLabel)}</span>
        <span>${escapeHtml(valueLabel)}</span>
        <span></span>
      </div>
      ${rows}
      <button class="ghost-button ghost-button--add" data-action="add-kv-row" data-field="${fieldName}">+ ${t("addRow")}</button>
    </div>
  `;
}

function renderEditableBody(draft) {
  const bodyType = draft.body?.type || "json";
  const bodyContent = draft.body?.content || "";

  return `
    <div class="body-panel body-panel--editable">
      <div class="body-panel__controls">
        <select class="settings-select" data-role="edit-body-type">
          <option value="json" ${bodyType === "json" ? "selected" : ""}>JSON</option>
          <option value="text" ${bodyType === "text" ? "selected" : ""}>Text</option>
          <option value="form" ${bodyType === "form" ? "selected" : ""}>Form</option>
        </select>
      </div>
      <textarea class="body-textarea" data-role="edit-body-content" rows="10" placeholder="${escapeAttr(t("noBody"))}">${escapeHtml(bodyContent)}</textarea>
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
  const tabs = RESPONSE_TABS.filter((tab) => {
    if (tab === "collection" && state.lastRun?.mode !== "collection") return false;
    return true;
  });
  return tabs;
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

  app.querySelectorAll("[data-action='load-snapshots']").forEach((button) => {
    button.addEventListener("click", loadSnapshots);
  });
  app.querySelectorAll("[data-action='run-diff']").forEach((button) => {
    button.addEventListener("click", runDiff);
  });
  app.querySelectorAll("[data-role='diff-left-radio']").forEach((radio) => {
    radio.addEventListener("change", () => {
      state.diffLeft = radio.value;
      state.diffResult = null;
      render();
    });
  });
  app.querySelectorAll("[data-role='diff-right-radio']").forEach((radio) => {
    radio.addEventListener("change", () => {
      state.diffRight = radio.value;
      state.diffResult = null;
      render();
    });
  });

  app.querySelectorAll("[data-action='import-curl']").forEach((button) => {
    button.addEventListener("click", () => {
      state.showImportModal = true;
      state.importCurlText = "";
      render();
    });
  });
  app.querySelectorAll("[data-action='close-import']").forEach((el) => {
    el.addEventListener("click", () => {
      state.showImportModal = false;
      render();
    });
  });
  app.querySelectorAll("[data-action='do-import-curl']").forEach((button) => {
    button.addEventListener("click", doImportCurl);
  });
  app.querySelectorAll("[data-action='edit-request']").forEach((button) => {
    button.addEventListener("click", startEditing);
  });
  app.querySelectorAll("[data-action='cancel-edit']").forEach((button) => {
    button.addEventListener("click", () => {
      state.editing = false;
      state.editDraft = null;
      render();
    });
  });
  app.querySelectorAll("[data-action='save-request']").forEach((button) => {
    button.addEventListener("click", saveRequest);
  });
  app.querySelectorAll("[data-action='add-kv-row']").forEach((button) => {
    button.addEventListener("click", () => {
      const field = button.dataset.field;
      if (!state.editDraft[field]) state.editDraft[field] = {};
      state.editDraft[field][""] = "";
      render();
    });
  });
  app.querySelectorAll("[data-action='remove-kv-row']").forEach((button) => {
    button.addEventListener("click", () => {
      const field = button.dataset.field;
      const idx = parseInt(button.dataset.index, 10);
      const entries = Object.entries(state.editDraft[field] || {});
      entries.splice(idx, 1);
      state.editDraft[field] = Object.fromEntries(entries);
      render();
    });
  });

  app.querySelectorAll("[data-role='edit-kv-key']").forEach((input) => {
    input.addEventListener("change", () => {
      syncKVEdits();
    });
  });
  app.querySelectorAll("[data-role='edit-kv-value']").forEach((input) => {
    input.addEventListener("change", () => {
      syncKVEdits();
    });
  });

  app.querySelector("[data-role='edit-method']")?.addEventListener("change", (event) => {
    if (state.editDraft) state.editDraft.method = event.target.value;
  });
  app.querySelector("[data-role='edit-url']")?.addEventListener("input", (event) => {
    if (state.editDraft) state.editDraft.url = event.target.value;
  });
  app.querySelector("[data-role='edit-body-type']")?.addEventListener("change", (event) => {
    if (state.editDraft) {
      if (!state.editDraft.body) state.editDraft.body = { type: "json", content: "" };
      state.editDraft.body.type = event.target.value;
    }
  });
  app.querySelector("[data-role='edit-body-content']")?.addEventListener("input", (event) => {
    if (state.editDraft) {
      if (!state.editDraft.body) state.editDraft.body = { type: "json", content: "" };
      state.editDraft.body.content = event.target.value;
    }
  });
  app.querySelector("[data-role='edit-auth-type']")?.addEventListener("change", (event) => {
    if (!state.editDraft) return;
    const type = event.target.value;
    if (type === "none") {
      state.editDraft.auth = null;
    } else {
      state.editDraft.auth = { type, ...(state.editDraft.auth || {}) };
      state.editDraft.auth.type = type;
    }
    render();
  });
  app.querySelector("[data-role='edit-auth-token']")?.addEventListener("input", (event) => {
    if (state.editDraft?.auth) state.editDraft.auth.token = event.target.value;
  });
  app.querySelector("[data-role='edit-auth-user']")?.addEventListener("input", (event) => {
    if (state.editDraft?.auth) state.editDraft.auth.user = event.target.value;
  });
  app.querySelector("[data-role='edit-auth-pass']")?.addEventListener("input", (event) => {
    if (state.editDraft?.auth) state.editDraft.auth.pass = event.target.value;
  });
  app.querySelector("[data-role='edit-auth-key']")?.addEventListener("input", (event) => {
    if (state.editDraft?.auth) state.editDraft.auth.key = event.target.value;
  });
  app.querySelector("[data-role='edit-auth-value']")?.addEventListener("input", (event) => {
    if (state.editDraft?.auth) state.editDraft.auth.value = event.target.value;
  });
  app.querySelector("[data-role='edit-auth-in']")?.addEventListener("change", (event) => {
    if (state.editDraft?.auth) state.editDraft.auth.in = event.target.value;
  });
  app.querySelector("[data-role='import-curl-input']")?.addEventListener("input", (event) => {
    state.importCurlText = event.target.value;
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

function startEditing() {
  const selected = selectedRequestEntry();
  if (!selected || selected.loadError) return;

  state.editing = true;
  state.editDraft = {
    name: selected.name || "",
    method: selected.method || "GET",
    url: selected.url || "",
    headers: { ...(selected.headers || {}) },
    query: { ...(selected.query || {}) },
    body: selected.body ? { type: selected.body.type || "json", content: selected.body.content || "" } : null,
    auth: selected.auth ? { ...selected.auth } : null,
    assertions: selected.assertions ? [...selected.assertions] : [],
    path: selected.path,
  };
  render();
}

function syncKVEdits() {
  if (!state.editDraft) return;

  for (const field of ["query", "headers"]) {
    const keyInputs = app.querySelectorAll(`[data-role="edit-kv-key"][data-field="${field}"]`);
    const valInputs = app.querySelectorAll(`[data-role="edit-kv-value"][data-field="${field}"]`);
    const newObj = {};
    keyInputs.forEach((keyInput, idx) => {
      const k = keyInput.value;
      const v = valInputs[idx]?.value || "";
      if (k) newObj[k] = v;
    });
    state.editDraft[field] = newObj;
  }
}

async function saveRequest() {
  if (!state.editDraft || !state.workspace) return;

  syncKVEdits();

  const draft = state.editDraft;

  // Build the spec to save.
  const spec = {
    name: draft.name || "Untitled",
    method: (draft.method || "GET").toUpperCase(),
    url: draft.url || "",
  };
  if (draft.headers && Object.keys(draft.headers).length > 0) spec.headers = draft.headers;
  if (draft.query && Object.keys(draft.query).length > 0) spec.query = draft.query;
  if (draft.assertions && draft.assertions.length > 0) spec.assertions = draft.assertions;

  if (draft.auth && draft.auth.type && draft.auth.type !== "none") {
    spec.auth = draft.auth;
  }

  if (draft.body && draft.body.content) {
    const bodyType = draft.body.type || "json";
    let content = draft.body.content;

    if (bodyType === "json") {
      try { content = JSON.parse(content); } catch { /* leave as string */ }
    } else if (bodyType === "text") {
      // text content is stored as a JSON string
    } else if (bodyType === "form") {
      try { content = JSON.parse(content); } catch { /* leave as string */ }
    }

    spec.body = { type: bodyType, content };
  }

  state.loading = true;
  state.error = "";
  render();

  try {
    await invoke("save_request_gui", {
      root: state.workspace.root,
      filePath: draft.path,
      spec,
    });
    state.editing = false;
    state.editDraft = null;
    state.status = t("saveSuccess");
    await loadWorkspace(state.workspace.root);
  } catch (error) {
    state.error = String(error);
  } finally {
    state.loading = false;
    render();
  }
}

async function loadSnapshots() {
  if (!state.workspace) return;

  try {
    const snapshots = await invoke("list_snapshots_gui", { root: state.workspace.root });
    state.snapshots = snapshots || [];
    state.diffLeft = "";
    state.diffRight = "";
    state.diffResult = null;
    state.status = `${state.snapshots.length} snapshot(s) found`;
  } catch (error) {
    state.error = String(error);
  }
  render();
}

async function runDiff() {
  if (!state.workspace || !state.diffLeft || !state.diffRight) return;

  state.loading = true;
  state.error = "";
  render();

  try {
    const result = await invoke("diff_snapshots_gui", {
      root: state.workspace.root,
      leftPath: state.diffLeft,
      rightPath: state.diffRight,
    });
    state.diffResult = result;
    state.status = result.same ? t("noDifferences") : `${result.changes.length} difference(s) found`;
  } catch (error) {
    state.error = String(error);
  } finally {
    state.loading = false;
    render();
  }
}

async function doImportCurl() {
  if (!state.importCurlText.trim() || !state.workspace) return;

  state.loading = true;
  state.error = "";
  state.showImportModal = false;
  render();

  try {
    const result = await invoke("import_curl_gui", {
      root: state.workspace.root,
      curlCmd: state.importCurlText,
      collection: state.workspace.collectionPath || "requests",
    });
    state.status = `${t("importSuccess")} → ${result.savedPath}`;
    state.importCurlText = "";
    await loadWorkspace(state.workspace.root);
    // Select the newly imported request.
    if (result.savedPath) {
      state.selectedRequest = result.savedPath;
    }
  } catch (error) {
    state.error = String(error);
  } finally {
    state.loading = false;
    render();
  }
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
