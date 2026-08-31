function createPlayerCard(player, playerDialog) {
	let card = document.createElement("div");
	let name = document.createElement("span");
	let button = document.createElement("button");
	let timer = document.createElement("span");

	card.id = player.id
	name.innerText = player.name;
	button.innerText = "Edit";
	timer.style = "display: none;";
	timer.innerText =
		`${Math.floor(player.punishment_time / 60)}m ` +
		`${Math.floor(player.punishment_time % 60)}s`;

	card.classList.add("player");
	card.setAttribute("draggable", !(
		player.punishment_time > 0 &&
		(player.punishment == "RED" || player.punishment == "DOUBLE")
	));

	if (player.punishment_time > 0) {
		if (player.punishment == "RED" || player.punishment == "DOUBLE")
			card.classList.add("red");
		else if (player.punishment == "YELLOW")
			card.classList.add("yellow");

		timer.style = "display: block;";
	}

	card.appendChild(name);
	card.appendChild(button);
	card.appendChild(timer);


	card.addEventListener("dragstart", (e) => {
		e.dataTransfer.setData("text/plain", player.id);

		card.style = "display: none;"
	});
	card.addEventListener("dragend", (_) => {
		card.style = "display: grid;"
	});
	button.addEventListener("click", (_) => {
		playerDialog.showModal();
	})

	return card;
}





function createPlayerDialog(player) {
	let dialog = document.createElement("dialog");
	let playerName = document.createElement("h2");

	playerName.innerText = player.name;

	let checkmarks = appendPunishment(dialog, player);
	dialog.appendChild(playerName);
	appendNumberInputs(dialog, player);

	dialog.addEventListener("toggle", (e) => {
		if (e.newState == "open") {
			checkmarks.forEach(tuple => {
				let input = tuple[0];
				let value = tuple[1];

				if (player.punishment_time < 0) {
					if (value == "NONE")
						input.checked = true;
				} else {
					if (value == player.punishment)
						input.checked = true;
				}
			});
		}
	});
	dialog.addEventListener("pointerdown", (e) => {
		let rect = dialog.getClientRects()[0];

		if (e.clientX < rect.left ||
			e.clientX > rect.right ||
			e.clientY < rect.top ||
			e.clientY > rect.bottom) {
			dialog.close();
			update();
		}
	});

	return dialog;
}

function appendPunishment(dialog, player) {
	let nRadio = document.createElement("input");
	let yRadio = document.createElement("input");
	let dRadio = document.createElement("input");
	let rRadio = document.createElement("input");

	[
		[nRadio, "NONE", "None"],
		[yRadio, "YELLOW", "🟨"],
		[dRadio, "DOUBLE", "🟨🟥"],
		[rRadio, "RED", "🟥"]
	].forEach(tuple => {
		let input = tuple[0];
		let value = tuple[1];
		let display = tuple[2];

		let label = document.createElement("label");

		input.setAttribute("type", "radio");
		input.setAttribute("id", `${value}-${player.id} `);
		input.setAttribute("name", `punishment-${player.id} `);
		label.setAttribute("for", `${value}-${player.id} `);
		label.innerText = display;

		input.addEventListener("change", async (e) => {
			player.punishment = value;

			// post update to server
			resp = await fetch("/api/punishment", {
				method: "POST",
				headers: {
					"Content-Type": "application/json"
				},
				body: JSON.stringify({
					"player": player
				})
			});

			if (resp.status == 200) {
				update();
			} else {
				alert(`Request to ${resp.url} failed with status: ${resp.status} ${resp.statusText} `);
			}
		})

		dialog.appendChild(input);
		dialog.appendChild(label);
	});

	return [
		[nRadio, "NONE"],
		[yRadio, "YELLOW"],
		[dRadio, "DOUBLE"],
		[rRadio, "RED"]
	];
}

function appendNumberInputs(dialog, player) {
	let playerDOP = document.createElement("input");
	let playerPAC = document.createElement("input");
	let playerSHO = document.createElement("input");
	let playerPAS = document.createElement("input");
	let playerDRI = document.createElement("input");
	let playerDEF = document.createElement("input");
	let playerPHY = document.createElement("input");

	let cancel = document.createElement("button");
	let submit = document.createElement("button");

	cancel.innerText = "Abbrechen";
	submit.innerText = "Speichern";

	[
		[playerDOP, "DOP", player.dop],
		[playerPAC, "PAC", player.pac],
		[playerDRI, "DRI", player.dri],
		[playerSHO, "SHO", player.sho],
		[playerDEF, "DEF", player.def],
		[playerPAS, "PAS", player.pas],
		[playerPHY, "PHY", player.phy]
	].forEach(tuple => {
		let input = tuple[0];
		let name = tuple[1];
		let value = tuple[2];

		let label = document.createElement("label");

		input.setAttribute("type", "number");
		input.setAttribute("max", "100");
		input.setAttribute("min", "0");
		input.setAttribute("value", value);
		input.setAttribute("id", `input - ${name} -${player.id} `);
		label.setAttribute("for", `input - ${name} -${player.id} `);
		label.setAttribute("type", "number");
		label.innerText = name;

		dialog.appendChild(input);
		dialog.appendChild(label);
	});

	cancel.addEventListener("click", (e) => {
		dialog.close();
		update();
	});
	["click", "keypress"].forEach(eventType => {
		submit.addEventListener(eventType, async (e) => {
			e.preventDefault();
			player.pac = playerPAC.valueAsNumber;
			player.sho = playerSHO.valueAsNumber;
			player.pas = playerPAS.valueAsNumber;
			player.dri = playerDRI.valueAsNumber;
			player.def = playerDEF.valueAsNumber;
			player.phy = playerPHY.valueAsNumber;
			player.dop = playerDOP.valueAsNumber;

			// post update to server
			resp = await fetch("/api/player", {
				method: "POST",
				headers: {
					"Content-Type": "application/json"
				},
				body: JSON.stringify({
					"player": player
				})
			});

			if (resp.status == 200) {
				update();
			} else {
				alert(`Request to ${resp.url} failed with status: ${resp.status} ${resp.statusText} `);
			}
		});
	});

	dialog.appendChild(cancel);
	dialog.appendChild(submit);
}
