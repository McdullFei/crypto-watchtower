const tokenInput = document.getElementById("token-input");
const languageSelect = document.getElementById("language-select");
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

const languageStorageKey = "cw-admin-language";
let activeLanguage = "en";

const translations = {
  en: {
    "action.reload": "Reload",
    "action.saveRule": "Save Rule",
    "auth.token": "Bearer Token",
    "brand.subtitle": "Operator Console",
    "brand.title": "CryptoWatchtower Admin",
    "detail.alerts24h": "alerts in the last 24h",
    "detail.appResponding": "HTTP service is responding",
    "detail.dependencyOk": "dependency ok",
    "detail.enabled": "enabled",
    "detail.error": "error",
    "detail.lastEvent": "last_event",
    "detail.market": "market",
    "detail.none": "none",
    "detail.reconnects": "reconnects",
    "detail.threshold": "threshold",
    "detail.window": "window",
    "field.enabled": "Enabled",
    "field.eventType": "Event Type",
    "field.exchange": "Exchange",
    "field.limit": "Limit",
    "field.ruleType": "Rule Type",
    "field.status": "Status",
    "field.symbol": "Symbol",
    "field.threshold": "Threshold",
    "field.windowSec": "Window Sec",
    "filter.title": "List Filters",
    "hero.eyebrow": "Monitoring Console",
    "hero.title": "Exchange anomaly operations",
    "language.label": "Language",
    "metric.alerts24h": "Alerts (24h)",
    "metric.enabledRules": "Enabled Rules",
    "metric.events24h": "Events (24h)",
    "metric.failed24h": "Failed (24h)",
    "metric.notifications24h": "Notifications (24h)",
    "metric.sent24h": "Sent (24h)",
    "noData.detail": "There is nothing to show yet.",
    "noData.title": "No data",
    "option.any": "Any",
    "panel.alerts": "Alerts",
    "panel.events": "Alert Events",
    "panel.notifications": "Notifications",
    "panel.ruleEditor": "Rule Editor",
    "panel.rules": "Rules",
    "panel.runtime": "Runtime Status",
    "panel.trends": "Trend Summary",
    "status.failed": "Failed",
    "status.loadFailed": "Load failed",
    "status.loaded": "Admin data loaded.",
    "status.loading": "Loading admin data...",
    "status.ready": "Ready.",
    "status.ruleSaved": "Rule saved.",
    "status.saveFailed": "Save failed",
    "status.saving": "Saving rule...",
    "status.sent": "Sent",
    "value.connected": "connected",
    "value.disconnected": "disconnected",
    "value.notAvailable": "n/a",
    "value.notionalNA": "notional n/a",
  },
  zh: {
    "action.reload": "刷新",
    "action.saveRule": "保存规则",
    "auth.token": "Bearer 令牌",
    "brand.subtitle": "运营后台",
    "brand.title": "CryptoWatchtower 管理后台",
    "detail.alerts24h": "最近 24 小时告警",
    "detail.appResponding": "HTTP 服务响应正常",
    "detail.dependencyOk": "依赖正常",
    "detail.enabled": "启用",
    "detail.error": "错误",
    "detail.lastEvent": "最近事件",
    "detail.market": "市场",
    "detail.none": "无",
    "detail.reconnects": "重连次数",
    "detail.threshold": "阈值",
    "detail.window": "窗口",
    "field.enabled": "启用",
    "field.eventType": "事件类型",
    "field.exchange": "交易所",
    "field.limit": "条数",
    "field.ruleType": "规则类型",
    "field.status": "状态",
    "field.symbol": "交易对",
    "field.threshold": "阈值",
    "field.windowSec": "窗口秒数",
    "filter.title": "列表筛选",
    "hero.eyebrow": "监控控制台",
    "hero.title": "交易所异动运营",
    "language.label": "语言",
    "metric.alerts24h": "告警 (24小时)",
    "metric.enabledRules": "启用规则",
    "metric.events24h": "事件 (24小时)",
    "metric.failed24h": "失败 (24小时)",
    "metric.notifications24h": "通知 (24小时)",
    "metric.sent24h": "已发送 (24小时)",
    "noData.detail": "暂无可展示内容。",
    "noData.title": "暂无数据",
    "option.any": "全部",
    "panel.alerts": "告警",
    "panel.events": "告警事件",
    "panel.notifications": "通知",
    "panel.ruleEditor": "规则编辑",
    "panel.rules": "规则",
    "panel.runtime": "运行状态",
    "panel.trends": "趋势汇总",
    "status.failed": "失败",
    "status.loadFailed": "加载失败",
    "status.loaded": "后台数据已加载。",
    "status.loading": "正在加载后台数据...",
    "status.ready": "就绪。",
    "status.ruleSaved": "规则已保存。",
    "status.saveFailed": "保存失败",
    "status.saving": "正在保存规则...",
    "status.sent": "已发送",
    "value.connected": "已连接",
    "value.disconnected": "已断开",
    "value.notAvailable": "无",
    "value.notionalNA": "名义金额无数据",
  },
};

