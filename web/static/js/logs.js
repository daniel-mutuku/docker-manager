// Drives the log viewer on the container detail page via SSE. Deliberately
// plain JS (not htmx's SSE extension, which swaps raw innerHTML): log lines
// come from the container itself and must be inserted as text, never HTML,
// so a container that prints "<script>...</script>" to its own stdout can't
// execute anything in the viewer's browser.
(function () {
  const panel = document.getElementById("logs-panel");
  if (!panel) return;

  // The panel lives inside a lazily-shown tab; only open the SSE stream once
  // the user actually views the Logs tab, not on every page load.
  let initialized = false;

  function init() {
    if (initialized) return;
    initialized = true;

    const containerId = panel.dataset.containerId;
    const output = document.getElementById("log-output");
    const tailInput = document.getElementById("log-tail");
    const followInput = document.getElementById("log-follow");
    const reloadBtn = document.getElementById("log-reload");
    let source;

    function connect() {
      if (source) source.close();
      output.textContent = "";
      const tail = tailInput.value || "200";
      const follow = followInput.checked;
      const url = `/containers/${encodeURIComponent(containerId)}/logs/stream?tail=${encodeURIComponent(tail)}&follow=${follow}`;
      source = new EventSource(url);
      source.onmessage = (e) => {
        output.appendChild(document.createTextNode(e.data + "\n"));
        output.scrollTop = output.scrollHeight;
      };
      source.onerror = () => {
        source.close();
      };
    }

    reloadBtn.addEventListener("click", connect);
    connect();
    window.addEventListener("beforeunload", () => {
      if (source) source.close();
    });
  }

  document.addEventListener("tab:activated", (e) => {
    if (e.detail && e.detail.panel === "logs") init();
  });
})();
