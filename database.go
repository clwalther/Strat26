package main

import (
	"database/sql"
	"errors"
	"fmt"

	_ "modernc.org/sqlite"
)

const coachSchema = `
CREATE TABLE IF NOT EXISTS coach_teams (
	id INTEGER PRIMARY KEY,
	name TEXT NOT NULL UNIQUE,
	color TEXT NOT NULL UNIQUE CHECK (color IN ('green', 'blue')),
	hymns INTEGER NOT NULL DEFAULT 8 CHECK (hymns >= 0),
	sponsor_level INTEGER NOT NULL DEFAULT 1 CHECK (sponsor_level >= 1)
);

CREATE TABLE IF NOT EXISTS coach_groups (
	id INTEGER PRIMARY KEY,
	team_id INTEGER NOT NULL REFERENCES coach_teams(id) ON DELETE CASCADE,
	number INTEGER NOT NULL CHECK (number BETWEEN 1 AND 4),
	name TEXT NOT NULL,
	fifa_attention INTEGER NOT NULL DEFAULT 0 CHECK (fifa_attention BETWEEN 0 AND 100),
	UNIQUE(team_id, number)
);

CREATE TABLE IF NOT EXISTS player_cards (
	id INTEGER PRIMARY KEY,
	team_id INTEGER NOT NULL REFERENCES coach_teams(id) ON DELETE CASCADE,
	number INTEGER NOT NULL CHECK (number BETWEEN 1 AND 26),
	name TEXT NOT NULL,
	position TEXT NOT NULL CHECK (position IN ('GK', 'DEF', 'MID', 'FWD')),
	attack INTEGER NOT NULL CHECK (attack BETWEEN 0 AND 99),
	defense INTEGER NOT NULL CHECK (defense BETWEEN 0 AND 99),
	fitness INTEGER NOT NULL CHECK (fitness BETWEEN 0 AND 99),
	morale INTEGER NOT NULL CHECK (morale BETWEEN 0 AND 99),
	doping_level INTEGER NOT NULL DEFAULT 0 CHECK (doping_level BETWEEN 0 AND 100),
	suspended INTEGER NOT NULL DEFAULT 0 CHECK (suspended IN (0, 1)),
	custodian_group_id INTEGER NOT NULL REFERENCES coach_groups(id),
	UNIQUE(team_id, number)
);

CREATE TABLE IF NOT EXISTS coach_matches (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	home_team_id INTEGER NOT NULL REFERENCES coach_teams(id),
	away_team_id INTEGER NOT NULL REFERENCES coach_teams(id),
	phase TEXT NOT NULL DEFAULT 'preparation' CHECK (phase IN ('preparation', 'first_half', 'halftime', 'second_half', 'finished')),
	minute INTEGER NOT NULL DEFAULT 0 CHECK (minute BETWEEN 0 AND 90),
	home_score INTEGER NOT NULL DEFAULT 0 CHECK (home_score >= 0),
	away_score INTEGER NOT NULL DEFAULT 0 CHECK (away_score >= 0),
	home_substitutions INTEGER NOT NULL DEFAULT 0,
	away_substitutions INTEGER NOT NULL DEFAULT 0,
	seed INTEGER NOT NULL DEFAULT 2026,
	created_at TEXT NOT NULL DEFAULT (datetime('now')),
	CHECK (home_team_id <> away_team_id)
);

CREATE TABLE IF NOT EXISTS coach_lineups (
	match_id INTEGER NOT NULL REFERENCES coach_matches(id) ON DELETE CASCADE,
	player_id INTEGER NOT NULL REFERENCES player_cards(id) ON DELETE CASCADE,
	team_id INTEGER NOT NULL REFERENCES coach_teams(id),
	on_field INTEGER NOT NULL DEFAULT 0 CHECK (on_field IN (0, 1)),
	slot TEXT NOT NULL DEFAULT 'BENCH',
	entered_at INTEGER NOT NULL DEFAULT 0,
	left_at INTEGER,
	PRIMARY KEY(match_id, player_id)
);

CREATE TABLE IF NOT EXISTS coach_events (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	match_id INTEGER NOT NULL REFERENCES coach_matches(id) ON DELETE CASCADE,
	minute INTEGER NOT NULL CHECK (minute BETWEEN 0 AND 90),
	kind TEXT NOT NULL,
	team_id INTEGER REFERENCES coach_teams(id),
	player_id INTEGER REFERENCES player_cards(id),
	text TEXT NOT NULL,
	created_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_cards_team_group ON player_cards(team_id, custodian_group_id);
CREATE INDEX IF NOT EXISTS idx_lineup_match_team ON coach_lineups(match_id, team_id, on_field);
CREATE INDEX IF NOT EXISTS idx_coach_events_match ON coach_events(match_id, id);`

