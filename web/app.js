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

function renderFingerprintCard(item) {
  return `
    <article class="item">
      <div class="row-head">
        <h3><a href="/fingerprint.html?id=${item.id}">#${item.id}</a></h3>
        <span class="chip">${item.sqlType}</span>
      </div>
      <pre>${item.normalizedSql}</pre>
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
    top.innerHTML = `<div class="empty">No slow SQL data has been ingested yet.</div>`;
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
    container.innerHTML = `<div class="empty">No fingerprints match the current filters.</div>`;
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
          <span class="chip">${fingerprint.sqlType}</span>
        </div>
        <pre>${fingerprint.normalizedSql}</pre>
        <div class="meta">
          <span>Hash: ${fingerprint.fingerprintHash}</span>
          <span>Main table: ${fingerprint.mainTableName || "n/a"}</span>
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
    detail.innerHTML = `<div class="empty">${error.message}</div>`;
    return;
  }

  const data = await fetchJSON(`/api/slow-sql/fingerprints/${id}/records?page=1&pageSize=20&sortBy=occurredAt&sortOrder=desc`);
  if (!data.items || data.items.length === 0) {
    records.innerHTML = `<div class="empty">No sample records available.</div>`;
    return;
  }
  records.innerHTML = `<div class="list">${data.items.map((item) => `
      <article class="item">
        <div class="meta">
          <span>Occurred: ${formatDate(item.occurredAt)}</span>
          <span>DB: ${item.dbName || "n/a"}</span>
          <span>User: ${item.userName || "n/a"}</span>
          <span>Host: ${item.clientHost || "n/a"}</span>
          <span>Query time: ${formatSeconds(item.queryTimeSec)}</span>
        </div>
        <pre>${item.rawSql}</pre>
      </article>
  `).join("")}</div>`;
}

const page = document.body.dataset.page;
if (page === "overview") {
  loadOverview().catch((error) => {
    document.getElementById("overview-top").innerHTML = `<div class="empty">${error.message}</div>`;
  });
}
if (page === "fingerprints") {
  loadFingerprints().catch((error) => {
    document.getElementById("fingerprint-list").innerHTML = `<div class="empty">${error.message}</div>`;
  });
}
if (page === "detail") {
  loadDetail().catch((error) => {
    document.getElementById("fingerprint-detail").innerHTML = `<div class="empty">${error.message}</div>`;
  });
}
