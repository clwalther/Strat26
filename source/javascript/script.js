(() => {
	"use strict";

	const THEME_KEY = "strat26.theme";
	const TEAM_KEY = "strat26.coach-team";
	let state = null;
	let selectedTeamID = Number(localStorage.getItem(TEAM_KEY) || 1);
	let selectedPlayerID = null;
	let toastTimer = null;

	const elements = {
		sidebar: document.querySelector(".sidebar"),
		pageTitle: document.getElementById("page-title"),
		hymns: document.getElementById("hymn-count"),
		serverStatus: document.getElementById("server-status"),
		connectionError: document.getElementById("connection-error"),
		stats: document.getElementById("stats"),
		groupOverview: document.getElementById("group-overview"),
		recentFeed: document.getElementById("recent-feed"),
		playerGrid: document.getElementById("player-grid"),
		groupFilter: document.getElementById("group-filter"),
		positionFilter: document.getElementById("position-filter"),
		cardStatusFilter: document.getElementById("card-status-filter"),
		formation: document.getElementById("formation"),
		benchList: document.getElementById("bench-list"),
		outPlayer: document.getElementById("out-player"),
		inPlayer: document.getElementById("in-player"),
		subsCount: document.getElementById("subs-count"),
		trainingPlayer: document.getElementById("training-player"),
		dopingPlayer: document.getElementById("doping-player"),
		inspectionGrid: document.getElementById("inspection-grid"),
		eventFeed: document.getElementById("event-feed"),
		playerDialog: document.getElementById("player-dialog"),
		playerDialogMeta: document.getElementById("player-dialog-meta"),
		playerDialogName: document.getElementById("player-dialog-name"),
		playerDetail: document.getElementById("player-detail"),
		playerGroupSelect: document.getElementById("player-group-select"),
		toast: document.getElementById("toast"),
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
				// Non-JSON server errors keep the status-based message.
			}
			throw new Error(message);
		}
		return response.status === 204 ? null : response.json();
	}

	async function load() {
		try {
			state = await api("/state");
			elements.serverStatus.textContent = "Backend verbunden";
			document.querySelector(".status-light").classList.remove("offline");
			elements.connectionError.classList.add("hidden");
			renderAll();
		} catch (error) {
			elements.serverStatus.textContent = "Backend offline";
			document.querySelector(".status-light").classList.add("offline");
			elements.connectionError.classList.remove("hidden");
			showToast(error.message, true);
		}
	}

	function renderAll() {
		if (!state) return;
		if (!state.teams.some((team) => team.id === selectedTeamID)) selectedTeamID = state.teams[0].id;
		document.querySelectorAll(".team-choice").forEach((button) => button.classList.toggle("active", Number(button.dataset.team) === selectedTeamID));
		const team = selectedTeam();
		elements.hymns.textContent = team.hymns;
		renderMatchSummary();
		renderStats();
		renderGroups();
		renderFeeds();
		renderFilterOptions();
		renderPlayers();
		renderLineup();
		renderActions();
	}

	function selectedTeam() { return state.teams.find((team) => team.id === selectedTeamID); }
	function selectedGroups() { return state.groups.filter((group) => group.teamId === selectedTeamID); }
	function selectedPlayers() { return state.players.filter((player) => player.teamId === selectedTeamID); }
	function teamByID(id) { return state.teams.find((team) => team.id === Number(id)); }
	function groupByID(id) { return state.groups.find((group) => group.id === Number(id)); }
	function playerByID(id) { return state.players.find((player) => player.id === Number(id)); }
	function lineupByPlayer(id) { return state.lineup.find((entry) => entry.playerId === Number(id)); }

	function renderMatchSummary() {
		const match = state.match;
		const phase = phaseLabel(match.phase);
		for (const id of ["hero-phase", "match-phase"]) document.getElementById(id).textContent = phase;
		for (const id of ["hero-home-score", "match-home-score"]) document.getElementById(id).textContent = match.homeScore;
		for (const id of ["hero-away-score", "match-away-score"]) document.getElementById(id).textContent = match.awayScore;
		for (const id of ["hero-minute", "match-minute"]) document.getElementById(id).textContent = match.minute;
		const finished = match.phase === "finished";
		document.getElementById("simulation-model").textContent = state.rules.simulation.model;
		for (const id of ["hero-advance", "advance-match"]) {
			const button = document.getElementById(id);
			button.disabled = finished;
			button.textContent = finished ? "Spiel beendet" : match.phase === "halftime" ? "2. Halbzeit starten" : "5 Minuten spielen";
		}
	}

	function renderStats() {
		const players = selectedPlayers();
		const field = players.filter((player) => lineupByPlayer(player.id)?.onField).length;
		const risky = players.filter((player) => player.dopingLevel > 0).length;
		const suspended = players.filter((player) => player.suspended).length;
		const attention = Math.max(0, ...selectedGroups().map((group) => group.fifaAttention));
		const data = [
			["Spielerkarten", players.length, "gemeinsamer Teamkader"],
			["Auf dem Feld", `${field} / ${state.rules.fieldPlayers}`, "Coach-Aufstellung"],
			["FIFA-Risiko", `${attention}%`, `${risky} auffällige Karten`],
			["Gesperrt", suspended, suspended ? "nicht einsetzbar" : "Kader vollständig"],
		];
		elements.stats.innerHTML = data.map(([label, value, detail]) => `<article class="stat"><span>${label}</span><strong>${value}</strong><small>${detail}</small></article>`).join("");
	}

	function renderGroups() {
		elements.groupOverview.innerHTML = selectedGroups().map((group) => {
			const cards = state.players.filter((player) => player.custodianGroupId === group.id).length;
			return `<article class="group-mini"><span>${escapeHTML(group.name)}</span><strong>${cards}</strong><small>Karten · FIFA ${group.fifaAttention}%</small><progress class="risk-meter" max="100" value="${group.fifaAttention}"></progress></article>`;
		}).join("");
	}

	function renderFeeds() {
		const events = state.events || [];
		elements.recentFeed.innerHTML = events.length ? events.slice(0, 5).map(eventHTML).join("") : emptyHTML("Noch keine Meldungen");
		elements.eventFeed.innerHTML = events.length ? events.map(eventHTML).join("") : emptyHTML("Die Chronik ist leer.");
	}

	function eventHTML(event) {
		const icons = { goal: "⚽", save: "◇", miss: "↗", simulation: "∿", training: "↗", doping: "!", inspection: "⌕", sponsor: "♫", substitution: "⇄", kickoff: "▶", halftime: "Ⅱ", fulltime: "■", setup: "●" };
		return `<article class="event ${event.kind}"><span>${event.minute}'</span><i class="event-icon">${icons[event.kind] || "·"}</i><span>${escapeHTML(event.text)}</span></article>`;
	}

	function renderFilterOptions() {
		const current = elements.groupFilter.value;
		elements.groupFilter.innerHTML = `<option value="">Alle Gruppen</option>${selectedGroups().map((group) => `<option value="${group.id}">${escapeHTML(group.name)}</option>`).join("")}`;
		elements.groupFilter.value = current;
	}

	function renderPlayers() {
		const groupID = Number(elements.groupFilter.value || 0);
		const position = elements.positionFilter.value;
		const status = elements.cardStatusFilter.value;
		const players = selectedPlayers().filter((player) => {
			const lineup = lineupByPlayer(player.id);
			if (groupID && player.custodianGroupId !== groupID) return false;
			if (position && player.position !== position) return false;
			if (status === "field" && !lineup?.onField) return false;
			if (status === "bench" && lineup?.onField) return false;
			if (status === "risk" && player.dopingLevel === 0) return false;
			if (status === "suspended" && !player.suspended) return false;
			return true;
		});
		elements.playerGrid.innerHTML = players.length ? players.map(playerCardHTML).join("") : emptyHTML("Keine Karten für diesen Filter.");
	}

	function playerCardHTML(player) {
		const team = teamByID(player.teamId);
		const group = groupByID(player.custodianGroupId);
		const lineup = lineupByPlayer(player.id);
		const classes = ["player-card", team.color, player.dopingLevel ? "risky" : "", player.suspended ? "suspended" : ""].filter(Boolean).join(" ");
		return `<button class="${classes}" type="button" data-player-id="${player.id}"><span class="card-top"><i class="shirt-number">${player.number}</i><span class="card-flags"><b class="flag">${player.position}</b>${lineup?.onField ? '<b class="flag">Feld</b>' : ''}${player.dopingLevel ? `<b class="flag risk">Risiko ${player.dopingLevel}</b>` : ""}${player.suspended ? '<b class="flag risk">Gesperrt</b>' : ""}</span></span><h3>${escapeHTML(player.name)}</h3><p>${escapeHTML(group?.name || "Keine Gruppe")} verwahrt die Karte</p><span class="mini-values"><span>ANG<b>${player.attack}</b></span><span>ABW<b>${player.defense}</b></span><span>FIT<b>${player.fitness}</b></span><span>MOR<b>${player.morale}</b></span></span></button>`;
	}

	function renderLineup() {
		const team = selectedTeam();
		const entries = state.lineup.filter((entry) => entry.teamId === selectedTeamID);
		const field = entries.filter((entry) => entry.onField);
		const bench = entries.filter((entry) => !entry.onField && entry.leftAt == null && !playerByID(entry.playerId)?.suspended);
		const rows = ["FWD", "MID", "DEF", "GK"];
		elements.formation.innerHTML = rows.map((position) => `<div class="formation-row">${field.filter((entry) => entry.position === position).map((entry) => `<article class="field-player ${team.color}"><i>${entry.playerNumber}</i><strong>${escapeHTML(entry.playerName)}</strong><small>${entry.position} · ${entry.slot}</small></article>`).join("")}</div>`).join("");
		elements.benchList.innerHTML = bench.map((entry) => `<article class="bench-row ${team.color}"><i>${entry.playerNumber}</i><span>${escapeHTML(entry.playerName)}</span><small>${entry.position}</small></article>`).join("");
		elements.outPlayer.innerHTML = field.map((entry) => `<option value="${entry.playerId}">#${entry.playerNumber} ${escapeHTML(entry.playerName)} · ${entry.position}</option>`).join("");
		elements.inPlayer.innerHTML = bench.map((entry) => `<option value="${entry.playerId}">#${entry.playerNumber} ${escapeHTML(entry.playerName)} · ${entry.position}</option>`).join("");
		const used = selectedTeamID === state.match.homeTeamId ? state.match.homeSubstitutions : state.match.awaySubstitutions;
		elements.subsCount.textContent = `${used} / ${state.rules.maxSubstitutions}`;
	}

	function renderActions() {
		const eligible = selectedPlayers().filter((player) => !player.suspended);
		const options = eligible.map((player) => `<option value="${player.id}">#${player.number} ${escapeHTML(player.name)} · ${player.position}</option>`).join("");
		const trainingValue = elements.trainingPlayer.value;
		const dopingValue = elements.dopingPlayer.value;
		elements.trainingPlayer.innerHTML = options;
		elements.dopingPlayer.innerHTML = options;
		elements.trainingPlayer.value = trainingValue;
		elements.dopingPlayer.value = dopingValue;
		document.getElementById("sponsor-reward").textContent = `+${state.rules.sponsorReward + selectedTeam().sponsorLevel - 1} Hymnen`;
		document.getElementById("training-rule").textContent = `${state.rules.trainingCost} ♫ · +${state.rules.trainingGain}`;
		document.getElementById("doping-rule").textContent = `${state.rules.dopingCost} ♫ · +${state.rules.dopingGain} / Risiko +${state.rules.dopingRisk}`;
		elements.inspectionGrid.innerHTML = selectedGroups().map((group) => {
			const cards = state.players.filter((player) => player.custodianGroupId === group.id);
			const risky = cards.filter((player) => player.dopingLevel > 0).length;
			return `<article class="inspection-card"><header><h4>${escapeHTML(group.name)}</h4><span>${group.fifaAttention}%</span></header><progress class="risk-meter" max="100" value="${group.fifaAttention}"></progress><p>${cards.length} Karten · ${risky} mit Dopingrisiko</p><button class="button secondary wide" type="button" data-inspect="${group.id}">Kontrolle auslösen</button></article>`;
		}).join("");
	}

	function openPlayer(id) {
		const player = playerByID(id);
		if (!player) return;
		selectedPlayerID = player.id;
		const group = groupByID(player.custodianGroupId);
		elements.playerDialogMeta.textContent = `#${player.number} · ${player.position} · ${group?.name || "ohne Gruppe"}`;
		elements.playerDialogName.textContent = player.name;
		elements.playerDetail.innerHTML = [
			["Angriff", player.attack], ["Abwehr", player.defense], ["Fitness", player.fitness], ["Moral", player.morale],
			["Dopingrisiko", `${player.dopingLevel}%`], ["Status", player.suspended ? "Gesperrt" : lineupByPlayer(player.id)?.onField ? "Feld" : "Bank"],
		].map(([label, value]) => `<span class="detail-value"><small>${label}</small><strong>${value}</strong></span>`).join("");
		elements.playerGroupSelect.innerHTML = selectedGroups().map((entry) => `<option value="${entry.id}">${escapeHTML(entry.name)}</option>`).join("");
		elements.playerGroupSelect.value = player.custodianGroupId;
		elements.playerDialog.showModal();
	}

	async function runAction(path, payload, success, button) {
		if (button) button.disabled = true;
		try {
			state = await api(path, { method: "POST", body: JSON.stringify(payload) });
			renderAll();
			showToast(success);
		} catch (error) {
			showToast(error.message, true);
		} finally {
			if (button) button.disabled = false;
		}
	}

	async function advance(button) {
		await runAction("/match/advance", { minutes: state.rules.stepMinutes }, "Spiel fortgesetzt.", button);
	}

	function switchView(view) {
		document.querySelectorAll(".view").forEach((section) => section.classList.toggle("active", section.id === `view-${view}`));
		document.querySelectorAll(".nav-item").forEach((button) => button.classList.toggle("active", button.dataset.view === view));
		const titles = { zentrale: "Zentrale", kader: "Spielerkarten", aufstellung: "Aufstellung", aktionen: "Aktionen", spiel: "Spielverlauf" };
		elements.pageTitle.textContent = titles[view] || "Zentrale";
		elements.sidebar.classList.remove("open");
		window.location.hash = view;
		window.scrollTo({ top: 0, behavior: "smooth" });
	}

	function phaseLabel(phase) {
		return { preparation: "Vorbereitung", first_half: "1. Halbzeit", halftime: "Halbzeit", second_half: "2. Halbzeit", finished: "Abpfiff" }[phase] || phase;
	}

	function showToast(message, error = false) {
		window.clearTimeout(toastTimer);
		elements.toast.textContent = message;
		elements.toast.classList.toggle("error", error);
		elements.toast.classList.add("visible");
		toastTimer = window.setTimeout(() => elements.toast.classList.remove("visible"), 3000);
	}

	function escapeHTML(value) { return String(value ?? "").replace(/[&<>'"]/g, (character) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", "'": "&#39;", '"': "&quot;" }[character])); }
	function emptyHTML(text) { return `<div class="empty">${escapeHTML(text)}</div>`; }

	document.addEventListener("click", (event) => {
		const navigation = event.target.closest("[data-view], [data-go]");
		if (navigation) switchView(navigation.dataset.view || navigation.dataset.go);
		const teamButton = event.target.closest("[data-team]");
		if (teamButton) {
			selectedTeamID = Number(teamButton.dataset.team);
			localStorage.setItem(TEAM_KEY, selectedTeamID);
			renderAll();
		}
		const playerButton = event.target.closest("[data-player-id]");
		if (playerButton) openPlayer(playerButton.dataset.playerId);
		const inspectionButton = event.target.closest("[data-inspect]");
		if (inspectionButton) runAction("/actions/inspect", { groupId: Number(inspectionButton.dataset.inspect) }, "FIFA-Kontrolle abgeschlossen.", inspectionButton);
		const closeButton = event.target.closest("[data-close]");
		if (closeButton) document.getElementById(closeButton.dataset.close).close();
	});

	document.getElementById("menu-button").addEventListener("click", () => elements.sidebar.classList.toggle("open"));
	document.getElementById("hero-advance").addEventListener("click", (event) => advance(event.currentTarget));
	document.getElementById("advance-match").addEventListener("click", (event) => advance(event.currentTarget));
	document.getElementById("sponsor-action").addEventListener("click", (event) => runAction("/actions/sponsor", { teamId: selectedTeamID }, "Sponsor gewonnen – Hymnen gutgeschrieben.", event.currentTarget));
	document.getElementById("training-form").addEventListener("submit", (event) => {
		event.preventDefault();
		runAction("/actions/train", { teamId: selectedTeamID, playerId: Number(elements.trainingPlayer.value), focus: document.getElementById("training-focus").value }, "Trainingscamp abgeschlossen.", event.submitter);
	});
	document.getElementById("doping-form").addEventListener("submit", (event) => {
		event.preventDefault();
		if (!window.confirm("Doping verbessert die Karte, kann bei einer FIFA-Kontrolle aber zur Sperre führen. Fortfahren?")) return;
		runAction("/actions/dope", { teamId: selectedTeamID, playerId: Number(elements.dopingPlayer.value) }, "Spieler aufgewertet – FIFA-Risiko gestiegen.", event.submitter);
	});
	document.getElementById("substitution-form").addEventListener("submit", (event) => {
		event.preventDefault();
		runAction("/match/substitute", { teamId: selectedTeamID, outPlayerId: Number(elements.outPlayer.value), inPlayerId: Number(elements.inPlayer.value) }, "Aufstellung aktualisiert.", event.submitter);
	});
	document.getElementById("save-player-group").addEventListener("click", async (event) => {
		event.currentTarget.disabled = true;
		try {
			state = await api(`/players/${selectedPlayerID}/group`, { method: "PATCH", body: JSON.stringify({ groupId: Number(elements.playerGroupSelect.value) }) });
			elements.playerDialog.close();
			renderAll();
			showToast("Kartenverantwortung übertragen.");
		} catch (error) {
			showToast(error.message, true);
		} finally {
			event.currentTarget.disabled = false;
		}
	});
	for (const filter of [elements.groupFilter, elements.positionFilter, elements.cardStatusFilter]) filter.addEventListener("change", renderPlayers);
	document.getElementById("reset-game").addEventListener("click", async (event) => {
		if (!window.confirm("Coach-Zentrale, Kaderwerte und Spiel vollständig zurücksetzen?")) return;
		await runAction("/reset", {}, "Spiel wurde zurückgesetzt.", event.currentTarget);
	});
	document.getElementById("theme-button").addEventListener("click", () => {
		const theme = document.documentElement.dataset.theme === "dark" ? "light" : "dark";
		document.documentElement.dataset.theme = theme;
		localStorage.setItem(THEME_KEY, theme);
	});

	document.documentElement.dataset.theme = localStorage.getItem(THEME_KEY) || (window.matchMedia("(prefers-color-scheme: light)").matches ? "light" : "dark");
	const initialView = window.location.hash.slice(1);
	if (["zentrale", "kader", "aufstellung", "aktionen", "spiel"].includes(initialView)) switchView(initialView);
	load();
})();
