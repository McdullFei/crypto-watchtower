const tokenInput = document.getElementById("token-input");
const reloadButton = document.getElementById("reload-button");
const statusText = document.getElementById("status-text");
const ruleForm = document.getElementById("rule-form");

const overviewGrid = document.getElementById("overview-grid");
const runtimeStatus = document.getElementById("runtime-status");
const trendGrid = document.getElementById("trend-grid");
const symbolTrends = document.getElementById("symbol-trends");
const rulesTable = document.getElementById("rules-table");
const alertsTable = document.getElementById("alerts-table");
const eventsTable = document.getElementById("events-table");
const notificationsTable = document.getElementById("notifications-table");

tokenInput.value = localStorage.getItem("cw-admin-token") || "";

reloadButton.addEventListener("click", async () => {
  localStorage.setItem("cw-admin-token", tokenInput.value.trim());
  await loadDashboard();
});

ruleForm.addEventListener("submit", async (event) => {
  event.preventDefault();
  await saveRule();
});

/**
 * Loads the admin dashboard data and renders the first-view operator panels.
 *
 * Author: __AUTHOR__
 * Date: 2026-06-29
 * modified by __AUTHOR__ on 2026-06-29
 */
async function loadDashboard() {
  statusText.textContent = "Loading admin data...";
  statusText.classList.remove("error");
  const headers = buildHeaders();
  try {
    const queries = buildListQueries();
    const [health, overview, trends, rules, alerts, events, notifications] = await Promise.all([
      fetchJSON("/health", {}),
      fetchJSON("/api/v1/admin/overview", headers),
      fetchJSON("/api/v1/admin/trends", headers),
      fetchJSON(`/api/v1/admin/rules?${queries.rules}`, headers),
      fetchJSON(`/api/v1/admin/alerts?${queries.alerts}`, headers),
      fetchJSON(`/api/v1/admin/events?${queries.events}`, headers),
      fetchJSON(`/api/v1/admin/notifications?${queries.notifications}`, headers),
    ]);

    renderRuntimeStatus(health.data);
    renderOverview(overview.data);
    renderTrends(trends.data);
    renderRows(rulesTable, rules.data, (item) => ({
      title: `${field(item, "symbol", "Symbol")} · ${field(item, "rule_type", "RuleType")}`,
      detail: `threshold=${Number(field(item, "threshold", "Threshold")).toFixed(2)} window=${field(item, "window_sec", "WindowSec")}s enabled=${field(item, "enabled", "Enabled")}`,
    }));
    renderRows(alertsTable, alerts.data, (item) => ({
      title: `${field(item, "symbol", "Symbol")} · ${field(item, "type", "Type")}`,
      detail: `${field(item, "title", "Title")} · ${formatTime(field(item, "created_at", "CreatedAt"))}`,
    }));
    renderRows(eventsTable, events.data, (item) => ({
      title: `${field(item, "symbol", "Symbol")} · ${field(item, "event_type", "EventType")}`,
      detail: `${field(item, "market_type", "MarketType") || "market"} · ${field(item, "side", "Side") || "n/a"} · ${formatNotional(field(item, "notional", "Notional"))} · ${formatTime(field(item, "event_time", "EventTime"))}`,
    }));
    renderRows(notificationsTable, notifications.data, (item) => ({
      title: `${field(item, "channel", "Channel")} · ${field(item, "status", "Status")}`,
      detail: `${field(item, "target", "Target")} · ${formatTime(field(item, "created_at", "CreatedAt"))}`,
    }));
    statusText.textContent = "Admin data loaded.";
  } catch (error) {
    statusText.textContent = `Load failed: ${error.message}`;
    statusText.classList.add("error");
  }
}

/**
 * Builds bounded query strings for Admin list APIs from sidebar filters.
 *
 * Author: __AUTHOR__
 * Date: 2026-06-29
 * @returns Query strings for each Admin list endpoint.
 */
function buildListQueries() {
  const symbol = document.getElementById("filter-symbol").value.trim().toUpperCase();
  const eventType = document.getElementById("filter-event-type").value.trim();
  const ruleType = document.getElementById("filter-rule-type").value.trim();
  const status = document.getElementById("filter-status").value;
  const limit = boundedLimit(document.getElementById("filter-limit").value);

  const rules = new URLSearchParams({ limit });
  const alerts = new URLSearchParams({ limit });
  const events = new URLSearchParams({ limit });
  const notifications = new URLSearchParams({ limit });
  if (symbol) {
    rules.set("symbol", symbol);
    alerts.set("symbol", symbol);
    events.set("symbol", symbol);
  }
  if (ruleType) {
    rules.set("rule_type", ruleType);
    alerts.set("rule_type", ruleType);
  }
  if (eventType) {
    events.set("event_type", eventType);
  }
  if (status) {
    notifications.set("status", status);
  }
  return {
    rules: rules.toString(),
    alerts: alerts.toString(),
    events: events.toString(),
    notifications: notifications.toString(),
  };
}

/**
 * Saves one system rule through the existing protected rule write API.
 *
 * Author: __AUTHOR__
 * Date: 2026-06-29
 */
