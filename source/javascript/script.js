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

	document.addEventListener("scroll", event => {
		if (title.getBoundingClientRect().top <= 0)
			header.classList.remove("hidden");
		else
			header.classList.add("hidden");
	});
})();

/* DIALOG */
(() => {
	const dialogs = document.getElementsByTagName("dialog");

	Array.from(dialogs).forEach(dialog => {
		dialog.addEventListener("click", event => {
			var bound = dialog.getBoundingClientRect();

			if (!(bound.left <= event.clientX && event.clientX <= bound.right
				&& bound.top <= event.clientY && event.clientY <= bound.bottom))
				dialog.close();
		})
	});
})();