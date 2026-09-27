// Drives the dashboard's live CPU/memory/network tiles via SSE
// (/dashboard/stats/stream), updating the big numbers and redrawing three
// hand-rolled SVG sparklines (no charting library) from a rolling window of
// recent samples.
(function () {
  const cpuValue = document.getElementById("live-cpu-value");
  if (!cpuValue) return;

  const memValue = document.getElementById("live-mem-value");
  const netRxValue = document.getElementById("live-net-rx");
  const netTxValue = document.getElementById("live-net-tx");
  const cpuLine = document.getElementById("live-cpu-line");
  const memLine = document.getElementById("live-mem-line");
  const netRxLine = document.getElementById("live-net-rx-line");
  const netTxLine = document.getElementById("live-net-tx-line");

  const MAX_POINTS = 30; // ~1 minute at the server's 2s tick
  const W = 240, H = 48, PAD = 3;

  const cpuHistory = [];
  const memHistory = [];
  const rxHistory = [];
  const txHistory = [];

  function pushCapped(arr, v) {
    arr.push(v);
    if (arr.length > MAX_POINTS) arr.shift();
  }

  function toPoints(values, min, max) {
    const n = values.length;
    if (n === 0) return "";
    const range = max - min || 1;
    return values
      .map((v, i) => {
        const x = n === 1 ? W : (i / (n - 1)) * W;
        const y = H - PAD - ((v - min) / range) * (H - PAD * 2);
        return `${x.toFixed(1)},${y.toFixed(1)}`;
      })
      .join(" ");
  }

  function formatBytes(bytes) {
    const units = ["B", "KB", "MB", "GB", "TB"];
    let value = bytes;
    let unit = 0;
    while (value >= 1024 && unit < units.length - 1) {
      value /= 1024;
      unit++;
    }
    return `${value.toFixed(1)} ${units[unit]}`;
  }

  const source = new EventSource("/dashboard/stats/stream");

  source.onmessage = (e) => {
    const s = JSON.parse(e.data);
    const rxBps = s.netRxBps || 0;
    const txBps = s.netTxBps || 0;

    pushCapped(cpuHistory, s.cpuPercent);
    pushCapped(memHistory, s.memUsageBytes);
    pushCapped(rxHistory, rxBps);
    pushCapped(txHistory, txBps);

    cpuValue.textContent = `${s.cpuPercent.toFixed(1)}%`;
    memValue.textContent = formatBytes(s.memUsageBytes);
    netRxValue.textContent = `↓ ${formatBytes(rxBps)}/s`;
    netTxValue.textContent = `↑ ${formatBytes(txBps)}/s`;

    cpuLine.setAttribute("points", toPoints(cpuHistory, 0, Math.max(100, ...cpuHistory)));
    memLine.setAttribute("points", toPoints(memHistory, 0, Math.max(1, ...memHistory)));

    const netMax = Math.max(1, ...rxHistory, ...txHistory);
    netRxLine.setAttribute("points", toPoints(rxHistory, 0, netMax));
    netTxLine.setAttribute("points", toPoints(txHistory, 0, netMax));
  };

  window.addEventListener("beforeunload", () => source.close());
})();
