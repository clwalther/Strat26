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
		html.setAttribute("theme", "light");
	else
		html.setAttribute("theme", "dark");

	/**
	 * Change lighting theme when user changes the browser preference.
	 */
	scheme.addEventListener('change', event => {
		if (event.matches)
			html.setAttribute("theme", "light");
		else
			html.setAttribute("theme", "dark");

		// clear stored preference
		localStorage.removeItem("theme");
	});
})();

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