func openDatabase(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(`PRAGMA foreign_keys = ON; PRAGMA busy_timeout = 5000;`); err != nil {
		db.Close()
		return nil, err
	}
	if _, err := db.Exec(coachSchema); err != nil {
		db.Close()
		return nil, fmt.Errorf("create coach schema: %w", err)
	}
	if err := seedCoachData(db); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

type databaseWriter interface {
	Exec(query string, args ...any) (sql.Result, error)
	QueryRow(query string, args ...any) *sql.Row
}

func seedCoachData(db *sql.DB) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := seedCoachDataTx(tx); err != nil {
		return err
	}
	return tx.Commit()
}

func seedCoachDataTx(tx *sql.Tx) error {
	for _, team := range []CoachTeam{
		{ID: 1, Name: "Team Grün", Color: "green", Hymns: 8, SponsorLevel: 1},
		{ID: 2, Name: "Team Blau", Color: "blue", Hymns: 8, SponsorLevel: 1},
	} {
		if _, err := tx.Exec(`INSERT OR IGNORE INTO coach_teams (id, name, color, hymns, sponsor_level) VALUES (?, ?, ?, ?, ?)`,
			team.ID, team.Name, team.Color, team.Hymns, team.SponsorLevel); err != nil {
			return err
		}
	}
	for teamID := int64(1); teamID <= 2; teamID++ {
		for number := 1; number <= 4; number++ {
			id := (teamID-1)*4 + int64(number)
			if _, err := tx.Exec(`INSERT OR IGNORE INTO coach_groups (id, team_id, number, name) VALUES (?, ?, ?, ?)`,
				id, teamID, number, fmt.Sprintf("Gruppe %d", number)); err != nil {
				return err
			}
		}
	}

	var cardCount int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM player_cards`).Scan(&cardCount); err != nil {
		return err
	}
	if cardCount == 0 {
		for teamID := int64(1); teamID <= 2; teamID++ {
			for number := 1; number <= 26; number++ {
				id := (teamID-1)*26 + int64(number)
				position := cardPosition(number)
				attack, defense := initialCardValues(position, number, int(teamID))
				fitness := 68 + (number*3+int(teamID)*5)%22
				morale := 66 + (number*5+int(teamID)*3)%24
				groupID := (teamID-1)*4 + int64((number-1)%4+1)
				name := fmt.Sprintf("%s %02d", map[int64]string{1: "Grün", 2: "Blau"}[teamID], number)
				if _, err := tx.Exec(`
					INSERT INTO player_cards (
						id, team_id, number, name, position, attack, defense, fitness, morale, custodian_group_id
					) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
					id, teamID, number, name, position, attack, defense, fitness, morale, groupID); err != nil {
					return err
				}
			}
		}
	}

	var matchID int64
	err := tx.QueryRow(`SELECT id FROM coach_matches ORDER BY id DESC LIMIT 1`).Scan(&matchID)
	if errors.Is(err, sql.ErrNoRows) {
		result, insertErr := tx.Exec(`INSERT INTO coach_matches (home_team_id, away_team_id, seed) VALUES (1, 2, 2026)`)
		if insertErr != nil {
			return insertErr
		}
		matchID, err = result.LastInsertId()
		if err != nil {
			return err
		}
	} else if err != nil {
		return err
	}

	var lineupCount int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM coach_lineups WHERE match_id = ?`, matchID).Scan(&lineupCount); err != nil {
		return err
	}
	if lineupCount == 0 {
		if err := seedLineups(tx, matchID); err != nil {
			return err
		}
		if _, err := tx.Exec(`INSERT INTO coach_events (match_id, minute, kind, text) VALUES (?, 0, 'setup', 'Die Coach-Zentrale ist einsatzbereit.')`, matchID); err != nil {
			return err
		}
	}
	return nil
}

func seedLineups(tx *sql.Tx, matchID int64) error {
	starters := map[int]string{
		1: "GK", 4: "DEF-1", 5: "DEF-2", 6: "DEF-3", 7: "DEF-4",
		12: "MID-1", 13: "MID-2", 14: "MID-3", 15: "MID-4",
		20: "FWD-1", 21: "FWD-2",
	}
	for teamID := int64(1); teamID <= 2; teamID++ {
		for number := 1; number <= 26; number++ {
			playerID := (teamID-1)*26 + int64(number)
			slot, onField := starters[number]
			if !onField {
				slot = "BENCH"
			}
			if _, err := tx.Exec(`INSERT INTO coach_lineups (match_id, player_id, team_id, on_field, slot) VALUES (?, ?, ?, ?, ?)`,
				matchID, playerID, teamID, onField, slot); err != nil {
				return err
			}
		}
	}
	return nil
}

func cardPosition(number int) string {
	switch {
	case number <= 3:
		return "GK"
	case number <= 11:
		return "DEF"
	case number <= 19:
		return "MID"
	default:
		return "FWD"
	}
}

func initialCardValues(position string, number, team int) (int, int) {
	variation := (number*7 + team*5) % 14
	switch position {
	case "GK":
		return 18 + variation/2, 70 + variation
	case "DEF":
		return 40 + variation, 65 + variation
	case "MID":
		return 56 + variation, 54 + variation
	default:
		return 68 + variation, 34 + variation
	}
}

func loadCoachState(db *sql.DB) (CoachState, error) {
	teams, err := listCoachTeams(db)
	if err != nil {
		return CoachState{}, err
	}
	groups, err := listSquadGroups(db)
	if err != nil {
		return CoachState{}, err
	}
	players, err := listPlayerCards(db)
	if err != nil {
		return CoachState{}, err
	}
	match, err := currentCoachMatch(db)
	if err != nil {
		return CoachState{}, err
	}
	lineup, err := listLineup(db, match.ID)
	if err != nil {
		return CoachState{}, err
	}
	events, err := listCoachEvents(db, match.ID)
	if err != nil {
		return CoachState{}, err
	}
	return CoachState{Rules: gameRules, Teams: teams, Groups: groups, Players: players, Match: match, Lineup: lineup, Events: events}, nil
}

func listCoachTeams(db *sql.DB) ([]CoachTeam, error) {
	rows, err := db.Query(`SELECT id, name, color, hymns, sponsor_level FROM coach_teams ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	teams := []CoachTeam{}
	for rows.Next() {
		var team CoachTeam
		if err := rows.Scan(&team.ID, &team.Name, &team.Color, &team.Hymns, &team.SponsorLevel); err != nil {
			return nil, err
		}
		teams = append(teams, team)
	}
	return teams, rows.Err()
}

