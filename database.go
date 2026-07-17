package main

import (
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strings"

	_ "modernc.org/sqlite"
)

const coreSchema = `
CREATE TABLE IF NOT EXISTS teams (
	id INTEGER PRIMARY KEY,
	name TEXT NOT NULL UNIQUE,
	short_name TEXT NOT NULL UNIQUE,
	color TEXT NOT NULL CHECK (color IN ('green', 'blue')),
	rating INTEGER NOT NULL CHECK (rating BETWEEN 1 AND 3000)
);

CREATE TABLE IF NOT EXISTS seasons (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	name TEXT NOT NULL UNIQUE,
	status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'archived')),
	created_at TEXT NOT NULL DEFAULT (datetime('now'))
);`

const gamesSchema = `
CREATE TABLE IF NOT EXISTS games (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	season_id INTEGER NOT NULL REFERENCES seasons(id) ON DELETE CASCADE,
	round INTEGER NOT NULL DEFAULT 0 CHECK (round >= 0),
	home_team_id INTEGER NOT NULL REFERENCES teams(id),
	away_team_id INTEGER NOT NULL REFERENCES teams(id),
	kickoff TEXT,
	status TEXT NOT NULL DEFAULT 'scheduled' CHECK (status IN ('scheduled', 'played')),
	home_score INTEGER,
	away_score INTEGER,
	simulation_seed INTEGER,
	created_at TEXT NOT NULL DEFAULT (datetime('now')),
	updated_at TEXT NOT NULL DEFAULT (datetime('now')),
	CHECK (home_team_id <> away_team_id),
	CHECK (home_score IS NULL OR home_score >= 0),
	CHECK (away_score IS NULL OR away_score >= 0),
	CHECK ((status = 'scheduled' AND home_score IS NULL AND away_score IS NULL)
		OR (status = 'played' AND home_score IS NOT NULL AND away_score IS NOT NULL))
);

CREATE TABLE IF NOT EXISTS game_events (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	game_id INTEGER NOT NULL REFERENCES games(id) ON DELETE CASCADE,
	minute INTEGER NOT NULL CHECK (minute BETWEEN 1 AND 120),
	kind TEXT NOT NULL CHECK (kind IN ('goal', 'yellow_card')),
	team_id INTEGER NOT NULL REFERENCES teams(id),
	player TEXT NOT NULL,
	detail TEXT NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS idx_games_season_round ON games(season_id, round, kickoff);
CREATE INDEX IF NOT EXISTS idx_games_teams ON games(home_team_id, away_team_id);
CREATE INDEX IF NOT EXISTS idx_events_game_minute ON game_events(game_id, minute);`

func openDatabase(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}

	// SQLite serializes writes. A single connection keeps transactions and
	// foreign-key settings predictable under concurrent HTTP requests.
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(`PRAGMA foreign_keys = ON; PRAGMA busy_timeout = 5000;`); err != nil {
		db.Close()
		return nil, err
	}
	if err := migrate(db); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

func migrate(db *sql.DB) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(coreSchema); err != nil {
		return fmt.Errorf("create core schema: %w", err)
	}
	if err := seedCoreData(tx); err != nil {
		return err
	}

	columns, err := tableColumns(tx, "games")
	if err != nil {
		return err
	}
	legacy := columns["home"] && !columns["home_team_id"]
	if legacy {
		if _, err := tx.Exec(`ALTER TABLE games RENAME TO games_legacy`); err != nil {
			return fmt.Errorf("rename legacy games table: %w", err)
		}
	}
	if _, err := tx.Exec(gamesSchema); err != nil {
		return fmt.Errorf("create games schema: %w", err)
	}

	if legacy {
		season, err := currentSeasonTx(tx)
		if err != nil {
			return err
		}
		_, err = tx.Exec(`
			INSERT INTO games (
				season_id, round, home_team_id, away_team_id, status,
				home_score, away_score, created_at, updated_at
			)
			SELECT ?, 0, home, away,
				CASE WHEN home_score IS NULL OR away_score IS NULL THEN 'scheduled' ELSE 'played' END,
				home_score, away_score, COALESCE(created_at, datetime('now')),
				COALESCE(created_at, datetime('now'))
			FROM games_legacy`, season.ID)
		if err != nil {
			return fmt.Errorf("import legacy games: %w", err)
		}
		if _, err := tx.Exec(`DROP TABLE games_legacy`); err != nil {
			return fmt.Errorf("drop legacy games table: %w", err)
		}
	}

	return tx.Commit()
}

