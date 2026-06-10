let sourceContext = null;

async function fetchJSON(url) {
  const response = await fetch(url);
  if (!response.ok) {
    const payload = await response.json().catch(() => ({ error: response.statusText }));
    throw new Error(payload.error || response.statusText);
  }
  return response.json();
}

function formatSeconds(value) {
  return Number(value || 0).toFixed(3) + "s";
}

function formatDate(value) {
  if (!value) return "n/a";
  return new Date(value).toLocaleString();
}

function formatBytes(value) {
  if (value === null || value === undefined) return "n/a";
  const size = Number(value);
  if (Number.isNaN(size)) return "n/a";
  if (size < 1024) return `${size} B`;
  if (size < 1024 * 1024) return `${(size / 1024).toFixed(1)} KB`;
  if (size < 1024 * 1024 * 1024) return `${(size / (1024 * 1024)).toFixed(1)} MB`;
  return `${(size / (1024 * 1024 * 1024)).toFixed(1)} GB`;
}

function escapeHTML(value) {
  return String(value ?? "")
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;");
}

function toneFromStatus(state, accessState) {
  if (state === "error" || state === "blocked" || accessState === "inaccessible") return "error";
  if (state === "degraded") return "warning";
  if (state === "healthy") return "ok";
  return "idle";
}

function collectorTone(status) {
  if (!status) return "idle";
  return toneFromStatus(status.collectorState, status.sourceAccessState);
}

function acquisitionTone(status) {
  if (!status) return "idle";
  return toneFromStatus(status.acquisitionState, status.remoteAccessState);
}

function sourceStateMessage(emptyMessage) {
  if (!sourceContext) {
    return emptyMessage;
  }

  const acquisition = sourceContext.acquisitionStatus;
  if (acquisition && ["error", "blocked", "degraded"].includes(acquisition.acquisitionState)) {
    return acquisition.lastErrorMessage || "Remote acquisition is not healthy right now.";
  }

  const collector = sourceContext.collectorStatus;
  if (!collector) {
    return emptyMessage;
  }
  if (collector.collectorState === "error" || collector.sourceAccessState === "inaccessible") {
    return collector.lastErrorMessage || "Collector cannot parse the configured source right now.";
  }
  if (collector.collectorState === "degraded") {
    return collector.lastErrorMessage || "Collector is running in a degraded state.";
  }
  return emptyMessage;
}

function renderStatusMessage(label, message) {
  if (!message) return "";
  return `<div class="status-message"><strong>${escapeHTML(label)}:</strong> ${escapeHTML(message)}</div>`;
}

function renderFingerprintCard(item) {
  return `
    <article class="item">
      <div class="row-head">
        <h3><a href="/fingerprint.html?id=${item.id}">#${item.id}</a></h3>
        <span class="chip">${escapeHTML(item.sqlType)}</span>
      </div>
      <pre>${escapeHTML(item.normalizedSql)}</pre>
      <div class="meta">
        <span>Total time: ${formatSeconds(item.totalQueryTimeSec)}</span>
        <span>Avg time: ${formatSeconds(item.avgQueryTimeSec)}</span>
        <span>Max time: ${formatSeconds(item.maxQueryTimeSec)}</span>
        <span>Count: ${item.totalCount}</span>
        <span>Last seen: ${formatDate(item.lastSeenAt)}</span>
      </div>
    </article>
  `;
}

