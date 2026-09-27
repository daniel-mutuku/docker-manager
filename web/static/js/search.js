// Generic instant client-side table filter. Markup contract:
//   <input data-search-input data-search-target="#some-table">
// Filters `tbody tr` of the target table by substring match against the
// row's text content, on every keystroke. Re-applies after htmx swaps the
// table body (e.g. the containers list' 10s poll) so the filter survives.
(function () {
	function apply(input) {
		var target = document.querySelector(input.dataset.searchTarget);
		if (!target) return;
		var q = input.value.trim().toLowerCase();
		target.querySelectorAll("tbody tr").forEach(function (row) {
			row.hidden = q !== "" && row.textContent.toLowerCase().indexOf(q) === -1;
		});
	}

	document.querySelectorAll("[data-search-input]").forEach(function (input) {
		input.addEventListener("input", function () { apply(input); });
	});

	document.addEventListener("htmx:afterSettle", function () {
		document.querySelectorAll("[data-search-input]").forEach(apply);
	});
})();
