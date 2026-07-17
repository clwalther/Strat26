package main

import (
	"database/sql"
	"errors"
	"fmt"
	"math/rand"
	"sort"
	"strings"
	"time"
)

func collectSponsor(db *sql.DB, teamID int64) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var team CoachTeam
	if err := tx.QueryRow(`SELECT id, name, color, hymns, sponsor_level FROM coach_teams WHERE id = ?`, teamID).Scan(
		&team.ID, &team.Name, &team.Color, &team.Hymns, &team.SponsorLevel); err != nil {
		return err
	}
	reward := gameRules.SponsorReward + team.SponsorLevel - 1
	if _, err := tx.Exec(`UPDATE coach_teams SET hymns = hymns + ? WHERE id = ?`, reward, teamID); err != nil {
		return err
	}
	match, err := currentCoachMatch(tx)
	if err != nil {
		return err
	}
	text := fmt.Sprintf("%s gewinnt einen Sponsor: +%d Hymnen.", team.Name, reward)
	if err := appendCoachEvent(tx, match, "sponsor", &teamID, nil, text); err != nil {
		return err
	}
	return tx.Commit()
}

func trainPlayer(db *sql.DB, input playerActionRequest) error {
	focus := strings.ToLower(strings.TrimSpace(input.Focus))
	allowed := map[string]bool{"attack": true, "defense": true, "fitness": true, "morale": true}
	if !allowed[focus] {
		return errors.New("focus must be attack, defense, fitness or morale")
	}
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	player, err := getPlayerCard(tx, input.PlayerID)
	if err != nil {
		return err
	}
	if player.TeamID != input.TeamID {
		return errors.New("player does not belong to team")
	}
	if player.Suspended {
		return errors.New("suspended players cannot attend training camp")
	}
	if err := spendHymns(tx, input.TeamID, gameRules.TrainingCost); err != nil {
		return err
	}
	query := fmt.Sprintf(`UPDATE player_cards SET %s = MIN(99, %s + ?) WHERE id = ?`, focus, focus)
	if _, err := tx.Exec(query, gameRules.TrainingGain, player.ID); err != nil {
		return err
	}
	match, err := currentCoachMatch(tx)
	if err != nil {
		return err
	}
	labels := map[string]string{"attack": "Angriff", "defense": "Abwehr", "fitness": "Fitness", "morale": "Moral"}
	text := fmt.Sprintf("Trainingscamp: %s verbessert %s um %d.", player.Name, labels[focus], gameRules.TrainingGain)
	if err := appendCoachEvent(tx, match, "training", &input.TeamID, &player.ID, text); err != nil {
		return err
	}
	return tx.Commit()
}

func dopePlayer(db *sql.DB, input playerActionRequest) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	player, err := getPlayerCard(tx, input.PlayerID)
	if err != nil {
		return err
	}
	if player.TeamID != input.TeamID {
		return errors.New("player does not belong to team")
	}
	if player.Suspended {
		return errors.New("suspended players cannot be doped")
	}
	if err := spendHymns(tx, input.TeamID, gameRules.DopingCost); err != nil {
		return err
	}
	if _, err := tx.Exec(`
		UPDATE player_cards SET
			attack = MIN(99, attack + ?), fitness = MIN(99, fitness + ?),
			doping_level = MIN(100, doping_level + ?)
		WHERE id = ?`, gameRules.DopingGain, gameRules.DopingGain, gameRules.DopingRisk, player.ID); err != nil {
		return err
	}
	if _, err := tx.Exec(`UPDATE coach_groups SET fifa_attention = MIN(100, fifa_attention + 12) WHERE id = ?`, player.CustodianGroupID); err != nil {
		return err
	}
	match, err := currentCoachMatch(tx)
	if err != nil {
		return err
	}
	text := fmt.Sprintf("Riskante Aufwertung: %s erhält +%d Angriff und Fitness. Das FIFA-Risiko steigt.", player.Name, gameRules.DopingGain)
	if err := appendCoachEvent(tx, match, "doping", &input.TeamID, &player.ID, text); err != nil {
		return err
	}
	return tx.Commit()
}

