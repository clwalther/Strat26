def player_stat(player, stat):
	fatigue = {
		"pac": 0.75,
		"sho": 0.40,
		"pas": 0.30,
		"dri": 0.45,
		"def": 0.40,
		"phy": 0.75,
	}

	return (
		player[stat]
		* (1 + player["dop"] / 100)
		* (1 - player["fatigue"] / 100 * fatigue[stat])
	)

def average_stat(players, stat):
	return sum(
		player_stat(player, stat) for player in players
	)

def get_eval(team, players):
	goalkeeper = [
		p for p in players
		if p['team'] == team and p['role'] == "GOALKEEPER"
	]
	defenders = [
		p for p in players
		if p['team'] == team and p['role'] == "DEFENDERS"
	]
	midfielders = [
		p for p in players
		if p['team'] == team and p['role'] == "MIDFIELDERS"
	]
	strikers = [
		p for p in players
		if p['team'] == team and p['role'] == "STRIKERS"
	]

	return (
		average_stat(goalkeeper, "phy") * 0.15 +
		average_stat(goalkeeper, "sho") * 0.10 +

		average_stat(defenders, "def") * 0.35 +
		average_stat(defenders, "phy") * 0.20 +
		average_stat(defenders, "pas") * 0.15 +
		average_stat(defenders, "pac") * 0.05 +

		average_stat(midfielders, "pas") * 0.15 +
		average_stat(midfielders, "pac") * 0.10 +

		average_stat(strikers, "sho") * 0.35 +
		average_stat(strikers, "pac") * 0.20 +
		average_stat(strikers, "dri") * 0.20
	)