function renderSourceContext(source, collectorStatus, acquisitionStatus) {
  const collectorToneValue = collectorTone(collectorStatus);
  const acquisitionToneValue = acquisitionTone(acquisitionStatus);
  const remoteEndpoint = [
    source.remoteUser ? `${source.remoteUser}@` : "",
    source.remoteHost || "",
    source.remotePort ? `:${source.remotePort}` : ""
  ].join("");

  return `
    <div class="card status-card tone-${collectorToneValue}">
      <div class="section-head">
        <div>
          <p class="eyebrow">Observed Source</p>
          <h2>${escapeHTML(source.instanceName)}</h2>
        </div>
        <div class="status-stack">
          <span class="status-pill tone-${acquisitionToneValue}">acquisition: ${escapeHTML(acquisitionStatus.acquisitionState || "idle")}</span>
          <span class="status-pill tone-${collectorToneValue}">parser: ${escapeHTML(collectorStatus.collectorState || "idle")}</span>
        </div>
      </div>

      <div class="status-grid">
        <section class="status-group">
          <h3>Source</h3>
          <div class="meta">
            <span>Mode: ${escapeHTML(source.logMode || "local_file")}</span>
            <span>Observed log: ${escapeHTML(source.slowLogPath)}</span>
            <span>Initial position: ${escapeHTML(source.initialPosition || "end")}</span>
            <span>MySQL host: ${escapeHTML(source.databaseHost || "n/a")}</span>
            <span>MySQL version: ${escapeHTML(source.databaseVersion || "n/a")}</span>
          </div>
        </section>

        <section class="status-group">
          <h3>Acquisition</h3>
          <div class="meta">
            <span>Transport: ${escapeHTML(acquisitionStatus.transportMode || source.logMode || "local_file")}</span>
            <span>Remote access: ${escapeHTML(acquisitionStatus.remoteAccessState || "unknown")}</span>
            <span>Remote endpoint: ${escapeHTML(remoteEndpoint || "n/a")}</span>
            <span>Remote path: ${escapeHTML(source.remoteSlowLogPath || "n/a")}</span>
            <span>Local spool: ${escapeHTML(source.localSpoolPath || "n/a")}</span>
            <span>Spool size: ${formatBytes(acquisitionStatus.lastSpoolSizeBytes)}</span>
            <span>Spool ceiling: ${formatBytes(source.localSpoolMaxBytes)}</span>
            <span>Last pull: ${formatDate(acquisitionStatus.lastSuccessfulPullAt)}</span>
            <span>Remote offset: ${acquisitionStatus.lastRemoteOffset ?? "n/a"}</span>
          </div>
        </section>

        <section class="status-group">
          <h3>Parser</h3>
          <div class="meta">
            <span>Collector access: ${escapeHTML(collectorStatus.sourceAccessState || "unknown")}</span>
            <span>Last ingest: ${formatDate(collectorStatus.lastSuccessfulIngestAt)}</span>
            <span>Checkpoint: ${collectorStatus.lastCheckpointOffset ?? "n/a"}</span>
            <span>Current file id: ${escapeHTML(collectorStatus.lastFileIdentity || "n/a")}</span>
          </div>
        </section>
      </div>

      ${renderStatusMessage("Acquisition", acquisitionStatus.lastErrorMessage)}
      ${renderStatusMessage("Parser", collectorStatus.lastErrorMessage)}
    </div>
  `;
}

async function loadSourceContext() {
  const target = document.getElementById("source-context");
  if (!target) return null;

  try {
    const [source, collectorStatus, acquisitionStatus] = await Promise.all([
      fetchJSON("/api/source"),
      fetchJSON("/api/collector/status"),
      fetchJSON("/api/acquisition/status")
    ]);
    sourceContext = { source, collectorStatus, acquisitionStatus };
    target.innerHTML = renderSourceContext(source, collectorStatus, acquisitionStatus);
    return sourceContext;
  } catch (error) {
    target.className = "card status-card tone-error";
    target.innerHTML = `<div class="empty">${escapeHTML(error.message)}</div>`;
    return null;
  }
}

async function loadOverview() {
  const metrics = document.getElementById("overview-metrics");
  const top = document.getElementById("overview-top");
  const data = await fetchJSON("/api/dashboard/overview");
  metrics.innerHTML = `
    <div class="metric"><span>Total records</span><strong>${data.totalRecords}</strong></div>
    <div class="metric"><span>Total fingerprints</span><strong>${data.totalFingerprints}</strong></div>
    <div class="metric"><span>Total query time</span><strong>${formatSeconds(data.totalQueryTimeSec)}</strong></div>
    <div class="metric"><span>Average query time</span><strong>${formatSeconds(data.avgQueryTimeSec)}</strong></div>
    <div class="metric"><span>Max query time</span><strong>${formatSeconds(data.maxQueryTimeSec)}</strong></div>
    <div class="metric"><span>Last ingested</span><strong>${formatDate(data.lastIngestedAt)}</strong></div>
  `;

  if (!data.topFingerprints || data.topFingerprints.length === 0) {
    top.innerHTML = `<div class="empty">${escapeHTML(sourceStateMessage("No slow SQL data has been ingested yet for this source."))}</div>`;
    return;
  }
  top.innerHTML = `<div class="list">${data.topFingerprints.map(renderFingerprintCard).join("")}</div>`;
}

