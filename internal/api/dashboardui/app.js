const loginForm = document.getElementById("login-form");
const registerForm = document.getElementById("register-form");
const passwordForm = document.getElementById("password-form");
const logoutButton = document.getElementById("logout-button");
const reloadButton = document.getElementById("reload-button");
const telegramBindButton = document.getElementById("telegram-bind-button");
const telegramUnbindButton = document.getElementById("telegram-unbind-button");
const statusText = document.getElementById("status-text");
const bindingStatus = document.getElementById("binding-status");
const subscriptionStatus = document.getElementById("subscription-status");
const planLimits = document.getElementById("plan-limits");
const telegramBindingToken = document.getElementById("telegram-binding-token");
const telegramBindingExpiry = document.getElementById("telegram-binding-expiry");
const telegramDeliveryEnabled = document.getElementById("telegram-delivery-enabled");
const telegramDeliveryStatus = document.getElementById("telegram-delivery-status");
const telegramPreferencesForm = document.getElementById("telegram-preferences-form");
const telegramQuietHoursEnabled = document.getElementById("telegram-quiet-hours-enabled");
const telegramQuietHoursStart = document.getElementById("telegram-quiet-hours-start");
const telegramQuietHoursEnd = document.getElementById("telegram-quiet-hours-end");
const telegramQuietHoursTimezone = document.getElementById("telegram-quiet-hours-timezone");
const telegramDigestEnabled = document.getElementById("telegram-digest-enabled");
const telegramDigestInterval = document.getElementById("telegram-digest-interval");
const ruleForm = document.getElementById("rule-form");
const rulesTable = document.getElementById("rules-table");
const alertsTable = document.getElementById("alerts-table");
const notificationsTable = document.getElementById("notifications-table");

loginForm.addEventListener("submit", async (event) => {
  event.preventDefault();
  await login();
});

registerForm.addEventListener("submit", async (event) => {
  event.preventDefault();
  await register();
});

passwordForm.addEventListener("submit", async (event) => {
  event.preventDefault();
  await changePassword();
});

logoutButton.addEventListener("click", async () => {
  await logout();
});

reloadButton.addEventListener("click", async () => {
  await loadDashboard();
});

telegramBindButton.addEventListener("click", async () => {
  await createTelegramBindingToken();
});

telegramUnbindButton.addEventListener("click", async () => {
  await unbindTelegram();
});

telegramDeliveryEnabled.addEventListener("change", async () => {
  await updateTelegramDelivery(telegramDeliveryEnabled.checked);
});

telegramPreferencesForm.addEventListener("submit", async (event) => {
  event.preventDefault();
  await updateTelegramPreferences();
});

ruleForm.addEventListener("submit", async (event) => {
  event.preventDefault();
  await saveRule();
});

/**
 * Registers a user account and loads the session dashboard.
 *
 * Author: __AUTHOR__
 * Date: 2026-07-01
 */
async function register() {
  setStatus("Registering...");
  try {
    await postJSON("/api/v1/auth/register", {
      email: document.getElementById("register-email").value.trim(),
      password: document.getElementById("register-password").value,
    });
    registerForm.reset();
    await loadDashboard();
    setStatus("Registered.");
  } catch (error) {
    setStatus(`Register failed: ${error.message}`, true);
  }
}

/**
 * Creates a short-lived Telegram binding token for the current account.
 *
 * Author: __AUTHOR__
 * Date: 2026-07-01
 */
async function createTelegramBindingToken() {
  setStatus("Creating Telegram binding token...");
  try {
    const payload = await postJSON("/api/v1/user/telegram/binding-token", {});
    renderTelegramBindingToken(payload.data);
    setStatus("Telegram binding token created.");
  } catch (error) {
    setStatus(`Telegram binding failed: ${error.message}`, true);
  }
}

/**
 * Clears Telegram binding for the current account.
 *
 * Author: __AUTHOR__
 * Date: 2026-07-01
 */
async function unbindTelegram() {
  setStatus("Unbinding Telegram...");
  try {
    const payload = await fetchJSON("/api/v1/user/telegram/binding", {
      method: "DELETE",
    });
    renderProfile(payload.data);
    telegramBindingToken.textContent = "/start <token>";
    telegramBindingExpiry.textContent = "No active binding token.";
    setStatus("Telegram unbound.");
  } catch (error) {
    setStatus(`Telegram unbind failed: ${error.message}`, true);
  }
}

/**
 * Updates Telegram delivery preference for the current account.
 *
 * Author: __AUTHOR__
 * Date: 2026-07-01
 * @param enabled Whether Telegram delivery should be enabled.
 */
