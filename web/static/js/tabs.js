// Generic tab controller. Markup contract:
//   <button data-tab-trigger="logs" class="tab-trigger active">Logs</button>
//   <div data-tab-panel="logs" class="tab-panel active">...</div>
// Clicking a trigger shows the matching panel and hides its siblings within
// the same .tabs container, then dispatches "tab:activated" on the panel
// (detail: {panel: id}) so lazy-init scripts (logs.js, stats.js, terminal.js)
// can start their stream only once the tab is actually viewed.
(function () {
	document.querySelectorAll(".tabs").forEach(function (tabs) {
		var triggers = tabs.querySelectorAll("[data-tab-trigger]");
		var panels = tabs.querySelectorAll("[data-tab-panel]");

		function activate(id) {
			triggers.forEach(function (t) {
				t.classList.toggle("active", t.dataset.tabTrigger === id);
			});
			panels.forEach(function (p) {
				var isActive = p.dataset.tabPanel === id;
				p.classList.toggle("active", isActive);
				if (isActive) {
					p.dispatchEvent(new CustomEvent("tab:activated", { bubbles: true, detail: { panel: id } }));
				}
			});
		}

		triggers.forEach(function (t) {
			t.addEventListener("click", function () { activate(t.dataset.tabTrigger); });
		});

		var initial = tabs.querySelector("[data-tab-trigger].active");
		if (initial) activate(initial.dataset.tabTrigger);
	});
})();