async function loadFingerprints(params = new URLSearchParams(window.location.search)) {
  const container = document.getElementById("fingerprint-list");
  const query = new URLSearchParams({
    page: "1",
    pageSize: "20",
    sortBy: params.get("sortBy") || "totalQueryTimeSec",
    sortOrder: "desc",
    keyword: params.get("keyword") || "",
    dbName: params.get("dbName") || "",
    sqlType: params.get("sqlType") || ""
  });

  const form = document.getElementById("filters");
  if (form) {
    form.keyword.value = query.get("keyword");
    form.dbName.value = query.get("dbName");
    form.sqlType.value = query.get("sqlType");
    form.sortBy.value = query.get("sortBy");
    form.addEventListener("submit", (event) => {
      event.preventDefault();
      const next = new URLSearchParams(new FormData(form));
      window.location.search = next.toString();
    });
  }

  const data = await fetchJSON("/api/slow-sql/fingerprints?" + query.toString());
  if (!data.items || data.items.length === 0) {
    container.innerHTML = `<div class="empty">${escapeHTML(sourceStateMessage("No fingerprints match the current filters for this source."))}</div>`;
    return;
  }
  container.innerHTML = `<div class="list">${data.items.map(renderFingerprintCard).join("")}</div>`;
}

async function loadDetail() {
  const params = new URLSearchParams(window.location.search);
  const id = params.get("id");
  const detail = document.getElementById("fingerprint-detail");
  const records = document.getElementById("fingerprint-records");
  if (!id) {
    detail.innerHTML = `<div class="empty">Missing fingerprint id.</div>`;
    return;
  }

  try {
    const fingerprint = await fetchJSON(`/api/slow-sql/fingerprints/${id}`);
    detail.innerHTML = `
      <div class="detail">
        <div class="row-head">
          <h2>Fingerprint #${fingerprint.id}</h2>
          <span class="chip">${escapeHTML(fingerprint.sqlType)}</span>
        </div>
        <pre>${escapeHTML(fingerprint.normalizedSql)}</pre>
        <div class="meta">
          <span>Hash: ${escapeHTML(fingerprint.fingerprintHash)}</span>
          <span>Main table: ${escapeHTML(fingerprint.mainTableName || "n/a")}</span>
          <span>First seen: ${formatDate(fingerprint.firstSeenAt)}</span>
          <span>Last seen: ${formatDate(fingerprint.lastSeenAt)}</span>
          <span>Total count: ${fingerprint.totalCount}</span>
          <span>Total query time: ${formatSeconds(fingerprint.totalQueryTimeSec)}</span>
          <span>Avg query time: ${formatSeconds(fingerprint.avgQueryTimeSec)}</span>
          <span>Max query time: ${formatSeconds(fingerprint.maxQueryTimeSec)}</span>
        </div>
      </div>
    `;
  } catch (error) {
    detail.innerHTML = `<div class="empty">${escapeHTML(error.message)}</div>`;
    return;
  }

  const data = await fetchJSON(`/api/slow-sql/fingerprints/${id}/records?page=1&pageSize=20&sortBy=occurredAt&sortOrder=desc`);
  if (!data.items || data.items.length === 0) {
    records.innerHTML = `<div class="empty">${escapeHTML(sourceStateMessage("No sample records are available for this fingerprint yet."))}</div>`;
    return;
  }
  records.innerHTML = `<div class="list">${data.items.map((item) => `
      <article class="item">
        <div class="meta">
          <span>Occurred: ${formatDate(item.occurredAt)}</span>
          <span>DB: ${escapeHTML(item.dbName || "n/a")}</span>
          <span>User: ${escapeHTML(item.userName || "n/a")}</span>
          <span>Host: ${escapeHTML(item.clientHost || "n/a")}</span>
          <span>Query time: ${formatSeconds(item.queryTimeSec)}</span>
        </div>
        <pre>${escapeHTML(item.rawSql)}</pre>
      </article>
  `).join("")}</div>`;
}

async function boot() {
  await loadSourceContext();

  const page = document.body.dataset.page;
  if (page === "overview") {
    await loadOverview();
  }
  if (page === "fingerprints") {
    await loadFingerprints();
  }
  if (page === "detail") {
    await loadDetail();
  }
}

if (globalThis.__SSO_TEST__) {
  globalThis.__SSO_TEST__ = {
    formatBytes,
    renderSourceContext,
    sourceStateMessage,
    setSourceContext(value) {
      sourceContext = value;
    }
  };
}

boot().catch((error) => {
  const fallback = document.getElementById("overview-top")
    || document.getElementById("fingerprint-list")
    || document.getElementById("fingerprint-detail");
  if (fallback) {
    fallback.innerHTML = `<div class="empty">${escapeHTML(error.message)}</div>`;
  }
});