async function updateTelegramDelivery(enabled) {
  setStatus("Updating Telegram delivery...");
  try {
    const payload = await fetchJSON("/api/v1/user/telegram/delivery", {
      method: "PUT",
      body: JSON.stringify({ enabled }),
    });
    renderProfile(payload.data);
    setStatus("Telegram delivery updated.");
  } catch (error) {
    telegramDeliveryEnabled.checked = !enabled;
    setStatus(`Telegram delivery update failed: ${error.message}`, true);
  }
}

/**
 * Updates quiet-hours and digest preferences for the current account.
 *
 * Author: __AUTHOR__
 * Date: 2026-07-01
 */
async function updateTelegramPreferences() {
  setStatus("Updating Telegram preferences...");
  try {
    const payload = await fetchJSON("/api/v1/user/telegram/preferences", {
      method: "PUT",
      body: JSON.stringify({
        telegram_quiet_hours_enabled: telegramQuietHoursEnabled.checked,
        telegram_quiet_hours_start: telegramQuietHoursStart.value || "22:00",
        telegram_quiet_hours_end: telegramQuietHoursEnd.value || "08:00",
        telegram_quiet_hours_timezone: telegramQuietHoursTimezone.value.trim() || "UTC",
        telegram_digest_enabled: telegramDigestEnabled.checked,
        telegram_digest_interval_min: Number(telegramDigestInterval.value || 60),
      }),
    });
    renderProfile(payload.data);
    setStatus("Telegram preferences updated.");
  } catch (error) {
    setStatus(`Telegram preference update failed: ${error.message}`, true);
  }
}


/**
 * Logs in and loads the session dashboard.
 *
 * Author: __AUTHOR__
 * Date: 2026-07-01
 */
async function login() {
  setStatus("Logging in...");
  try {
    await postJSON("/api/v1/auth/login", {
      email: document.getElementById("login-email").value.trim(),
      password: document.getElementById("login-password").value,
    });
    loginForm.reset();
    await loadDashboard();
    setStatus("Logged in.");
  } catch (error) {
    setStatus(`Login failed: ${error.message}`, true);
  }
}

/**
 * Logs out and clears rendered session data.
 *
 * Author: __AUTHOR__
 * Date: 2026-07-01
 */
async function logout() {
  setStatus("Logging out...");
  try {
    await postJSON("/api/v1/auth/logout", {});
    clearDashboard();
    setStatus("Logged out.");
  } catch (error) {
    setStatus(`Logout failed: ${error.message}`, true);
  }
}

/**
 * Changes the current account password.
 *
 * Author: __AUTHOR__
 * Date: 2026-07-01
 */
async function changePassword() {
  setStatus("Changing password...");
  try {
    await postJSON("/api/v1/user/password", {
      current_password: document.getElementById("current-password").value,
      new_password: document.getElementById("new-password").value,
    });
    passwordForm.reset();
    setStatus("Password changed.");
  } catch (error) {
    setStatus(`Password change failed: ${error.message}`, true);
  }
}

/**
 * Loads profile, personal rules, alert history, and notification logs.
 *
 * Author: __AUTHOR__
 * Date: 2026-06-30
 * modified by __AUTHOR__ on 2026-07-01
 */
async function loadDashboard() {
  setStatus("Loading...");
  try {
    const [profile, rules, alerts, notifications] = await Promise.all([
      fetchJSON("/api/v1/user/profile"),
      fetchJSON("/api/v1/user/rules"),
      fetchJSON("/api/v1/user/alerts?limit=20"),
      fetchJSON("/api/v1/user/notifications?limit=20"),
    ]);

    renderProfile(profile.data);
    renderRules(rules.data || []);
    renderAlerts(alerts.data || []);
    renderNotifications(notifications.data || []);
    setStatus("Loaded.");
  } catch (error) {
    clearDashboard();
    setStatus(`Load failed: ${error.message}`, true);
  }
}

/**
 * Saves one user-owned alert rule and reloads the dashboard.
 *
 * Author: __AUTHOR__
 * Date: 2026-06-30
 * modified by __AUTHOR__ on 2026-07-01
 */
async function saveRule() {
  const payload = {
    exchange: document.getElementById("rule-exchange").value,
    symbol: document.getElementById("rule-symbol").value.trim().toUpperCase(),
    rule_type: document.getElementById("rule-type").value,
    threshold: Number(document.getElementById("rule-threshold").value),
    window_sec: Number(document.getElementById("rule-window-sec").value || 60),
    enabled: document.getElementById("rule-enabled").checked,
  };
  setStatus("Saving...");
  try {
    await postJSON("/api/v1/user/rules", payload);
    await loadDashboard();
    setStatus("Rule saved.");
  } catch (error) {
    setStatus(`Save failed: ${error.message}`, true);
  }
}