func inspectGroup(db *sql.DB, input inspectionRequest) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	group, err := getSquadGroup(tx, input.GroupID)
	if err != nil {
		return err
	}
	match, err := currentCoachMatch(tx)
	if err != nil {
		return err
	}
	seed := time.Now().UnixNano()
	if input.Seed != nil {
		seed = *input.Seed
	}
	rng := rand.New(rand.NewSource(seed))
	rows, err := tx.Query(`
		SELECT id, team_id, number, name, position, attack, defense, fitness, morale,
			doping_level, suspended, custodian_group_id
		FROM player_cards WHERE custodian_group_id = ? ORDER BY number`, group.ID)
	if err != nil {
		return err
	}
	players := []PlayerCard{}
	for rows.Next() {
		var player PlayerCard
		if err := rows.Scan(&player.ID, &player.TeamID, &player.Number, &player.Name, &player.Position,
			&player.Attack, &player.Defense, &player.Fitness, &player.Morale,
			&player.DopingLevel, &player.Suspended, &player.CustodianGroupID); err != nil {
			rows.Close()
			return err
		}
		players = append(players, player)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	if err := rows.Close(); err != nil {
		return err
	}

	detected := []PlayerCard{}
	for _, player := range players {
		if player.DopingLevel == 0 || player.Suspended {
			continue
		}
		chance := clampFloat(0.05+float64(player.DopingLevel)*0.009+float64(group.FIFAAttention)*0.0015, 0, 1)
		if rng.Float64() <= chance {
			detected = append(detected, player)
			if _, err := tx.Exec(`UPDATE player_cards SET suspended = 1, doping_level = 0, morale = MAX(0, morale - 10) WHERE id = ?`, player.ID); err != nil {
				return err
			}
			if err := removeAndReplaceSuspended(tx, match, player); err != nil {
				return err
			}
		}
	}
	if _, err := tx.Exec(`UPDATE coach_groups SET fifa_attention = 0 WHERE id = ?`, group.ID); err != nil {
		return err
	}
	text := fmt.Sprintf("FIFA-Kontrolle bei %s: keine Auffälligkeiten.", group.Name)
	if len(detected) > 0 {
		names := make([]string, len(detected))
		for index, player := range detected {
			names[index] = player.Name
		}
		text = fmt.Sprintf("FIFA-Kontrolle bei %s: %s gesperrt.", group.Name, strings.Join(names, ", "))
	}
	if err := appendCoachEvent(tx, match, "inspection", &group.TeamID, nil, text); err != nil {
		return err
	}
	return tx.Commit()
}

