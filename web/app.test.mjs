import assert from "node:assert/strict";
import fs from "node:fs";
import vm from "node:vm";

const source = fs.readFileSync(new URL("./app.js", import.meta.url), "utf8");

const context = {
  console,
  URLSearchParams,
  FormData: class {},
  window: { location: { search: "" } },
  document: {
    body: { dataset: {} },
    getElementById() {
      return null;
    }
  },
  fetch: async () => ({ ok: true, json: async () => ({}) }),
  __SSO_TEST__: {}
};

vm.createContext(context);
vm.runInContext(source, context, { filename: "web/app.js" });

const helpers = context.__SSO_TEST__;
assert.ok(helpers, "expected UI test helpers to be exported");

helpers.setSourceContext({
  acquisitionStatus: {
    acquisitionState: "error",
    lastErrorMessage: "SSH pull failed"
  },
  collectorStatus: {
    collectorState: "healthy",
    sourceAccessState: "accessible"
  }
});
assert.equal(
  helpers.sourceStateMessage("empty"),
  "SSH pull failed",
  "expected acquisition error to win over empty-state fallback"
);

const rendered = helpers.renderSourceContext(
  {
    instanceName: "remote-mysql",
    logMode: "ssh_pull",
    slowLogPath: "/var/log/mysql/slow.log",
    remoteHost: "db-prod",
    remotePort: 22,
    remoteUser: "observer",
    remoteSlowLogPath: "/var/log/mysql/slow.log",
    localSpoolPath: "./var/spool/remote.log",
    localSpoolMaxBytes: 1024,
    initialPosition: "end"
  },
  {
    collectorState: "healthy",
    sourceAccessState: "accessible",
    lastSuccessfulIngestAt: null,
    lastCheckpointOffset: 0,
    lastFileIdentity: "local:inode"
  },
  {
    acquisitionState: "blocked",
    remoteAccessState: "inaccessible",
    transportMode: "ssh_pull",
    lastSuccessfulPullAt: null,
    lastRemoteOffset: 512,
    lastSpoolSizeBytes: 128,
    lastErrorMessage: "spool limit reached"
  }
);

assert.match(rendered, /acquisition: blocked/i);
assert.match(rendered, /parser: healthy/i);
assert.match(rendered, /Remote endpoint: observer@db-prod:22/);
assert.match(rendered, /Spool size: 128 B/);

const daySelect = {
  value: "30",
  options: [
    { value: "1", disabled: false },
    { value: "7", disabled: false },
    { value: "30", disabled: false }
  ]
};
helpers.syncTrendDayOptions(daySelect, "hour");
assert.equal(daySelect.value, "7");
assert.equal(daySelect.options[2].disabled, true);

const chart = helpers.renderTrendChart(
  [
    { bucketStart: "2026-06-10T00:00:00Z", totalQueryTimeSec: 1.2 },
    { bucketStart: "2026-06-11T00:00:00Z", totalQueryTimeSec: 2.4 }
  ],
  {
    title: "Total query time trend",
    valueKey: "totalQueryTimeSec",
    valueFormatter: (value) => `${value.toFixed(1)}s`,
    bucket: "day",
    emptyMessage: "no data"
  }
);
assert.match(chart, /svg/i);
assert.match(chart, /Latest: 2\.4s/);
