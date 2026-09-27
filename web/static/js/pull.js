// Drives the "pull image" form on the images page. Uses fetch() with a
// streamed response body rather than SSE/EventSource: pulling an image is a
// mutating action that belongs behind POST, and EventSource can only issue
// GET requests. Each line of the response body is one of Docker's pull
// progress JSON objects; we only display its "status" field, as text (never
// innerHTML), since a malicious registry could otherwise inject markup.
(function () {
  const form = document.getElementById("pull-form");
  if (!form) return;

  const refInput = document.getElementById("pull-ref");
  const output = document.getElementById("pull-output");

  form.addEventListener("submit", async (e) => {
    e.preventDefault();
    const ref = refInput.value.trim();
    if (!ref) return;

    output.hidden = false;
    output.textContent = `Pulling ${ref}...\n`;

    const submitBtn = form.querySelector("button[type=submit]");
    submitBtn.disabled = true;

    try {
      const resp = await fetch("/images/pull", {
        method: "POST",
        headers: { "Content-Type": "application/x-www-form-urlencoded" },
        body: `ref=${encodeURIComponent(ref)}`,
      });

      if (!resp.ok || !resp.body) {
        output.appendChild(document.createTextNode(`Failed: ${resp.status} ${await resp.text()}\n`));
        return;
      }

      const reader = resp.body.getReader();
      const decoder = new TextDecoder();
      let buffer = "";

      while (true) {
        const { done, value } = await reader.read();
        if (done) break;
        buffer += decoder.decode(value, { stream: true });
        const lines = buffer.split("\n");
        buffer = lines.pop();
        for (const line of lines) {
          if (!line) continue;
          let status = line;
          try {
            status = JSON.parse(line).status || line;
          } catch {
            // not JSON; show the raw line
          }
          output.appendChild(document.createTextNode(status + "\n"));
          output.scrollTop = output.scrollHeight;
        }
      }

      output.appendChild(document.createTextNode("Done. Reloading...\n"));
      setTimeout(() => window.location.reload(), 800);
    } catch (err) {
      output.appendChild(document.createTextNode(`Error: ${err}\n`));
    } finally {
      submitBtn.disabled = false;
    }
  });
})();