/**
 * Renders Telegram binding and subscription state.
 *
 * Author: __AUTHOR__
 * Date: 2026-06-30
 * modified by __AUTHOR__ on 2026-07-01
 * @param profile User profile response payload.
 */
function renderProfile(profile) {
  if (profile?.telegram_bound) {
    bindingStatus.textContent = `Telegram Binding: ${profile.telegram_chat_id_masked || "bound"}`;
  } else {
    bindingStatus.textContent = "Telegram Binding: not bound";
  }
  const deliveryEnabled = profile?.telegram_delivery_enabled !== false;
  telegramDeliveryEnabled.checked = deliveryEnabled;
  const recentStatus = profile?.recent_delivery_status || "none";
  telegramDeliveryStatus.textContent = `Delivery status: ${deliveryEnabled ? "enabled" : "disabled"} · recent ${recentStatus}`;
  renderTelegramPreferences(profile?.notification_preferences || {});
  const plan = profile?.plan || "free";
  subscriptionStatus.textContent = `Subscription: ${plan}`;
  renderPlanLimits(profile?.limits || {});
}

/**
 * Renders quiet-hours and digest preferences.
 *
 * Author: __AUTHOR__
 * Date: 2026-07-01
 * @param preferences Telegram notification preference payload.
 */
function renderTelegramPreferences(preferences) {
  telegramQuietHoursEnabled.checked = Boolean(field(preferences, "telegram_quiet_hours_enabled", "TelegramQuietHoursEnabled"));
  telegramQuietHoursStart.value = field(preferences, "telegram_quiet_hours_start", "TelegramQuietHoursStart") || "22:00";
  telegramQuietHoursEnd.value = field(preferences, "telegram_quiet_hours_end", "TelegramQuietHoursEnd") || "08:00";
  telegramQuietHoursTimezone.value = field(preferences, "telegram_quiet_hours_timezone", "TelegramQuietHoursTimezone") || "UTC";
  telegramDigestEnabled.checked = Boolean(field(preferences, "telegram_digest_enabled", "TelegramDigestEnabled"));
  telegramDigestInterval.value = field(preferences, "telegram_digest_interval_min", "TelegramDigestIntervalMin") || 60;
}

/**
 * Renders subscription entitlement counters.
 *
 * Author: __AUTHOR__
 * Date: 2026-07-01
 * @param limits Plan limit payload.
 */
function renderPlanLimits(limits) {
  const rows = [
    ["Plan Rules", limits.max_rules ?? "n/a"],
    ["Alert History", limits.alert_history ?? "n/a"],
  ];
  planLimits.innerHTML = "";
  rows.forEach(([label, value]) => {
    const row = document.createElement("div");
    row.className = "limit-item";
    row.innerHTML = `<strong></strong><span></span>`;
    row.querySelector("strong").textContent = label;
    row.querySelector("span").textContent = value;
    planLimits.appendChild(row);
  });
}

/**
 * Renders a Telegram /start binding command.
 *
 * Author: __AUTHOR__
 * Date: 2026-07-01
 * @param data Binding token payload.
 */
function renderTelegramBindingToken(data) {
  const token = data?.token || "<token>";
  telegramBindingToken.textContent = `/start ${token}`;
  telegramBindingExpiry.textContent = data?.expires_at ? `Expires at ${formatTime(data.expires_at)}` : "No active binding token.";
}

/**
 * Renders personal alert rules.
 *
 * Author: __AUTHOR__
 * Date: 2026-06-30
 * @param rules Alert rule records.
 */
function renderRules(rules) {
  renderRows(rulesTable, rules, (item) => ({
    title: `${field(item, "symbol", "Symbol")} · ${field(item, "rule_type", "RuleType")}`,
    detail: `${field(item, "exchange", "Exchange")} · threshold ${field(item, "threshold", "Threshold")} · window ${field(item, "window_sec", "WindowSec")}s · ${field(item, "enabled", "Enabled") ? "enabled" : "disabled"}`,
  }));
}

/**
 * Renders personal alert history.
 *
 * Author: __AUTHOR__
 * Date: 2026-06-30
 * @param alerts Alert records.
 */
