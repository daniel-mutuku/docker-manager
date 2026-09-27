// Drives the interactive terminal tab on the container detail page: opens a
// WebSocket to /containers/{id}/terminal (an attached `docker exec` shell)
// and pipes it through xterm.js. Keystrokes go out as binary frames; resize
// events go out as JSON text frames; everything the shell prints comes back
// as binary frames written straight into the terminal.
(function () {
  const panel = document.getElementById("terminal-panel");
  if (!panel) return;

  const QUICK_ACTIONS = [
    { match: /laravel|php/i, commands: ["php artisan migrate", "php artisan tinker", "composer install"] },
    { match: /node|npm/i, commands: ["npm install", "npm run dev"] },
    { match: /python|django/i, commands: ["python manage.py migrate", "pip install -r requirements.txt"] },
  ];

  let initialized = false;

  function init() {
    if (initialized) return;
    initialized = true;

    const containerId = panel.dataset.containerId;
    const image = panel.dataset.image || "";
    const statusEl = document.getElementById("terminal-status");
    const mount = document.getElementById("terminal-mount");
    const chipRow = document.getElementById("terminal-chips");

    const term = new Terminal({
      cursorBlink: true,
      fontSize: 13,
      fontFamily: "ui-monospace, SFMono-Regular, Menlo, monospace",
      theme: { background: "#0b1120" },
    });
    const fit = new FitAddon.FitAddon();
    term.loadAddon(fit);
    term.open(mount);
    fit.fit();

    const proto = location.protocol === "https:" ? "wss:" : "ws:";
    const ws = new WebSocket(`${proto}//${location.host}/containers/${encodeURIComponent(containerId)}/terminal`);
    ws.binaryType = "arraybuffer";
    const encoder = new TextEncoder();

    function sendResize() {
      if (ws.readyState !== WebSocket.OPEN) return;
      ws.send(JSON.stringify({ type: "resize", cols: term.cols, rows: term.rows }));
    }

    ws.onopen = () => {
      statusEl.textContent = "Connected";
      fit.fit();
      sendResize();
    };
    ws.onmessage = (e) => term.write(new Uint8Array(e.data));
    ws.onclose = () => {
      statusEl.textContent = "Session ended.";
      term.write("\r\n\x1b[33m[session ended]\x1b[0m\r\n");
    };
    ws.onerror = () => {
      statusEl.textContent = "Connection error.";
    };

    term.onData((data) => {
      // WebSocket.send(string) always sends a text frame; encode to bytes
      // so it arrives as binary, matching the server's data/control split.
      if (ws.readyState === WebSocket.OPEN) ws.send(encoder.encode(data));
    });

    window.addEventListener("resize", () => {
      fit.fit();
      sendResize();
    });

    QUICK_ACTIONS.filter((group) => group.match.test(image)).forEach((group) => {
      group.commands.forEach((cmd) => {
        const chip = document.createElement("button");
        chip.type = "button";
        chip.className = "chip";
        chip.textContent = cmd;
        chip.addEventListener("click", () => {
          if (ws.readyState === WebSocket.OPEN) ws.send(encoder.encode(cmd + "\n"));
          term.focus();
        });
        chipRow.appendChild(chip);
      });
    });
  }

  document.addEventListener("tab:activated", (e) => {
    if (e.detail && e.detail.panel === "terminal") init();
  });
})();