func listSquadGroups(db *sql.DB) ([]SquadGroup, error) {
	rows, err := db.Query(`SELECT id, team_id, number, name, fifa_attention FROM coach_groups ORDER BY team_id, number`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	groups := []SquadGroup{}
	for rows.Next() {
		var group SquadGroup
		if err := rows.Scan(&group.ID, &group.TeamID, &group.Number, &group.Name, &group.FIFAAttention); err != nil {
			return nil, err
		}
		groups = append(groups, group)
	}
	return groups, rows.Err()
}

func listPlayerCards(db *sql.DB) ([]PlayerCard, error) {
	rows, err := db.Query(`
		SELECT id, team_id, number, name, position, attack, defense, fitness, morale,
			doping_level, suspended, custodian_group_id
		FROM player_cards ORDER BY team_id, number`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	players := []PlayerCard{}
	for rows.Next() {
		var player PlayerCard
		if err := rows.Scan(&player.ID, &player.TeamID, &player.Number, &player.Name, &player.Position,
			&player.Attack, &player.Defense, &player.Fitness, &player.Morale,
			&player.DopingLevel, &player.Suspended, &player.CustodianGroupID); err != nil {
			return nil, err
		}
		players = append(players, player)
	}
	return players, rows.Err()
}

func currentCoachMatch(q databaseWriter) (CoachMatch, error) {
	var match CoachMatch
	err := q.QueryRow(`
		SELECT id, home_team_id, away_team_id, phase, minute, home_score, away_score,
			home_substitutions, away_substitutions, seed
		FROM coach_matches ORDER BY id DESC LIMIT 1`).Scan(
		&match.ID, &match.HomeTeamID, &match.AwayTeamID, &match.Phase, &match.Minute,
		&match.HomeScore, &match.AwayScore, &match.HomeSubstitutions, &match.AwaySubstitutions, &match.Seed)
	return match, err
}

func getPlayerCard(q databaseWriter, id int64) (PlayerCard, error) {
	var player PlayerCard
	err := q.QueryRow(`
		SELECT id, team_id, number, name, position, attack, defense, fitness, morale,
			doping_level, suspended, custodian_group_id
		FROM player_cards WHERE id = ?`, id).Scan(
		&player.ID, &player.TeamID, &player.Number, &player.Name, &player.Position,
		&player.Attack, &player.Defense, &player.Fitness, &player.Morale,
		&player.DopingLevel, &player.Suspended, &player.CustodianGroupID)
	return player, err
}

func getSquadGroup(q databaseWriter, id int64) (SquadGroup, error) {
	var group SquadGroup
	err := q.QueryRow(`SELECT id, team_id, number, name, fifa_attention FROM coach_groups WHERE id = ?`, id).Scan(
		&group.ID, &group.TeamID, &group.Number, &group.Name, &group.FIFAAttention)
	return group, err
}

func listLineup(db *sql.DB, matchID int64) ([]LineupEntry, error) {
	rows, err := db.Query(`
		SELECT l.player_id, l.team_id, l.on_field, l.slot, l.entered_at, l.left_at,
			p.number, p.name, p.position
		FROM coach_lineups l JOIN player_cards p ON p.id = l.player_id
		WHERE l.match_id = ? ORDER BY l.team_id, l.on_field DESC, p.position, p.number`, matchID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	entries := []LineupEntry{}
	for rows.Next() {
		var entry LineupEntry
		var leftAt sql.NullInt64
		if err := rows.Scan(&entry.PlayerID, &entry.TeamID, &entry.OnField, &entry.Slot,
			&entry.EnteredAt, &leftAt, &entry.PlayerNumber, &entry.PlayerName, &entry.Position); err != nil {
			return nil, err
		}
		if leftAt.Valid {
			value := int(leftAt.Int64)
			entry.LeftAt = &value
		}
		entries = append(entries, entry)
	}
	return entries, rows.Err()
}

func listCoachEvents(db *sql.DB, matchID int64) ([]CoachEvent, error) {
	rows, err := db.Query(`SELECT id, match_id, minute, kind, team_id, player_id, text FROM coach_events WHERE match_id = ? ORDER BY id DESC LIMIT 100`, matchID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	events := []CoachEvent{}
	for rows.Next() {
		var event CoachEvent
		var teamID, playerID sql.NullInt64
		if err := rows.Scan(&event.ID, &event.MatchID, &event.Minute, &event.Kind, &teamID, &playerID, &event.Text); err != nil {
			return nil, err
		}
		if teamID.Valid {
			value := teamID.Int64
			event.TeamID = &value
		}
		if playerID.Valid {
			value := playerID.Int64
			event.PlayerID = &value
		}
		events = append(events, event)
	}
	return events, rows.Err()
}

func appendCoachEvent(tx *sql.Tx, match CoachMatch, kind string, teamID, playerID *int64, text string) error {
	_, err := tx.Exec(`INSERT INTO coach_events (match_id, minute, kind, team_id, player_id, text) VALUES (?, ?, ?, ?, ?, ?)`,
		match.ID, match.Minute, kind, teamID, playerID, text)
	return err
}

func resetCoachGame(db *sql.DB) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, table := range []string{"coach_events", "coach_lineups", "coach_matches", "player_cards", "coach_groups", "coach_teams"} {
		if _, err := tx.Exec(`DELETE FROM ` + table); err != nil {
			return err
		}
	}
	if err := seedCoachDataTx(tx); err != nil {
		return err
	}
	return tx.Commit()
}
