const tokenInput = document.getElementById("token-input");
const reloadButton = document.getElementById("reload-button");
const statusText = document.getElementById("status-text");

const overviewGrid = document.getElementById("overview-grid");
const rulesTable = document.getElementById("rules-table");
const alertsTable = document.getElementById("alerts-table");
const notificationsTable = document.getElementById("notifications-table");

tokenInput.value = localStorage.getItem("cw-admin-token") || "";

reloadButton.addEventListener("click", async () => {
  localStorage.setItem("cw-admin-token", tokenInput.value.trim());
  await loadDashboard();
});

async function loadDashboard() {
  statusText.textContent = "Loading admin data...";
  const headers = buildHeaders();
  try {
    const [overview, rules, alerts, notifications] = await Promise.all([
      fetchJSON("/api/v1/admin/overview", headers),
      fetchJSON("/api/v1/admin/rules?limit=8", headers),
      fetchJSON("/api/v1/admin/alerts?limit=8", headers),
      fetchJSON("/api/v1/admin/notifications?limit=8", headers),
    ]);

    renderOverview(overview.data);
    renderRows(rulesTable, rules.data, (item) => ({
      title: `${item.symbol} · ${item.rule_type}`,
      detail: `threshold=${Number(item.threshold).toFixed(2)} window=${item.window_sec}s enabled=${item.enabled}`,
    }));
    renderRows(alertsTable, alerts.data, (item) => ({
      title: `${item.symbol} · ${item.type}`,
      detail: `${item.title} · ${formatTime(item.created_at)}`,
    }));
    renderRows(notificationsTable, notifications.data, (item) => ({
      title: `${item.channel} · ${item.status}`,
      detail: `${item.target} · ${formatTime(item.created_at)}`,
    }));
    statusText.textContent = "Admin data loaded.";
  } catch (error) {
    statusText.textContent = `Load failed: ${error.message}`;
    statusText.classList.add("error");
  }
}

function buildHeaders() {
  const token = tokenInput.value.trim();
  const headers = {};
  if (token) {
    headers.Authorization = `Bearer ${token}`;
  }
  return headers;
}

async function fetchJSON(url, headers) {
  const response = await fetch(url, { headers });
  const payload = await response.json();
  if (!response.ok || payload.code !== 0) {
    throw new Error(payload.message || "request failed");
  }
  return payload;
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

function escapeHTML(value) {
  return String(value)
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;")
    .replaceAll("'", "&#39;");
}

loadDashboard();