tokenInput.value = localStorage.getItem("cw-admin-token") || "";
languageSelect.value = resolveInitialLanguage();
applyLanguage(languageSelect.value);

languageSelect.addEventListener("change", async () => {
  localStorage.setItem(languageStorageKey, languageSelect.value);
  applyLanguage(languageSelect.value);
  await loadDashboard();
});

reloadButton.addEventListener("click", async () => {
  localStorage.setItem("cw-admin-token", tokenInput.value.trim());
  await loadDashboard();
});

ruleForm.addEventListener("submit", async (event) => {
  event.preventDefault();
  await saveRule();
});

/**
 * Resolves the initial Admin UI language from local storage or the browser locale.
 *
 * Author: monsterfei
 * Date: 2026-06-29
 * @returns Supported language code.
 */
function resolveInitialLanguage() {
  const storedLanguage = localStorage.getItem(languageStorageKey);
  if (translations[storedLanguage]) {
    return storedLanguage;
  }
  if (navigator.language && navigator.language.toLowerCase().startsWith("zh")) {
    return "zh";
  }
  return "en";
}

/**
 * Applies translated static labels and updates the document language.
 *
 * Author: monsterfei
 * Date: 2026-06-29
 * @param language Supported language code.
 */
function applyLanguage(language) {
  activeLanguage = translations[language] ? language : "en";
  document.documentElement.lang = activeLanguage === "zh" ? "zh-CN" : "en";
  document.querySelectorAll("[data-i18n]").forEach((node) => {
    node.textContent = t(node.dataset.i18n);
  });
  document.querySelectorAll("[data-i18n-placeholder]").forEach((node) => {
    node.setAttribute("placeholder", t(node.dataset.i18nPlaceholder));
  });
}

/**
 * Looks up one translated Admin UI string.
 *
 * Author: monsterfei
 * Date: 2026-06-29
 * @param key Translation key.
 * @returns Translated label or the key when missing.
 */
function t(key) {
  return translations[activeLanguage]?.[key] || translations.en[key] || key;
}

/**
 * Loads the admin dashboard data and renders the first-view operator panels.
 *
 * Author: monsterfei
 * Date: 2026-06-29
 * modified by monsterfei on 2026-06-29
 */
async function loadDashboard() {
  statusText.textContent = t("status.loading");
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
      detail: `${t("detail.threshold")}=${Number(field(item, "threshold", "Threshold")).toFixed(2)} ${t("detail.window")}=${field(item, "window_sec", "WindowSec")}s ${t("detail.enabled")}=${field(item, "enabled", "Enabled")}`,
    }));
    renderRows(alertsTable, alerts.data, (item) => ({
      title: `${field(item, "symbol", "Symbol")} · ${field(item, "type", "Type")}`,
      detail: `${field(item, "title", "Title")} · ${formatTime(field(item, "created_at", "CreatedAt"))}`,
    }));
    renderRows(eventsTable, events.data, (item) => ({
      title: `${field(item, "symbol", "Symbol")} · ${field(item, "event_type", "EventType")}`,
      detail: `${field(item, "market_type", "MarketType") || t("detail.market")} · ${field(item, "side", "Side") || t("value.notAvailable")} · ${formatNotional(field(item, "notional", "Notional"))} · ${formatTime(field(item, "event_time", "EventTime"))}`,
    }));
    renderRows(notificationsTable, notifications.data, (item) => ({
      title: `${field(item, "channel", "Channel")} · ${field(item, "status", "Status")}`,
      detail: `${field(item, "target", "Target")} · ${formatTime(field(item, "created_at", "CreatedAt"))}`,
    }));
    statusText.textContent = t("status.loaded");
  } catch (error) {
    statusText.textContent = `${t("status.loadFailed")}: ${error.message}`;
    statusText.classList.add("error");
  }
}

/**
 * Builds bounded query strings for Admin list APIs from sidebar filters.
 *
 * Author: monsterfei
 * Date: 2026-06-29
 * @returns Query strings for each Admin list endpoint.
 * modified by monsterfei on 2026-06-29
 */
