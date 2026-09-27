// Drives the live stats panel on the container detail page via SSE.
(function () {
  const panel = document.getElementById("stats-panel");
  if (!panel) return;

  // Lazily open the SSE stream only once the Stats tab is actually viewed.
  let initialized = false;

  function init() {
    if (initialized) return;
    initialized = true;

    const containerId = panel.dataset.containerId;
    const cpuEl = document.getElementById("stat-cpu");
    const memEl = document.getElementById("stat-mem");
    const netEl = document.getElementById("stat-net");

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

    const url = `/containers/${encodeURIComponent(containerId)}/stats/stream`;
    const source = new EventSource(url);

    source.onmessage = (e) => {
      const stats = JSON.parse(e.data);
      cpuEl.textContent = `${stats.cpuPercent.toFixed(1)}%`;
      memEl.textContent = `${formatBytes(stats.memUsageBytes)} / ${formatBytes(stats.memLimitBytes)} (${stats.memPercent.toFixed(1)}%)`;
      netEl.textContent = `↓ ${formatBytes(stats.netRxBytes)} / ↑ ${formatBytes(stats.netTxBytes)}`;
    };

    source.addEventListener("streamerror", () => {
      cpuEl.textContent = "unavailable (container not running?)";
      source.close();
    });
    source.onerror = () => {
      source.close();
    };

    window.addEventListener("beforeunload", () => {
      source.close();
    });
  }

  document.addEventListener("tab:activated", (e) => {
    if (e.detail && e.detail.panel === "stats") init();
  });
})();
