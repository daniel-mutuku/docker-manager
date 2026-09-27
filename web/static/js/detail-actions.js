// Reloads (or redirects) the page after a container action succeeds, without
// inline event-handler attributes — the app's CSP is strict `default-src
// 'self'` with no 'unsafe-inline'/'unsafe-eval', so behavior has to live in
// an external script rather than an hx-on:: attribute.
(function () {
	document.body.addEventListener("htmx:afterRequest", function (e) {
		const el = e.target;
		if (!el.matches || !el.matches("[data-reload-on-success]")) return;
		if (!e.detail.successful) return;
		const redirect = el.dataset.redirectOnSuccess;
		if (redirect) {
			window.location = redirect;
		} else {
			window.location.reload();
		}
	});
})();