function renderAlerts(alerts) {
  renderRows(alertsTable, alerts, (item) => ({
    title: `${field(item, "symbol", "Symbol")} · ${field(item, "type", "Type")}`,
    detail: `${field(item, "title", "Title") || "Alert"} · ${formatTime(field(item, "created_at", "CreatedAt"))}`,
  }));
}

/**
 * Renders personal notification delivery logs.
 *
 * Author: __AUTHOR__
 * Date: 2026-07-01
 * @param notifications Notification log records.
 */
function renderNotifications(notifications) {
  renderRows(notificationsTable, notifications, (item) => ({
    title: `${field(item, "channel", "Channel")} · ${field(item, "status", "Status")}`,
    detail: `${field(item, "target", "Target")} · ${field(item, "alert_id", "AlertID")} · ${field(item, "error_message", "ErrorMessage") || "ok"} · ${formatTime(field(item, "created_at", "CreatedAt"))}`,
  }));
}

/**
 * Clears rendered dashboard data.
 *
 * Author: __AUTHOR__
 * Date: 2026-07-01
 */
function clearDashboard() {
  bindingStatus.textContent = "Telegram Binding: n/a";
  subscriptionStatus.textContent = "Subscription: n/a";
  telegramBindingToken.textContent = "/start <token>";
  telegramBindingExpiry.textContent = "No active binding token.";
  telegramDeliveryEnabled.checked = true;
  telegramDeliveryStatus.textContent = "Delivery status: n/a";
  renderTelegramPreferences({});
  planLimits.innerHTML = "";
  rulesTable.innerHTML = "";
  alertsTable.innerHTML = "";
  notificationsTable.innerHTML = "";
}

/**
 * Renders a compact list using a row mapper.
 *
 * Author: __AUTHOR__
 * Date: 2026-06-30
 * @param target Element that receives list rows.
 * @param items Source records.
 * @param mapper Function that maps one item to title/detail text.
 */
function renderRows(target, items, mapper) {
  target.innerHTML = "";
  if (!items.length) {
    const row = document.createElement("div");
    row.className = "list-row empty-row";
    row.textContent = "No data";
    target.appendChild(row);
    return;
  }
  items.forEach((item) => {
    const mapped = mapper(item);
    const row = document.createElement("div");
    row.className = "list-row";
    row.innerHTML = `<strong></strong><span></span>`;
    row.querySelector("strong").textContent = mapped.title;
    row.querySelector("span").textContent = mapped.detail;
    target.appendChild(row);
  });
}

/**
 * Posts a JSON request with same-origin session credentials.
 *
 * Author: __AUTHOR__
 * Date: 2026-07-01
 * @param url API URL.
 * @param payload JSON payload.
 * @returns Parsed JSON body.
 */
async function postJSON(url, payload) {
  return fetchJSON(url, {
    method: "POST",
    body: JSON.stringify(payload),
  });
}

/**
 * Fetches JSON and unwraps HTTP errors into exceptions.
 *
 * Author: __AUTHOR__
 * Date: 2026-06-30
 * modified by __AUTHOR__ on 2026-07-01
 * @param url API URL.
 * @param options Fetch options.
 * @returns Parsed JSON body.
 */
async function fetchJSON(url, options = {}) {
  const response = await fetch(url, {
    credentials: "same-origin",
    ...options,
    headers: {
      "Content-Type": "application/json",
      ...(options.headers || {}),
    },
  });
  const payload = await response.json();
  if (!response.ok || payload.code !== 0) {
    throw new Error(payload.message || response.statusText);
  }
  return payload;
}

/**
 * Reads a snake_case or Go-style field from API JSON.
 *
 * Author: __AUTHOR__
 * Date: 2026-06-30
 * @param item Source object.
 * @param snake Snake-case field name.
 * @param goName Go-style field name.
 * @returns Field value.
 */
function field(item, snake, goName) {
  return item?.[snake] ?? item?.[goName] ?? "";
}

/**
 * Formats a timestamp for compact display.
 *
 * Author: __AUTHOR__
 * Date: 2026-06-30
 * @param value Timestamp value.
 * @returns Localized timestamp or n/a.
 */
function formatTime(value) {
  if (!value) {
    return "n/a";
  }
  const parsed = new Date(value);
  if (Number.isNaN(parsed.getTime())) {
    return "n/a";
  }
  return parsed.toLocaleString();
}

/**
 * Updates the dashboard status line.
 *
 * Author: __AUTHOR__
 * Date: 2026-06-30
 * @param message Status text.
 * @param isError Whether the status is an error.
 */
function setStatus(message, isError = false) {
  statusText.textContent = message;
  statusText.classList.toggle("error", isError);
}

loadDashboard();