func tableColumns(tx *sql.Tx, table string) (map[string]bool, error) {
	rows, err := tx.Query(`PRAGMA table_info(` + table + `)`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	columns := map[string]bool{}
	for rows.Next() {
		var cid, notNull, primaryKey int
		var name, columnType string
		var defaultValue any
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &primaryKey); err != nil {
			return nil, err
		}
		columns[name] = true
	}
	return columns, rows.Err()
}

func seedCoreData(tx *sql.Tx) error {
	teams := []Team{
		{1, "Gruppe 1", "G1", "green", 1580},
		{2, "Gruppe 2", "G2", "green", 1510},
		{3, "Gruppe 3", "G3", "green", 1460},
		{4, "Gruppe 4", "G4", "green", 1420},
		{5, "Gruppe 5", "B5", "blue", 1560},
		{6, "Gruppe 6", "B6", "blue", 1525},
		{7, "Gruppe 7", "B7", "blue", 1485},
		{8, "Gruppe 8", "B8", "blue", 1440},
	}
	for _, team := range teams {
		if _, err := tx.Exec(`
			INSERT INTO teams (id, name, short_name, color, rating)
			VALUES (?, ?, ?, ?, ?)
			ON CONFLICT(id) DO UPDATE SET
				name = excluded.name, short_name = excluded.short_name,
				color = excluded.color, rating = excluded.rating`,
			team.ID, team.Name, team.ShortName, team.Color, team.Rating); err != nil {
			return fmt.Errorf("seed teams: %w", err)
		}
	}
	if _, err := tx.Exec(`
		INSERT INTO seasons (name, status)
		SELECT 'Saison 2026', 'active'
		WHERE NOT EXISTS (SELECT 1 FROM seasons WHERE status = 'active')`); err != nil {
		return fmt.Errorf("seed season: %w", err)
	}
	return nil
}

func currentSeasonTx(tx *sql.Tx) (Season, error) {
	var season Season
	err := tx.QueryRow(`
		SELECT id, name, status, created_at
		FROM seasons WHERE status = 'active' ORDER BY id DESC LIMIT 1`).Scan(
		&season.ID, &season.Name, &season.Status, &season.CreatedAt)
	return season, err
}

func currentSeason(db *sql.DB) (Season, error) {
	var season Season
	err := db.QueryRow(`
		SELECT id, name, status, created_at
		FROM seasons WHERE status = 'active' ORDER BY id DESC LIMIT 1`).Scan(
		&season.ID, &season.Name, &season.Status, &season.CreatedAt)
	return season, err
}

func listTeams(db *sql.DB) ([]Team, error) {
	rows, err := db.Query(`SELECT id, name, short_name, color, rating FROM teams ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	teams := []Team{}
	for rows.Next() {
		var team Team
		if err := rows.Scan(&team.ID, &team.Name, &team.ShortName, &team.Color, &team.Rating); err != nil {
			return nil, err
		}
		teams = append(teams, team)
	}
	return teams, rows.Err()
}

type queryRower interface {
	QueryRow(query string, args ...any) *sql.Row
}

func getGame(q queryRower, id int64) (Game, error) {
	var game Game
	var kickoff sql.NullString
	var homeScore, awayScore sql.NullInt64
	var simSeed sql.NullInt64
	err := q.QueryRow(`
		SELECT id, season_id, round, home_team_id, away_team_id, kickoff,
			status, home_score, away_score, simulation_seed, created_at, updated_at
		FROM games WHERE id = ?`, id).Scan(
		&game.ID, &game.SeasonID, &game.Round, &game.HomeTeamID, &game.AwayTeamID,
		&kickoff, &game.Status, &homeScore, &awayScore, &simSeed,
		&game.CreatedAt, &game.UpdatedAt)
	if err != nil {
		return Game{}, err
	}
	if kickoff.Valid {
		game.Kickoff = &kickoff.String
	}
	if homeScore.Valid {
		score := int(homeScore.Int64)
		game.HomeScore = &score
	}
	if awayScore.Valid {
		score := int(awayScore.Int64)
		game.AwayScore = &score
	}
	if simSeed.Valid {
		seed := simSeed.Int64
		game.SimSeed = &seed
	}
	return game, nil
}

func listGames(db *sql.DB) ([]Game, error) {
	rows, err := db.Query(`SELECT id FROM games ORDER BY round, COALESCE(kickoff, ''), id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}

	games := make([]Game, 0, len(ids))
	for _, id := range ids {
		game, err := getGame(db, id)
		if err != nil {
			return nil, err
		}
		games = append(games, game)
	}
	return games, nil
}