function buildListQueries() {
  const exchange = document.getElementById("filter-exchange").value;
  const symbol = document.getElementById("filter-symbol").value.trim().toUpperCase();
  const eventType = document.getElementById("filter-event-type").value.trim();
  const ruleType = document.getElementById("filter-rule-type").value.trim();
  const status = document.getElementById("filter-status").value;
  const limit = boundedLimit(document.getElementById("filter-limit").value);

  const rules = new URLSearchParams({ limit });
  const alerts = new URLSearchParams({ limit });
  const events = new URLSearchParams({ limit });
  const notifications = new URLSearchParams({ limit });
  if (exchange) {
    rules.set("exchange", exchange);
    alerts.set("exchange", exchange);
    events.set("exchange", exchange);
  }
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
 * Author: monsterfei
 * Date: 2026-06-29
 * modified by monsterfei on 2026-06-29
 */
async function saveRule() {
  statusText.textContent = t("status.saving");
  statusText.classList.remove("error");
  const payload = {
    exchange: document.getElementById("rule-exchange").value,
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
    statusText.textContent = t("status.ruleSaved");
    await loadDashboard();
  } catch (error) {
    statusText.textContent = `${t("status.saveFailed")}: ${error.message}`;
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
 * Author: monsterfei
 * Date: 2026-06-29
 * @param data Health API payload.
 * modified by monsterfei on 2026-06-29
 */
function renderRuntimeStatus(data) {
  const dependencies = data.dependencies || {};
  const dependencyRows = Object.entries(dependencies).map(([name, item]) => ({
    title: `${name} · ${item.status}`,
    detail: item.error || t("detail.dependencyOk"),
  }));
  const collectorRows = (data.collectors || []).map((item) => ({
    title: `${item.name} · ${item.connected ? t("value.connected") : t("value.disconnected")}`,
    detail: `${t("detail.reconnects")}=${item.reconnects || 0} ${t("detail.lastEvent")}=${formatTime(item.last_event_at)} ${t("detail.error")}=${item.last_error || t("detail.none")}`,
  }));
  renderRows(runtimeStatus, [{ title: `app · ${data.status}`, detail: t("detail.appResponding") }, ...dependencyRows, ...collectorRows], (item) => item);
}

/**
 * Renders translated first-view Admin counters.
 *
 * Author: monsterfei
 * Date: 2026-06-29
 * @param data Admin Overview API payload.
 */
function renderOverview(data) {
  const metrics = [
    [t("metric.enabledRules"), data.rule_count],
    [t("metric.alerts24h"), data.alert_count_24h],
    [t("metric.events24h"), data.event_count_24h],
    [t("metric.notifications24h"), data.notification_count],
  ];
  overviewGrid.innerHTML = metrics
    .map(([label, value]) => `<article class="metric"><span>${label}</span><strong>${value}</strong></article>`)
    .join("");
}

/**
 * Renders lightweight alert and notification trends.
 *
 * Author: monsterfei
 * Date: 2026-06-29
 * @param data Admin Trends API payload.
 * modified by monsterfei on 2026-06-29
 */
function renderTrends(data) {
  const metrics = [
    [t("metric.alerts24h"), data.alerts_24h],
    [t("metric.sent24h"), data.notifications_24h?.sent || 0],
    [t("metric.failed24h"), data.notifications_24h?.failed || 0],
  ];
  trendGrid.innerHTML = metrics
    .map(([label, value]) => `<article class="metric"><span>${label}</span><strong>${value}</strong></article>`)
    .join("");
  renderRows(symbolTrends, data.symbol_alerts_24h || [], (item) => ({
    title: item.symbol,
    detail: `${item.count} ${t("detail.alerts24h")}`,
  }));
}

/**
 * Renders mapped rows with a translated empty state.
 *
 * Author: monsterfei
 * Date: 2026-06-29
 * @param container Row container element.
 * @param items API row list.
 * @param mapper Row mapper.
 */
function renderRows(container, items, mapper) {
  if (!Array.isArray(items) || items.length === 0) {
    container.innerHTML = `<div class="table-row"><strong>${escapeHTML(t("noData.title"))}</strong><span>${escapeHTML(t("noData.detail"))}</span></div>`;
    return;
  }
  container.innerHTML = items
    .map((item) => {
      const row = mapper(item);
      return `<article class="table-row"><strong>${escapeHTML(row.title)}</strong><span>${escapeHTML(row.detail)}</span></article>`;
    })
    .join("");
}

/**
 * Formats timestamps using the active Admin UI locale.
 *
 * Author: monsterfei
 * Date: 2026-06-29
 * @param value Timestamp value.
 * @returns Localized timestamp text.
 */
function formatTime(value) {
  if (!value) {
    return t("value.notAvailable");
  }
  return new Date(value).toLocaleString(activeLanguage === "zh" ? "zh-CN" : "en-US");
}

/**
 * Normalizes list limits to the backend-supported bounded range.
 *
 * Author: monsterfei
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
 * Author: monsterfei
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
 * Author: monsterfei
 * Date: 2026-06-29
 * @param value Event notional value from the Admin Events API.
 * @returns Human-readable USDT notional text.
 * modified by monsterfei on 2026-06-29
 */
function formatNotional(value) {
  const number = Number(value);
  if (!Number.isFinite(number) || number <= 0) {
    return t("value.notionalNA");
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