func removeAndReplaceSuspended(tx *sql.Tx, match CoachMatch, player PlayerCard) error {
	var onField bool
	var slot string
	err := tx.QueryRow(`SELECT on_field, slot FROM coach_lineups WHERE match_id = ? AND player_id = ?`, match.ID, player.ID).Scan(&onField, &slot)
	if err != nil {
		return err
	}
	if !onField {
		return nil
	}
	if _, err := tx.Exec(`UPDATE coach_lineups SET on_field = 0, slot = 'SUSPENDED', left_at = ? WHERE match_id = ? AND player_id = ?`, match.Minute, match.ID, player.ID); err != nil {
		return err
	}
	var replacementID int64
	err = tx.QueryRow(`
		SELECT p.id FROM player_cards p
		JOIN coach_lineups l ON l.player_id = p.id AND l.match_id = ?
		WHERE p.team_id = ? AND p.suspended = 0 AND l.on_field = 0
			AND l.slot <> 'SUSPENDED' AND l.left_at IS NULL
		ORDER BY (p.position = ?) DESC, p.fitness + p.attack + p.defense DESC LIMIT 1`,
		match.ID, player.TeamID, player.Position).Scan(&replacementID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	_, err = tx.Exec(`UPDATE coach_lineups SET on_field = 1, slot = ?, entered_at = ?, left_at = NULL WHERE match_id = ? AND player_id = ?`,
		slot, match.Minute, match.ID, replacementID)
	return err
}

func substitutePlayer(db *sql.DB, input substitutionRequest) error {
	if input.OutPlayerID == input.InPlayerID {
		return errors.New("outgoing and incoming player must differ")
	}
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	match, err := currentCoachMatch(tx)
	if err != nil {
		return err
	}
	if match.Phase == "finished" {
		return errors.New("match is already finished")
	}
	outPlayer, err := getPlayerCard(tx, input.OutPlayerID)
	if err != nil {
		return err
	}
	inPlayer, err := getPlayerCard(tx, input.InPlayerID)
	if err != nil {
		return err
	}
	if outPlayer.TeamID != input.TeamID || inPlayer.TeamID != input.TeamID {
		return errors.New("both players must belong to the selected team")
	}
	if inPlayer.Suspended {
		return errors.New("a suspended player cannot enter the match")
	}
	var outOnField, inOnField bool
	var inLeftAt sql.NullInt64
	var slot string
	if err := tx.QueryRow(`SELECT on_field, slot FROM coach_lineups WHERE match_id = ? AND player_id = ?`, match.ID, outPlayer.ID).Scan(&outOnField, &slot); err != nil {
		return err
	}
	if err := tx.QueryRow(`SELECT on_field, left_at FROM coach_lineups WHERE match_id = ? AND player_id = ?`, match.ID, inPlayer.ID).Scan(&inOnField, &inLeftAt); err != nil {
		return err
	}
	if !outOnField || inOnField {
		return errors.New("select one field player and one bench player")
	}
	if inLeftAt.Valid {
		return errors.New("a substituted player cannot return to the match")
	}
	used := match.HomeSubstitutions
	column := "home_substitutions"
	if input.TeamID == match.AwayTeamID {
		used = match.AwaySubstitutions
		column = "away_substitutions"
	} else if input.TeamID != match.HomeTeamID {
		return errors.New("team does not participate in current match")
	}
	countsAsSubstitution := match.Minute > 0
	if countsAsSubstitution && used >= gameRules.MaxSubstitutions {
		return errors.New("substitution limit reached")
	}
	if match.Minute == 0 {
		if _, err := tx.Exec(`UPDATE coach_lineups SET on_field = 0, slot = 'BENCH', left_at = NULL WHERE match_id = ? AND player_id = ?`, match.ID, outPlayer.ID); err != nil {
			return err
		}
	} else {
		if _, err := tx.Exec(`UPDATE coach_lineups SET on_field = 0, slot = 'BENCH', left_at = ? WHERE match_id = ? AND player_id = ?`, match.Minute, match.ID, outPlayer.ID); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(`UPDATE coach_lineups SET on_field = 1, slot = ?, entered_at = ?, left_at = NULL WHERE match_id = ? AND player_id = ?`, slot, match.Minute, match.ID, inPlayer.ID); err != nil {
		return err
	}
	if countsAsSubstitution {
		if _, err := tx.Exec(`UPDATE coach_matches SET `+column+` = `+column+` + 1 WHERE id = ?`, match.ID); err != nil {
			return err
		}
	}
	text := fmt.Sprintf("Wechsel: %s kommt für %s.", inPlayer.Name, outPlayer.Name)
	if match.Minute == 0 {
		text = fmt.Sprintf("Startelf geändert: %s ersetzt %s.", inPlayer.Name, outPlayer.Name)
	}
	if err := appendCoachEvent(tx, match, "substitution", &input.TeamID, &inPlayer.ID, text); err != nil {
		return err
	}
	return tx.Commit()
}

func assignCardToGroup(db *sql.DB, playerID, groupID int64) error {
	player, err := getPlayerCard(db, playerID)
	if err != nil {
		return err
	}
	group, err := getSquadGroup(db, groupID)
	if err != nil {
		return err
	}
	if player.TeamID != group.TeamID {
		return errors.New("card can only be assigned within its team")
	}
	_, err = db.Exec(`UPDATE player_cards SET custodian_group_id = ? WHERE id = ?`, group.ID, player.ID)
	return err
}

func advanceCoachMatch(db *sql.DB, input advanceRequest) error {
	minutes := input.Minutes
	if minutes == 0 {
		minutes = gameRules.StepMinutes
	}
	if minutes < 1 || minutes > 15 || minutes%gameRules.StepMinutes != 0 {
		return errors.New("minutes must be 5, 10 or 15")
	}
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	match, err := currentCoachMatch(tx)
	if err != nil {
		return err
	}
	if match.Phase == "finished" {
		return errors.New("match is already finished")
	}
	seed := match.Seed
	if input.Seed != nil {
		seed = *input.Seed
	}
	if match.Phase == "preparation" {
		match.Phase = "first_half"
		if err := insertEventAt(tx, match, 0, "kickoff", nil, nil, "Anpfiff! Team Grün gegen Team Blau."); err != nil {
			return err
		}
	} else if match.Phase == "halftime" {
		match.Phase = "second_half"
		if err := insertEventAt(tx, match, 45, "kickoff", nil, nil, "Wiederanpfiff zur zweiten Halbzeit."); err != nil {
			return err
		}
	}
	limit := 90
	if match.Phase == "first_half" {
		limit = 45
	}
	target := minInt(match.Minute+minutes, limit)
	for match.Minute < target {
		nextMinute := minInt(match.Minute+gameRules.StepMinutes, target)
		home, err := lineupStrength(tx, match.ID, match.HomeTeamID)
		if err != nil {
			return err
		}
		away, err := lineupStrength(tx, match.ID, match.AwayTeamID)
		if err != nil {
			return err
		}
		rng := rand.New(rand.NewSource(seed + int64(nextMinute)*104729))
		homeChance := goalChance(home.attack+2, away.defense, home.fitness, away.fitness)
		awayChance := goalChance(away.attack, home.defense, away.fitness, home.fitness)
		if rng.Float64() < homeChance {
			match.HomeScore++
			if err := recordGoal(tx, match, match.HomeTeamID, nextMinute, rng); err != nil {
				return err
			}
		}
		if rng.Float64() < awayChance {
			match.AwayScore++
			if err := recordGoal(tx, match, match.AwayTeamID, nextMinute, rng); err != nil {
				return err
			}
		}
		match.Minute = nextMinute
	}
	if match.Minute == 45 && match.Phase == "first_half" {
		match.Phase = "halftime"
		if err := insertEventAt(tx, match, 45, "halftime", nil, nil, fmt.Sprintf("Halbzeitstand %d:%d.", match.HomeScore, match.AwayScore)); err != nil {
			return err
		}
	}
	if match.Minute == 90 {
		match.Phase = "finished"
		if err := insertEventAt(tx, match, 90, "fulltime", nil, nil, fmt.Sprintf("Abpfiff. Endstand %d:%d.", match.HomeScore, match.AwayScore)); err != nil {
			return err
		}
	}
	_, err = tx.Exec(`
		UPDATE coach_matches SET phase = ?, minute = ?, home_score = ?, away_score = ?, seed = ? WHERE id = ?`,
		match.Phase, match.Minute, match.HomeScore, match.AwayScore, seed, match.ID)
	if err != nil {
		return err
	}
	return tx.Commit()
}

type strength struct {
	attack  float64
	defense float64
	fitness float64
}

func lineupStrength(tx *sql.Tx, matchID, teamID int64) (strength, error) {
	rows, err := tx.Query(`
		SELECT p.attack, p.defense, p.fitness, p.morale
		FROM coach_lineups l JOIN player_cards p ON p.id = l.player_id
		WHERE l.match_id = ? AND l.team_id = ? AND l.on_field = 1 AND p.suspended = 0`, matchID, teamID)
	if err != nil {
		return strength{}, err
	}
	defer rows.Close()
	var result strength
	count := 0
	for rows.Next() {
		var attack, defense, fitness, morale int
		if err := rows.Scan(&attack, &defense, &fitness, &morale); err != nil {
			return strength{}, err
		}
		result.attack += float64(attack) + float64(morale-50)*0.08
		result.defense += float64(defense) + float64(morale-50)*0.06
		result.fitness += float64(fitness)
		count++
	}
	if err := rows.Err(); err != nil {
		return strength{}, err
	}
	if count == 0 {
		return strength{}, errors.New("team has no eligible field players")
	}
	result.attack /= float64(gameRules.FieldPlayers)
	result.defense /= float64(gameRules.FieldPlayers)
	result.fitness /= float64(gameRules.FieldPlayers)
	return result, nil
}

func goalChance(attack, opponentDefense, fitness, opponentFitness float64) float64 {
	chance := 0.065 + (attack-opponentDefense)/520 + (fitness-opponentFitness)/1200
	return clampFloat(chance, 0.015, 0.18)
}

func recordGoal(tx *sql.Tx, match CoachMatch, teamID int64, minute int, rng *rand.Rand) error {
	rows, err := tx.Query(`
		SELECT p.id, p.name, p.attack FROM coach_lineups l
		JOIN player_cards p ON p.id = l.player_id
		WHERE l.match_id = ? AND l.team_id = ? AND l.on_field = 1 AND p.suspended = 0
		ORDER BY p.position = 'FWD' DESC, p.position = 'MID' DESC, p.number`, match.ID, teamID)
	if err != nil {
		return err
	}
	type scorer struct {
		id     int64
		name   string
		weight int
	}
	scorers := []scorer{}
	total := 0
	for rows.Next() {
		var player scorer
		if err := rows.Scan(&player.id, &player.name, &player.weight); err != nil {
			rows.Close()
			return err
		}
		player.weight = maxInt(player.weight, 1)
		total += player.weight
		scorers = append(scorers, player)
	}
	if err := rows.Close(); err != nil {
		return err
	}
	if len(scorers) == 0 {
		return errors.New("no scorer available")
	}
	pick := rng.Intn(total)
	selected := scorers[0]
	for _, player := range scorers {
		pick -= player.weight
		if pick < 0 {
			selected = player
			break
		}
	}
	text := fmt.Sprintf("Tor für %s durch %s!", map[int64]string{1: "Team Grün", 2: "Team Blau"}[teamID], selected.name)
	return insertEventAt(tx, match, minute, "goal", &teamID, &selected.id, text)
}

func insertEventAt(tx *sql.Tx, match CoachMatch, minute int, kind string, teamID, playerID *int64, text string) error {
	_, err := tx.Exec(`INSERT INTO coach_events (match_id, minute, kind, team_id, player_id, text) VALUES (?, ?, ?, ?, ?, ?)`,
		match.ID, minute, kind, teamID, playerID, text)
	return err
}

func spendHymns(tx *sql.Tx, teamID int64, amount int) error {
	result, err := tx.Exec(`UPDATE coach_teams SET hymns = hymns - ? WHERE id = ? AND hymns >= ?`, amount, teamID, amount)
	if err != nil {
		return err
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return errors.New("not enough hymns")
	}
	return nil
}

func clampFloat(value, low, high float64) float64 {
	if value < low {
		return low
	}
	if value > high {
		return high
	}
	return value
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func sortedPlayerIDs(players []PlayerCard) []int64 {
	sort.Slice(players, func(i, j int) bool { return players[i].Number < players[j].Number })
	ids := make([]int64, len(players))
	for index, player := range players {
		ids[index] = player.ID
	}
	return ids
}