func listGameEvents(db *sql.DB, gameID int64) ([]GameEvent, error) {
	rows, err := db.Query(`
		SELECT id, game_id, minute, kind, team_id, player, detail
		FROM game_events WHERE game_id = ? ORDER BY minute, id`, gameID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	events := []GameEvent{}
	for rows.Next() {
		var event GameEvent
		if err := rows.Scan(&event.ID, &event.GameID, &event.Minute, &event.Kind,
			&event.TeamID, &event.Player, &event.Detail); err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	return events, rows.Err()
}

func calculateStandings(db *sql.DB) ([]Standing, error) {
	teams, err := listTeams(db)
	if err != nil {
		return nil, err
	}
	games, err := listGames(db)
	if err != nil {
		return nil, err
	}

	byTeam := make(map[int64]*Standing, len(teams))
	for _, team := range teams {
		standing := &Standing{
			TeamID: team.ID, TeamName: team.Name, ShortName: team.ShortName, Color: team.Color,
		}
		byTeam[team.ID] = standing
	}

	for _, game := range games {
		if game.Status != "played" || game.HomeScore == nil || game.AwayScore == nil {
			continue
		}
		home, homeOK := byTeam[game.HomeTeamID]
		away, awayOK := byTeam[game.AwayTeamID]
		if !homeOK || !awayOK {
			continue
		}
		home.Played++
		away.Played++
		home.GoalsFor += *game.HomeScore
		home.GoalsAgainst += *game.AwayScore
		away.GoalsFor += *game.AwayScore
		away.GoalsAgainst += *game.HomeScore
		switch {
		case *game.HomeScore > *game.AwayScore:
			home.Won++
			home.Points += 3
			away.Lost++
		case *game.HomeScore < *game.AwayScore:
			away.Won++
			away.Points += 3
			home.Lost++
		default:
			home.Drawn++
			away.Drawn++
			home.Points++
			away.Points++
		}
	}

	standings := make([]Standing, 0, len(byTeam))
	for _, standing := range byTeam {
		standing.GoalDifference = standing.GoalsFor - standing.GoalsAgainst
		standings = append(standings, *standing)
	}
	sort.SliceStable(standings, func(i, j int) bool {
		if standings[i].Points != standings[j].Points {
			return standings[i].Points > standings[j].Points
		}
		if standings[i].GoalDifference != standings[j].GoalDifference {
			return standings[i].GoalDifference > standings[j].GoalDifference
		}
		if standings[i].GoalsFor != standings[j].GoalsFor {
			return standings[i].GoalsFor > standings[j].GoalsFor
		}
		return standings[i].TeamID < standings[j].TeamID
	})
	for index := range standings {
		standings[index].Rank = index + 1
	}
	return standings, nil
}

func validateGameTeams(db *sql.DB, homeID, awayID int64) error {
	if homeID == awayID {
		return errors.New("home and away team must differ")
	}
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM teams WHERE id IN (?, ?)`, homeID, awayID).Scan(&count); err != nil {
		return err
	}
	if count != 2 {
		return errors.New("unknown team")
	}
	return nil
}

func filterGames(games []Game, status string, teamID int64) []Game {
	if status == "" && teamID == 0 {
		return games
	}
	filtered := make([]Game, 0, len(games))
	for _, game := range games {
		if status != "" && game.Status != status {
			continue
		}
		if teamID != 0 && game.HomeTeamID != teamID && game.AwayTeamID != teamID {
			continue
		}
		filtered = append(filtered, game)
	}
	return filtered
}

func normalizeStatus(value string) (string, error) {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" || value == "scheduled" || value == "played" {
		return value, nil
	}
	return "", errors.New("status must be scheduled or played")
}
