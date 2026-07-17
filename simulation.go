package main

import (
	"database/sql"
	"fmt"
	"math"
	"math/rand"
	"sort"
)

type simulatedEvent struct {
	Minute int
	Kind   string
	TeamID int64
	Player string
	Detail string
}

func simulateGame(db *sql.DB, gameID, seed int64) (GameDetails, error) {
	tx, err := db.Begin()
	if err != nil {
		return GameDetails{}, err
	}
	defer tx.Rollback()

	game, err := getGame(tx, gameID)
	if err != nil {
		return GameDetails{}, err
	}
	home, err := getTeamTx(tx, game.HomeTeamID)
	if err != nil {
		return GameDetails{}, err
	}
	away, err := getTeamTx(tx, game.AwayTeamID)
	if err != nil {
		return GameDetails{}, err
	}

	rng := rand.New(rand.NewSource(seed))
	homeExpectation := clamp(1.38+float64(home.Rating-away.Rating)/360.0+0.16, 0.25, 3.8)
	awayExpectation := clamp(1.24+float64(away.Rating-home.Rating)/360.0, 0.25, 3.8)
	homeScore := min(poisson(rng, homeExpectation), 9)
	awayScore := min(poisson(rng, awayExpectation), 9)
	events := buildEvents(rng, home, away, homeScore, awayScore)

	if _, err := tx.Exec(`DELETE FROM game_events WHERE game_id = ?`, gameID); err != nil {
		return GameDetails{}, err
	}
	if _, err := tx.Exec(`
		UPDATE games SET status = 'played', home_score = ?, away_score = ?,
			simulation_seed = ?, updated_at = datetime('now') WHERE id = ?`,
		homeScore, awayScore, seed, gameID); err != nil {
		return GameDetails{}, err
	}
	for _, event := range events {
		if _, err := tx.Exec(`
			INSERT INTO game_events (game_id, minute, kind, team_id, player, detail)
			VALUES (?, ?, ?, ?, ?, ?)`,
			gameID, event.Minute, event.Kind, event.TeamID, event.Player, event.Detail); err != nil {
			return GameDetails{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return GameDetails{}, err
	}

	updated, err := getGame(db, gameID)
	if err != nil {
		return GameDetails{}, err
	}
	storedEvents, err := listGameEvents(db, gameID)
	if err != nil {
		return GameDetails{}, err
	}
	return GameDetails{Game: updated, Events: storedEvents}, nil
}

func getTeamTx(tx *sql.Tx, id int64) (Team, error) {
	var team Team
	err := tx.QueryRow(`SELECT id, name, short_name, color, rating FROM teams WHERE id = ?`, id).Scan(
		&team.ID, &team.Name, &team.ShortName, &team.Color, &team.Rating)
	return team, err
}

func poisson(rng *rand.Rand, lambda float64) int {
	limit := math.Exp(-lambda)
	product := 1.0
	value := 0
	for product > limit {
		value++
		product *= rng.Float64()
	}
	return value - 1
}

func buildEvents(rng *rand.Rand, home, away Team, homeGoals, awayGoals int) []simulatedEvent {
	events := make([]simulatedEvent, 0, homeGoals+awayGoals+6)
	addGoalEvents := func(team Team, count int) {
		for index := 0; index < count; index++ {
			shirt := 1 + rng.Intn(18)
			events = append(events, simulatedEvent{
				Minute: 1 + rng.Intn(90), Kind: "goal", TeamID: team.ID,
				Player: fmt.Sprintf("%s · Spieler %d", team.ShortName, shirt), Detail: "Tor",
			})
		}
	}
	addCardEvents := func(team Team) {
		cards := min(poisson(rng, 1.05), 4)
		for index := 0; index < cards; index++ {
			shirt := 1 + rng.Intn(18)
			events = append(events, simulatedEvent{
				Minute: 5 + rng.Intn(86), Kind: "yellow_card", TeamID: team.ID,
				Player: fmt.Sprintf("%s · Spieler %d", team.ShortName, shirt), Detail: "Gelbe Karte",
			})
		}
	}
	addGoalEvents(home, homeGoals)
	addGoalEvents(away, awayGoals)
	addCardEvents(home)
	addCardEvents(away)
	sort.SliceStable(events, func(i, j int) bool { return events[i].Minute < events[j].Minute })
	return events
}

func clamp(value, low, high float64) float64 {
	return math.Max(low, math.Min(high, value))
}
