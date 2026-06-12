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

function formatThreshold(value) {
  const numeric = Number(value || 0);
  if (numeric <= 0) return "all collected records";
  return `query time >= ${numeric.toFixed(3)}s`;
}

function formatBucketLabel(value, bucket) {
  const date = new Date(value);
  if (bucket === "hour") {
    return date.toLocaleString([], {
      month: "short",
      day: "numeric",
      hour: "2-digit",
      minute: "2-digit"
    });
  }
  return date.toLocaleDateString([], {
    month: "short",
    day: "numeric"
  });
}

function syncTrendDayOptions(select, bucket) {
  if (!select) return;
  const maxDays = bucket === "hour" ? 7 : 30;
  const options = Array.from(select.options);
  let hasSelected = false;
  for (const option of options) {
    const numeric = Number(option.value);
    const enabled = numeric <= maxDays;
    option.disabled = !enabled;
    if (enabled && option.value === select.value) {
      hasSelected = true;
    }
  }
  if (!hasSelected) {
    select.value = bucket === "hour" ? "7" : "30";
  }
}

function summarizeSeries(series, valueKey) {
  const values = series.map((item) => Number(item[valueKey] || 0));
  const total = values.reduce((sum, value) => sum + value, 0);
  const max = values.reduce((best, value) => Math.max(best, value), 0);
  return { total, max };
}

function renderTrendChart(series, options) {
  const values = series.map((item) => Number(item[options.valueKey] || 0));
  const hasData = values.some((value) => value > 0);
  if (!hasData) {
    return `<div class="empty">${escapeHTML(options.emptyMessage)}</div>`;
  }

  const width = 920;
  const height = 260;
  const paddingX = 42;
  const paddingY = 24;
  const innerWidth = width - paddingX * 2;
  const innerHeight = height - paddingY * 2;
  const maxValue = Math.max(...values, 1);

  const points = values.map((value, index) => {
    const x = series.length === 1
      ? width / 2
      : paddingX + (innerWidth * index) / (series.length - 1);
    const y = paddingY + innerHeight - (value / maxValue) * innerHeight;
    return { x, y, value, item: series[index] };
  });

  const line = points.map((point) => `${point.x},${point.y}`).join(" ");
  const area = `M ${paddingX} ${paddingY + innerHeight} L ${points.map((point) => `${point.x} ${point.y}`).join(" L ")} L ${paddingX + innerWidth} ${paddingY + innerHeight} Z`;
  const latest = points[points.length - 1];

  return `
    <div class="trend-chart">
      <div class="trend-chart-head">
        <strong>${escapeHTML(options.title)}</strong>
        <span>Latest: ${escapeHTML(options.valueFormatter(latest.value))}</span>
      </div>
      <svg viewBox="0 0 ${width} ${height}" role="img" aria-label="${escapeHTML(options.title)}">
        <defs>
          <linearGradient id="trendFill" x1="0" x2="0" y1="0" y2="1">
            <stop offset="0%" stop-color="#f77f00" stop-opacity="0.28"></stop>
            <stop offset="100%" stop-color="#f77f00" stop-opacity="0.02"></stop>
          </linearGradient>
        </defs>
        <line x1="${paddingX}" y1="${paddingY + innerHeight}" x2="${paddingX + innerWidth}" y2="${paddingY + innerHeight}" stroke="#d9e2ec" stroke-width="2"></line>
        <path d="${area}" fill="url(#trendFill)"></path>
        <polyline fill="none" stroke="#f77f00" stroke-width="4" stroke-linecap="round" stroke-linejoin="round" points="${line}"></polyline>
        ${points.map((point) => `
          <g>
            <title>${formatBucketLabel(point.item.bucketStart, options.bucket)}: ${options.valueFormatter(point.value)}</title>
            <circle cx="${point.x}" cy="${point.y}" r="5" fill="#14213d"></circle>
          </g>
        `).join("")}
      </svg>
      <div class="axis-labels">
        <span>${escapeHTML(formatBucketLabel(series[0].bucketStart, options.bucket))}</span>
        <span>${escapeHTML(formatBucketLabel(latest.item.bucketStart, options.bucket))}</span>
      </div>
    </div>
  `;
}

