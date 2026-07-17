(() => {
	"use strict";

	const STORAGE_KEY = "strat26.offline.v2";
	const THEME_KEY = "strat26.theme";
	const fallbackTeams = [
		{ id: 1, name: "Gruppe 1", shortName: "G1", color: "green", rating: 1580 },
		{ id: 2, name: "Gruppe 2", shortName: "G2", color: "green", rating: 1510 },
		{ id: 3, name: "Gruppe 3", shortName: "G3", color: "green", rating: 1460 },
		{ id: 4, name: "Gruppe 4", shortName: "G4", color: "green", rating: 1420 },
		{ id: 5, name: "Gruppe 5", shortName: "B5", color: "blue", rating: 1560 },
		{ id: 6, name: "Gruppe 6", shortName: "B6", color: "blue", rating: 1525 },
		{ id: 7, name: "Gruppe 7", shortName: "B7", color: "blue", rating: 1485 },
		{ id: 8, name: "Gruppe 8", shortName: "B8", color: "blue", rating: 1440 },
	];

	let state = { season: { id: 1, name: "Saison 2026", status: "active" }, teams: fallbackTeams, games: [], standing: [] };
	let serverAvailable = true;
	let activeView = "dashboard";
	let currentGameID = null;
	let toastTimer = null;

	const elements = {
		pageTitle: document.getElementById("page-title"),
		seasonName: document.getElementById("season-name"),
		stats: document.getElementById("stats-grid"),
		upcoming: document.getElementById("upcoming-list"),
		miniStanding: document.getElementById("mini-standing"),
		games: document.getElementById("games-list"),
		standing: document.getElementById("standing-body"),
		teams: document.getElementById("team-grid"),
		statusFilter: document.getElementById("status-filter"),
		teamFilter: document.getElementById("team-filter"),
		connectionLabel: document.getElementById("connection-label"),
		connectionDetail: document.getElementById("connection-detail"),
		offlineBanner: document.getElementById("offline-banner"),
		createDialog: document.getElementById("create-dialog"),
		gameDialog: document.getElementById("game-dialog"),
		createForm: document.getElementById("create-form"),
		createError: document.getElementById("create-error"),
		homeTeam: document.getElementById("home-team"),
		awayTeam: document.getElementById("away-team"),
		gameMeta: document.getElementById("game-dialog-meta"),
		gameTitle: document.getElementById("game-dialog-title"),
		gameScoreboard: document.getElementById("game-scoreboard"),
		gameTimeline: document.getElementById("game-timeline"),
		toast: document.getElementById("toast"),
		sidebar: document.querySelector(".sidebar"),
	};

	async function api(path, options = {}) {
		const response = await fetch(`/api${path}`, {
			headers: { "Content-Type": "application/json", ...(options.headers || {}) },
			...options,
		});
		if (!response.ok) {
			let message = `Anfrage fehlgeschlagen (${response.status})`;
			try {
				const payload = await response.json();
				message = payload.error || message;
			} catch (_) {
				// Keep the status-based message for non-JSON errors.
			}
			const error = new Error(message);
			error.status = response.status;
			throw error;
		}
		if (response.status === 204) return null;
		return response.json();
	}

	async function load() {
		try {
			state = await api("/state");
			serverAvailable = true;
		} catch (error) {
			console.warn("Strat26 API unavailable, using local storage", error);
			serverAvailable = false;
			state = loadOfflineState();
		}
		updateConnection();
		renderAll();
	}

	function loadOfflineState() {
		let games = [];
		try {
			const stored = JSON.parse(localStorage.getItem(STORAGE_KEY) || "[]");
			if (Array.isArray(stored)) games = stored;
		} catch (_) {
			games = [];
		}
		return {
			season: { id: 1, name: "Saison 2026", status: "active", createdAt: new Date().toISOString() },
			teams: fallbackTeams,
			games,
			standing: computeStanding(fallbackTeams, games),
		};
	}

	function saveOfflineGames(games) {
		localStorage.setItem(STORAGE_KEY, JSON.stringify(games));
		state.games = games;
		state.standing = computeStanding(state.teams, games);
	}

	function updateConnection() {
		const dot = document.querySelector(".live-dot");
		elements.offlineBanner.classList.toggle("hidden", serverAvailable);
		dot.classList.toggle("offline", !serverAvailable);
		elements.connectionLabel.textContent = serverAvailable ? "Server verbunden" : "Offline-Modus";
		elements.connectionDetail.textContent = serverAvailable ? "SQLite · Live-Daten" : "Lokaler Browser-Speicher";
	}

	function renderAll() {
		state.standing = state.standing || computeStanding(state.teams, state.games);
		elements.seasonName.textContent = state.season?.name || "Aktive Saison";
		renderTeamOptions();
		renderStats();
		renderUpcoming();
		renderMiniStanding();
		renderGames();
		renderStanding();
		renderTeams();
	}

	function renderTeamOptions() {
		const filterValue = elements.teamFilter.value;
		const homeValue = elements.homeTeam.value;
		const awayValue = elements.awayTeam.value;
		const options = state.teams.map((team) => `<option value="${team.id}">${escapeHTML(team.name)}</option>`).join("");
		elements.teamFilter.innerHTML = `<option value="">Alle Teams</option>${options}`;
		elements.homeTeam.innerHTML = options;
		elements.awayTeam.innerHTML = options;
		elements.teamFilter.value = filterValue;
		elements.homeTeam.value = homeValue || String(state.teams[0]?.id || "");
		elements.awayTeam.value = awayValue || String(state.teams[4]?.id || state.teams[1]?.id || "");
	}

	function renderStats() {
		const played = state.games.filter((game) => game.status === "played").length;
		const scheduled = state.games.length - played;
		const goals = state.games.reduce((sum, game) => sum + (game.homeScore ?? 0) + (game.awayScore ?? 0), 0);
		const leader = state.standing[0];
		const stats = [
			["Spiele", state.games.length, `${played} abgeschlossen`, "◫"],
			["Offen", scheduled, scheduled ? "bereit zur Simulation" : "Saison aktuell", "◷"],
			["Tore", goals, played ? `${(goals / played).toFixed(1)} pro Spiel` : "noch keine Ergebnisse", "◎"],
			["Spitzenreiter", leader?.shortName || "–", leader ? `${leader.points} Punkte` : "noch ohne Wertung", "↑"],
		];
		elements.stats.innerHTML = stats.map(([label, value, detail, icon]) => `
			<article class="stat-card"><div class="stat-top"><span>${label}</span><span class="stat-icon">${icon}</span></div><strong>${value}</strong><small>${detail}</small></article>
		`).join("");
	}

	function renderUpcoming() {
		const scheduled = state.games.filter((game) => game.status === "scheduled");
		const games = (scheduled.length ? scheduled : [...state.games].reverse()).slice(0, 4);
		elements.upcoming.innerHTML = games.length ? games.map(matchCard).join("") : emptyState("Noch kein Spielplan", "Erstelle den Liga-Spielplan mit einem Klick.");
	}

	function renderMiniStanding() {
		elements.miniStanding.innerHTML = state.standing.slice(0, 5).map((row) => `
			<div class="mini-row"><span class="mini-rank">${row.rank}</span><span class="mini-team"><i class="color-dot ${row.color}"></i>${escapeHTML(row.teamName)}</span><span class="mini-points">${row.points} P</span></div>
		`).join("");
	}

	function renderGames() {
		const status = elements.statusFilter.value;
		const teamID = Number(elements.teamFilter.value || 0);
		const games = state.games.filter((game) => (!status || game.status === status) && (!teamID || game.homeTeamId === teamID || game.awayTeamId === teamID));
		elements.games.innerHTML = games.length ? games.map(matchCard).join("") : emptyState("Keine Begegnungen", "Passe die Filter an oder lege ein neues Spiel an.");
	}

	function matchCard(game) {
		const home = teamByID(game.homeTeamId);
		const away = teamByID(game.awayTeamId);
		if (!home || !away) return "";
		const score = game.status === "played" ? `${game.homeScore} : ${game.awayScore}` : "vs";
		const scoreClass = game.status === "played" ? "" : " scheduled";
		const meta = `${game.round ? `Spieltag ${game.round}` : "Freundschaftsspiel"} · ${formatKickoff(game.kickoff)}`;
		return `<button class="match-card" type="button" data-game-id="${game.id}">
			<span class="team-label"><i class="team-badge ${home.color}">${home.shortName}</i><span>${escapeHTML(home.name)}</span></span>
			<strong class="score${scoreClass}">${score}</strong>
			<span class="team-label away"><span>${escapeHTML(away.name)}</span><i class="team-badge ${away.color}">${away.shortName}</i></span>
			<span class="match-meta">${meta}</span>
		</button>`;
	}

	function renderStanding() {
		elements.standing.innerHTML = state.standing.map((row) => `
			<tr><td>${row.rank}</td><td><span class="table-team"><i class="team-badge ${row.color}">${row.shortName}</i>${escapeHTML(row.teamName)}</span></td><td>${row.played}</td><td>${row.won}</td><td>${row.drawn}</td><td>${row.lost}</td><td>${row.goalsFor}:${row.goalsAgainst}</td><td>${signed(row.goalDifference)}</td><td class="table-points">${row.points}</td></tr>
		`).join("");
	}

	function renderTeams() {
		elements.teams.innerHTML = state.teams.map((team) => {
			const ratingValue = Math.max(90, Math.min(500, team.rating - 1200));
			return `<article class="team-card ${team.color}"><div class="team-card-top"><i class="team-badge ${team.color}">${team.shortName}</i><span class="rating">Stärke<strong>${team.rating}</strong></span></div><h3>${escapeHTML(team.name)}</h3><p>${team.color === "green" ? "Team Grün" : "Team Blau"}</p><progress class="rating-bar" max="500" value="${ratingValue}" aria-label="Rating ${team.rating}"></progress></article>`;
		}).join("");
	}

	async function generateSchedule() {
		const replace = state.games.length > 0;
		if (replace && !window.confirm("Der bestehende Spielplan wird vollständig ersetzt. Fortfahren?")) return;
		await withBusy(document.getElementById("generate-schedule"), async () => {
			if (serverAvailable) {
				state = await api("/schedule/generate", { method: "POST", body: JSON.stringify({ replace, intervalDays: 7 }) });
			} else {
				saveOfflineGames(generateLocalSchedule());
			}
			renderAll();
			showToast("Der Spielplan mit 16 Begegnungen ist bereit.");
		});
	}

	function generateLocalSchedule() {
		const greens = [1, 2, 3, 4];
		const blues = [5, 6, 7, 8];
		const next = new Date();
		next.setDate(next.getDate() + ((6 - next.getDay() + 7) % 7 || 7));
		next.setHours(15, 0, 0, 0);
		const games = [];
		for (let round = 0; round < 4; round += 1) {
			for (let index = 0; index < 4; index += 1) {
				const kickoff = new Date(next);
				kickoff.setDate(kickoff.getDate() + round * 7);
				games.push({ id: round * 4 + index + 1, seasonId: 1, round: round + 1, homeTeamId: greens[index], awayTeamId: blues[(index + round) % 4], kickoff: kickoff.toISOString(), status: "scheduled", homeScore: null, awayScore: null, simulationSeed: null, events: [] });
			}
		}
		return games;
	}

	async function simulateAll() {
		if (!state.games.length) {
			showToast("Erstelle zuerst einen Spielplan.", true);
			return;
		}
		await withBusy(document.getElementById("simulate-all"), async () => {
			if (serverAvailable) {
				state = await api("/simulate", { method: "POST", body: JSON.stringify({ seed: Date.now() }) });
			} else {
				const seed = Date.now();
				saveOfflineGames(state.games.map((game) => game.status === "scheduled" ? simulateLocal(game, seed + game.id * 7919) : game));
			}
			renderAll();
			showToast("Alle offenen Spiele wurden simuliert.");
		});
	}

	async function createGame(event) {
		event.preventDefault();
		elements.createError.classList.add("hidden");
		const data = new FormData(elements.createForm);
		const homeScore = data.get("homeScore");
		const awayScore = data.get("awayScore");
		if (data.get("homeTeamId") === data.get("awayTeamId")) {
			return showFormError("Heim- und Auswärtsteam müssen verschieden sein.");
		}
		if ((homeScore === "") !== (awayScore === "")) {
			return showFormError("Für ein Ergebnis müssen beide Torzahlen gesetzt sein.");
		}
		const payload = {
			homeTeamId: Number(data.get("homeTeamId")),
			awayTeamId: Number(data.get("awayTeamId")),
			round: Number(data.get("round") || 0),
			kickoff: data.get("kickoff") || null,
			homeScore: homeScore === "" ? null : Number(homeScore),
			awayScore: awayScore === "" ? null : Number(awayScore),
		};
		try {
			if (serverAvailable) {
				await api("/games", { method: "POST", body: JSON.stringify(payload) });
				state = await api("/state");
			} else {
				const now = Date.now();
				const game = { id: now, seasonId: 1, ...payload, status: payload.homeScore === null ? "scheduled" : "played", simulationSeed: null, events: [] };
				saveOfflineGames([...state.games, game]);
			}
			elements.createDialog.close();
			elements.createForm.reset();
			renderAll();
			showToast("Spiel wurde gespeichert.");
		} catch (error) {
			showFormError(error.message);
		}
	}

	async function openGame(id) {
		currentGameID = Number(id);
		let details;
		try {
			if (serverAvailable) {
				details = await api(`/games/${currentGameID}`);
			} else {
				const game = state.games.find((entry) => entry.id === currentGameID);
				details = { game, events: game?.events || [] };
			}
			if (!details.game) throw new Error("Spiel nicht gefunden");
			renderGameDialog(details);
			elements.gameDialog.showModal();
		} catch (error) {
			showToast(error.message, true);
		}
	}

	function renderGameDialog(details) {
		const game = details.game;
		const home = teamByID(game.homeTeamId);
		const away = teamByID(game.awayTeamId);
		elements.gameMeta.textContent = `${game.round ? `Spieltag ${game.round}` : "Freundschaftsspiel"} · ${formatKickoff(game.kickoff)}`;
		elements.gameTitle.textContent = `${home.name} gegen ${away.name}`;
		const score = game.status === "played" ? `${game.homeScore} : ${game.awayScore}` : "– : –";
		elements.gameScoreboard.innerHTML = `
			<div class="scoreboard-team"><i class="team-badge ${home.color}">${home.shortName}</i><span>${escapeHTML(home.name)}</span></div>
			<div class="scoreboard-score">${score}<small>${game.status === "played" ? "Endstand" : "Geplant"}</small></div>
			<div class="scoreboard-team"><i class="team-badge ${away.color}">${away.shortName}</i><span>${escapeHTML(away.name)}</span></div>`;
		elements.gameTimeline.innerHTML = details.events.length ? details.events.map((event) => `
			<div class="event-row"><span class="event-minute">${event.minute}'</span><span>${event.kind === "goal" ? "⚽" : "▰"}</span><span class="event-detail">${escapeHTML(event.player)}<small>${escapeHTML(event.detail)}</small></span></div>
		`).join("") : emptyState(game.status === "played" ? "Manuelles Ergebnis" : "Noch nicht angepfiffen", game.status === "played" ? "Für manuelle Ergebnisse gibt es keine Ereignis-Timeline." : "Starte die Simulation für Ergebnis und Ereignisse.");
		document.getElementById("simulate-game").textContent = game.status === "played" ? "Neu simulieren" : "Spiel simulieren";
	}

	async function simulateCurrentGame() {
		if (!currentGameID) return;
		try {
			let details;
			if (serverAvailable) {
				details = await api(`/games/${currentGameID}/simulate`, { method: "POST", body: JSON.stringify({ seed: Date.now() }) });
				state = await api("/state");
			} else {
				const games = state.games.map((game) => game.id === currentGameID ? simulateLocal(game, Date.now()) : game);
				saveOfflineGames(games);
				const game = games.find((entry) => entry.id === currentGameID);
				details = { game, events: game.events || [] };
			}
			renderGameDialog(details);
			renderAll();
			showToast("Simulation abgeschlossen.");
		} catch (error) {
			showToast(error.message, true);
		}
	}

	async function deleteCurrentGame() {
		if (!currentGameID || !window.confirm("Dieses Spiel wirklich löschen?")) return;
		try {
			if (serverAvailable) {
				await api(`/games/${currentGameID}`, { method: "DELETE" });
				state = await api("/state");
			} else {
				saveOfflineGames(state.games.filter((game) => game.id !== currentGameID));
			}
			elements.gameDialog.close();
			renderAll();
			showToast("Spiel gelöscht.");
		} catch (error) {
			showToast(error.message, true);
		}
	}

	function simulateLocal(game, seed) {
		const home = teamByID(game.homeTeamId);
		const away = teamByID(game.awayTeamId);
		const random = mulberry32(Number(seed) >>> 0);
		const homeExpectation = clamp(1.54 + (home.rating - away.rating) / 360, .25, 3.8);
		const awayExpectation = clamp(1.24 + (away.rating - home.rating) / 360, .25, 3.8);
		const homeScore = Math.min(poissonLocal(random, homeExpectation), 9);
		const awayScore = Math.min(poissonLocal(random, awayExpectation), 9);
		const events = [];
		for (const [team, goals] of [[home, homeScore], [away, awayScore]]) {
			for (let index = 0; index < goals; index += 1) {
				events.push({ id: events.length + 1, gameId: game.id, minute: 1 + Math.floor(random() * 90), kind: "goal", teamId: team.id, player: `${team.shortName} · Spieler ${1 + Math.floor(random() * 18)}`, detail: "Tor" });
			}
		}
		events.sort((a, b) => a.minute - b.minute);
		return { ...game, status: "played", homeScore, awayScore, simulationSeed: seed, events };
	}

	function computeStanding(teams, games) {
		const rows = new Map(teams.map((team) => [team.id, { rank: 0, teamId: team.id, teamName: team.name, shortName: team.shortName, color: team.color, played: 0, won: 0, drawn: 0, lost: 0, goalsFor: 0, goalsAgainst: 0, goalDifference: 0, points: 0 }]));
		for (const game of games) {
			if (game.status !== "played" || game.homeScore == null || game.awayScore == null) continue;
			const home = rows.get(game.homeTeamId);
			const away = rows.get(game.awayTeamId);
			if (!home || !away) continue;
			home.played += 1; away.played += 1;
			home.goalsFor += game.homeScore; home.goalsAgainst += game.awayScore;
			away.goalsFor += game.awayScore; away.goalsAgainst += game.homeScore;
			if (game.homeScore > game.awayScore) { home.won += 1; home.points += 3; away.lost += 1; }
			else if (game.homeScore < game.awayScore) { away.won += 1; away.points += 3; home.lost += 1; }
			else { home.drawn += 1; away.drawn += 1; home.points += 1; away.points += 1; }
		}
		return [...rows.values()].map((row) => ({ ...row, goalDifference: row.goalsFor - row.goalsAgainst })).sort((a, b) => b.points - a.points || b.goalDifference - a.goalDifference || b.goalsFor - a.goalsFor || a.teamId - b.teamId).map((row, index) => ({ ...row, rank: index + 1 }));
	}

	function switchView(view) {
		activeView = view;
		document.querySelectorAll(".view").forEach((section) => section.classList.toggle("active", section.id === `view-${view}`));
		document.querySelectorAll(".nav-item").forEach((button) => button.classList.toggle("active", button.dataset.view === view));
		const titles = { dashboard: "Übersicht", games: "Spiele", standings: "Tabelle", teams: "Teams" };
		elements.pageTitle.textContent = titles[view] || "Strat26";
		elements.sidebar.classList.remove("open");
		window.location.hash = view;
		window.scrollTo({ top: 0, behavior: "smooth" });
	}

	function teamByID(id) { return state.teams.find((team) => team.id === Number(id)); }
	function signed(value) { return value > 0 ? `+${value}` : String(value); }
	function clamp(value, min, max) { return Math.max(min, Math.min(max, value)); }
	function formatKickoff(value) {
		if (!value) return "Termin offen";
		const date = new Date(value);
		if (Number.isNaN(date.getTime())) return value;
		return new Intl.DateTimeFormat("de-DE", { day: "2-digit", month: "short", hour: "2-digit", minute: "2-digit" }).format(date);
	}
	function emptyState(title, detail) { return `<div class="empty-state"><strong>${title}</strong><br><span>${detail}</span></div>`; }
	function escapeHTML(value) { return String(value ?? "").replace(/[&<>'"]/g, (character) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", "'": "&#39;", '"': "&quot;" }[character])); }
	function poissonLocal(random, lambda) { const limit = Math.exp(-lambda); let product = 1; let value = 0; do { value += 1; product *= random(); } while (product > limit); return value - 1; }
	function mulberry32(seed) { return function random() { let value = seed += 0x6D2B79F5; value = Math.imul(value ^ value >>> 15, value | 1); value ^= value + Math.imul(value ^ value >>> 7, value | 61); return ((value ^ value >>> 14) >>> 0) / 4294967296; }; }

	function showToast(message, error = false) {
		window.clearTimeout(toastTimer);
		elements.toast.textContent = message;
		elements.toast.classList.toggle("error", error);
		elements.toast.classList.add("visible");
		toastTimer = window.setTimeout(() => elements.toast.classList.remove("visible"), 3000);
	}

	function showFormError(message) {
		elements.createError.textContent = message;
		elements.createError.classList.remove("hidden");
	}

	async function withBusy(button, action) {
		button.disabled = true;
		try { await action(); }
		catch (error) { showToast(error.message, true); }
		finally { button.disabled = false; }
	}

	function initializeTheme() {
		const stored = localStorage.getItem(THEME_KEY);
		const theme = stored || (window.matchMedia("(prefers-color-scheme: light)").matches ? "light" : "dark");
		document.documentElement.dataset.theme = theme;
	}

	document.addEventListener("click", (event) => {
		const viewButton = event.target.closest("[data-view], [data-go]");
		if (viewButton) switchView(viewButton.dataset.view || viewButton.dataset.go);
		const gameButton = event.target.closest("[data-game-id]");
		if (gameButton) openGame(gameButton.dataset.gameId);
		const closeButton = event.target.closest("[data-close]");
		if (closeButton) document.getElementById(closeButton.dataset.close).close();
	});
	document.getElementById("mobile-menu").addEventListener("click", () => elements.sidebar.classList.toggle("open"));
	document.getElementById("open-create").addEventListener("click", () => elements.createDialog.showModal());
	document.getElementById("generate-schedule").addEventListener("click", generateSchedule);
	document.getElementById("simulate-all").addEventListener("click", simulateAll);
	document.getElementById("simulate-game").addEventListener("click", simulateCurrentGame);
	document.getElementById("delete-game").addEventListener("click", deleteCurrentGame);
	elements.createForm.addEventListener("submit", createGame);
	elements.statusFilter.addEventListener("change", renderGames);
	elements.teamFilter.addEventListener("change", renderGames);
	document.getElementById("theme-toggle").addEventListener("click", () => {
		const theme = document.documentElement.dataset.theme === "dark" ? "light" : "dark";
		document.documentElement.dataset.theme = theme;
		localStorage.setItem(THEME_KEY, theme);
	});

	initializeTheme();
	const initialView = window.location.hash.slice(1);
	if (["dashboard", "games", "standings", "teams"].includes(initialView)) switchView(initialView);
	load();
})();