async function saveRule() {
  statusText.textContent = "Saving rule...";
  statusText.classList.remove("error");
  const payload = {
    exchange: "binance",
    symbol: document.getElementById("rule-symbol").value.trim().toUpperCase(),
    rule_type: document.getElementById("rule-type").value,
    threshold: Number(document.getElementById("rule-threshold").value),
    window_sec: Number(document.getElementById("rule-window-sec").value || 60),
    enabled: document.getElementById("rule-enabled").checked,
  };
  try {
    await fetchJSON("/api/v1/rules", buildHeaders(), {
      method: "POST",
      body: JSON.stringify(payload),
    });
    statusText.textContent = "Rule saved.";
    await loadDashboard();
  } catch (error) {
    statusText.textContent = `Save failed: ${error.message}`;
    statusText.classList.add("error");
  }
}

function buildHeaders() {
  const token = tokenInput.value.trim();
  const headers = { "Content-Type": "application/json" };
  if (token) {
    headers.Authorization = `Bearer ${token}`;
  }
  return headers;
}

async function fetchJSON(url, headers, options = {}) {
  const response = await fetch(url, { headers, ...options });
  const payload = await response.json();
  if (!response.ok || payload.code !== 0) {
    throw new Error(payload.message || "request failed");
  }
  return payload;
}

/**
 * Renders runtime health and collector status for quick operator checks.
 *
 * Author: __AUTHOR__
 * Date: 2026-06-29
 * @param data Health API payload.
 */
function renderRuntimeStatus(data) {
  const dependencies = data.dependencies || {};
  const dependencyRows = Object.entries(dependencies).map(([name, item]) => ({
    title: `${name} · ${item.status}`,
    detail: item.error || "dependency ok",
  }));
  const collectorRows = (data.collectors || []).map((item) => ({
    title: `${item.name} · ${item.connected ? "connected" : "disconnected"}`,
    detail: `reconnects=${item.reconnects || 0} last_event=${formatTime(item.last_event_at)} error=${item.last_error || "none"}`,
  }));
  renderRows(runtimeStatus, [{ title: `app · ${data.status}`, detail: "HTTP service is responding" }, ...dependencyRows, ...collectorRows], (item) => item);
}

function renderOverview(data) {
  const metrics = [
    ["Enabled Rules", data.rule_count],
    ["Alerts (24h)", data.alert_count_24h],
    ["Events (24h)", data.event_count_24h],
    ["Notifications (24h)", data.notification_count],
  ];
  overviewGrid.innerHTML = metrics
    .map(([label, value]) => `<article class="metric"><span>${label}</span><strong>${value}</strong></article>`)
    .join("");
}

/**
 * Renders lightweight alert and notification trends.
 *
 * Author: __AUTHOR__
 * Date: 2026-06-29
 * @param data Admin Trends API payload.
 */
function renderTrends(data) {
  const metrics = [
    ["Alerts (24h)", data.alerts_24h],
    ["Sent (24h)", data.notifications_24h?.sent || 0],
    ["Failed (24h)", data.notifications_24h?.failed || 0],
  ];
  trendGrid.innerHTML = metrics
    .map(([label, value]) => `<article class="metric"><span>${label}</span><strong>${value}</strong></article>`)
    .join("");
  renderRows(symbolTrends, data.symbol_alerts_24h || [], (item) => ({
    title: item.symbol,
    detail: `${item.count} alerts in the last 24h`,
  }));
}

function renderRows(container, items, mapper) {
  if (!Array.isArray(items) || items.length === 0) {
    container.innerHTML = '<div class="table-row"><strong>No data</strong><span>There is nothing to show yet.</span></div>';
    return;
  }
  container.innerHTML = items
    .map((item) => {
      const row = mapper(item);
      return `<article class="table-row"><strong>${escapeHTML(row.title)}</strong><span>${escapeHTML(row.detail)}</span></article>`;
    })
    .join("");
}

function formatTime(value) {
  if (!value) {
    return "n/a";
  }
  return new Date(value).toLocaleString();
}

/**
 * Normalizes list limits to the backend-supported bounded range.
 *
 * Author: __AUTHOR__
 * Date: 2026-06-29
 * @param value Raw limit input value.
 * @returns Limit value between 1 and 200.
 */
function boundedLimit(value) {
  const number = Number(value);
  if (!Number.isFinite(number) || number <= 0) {
    return "8";
  }
  return String(Math.min(Math.floor(number), 200));
}

/**
 * Reads either snake_case API fields or Go default PascalCase JSON fields.
 *
 * Author: __AUTHOR__
 * Date: 2026-06-29
 * @param item API response object.
 * @param snake Snake-case field name.
 * @param pascal PascalCase field name.
 * @returns Field value when present.
 */
function field(item, snake, pascal) {
  if (item && Object.prototype.hasOwnProperty.call(item, snake)) {
    return item[snake];
  }
  if (item && Object.prototype.hasOwnProperty.call(item, pascal)) {
    return item[pascal];
  }
  return "";
}

/**
 * Formats event notional values for compact alert-related event rows.
 *
 * Author: __AUTHOR__
 * Date: 2026-06-29
 * @param value Event notional value from the Admin Events API.
 * @returns Human-readable USDT notional text.
 */
function formatNotional(value) {
  const number = Number(value);
  if (!Number.isFinite(number) || number <= 0) {
    return "notional n/a";
  }
  return `${number.toFixed(2)} USDT`;
}

function escapeHTML(value) {
  return String(value)
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;")
    .replaceAll("'", "&#39;");
}

loadDashboard();
