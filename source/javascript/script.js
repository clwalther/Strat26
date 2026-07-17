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

/* === ASIDE === */
(() => {
	const aside = document.querySelector("aside");
	const menuLabel = document.querySelector('label[for="menu-button"]');

	// same breakpoints as the aside media query in layout.css
	const compact = window.matchMedia(
		'(max-width: 600px), (orientation: landscape) and (max-height: 600px)');

	// start collapsed on small screens so the content is visible first
	if (compact.matches)
		aside.removeAttribute("data-open");

	menuLabel.addEventListener('click', () => {
		if (aside.hasAttribute("data-open"))
			aside.removeAttribute("data-open");
		else
			aside.setAttribute("data-open", "");
	});
})();

/* === GAMES === */
(() => {
	const aside = document.querySelector("aside");
	const compact = window.matchMedia(
		'(max-width: 600px), (orientation: landscape) and (max-height: 600px)');

	const list = document.getElementById("games-list");
	const emptyState = document.getElementById("empty-state");
	const title = document.getElementById("title");
	const headerTitle = document.getElementById("header-title");

	const addDialog = document.getElementById("add-dialog");
	const addForm = document.getElementById("add-form");
	const addError = document.getElementById("add-error");
	const homeSelect = document.getElementById("add-home");
	const awaySelect = document.getElementById("add-away");
	const homeScore = document.getElementById("add-home-score");
	const awayScore = document.getElementById("add-away-score");

	// same path as the dialog close icons
	const CLOSE_PATH = "M480-442.85 309.08-271.92q-8.31 8.3-17.89 8-9.57-.31-18.27-9-8.69-8.7-8.69-18.58 0-9.88 8.69-18.58L442.85-480 271.92-650.92q-8.3-8.31-8-18.39.31-10.07 9-18.77 8.7-8.69 18.58-8.69 9.88 0 18.58 8.69L480-517.15l170.92-170.93q8.31-8.3 18.39-8.5 10.07-.19 18.77 8.5 8.69 8.7 8.69 18.58 0 9.88-8.69 18.58L517.15-480l170.93 170.92q8.3 8.31 8.5 17.89.19 9.57-8.5 18.27-8.7 8.69-18.58 8.69-9.88 0-18.58-8.69L480-442.85Z";

	var games = [];
	var filter = null;

	/**
	 * Games live in the Go server's SQLite database when the API is
	 * reachable. Without an API (e.g. on a static deployment) they are
	 * kept in localStorage instead.
	 */
	var useServer = false;

	function loadLocal() {
		try {
			const stored = JSON.parse(localStorage.getItem("games"));

			if (!Array.isArray(stored))
				return [];

			return stored.filter(game =>
				Number.isInteger(game.home) && game.home >= 1 && game.home <= 8 &&
				Number.isInteger(game.away) && game.away >= 1 && game.away <= 8);
		}
		catch {
			return [];
		}
	}

	function saveLocal() {
		localStorage.setItem("games", JSON.stringify(games));
	}

	async function initStore() {
		try {
			const response = await fetch("/api/games");

			if (!response.ok)
				throw new Error(response.status);

			games = await response.json() ?? [];
			useServer = true;
		}
		catch {
			games = loadLocal();
			useServer = false;
		}

		render();
	}

	async function addGame(game) {
		if (useServer) {
			const response = await fetch("/api/games", {
				method: "POST",
				headers: { "Content-Type": "application/json" },
				body: JSON.stringify(game)
			});

			if (!response.ok)
				throw new Error(response.status);

			games.push(await response.json());
		}
		else {
			game.id = Date.now();
			games.push(game);
			saveLocal();
		}

		render();
	}

	async function deleteGame(id) {
		if (useServer) {
			const response = await fetch("/api/games/" + id, { method: "DELETE" });

			if (!response.ok)
				throw new Error(response.status);
		}

		games = games.filter(game => String(game.id) !== String(id));

		if (!useServer)
			saveLocal();

		render();
	}

	async function clearGames() {
		if (useServer) {
			const response = await fetch("/api/games", { method: "DELETE" });

			if (!response.ok)
				throw new Error(response.status);
		}

		games = [];

		if (!useServer)
			saveLocal();

		render();
	}

	function scoreValue(input) {
		const value = Number(input.value);
		return (input.value === "" || !Number.isFinite(value)) ? null : value;
	}

	// groups 1-4 belong to Team Grün, 5-8 to Team Blau
	function teamColor(group) {
		return (group <= 4) ? "green-bg" : "blue-bg";
	}

	function cardHTML(game) {
		const score = (game.homeScore === null || game.homeScore === undefined ||
			game.awayScore === null || game.awayScore === undefined)
			? "vs"
			: game.homeScore + " : " + game.awayScore;

		return `
		<article class="game-card" data-id="${game.id}">
			<div class="team">
				<span class="dot ${teamColor(game.home)}"></span>
				<span>Gruppe ${game.home}</span>
			</div>

			<span class="score">${score}</span>

			<div class="team away">
				<span>Gruppe ${game.away}</span>
				<span class="dot ${teamColor(game.away)}"></span>
			</div>

			<button type="button" class="icon-button" data-delete aria-label="Spiel löschen">
				<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 -960 960 960">
					<path d="${CLOSE_PATH}" />
				</svg>
			</button>
		</article>`;
	}

	function render() {
		const visible = (filter === null)
			? games
			: games.filter(game => game.home === filter || game.away === filter);

		// newest game first
		list.innerHTML = [...visible].reverse().map(cardHTML).join("");

		emptyState.hidden = visible.length > 0;
		emptyState.textContent = (games.length === 0)
			? "Noch keine Spiele – über das Plus oben links kannst du das erste Spiel anlegen."
			: "Keine Spiele für diese Gruppe.";
	}

	function setTitle(text) {
		title.textContent = text;
		headerTitle.textContent = text;
	}

	/* filter by group via the sidebar */
	document.querySelectorAll('aside label[data-group]').forEach(label => {
		label.addEventListener('click', () => {
			filter = (label.dataset.group === "") ? null : Number(label.dataset.group);
			setTitle(filter === null ? "Alle Spiele" : "Gruppe " + filter);

			if (compact.matches)
				aside.removeAttribute("data-open");

			render();
		});
	});

	/* add a game */
	document.querySelector('label[for="add-confirm-button"]').addEventListener('click', async () => {
		const home = Number(homeSelect.value);
		const away = Number(awaySelect.value);

		if (home === away) {
			addError.textContent = "Bitte zwei verschiedene Gruppen wählen.";
			addError.hidden = false;
			return;
		}

		try {
			await addGame({
				home: home,
				away: away,
				homeScore: scoreValue(homeScore),
				awayScore: scoreValue(awayScore)
			});
		}
		catch {
			addError.textContent = "Speichern fehlgeschlagen – ist der Server erreichbar?";
			addError.hidden = false;
			return;
		}

		addError.hidden = true;
		addForm.reset();
		addDialog.close();
	});

	/* delete a single game */
	list.addEventListener('click', event => {
		const button = event.target.closest('[data-delete]');

		if (button === null)
			return;

		const id = button.closest('.game-card').dataset.id;
		deleteGame(id).catch(error => console.error("delete failed:", error));
	});

	/* delete all games (more dialog) */
	document.getElementById("reset-button").addEventListener('click', () => {
		if (!confirm("Wirklich alle Spiele löschen?"))
			return;

		clearGames()
			.then(() => document.getElementById("more-dialog").close())
			.catch(error => console.error("delete all failed:", error));
	});

	initStore();
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
