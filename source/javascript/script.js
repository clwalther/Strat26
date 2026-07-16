/* === DARK / LIGHT - THEME ===*/
(() => {
	const html = document.getElementsByTagName("html")[0];
	const scheme = window.matchMedia('(prefers-color-scheme: light)');

	/**
	 * Set lighting theme to user prefered theme.
	 */
	var preferedTheme = localStorage.getItem("theme");

	if (preferedTheme !== "light" && preferedTheme !== "dark")
		preferedTheme = (scheme.matches ? "light" : "dark");

	if (preferedTheme === "light")
		html.setAttribute("data-theme", "light");
	else
		html.setAttribute("data-theme", "dark");

	/**
	 * Change lighting theme when user changes the browser preference.
	 * A theme explicitly picked with the toggle button wins over the
	 * browser preference.
	 */
	scheme.addEventListener('change', event => {
		if (localStorage.getItem("theme") !== null)
			return;

		if (event.matches)
			html.setAttribute("data-theme", "light");
		else
			html.setAttribute("data-theme", "dark");
	});
})();

/**
 * Button method for toggling the lighting theme.
 */
function toggleTheme() {
	const html = document.getElementsByTagName("html")[0];

	if (html.getAttribute("data-theme") == "dark") {
		html.setAttribute("data-theme", "light");
		localStorage.setItem("theme", "light");
	}
	else {
		html.setAttribute("data-theme", "dark");
		localStorage.setItem("theme", "dark");
	}
}

/* TITLE EVENT */
(() => {
	const title = document.getElementById("title");
	const header = document.getElementById("header-title");

	var observer = new IntersectionObserver((entries, _) => {
		entries.forEach(entry => {
			if (entry.isIntersecting)
				header.classList.add("hidden");
			else
				header.classList.remove("hidden");
		});
	}, { threshold: 1.0 });

	observer.observe(title);
})();