function renderTrendSummaries(items) {
  return `<div class="trend-summary-grid">${items.map((item) => `
    <div class="trend-summary">
      <span>${escapeHTML(item.label)}</span>
      <strong>${escapeHTML(item.value)}</strong>
    </div>
  `).join("")}</div>`;
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

function discoveryTone(discovery) {
  if (!discovery) return "idle";
  return toneFromStatus(discovery.discoveryState, null);
}

function sourceStateMessage(emptyMessage) {
  if (!sourceContext) {
    return emptyMessage;
  }

  const acquisition = sourceContext.acquisitionStatus;
  const discovery = sourceContext.discoveryStatus;

  if (discovery && ["error", "blocked"].includes(discovery.discoveryState)) {
    return discovery.diagnosticMessage || "Source discovery failed.";
  }

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

function renderFingerprintCard(item, thresholdQuery = "") {
  const href = thresholdQuery
    ? `/fingerprint.html?id=${item.id}&${thresholdQuery}`
    : `/fingerprint.html?id=${item.id}`;
  return `
    <article class="item">
      <div class="row-head">
        <h3><a href="${href}">#${item.id}</a></h3>
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

function renderSourceContext(source, collectorStatus, acquisitionStatus, discoveryStatus) {
  const collectorToneValue = collectorTone(collectorStatus);
  const acquisitionToneValue = acquisitionTone(acquisitionStatus);
  const discoveryToneValue = discoveryTone(discoveryStatus);
  const remoteEndpoint = [
    source.remoteUser ? `${source.remoteUser}@` : "",
    source.remoteHost || "",
    source.remotePort ? `:${source.remotePort}` : ""
  ].join("");

  const isMySQLAuto = source.logMode === "mysql_auto";
  const effectiveMode = discoveryStatus && discoveryStatus.effectiveAcquisitionMode
    ? discoveryStatus.effectiveAcquisitionMode
    : (acquisitionStatus.transportMode || source.logMode || "local_file");
  const needsSSH = effectiveMode === "mysql_file" || effectiveMode === "ssh_pull";
  const isTableMode = effectiveMode === "mysql_table";

  let discoverySection = "";
  if (isMySQLAuto && discoveryStatus) {
    discoverySection = `
      <section class="status-group">
        <h3>Discovery</h3>
        <div class="meta">
          <span>State: ${escapeHTML(discoveryStatus.discoveryState || "unknown")}</span>
          <span>Slow log enabled: ${discoveryStatus.slowLogEnabled === null || discoveryStatus.slowLogEnabled === undefined ? "n/a" : discoveryStatus.slowLogEnabled ? "yes" : "no"}</span>
          <span>Log output: ${escapeHTML(discoveryStatus.discoveredLogOutput || "n/a")}</span>
          <span>Discovered file path: ${escapeHTML(discoveryStatus.discoveredFilePath || "n/a")}</span>
          <span>Effective mode: ${escapeHTML(effectiveMode)}</span>
          <span>Source version: ${escapeHTML(discoveryStatus.sourceVersion || "n/a")}</span>
          <span>Source host: ${escapeHTML(discoveryStatus.sourceHost || "n/a")}</span>
        </div>
      </section>
    `;
  }

  let transportHint = "";
  if (isTableMode) {
    transportHint = `<span>Transport: direct from mysql.slow_log (no SSH required)</span>`;
  } else if (needsSSH) {
    transportHint = `<span>Transport: SSH-based file acquisition${isMySQLAuto ? " (discovered path)" : ""}</span>`;
  } else {
    transportHint = `<span>Transport: ${escapeHTML(acquisitionStatus.transportMode || source.logMode || "local_file")}</span>`;
  }

  return `
    <div class="card status-card tone-${collectorToneValue}">
      <div class="section-head">
        <div>
          <p class="eyebrow">Observed Source</p>
          <h2>${escapeHTML(source.instanceName)}</h2>
        </div>
        <div class="status-stack">
          ${isMySQLAuto ? `<span class="status-pill tone-${discoveryToneValue}">discovery: ${escapeHTML(discoveryStatus ? discoveryStatus.discoveryState || "unknown" : "unknown")}</span>` : ""}
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

        ${discoverySection}

        <section class="status-group">
          <h3>Acquisition</h3>
          <div class="meta">
            ${transportHint}
            <span>Remote access: ${escapeHTML(acquisitionStatus.remoteAccessState || "unknown")}</span>
            <span>Remote endpoint: ${escapeHTML(remoteEndpoint || "n/a")}</span>
            <span>Remote path: ${escapeHTML(source.remoteSlowLogPath || (discoveryStatus && discoveryStatus.discoveredFilePath) || "n/a")}</span>
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

      ${isMySQLAuto && discoveryStatus && discoveryStatus.diagnosticMessage ? renderStatusMessage("Discovery", discoveryStatus.diagnosticMessage) : ""}
      ${renderStatusMessage("Acquisition", acquisitionStatus.lastErrorMessage)}
      ${renderStatusMessage("Parser", collectorStatus.lastErrorMessage)}
    </div>
  `;
}

async function loadSourceContext() {
  const target = document.getElementById("source-context");
  if (!target) return null;

  try {
    const [source, collectorStatus, acquisitionStatus, discoveryStatus] = await Promise.all([
      fetchJSON("/api/source"),
      fetchJSON("/api/collector/status"),
      fetchJSON("/api/acquisition/status"),
      fetchJSON("/api/discovery/status")
    ]);
    sourceContext = { source, collectorStatus, acquisitionStatus, discoveryStatus };
    target.innerHTML = renderSourceContext(source, collectorStatus, acquisitionStatus, discoveryStatus);
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
  const thresholdNote = document.getElementById("overview-threshold");
  const form = document.getElementById("overview-filters");
  const trendForm = document.getElementById("overview-trend-filters");
  const trendTarget = document.getElementById("overview-trend");
  const trendSummary = document.getElementById("overview-trend-summary");
  const params = new URLSearchParams(window.location.search);
  const query = new URLSearchParams();
  if (params.get("minQueryTimeSec")) {
    query.set("minQueryTimeSec", params.get("minQueryTimeSec"));
  }
  if (form) {
    form.minQueryTimeSec.value = query.get("minQueryTimeSec");
    form.addEventListener("submit", (event) => {
      event.preventDefault();
      const next = new URLSearchParams(window.location.search);
      const value = form.minQueryTimeSec.value.trim();
      if (value) {
        next.set("minQueryTimeSec", value);
      } else {
        next.delete("minQueryTimeSec");
      }
      window.location.search = next.toString();
    });
  }
  if (trendForm) {
    trendForm.trendBucket.value = params.get("trendBucket") || "day";
    trendForm.trendDays.value = params.get("trendDays") || "7";
    trendForm.trendDbName.value = params.get("trendDbName") || "";
    syncTrendDayOptions(trendForm.trendDays, trendForm.trendBucket.value);
    trendForm.trendBucket.addEventListener("change", () => {
      syncTrendDayOptions(trendForm.trendDays, trendForm.trendBucket.value);
    });
    trendForm.addEventListener("submit", (event) => {
      event.preventDefault();
      const next = new URLSearchParams(window.location.search);
      next.set("trendBucket", trendForm.trendBucket.value);
      next.set("trendDays", trendForm.trendDays.value);
      if (trendForm.trendDbName.value.trim()) {
        next.set("trendDbName", trendForm.trendDbName.value.trim());
      } else {
        next.delete("trendDbName");
      }
      window.location.search = next.toString();
    });
  }

  const data = await fetchJSON("/api/dashboard/overview" + (query.toString() ? `?${query.toString()}` : ""));
  if (form && !query.get("minQueryTimeSec")) {
    form.minQueryTimeSec.value = data.activeMinQueryTimeSec || 0;
  }
  if (thresholdNote) {
    thresholdNote.textContent = `Showing ${formatThreshold(data.activeMinQueryTimeSec)}`;
  }
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
  } else {
    top.innerHTML = `<div class="list">${data.topFingerprints.map((item) => renderFingerprintCard(item, query.toString())).join("")}</div>`;
  }

  if (trendTarget) {
    const trendQuery = new URLSearchParams();
    const trendBucket = trendForm ? trendForm.trendBucket.value : (params.get("trendBucket") || "day");
    const trendDays = trendForm ? trendForm.trendDays.value : (params.get("trendDays") || "7");
    const trendDbName = trendForm ? trendForm.trendDbName.value.trim() : (params.get("trendDbName") || "");
    trendQuery.set("bucket", trendBucket);
    trendQuery.set("days", trendDays);
    if (trendDbName) {
      trendQuery.set("dbName", trendDbName);
    }
    if (params.get("minQueryTimeSec")) {
      trendQuery.set("minQueryTimeSec", params.get("minQueryTimeSec"));
    }

    try {
      const trends = await fetchJSON("/api/dashboard/trends?" + trendQuery.toString());
      if (trendSummary) {
        trendSummary.textContent = `${trends.bucket} buckets over ${trends.days} day(s)`;
      }
      const totals = summarizeSeries(trends.series, "totalQueryTimeSec");
      const recordTotals = summarizeSeries(trends.series, "totalRecords");
      trendTarget.innerHTML = `
        <div class="trend-panel">
          ${renderTrendChart(trends.series, {
            title: "Total query time trend",
            valueKey: "totalQueryTimeSec",
            valueFormatter: formatSeconds,
            bucket: trends.bucket,
            emptyMessage: "No qualifying trend data exists for the current window and filters."
          })}
          ${renderTrendSummaries([
            { label: "Window total query time", value: formatSeconds(totals.total) },
            { label: "Peak bucket query time", value: formatSeconds(totals.max) },
            { label: "Window total records", value: String(recordTotals.total) },
            { label: "Database scope", value: trends.dbName || "All databases" }
          ])}
        </div>
      `;
    } catch (error) {
      trendTarget.innerHTML = `<div class="empty">${escapeHTML(error.message)}</div>`;
    }
  }
}

async function loadFingerprints(params = new URLSearchParams(window.location.search)) {
  const container = document.getElementById("fingerprint-list");
  const thresholdNote = document.getElementById("fingerprint-threshold");
  const query = new URLSearchParams({
    page: "1",
    pageSize: "20",
    sortBy: params.get("sortBy") || "totalQueryTimeSec",
    sortOrder: "desc",
    keyword: params.get("keyword") || "",
    dbName: params.get("dbName") || "",
    sqlType: params.get("sqlType") || "",
    minQueryTimeSec: params.get("minQueryTimeSec") || ""
  });

  const form = document.getElementById("filters");
  if (form) {
    form.keyword.value = query.get("keyword");
    form.dbName.value = query.get("dbName");
    form.sqlType.value = query.get("sqlType");
    form.sortBy.value = query.get("sortBy");
    form.minQueryTimeSec.value = query.get("minQueryTimeSec");
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
  if (form && !query.get("minQueryTimeSec")) {
    form.minQueryTimeSec.value = data.activeMinQueryTimeSec || 0;
  }
  if (thresholdNote) {
    thresholdNote.textContent = `Showing ${formatThreshold(data.activeMinQueryTimeSec)}`;
  }
  const thresholdQuery = new URLSearchParams();
  if (query.get("minQueryTimeSec")) {
    thresholdQuery.set("minQueryTimeSec", query.get("minQueryTimeSec"));
  }
  container.innerHTML = `<div class="list">${data.items.map((item) => renderFingerprintCard(item, thresholdQuery.toString())).join("")}</div>`;
}

async function loadDetail() {
  const params = new URLSearchParams(window.location.search);
  const id = params.get("id");
  const detail = document.getElementById("fingerprint-detail");
  const records = document.getElementById("fingerprint-records");
  const thresholdNote = document.getElementById("detail-threshold");
  const form = document.getElementById("detail-filters");
  const trendTarget = document.getElementById("fingerprint-trend");
  const trendSummary = document.getElementById("detail-trend-summary");
  const trendForm = document.getElementById("detail-trend-filters");
  if (!id) {
    detail.innerHTML = `<div class="empty">Missing fingerprint id.</div>`;
    return;
  }

  if (form) {
    form.minQueryTimeSec.value = params.get("minQueryTimeSec") || "";
    form.addEventListener("submit", (event) => {
      event.preventDefault();
      const next = new URLSearchParams(window.location.search);
      const value = form.minQueryTimeSec.value.trim();
      if (value) {
        next.set("minQueryTimeSec", value);
      } else {
        next.delete("minQueryTimeSec");
      }
      window.location.search = next.toString();
    });
  }
  if (trendForm) {
    trendForm.trendBucket.value = params.get("trendBucket") || "day";
    trendForm.trendDays.value = params.get("trendDays") || "7";
    syncTrendDayOptions(trendForm.trendDays, trendForm.trendBucket.value);
    trendForm.trendBucket.addEventListener("change", () => {
      syncTrendDayOptions(trendForm.trendDays, trendForm.trendBucket.value);
    });
    trendForm.addEventListener("submit", (event) => {
      event.preventDefault();
      const next = new URLSearchParams(window.location.search);
      next.set("trendBucket", trendForm.trendBucket.value);
      next.set("trendDays", trendForm.trendDays.value);
      window.location.search = next.toString();
    });
  }

  const detailQuery = params.get("minQueryTimeSec")
    ? `?minQueryTimeSec=${encodeURIComponent(params.get("minQueryTimeSec"))}`
    : "";
  try {
    const fingerprint = await fetchJSON(`/api/slow-sql/fingerprints/${id}${detailQuery}`);
    if (form && !params.get("minQueryTimeSec")) {
      form.minQueryTimeSec.value = fingerprint.activeMinQueryTimeSec || 0;
    }
    if (thresholdNote) {
      thresholdNote.textContent = `Showing ${formatThreshold(fingerprint.activeMinQueryTimeSec)}`;
    }
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

  const recordParams = new URLSearchParams({
    page: "1",
    pageSize: "20",
    sortBy: "occurredAt",
    sortOrder: "desc"
  });
  if (params.get("minQueryTimeSec")) {
    recordParams.set("minQueryTimeSec", params.get("minQueryTimeSec"));
  }
  const data = await fetchJSON(`/api/slow-sql/fingerprints/${id}/records?${recordParams.toString()}`);
  if (!data.items || data.items.length === 0) {
    records.innerHTML = `<div class="empty">${escapeHTML(sourceStateMessage("No sample records are available for this fingerprint yet."))}</div>`;
  } else {
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

  if (trendTarget) {
    const trendQuery = new URLSearchParams({
      bucket: trendForm ? trendForm.trendBucket.value : (params.get("trendBucket") || "day"),
      days: trendForm ? trendForm.trendDays.value : (params.get("trendDays") || "7")
    });
    if (params.get("minQueryTimeSec")) {
      trendQuery.set("minQueryTimeSec", params.get("minQueryTimeSec"));
    }
    try {
      const trends = await fetchJSON(`/api/slow-sql/fingerprints/${id}/trends?${trendQuery.toString()}`);
      if (trendSummary) {
        trendSummary.textContent = `${trends.bucket} buckets over ${trends.days} day(s)`;
      }
      const totals = summarizeSeries(trends.series, "totalQueryTimeSec");
      const countTotals = summarizeSeries(trends.series, "totalCount");
      trendTarget.innerHTML = `
        <div class="trend-panel">
          ${renderTrendChart(trends.series, {
            title: "Fingerprint query time trend",
            valueKey: "totalQueryTimeSec",
            valueFormatter: formatSeconds,
            bucket: trends.bucket,
            emptyMessage: "No qualifying trend data exists for this fingerprint under the current filters."
          })}
          ${renderTrendSummaries([
            { label: "Window total query time", value: formatSeconds(totals.total) },
            { label: "Peak bucket query time", value: formatSeconds(totals.max) },
            { label: "Window total executions", value: String(countTotals.total) },
            { label: "Threshold", value: formatThreshold(trends.activeMinQueryTimeSec) }
          ])}
        </div>
      `;
    } catch (error) {
      trendTarget.innerHTML = `<div class="empty">${escapeHTML(error.message)}</div>`;
    }
  }
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
    renderTrendChart,
    renderSourceContext,
    syncTrendDayOptions,
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
