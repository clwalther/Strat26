package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

type application struct {
	db *sql.DB
}

func newServer(db *sql.DB) http.Handler {
	app := &application{db: db}
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/health", app.handleHealth)
	mux.HandleFunc("GET /api/state", app.handleState)
	mux.HandleFunc("GET /api/teams", app.handleTeams)
	mux.HandleFunc("GET /api/seasons/current", app.handleCurrentSeason)
	mux.HandleFunc("GET /api/standings", app.handleStandings)
	mux.HandleFunc("GET /api/games", app.handleListGames)
	mux.HandleFunc("POST /api/games", app.handleCreateGame)
	mux.HandleFunc("DELETE /api/games", app.handleDeleteAllGames)
	mux.HandleFunc("GET /api/games/{id}", app.handleGetGame)
	mux.HandleFunc("PATCH /api/games/{id}", app.handleUpdateScore)
	mux.HandleFunc("DELETE /api/games/{id}", app.handleDeleteGame)
	mux.HandleFunc("POST /api/games/{id}/simulate", app.handleSimulateGame)
	mux.HandleFunc("POST /api/schedule/generate", app.handleGenerateSchedule)
	mux.HandleFunc("POST /api/simulate", app.handleSimulateAll)
	mux.HandleFunc("POST /api/reset", app.handleReset)
	mux.Handle("GET /", http.FileServer(http.Dir("./source")))

	return securityHeaders(mux)
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "same-origin")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; style-src 'self'; script-src 'self'; img-src 'self' data:; connect-src 'self'")
		next.ServeHTTP(w, r)
	})
}

func (app *application) handleHealth(w http.ResponseWriter, _ *http.Request) {
	if err := app.db.Ping(); err != nil {
		writeError(w, http.StatusServiceUnavailable, "database unavailable")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (app *application) handleState(w http.ResponseWriter, _ *http.Request) {
	state, err := app.state()
	if err != nil {
		app.internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, state)
}

func (app *application) state() (APIState, error) {
	season, err := currentSeason(app.db)
	if err != nil {
		return APIState{}, err
	}
	teams, err := listTeams(app.db)
	if err != nil {
		return APIState{}, err
	}
	games, err := listGames(app.db)
	if err != nil {
		return APIState{}, err
	}
	standings, err := calculateStandings(app.db)
	if err != nil {
		return APIState{}, err
	}
	return APIState{Season: season, Teams: teams, Games: games, Standing: standings}, nil
}

func (app *application) handleTeams(w http.ResponseWriter, _ *http.Request) {
	teams, err := listTeams(app.db)
	if err != nil {
		app.internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, teams)
}

func (app *application) handleCurrentSeason(w http.ResponseWriter, _ *http.Request) {
	season, err := currentSeason(app.db)
	if err != nil {
		app.internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, season)
}

func (app *application) handleStandings(w http.ResponseWriter, _ *http.Request) {
	standings, err := calculateStandings(app.db)
	if err != nil {
		app.internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, standings)
}

func (app *application) handleListGames(w http.ResponseWriter, r *http.Request) {
	status, err := normalizeStatus(r.URL.Query().Get("status"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	var teamID int64
	if rawTeam := r.URL.Query().Get("team"); rawTeam != "" {
		teamID, err = strconv.ParseInt(rawTeam, 10, 64)
		if err != nil || teamID < 1 {
			writeError(w, http.StatusBadRequest, "team must be a positive integer")
			return
		}
	}
	games, err := listGames(app.db)
	if err != nil {
		app.internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, filterGames(games, status, teamID))
}

func (app *application) handleGetGame(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r.PathValue("id"))
	if !ok {
		return
	}
	game, err := getGame(app.db, id)
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusNotFound, "game not found")
		return
	}
	if err != nil {
		app.internalError(w, err)
		return
	}
	events, err := listGameEvents(app.db, id)
	if err != nil {
		app.internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, GameDetails{Game: game, Events: events})
}

func (app *application) handleCreateGame(w http.ResponseWriter, r *http.Request) {
	var input createGameRequest
	if err := decodeJSON(w, r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if input.HomeTeamID == 0 {
		input.HomeTeamID = input.Home
	}
	if input.AwayTeamID == 0 {
		input.AwayTeamID = input.Away
	}
	if err := validateGameTeams(app.db, input.HomeTeamID, input.AwayTeamID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if input.Round < 0 {
		writeError(w, http.StatusBadRequest, "round must not be negative")
		return
	}
	if (input.HomeScore == nil) != (input.AwayScore == nil) {
		writeError(w, http.StatusBadRequest, "both scores are required for a result")
		return
	}
	if scoreIsNegative(input.HomeScore) || scoreIsNegative(input.AwayScore) {
		writeError(w, http.StatusBadRequest, "scores must not be negative")
		return
	}
	kickoff, err := normalizeKickoff(input.Kickoff)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if input.SeasonID == 0 {
		season, err := currentSeason(app.db)
		if err != nil {
			app.internalError(w, err)
			return
		}
		input.SeasonID = season.ID
	}
	status := "scheduled"
	if input.HomeScore != nil {
		status = "played"
	}
	result, err := app.db.Exec(`
		INSERT INTO games (
			season_id, round, home_team_id, away_team_id, kickoff, status, home_score, away_score
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		input.SeasonID, input.Round, input.HomeTeamID, input.AwayTeamID,
		nullableString(kickoff), status, input.HomeScore, input.AwayScore)
	if err != nil {
		if strings.Contains(err.Error(), "FOREIGN KEY") {
			writeError(w, http.StatusBadRequest, "unknown season or team")
			return
		}
		app.internalError(w, err)
		return
	}
	id, err := result.LastInsertId()
	if err != nil {
		app.internalError(w, err)
		return
	}
	game, err := getGame(app.db, id)
	if err != nil {
		app.internalError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, game)
}

func (app *application) handleUpdateScore(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r.PathValue("id"))
	if !ok {
		return
	}
	var input scoreRequest
	if err := decodeJSON(w, r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if input.HomeScore == nil || input.AwayScore == nil {
		writeError(w, http.StatusBadRequest, "homeScore and awayScore are required")
		return
	}
	if scoreIsNegative(input.HomeScore) || scoreIsNegative(input.AwayScore) {
		writeError(w, http.StatusBadRequest, "scores must not be negative")
		return
	}
	tx, err := app.db.Begin()
	if err != nil {
		app.internalError(w, err)
		return
	}
	defer tx.Rollback()
	result, err := tx.Exec(`
		UPDATE games SET status = 'played', home_score = ?, away_score = ?,
			simulation_seed = NULL, updated_at = datetime('now') WHERE id = ?`,
		*input.HomeScore, *input.AwayScore, id)
	if err != nil {
		app.internalError(w, err)
		return
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		writeError(w, http.StatusNotFound, "game not found")
		return
	}
	if _, err := tx.Exec(`DELETE FROM game_events WHERE game_id = ?`, id); err != nil {
		app.internalError(w, err)
		return
	}
	if err := tx.Commit(); err != nil {
		app.internalError(w, err)
		return
	}
	game, err := getGame(app.db, id)
	if err != nil {
		app.internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, game)
}

func (app *application) handleDeleteGame(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r.PathValue("id"))
	if !ok {
		return
	}
	result, err := app.db.Exec(`DELETE FROM games WHERE id = ?`, id)
	if err != nil {
		app.internalError(w, err)
		return
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		writeError(w, http.StatusNotFound, "game not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (app *application) handleDeleteAllGames(w http.ResponseWriter, _ *http.Request) {
	if _, err := app.db.Exec(`DELETE FROM games`); err != nil {
		app.internalError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (app *application) handleSimulateGame(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r.PathValue("id"))
	if !ok {
		return
	}
	input := simulateRequest{}
	if err := decodeOptionalJSON(w, r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	seed := time.Now().UnixNano()
	if input.Seed != nil {
		seed = *input.Seed
	}
	details, err := simulateGame(app.db, id, seed)
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusNotFound, "game not found")
		return
	}
	if err != nil {
		app.internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, details)
}

func (app *application) handleSimulateAll(w http.ResponseWriter, r *http.Request) {
	input := simulateRequest{}
	if err := decodeOptionalJSON(w, r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	seed := time.Now().UnixNano()
	if input.Seed != nil {
		seed = *input.Seed
	}
	games, err := listGames(app.db)
	if err != nil {
		app.internalError(w, err)
		return
	}
	for _, game := range games {
		if game.Status == "played" && !input.IncludePlayed {
			continue
		}
		if _, err := simulateGame(app.db, game.ID, seed+game.ID*7919); err != nil {
			app.internalError(w, err)
			return
		}
	}
	state, err := app.state()
	if err != nil {
		app.internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, state)
}

func (app *application) handleGenerateSchedule(w http.ResponseWriter, r *http.Request) {
	input := scheduleRequest{IntervalDays: 7}
	if err := decodeOptionalJSON(w, r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if input.IntervalDays < 1 || input.IntervalDays > 60 {
		writeError(w, http.StatusBadRequest, "intervalDays must be between 1 and 60")
		return
	}
	start, err := scheduleStart(input.StartAt)
	if err != nil {
		writeError(w, http.StatusBadRequest, "startAt must be RFC3339")
		return
	}
	season, err := currentSeason(app.db)
	if err != nil {
		app.internalError(w, err)
		return
	}

	tx, err := app.db.Begin()
	if err != nil {
		app.internalError(w, err)
		return
	}
	defer tx.Rollback()
	var count int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM games WHERE season_id = ?`, season.ID).Scan(&count); err != nil {
		app.internalError(w, err)
		return
	}
	if count > 0 && !input.Replace {
		writeError(w, http.StatusConflict, "schedule already contains games; set replace to true")
		return
	}
	if input.Replace {
		if _, err := tx.Exec(`DELETE FROM games WHERE season_id = ?`, season.ID); err != nil {
			app.internalError(w, err)
			return
		}
	}
	greens := []int64{1, 2, 3, 4}
	blues := []int64{5, 6, 7, 8}
	for round := 0; round < 4; round++ {
		kickoff := start.AddDate(0, 0, round*input.IntervalDays).Format(time.RFC3339)
		for index, home := range greens {
			away := blues[(index+round)%len(blues)]
			if _, err := tx.Exec(`
				INSERT INTO games (season_id, round, home_team_id, away_team_id, kickoff)
				VALUES (?, ?, ?, ?, ?)`, season.ID, round+1, home, away, kickoff); err != nil {
				app.internalError(w, err)
				return
			}
		}
	}
	if err := tx.Commit(); err != nil {
		app.internalError(w, err)
		return
	}
	state, err := app.state()
	if err != nil {
		app.internalError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, state)
}

func (app *application) handleReset(w http.ResponseWriter, _ *http.Request) {
	if _, err := app.db.Exec(`DELETE FROM games`); err != nil {
		app.internalError(w, err)
		return
	}
	state, err := app.state()
	if err != nil {
		app.internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, state)
}

func (app *application) internalError(w http.ResponseWriter, err error) {
	log.Printf("request failed: %v", err)
	writeError(w, http.StatusInternalServerError, "internal server error")
}

func decodeJSON(w http.ResponseWriter, r *http.Request, destination any) error {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("request body must contain one JSON object")
	}
	return nil
}

func decodeOptionalJSON(w http.ResponseWriter, r *http.Request, destination any) error {
	if r.Body == nil || r.ContentLength == 0 {
		return nil
	}
	return decodeJSON(w, r, destination)
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		log.Printf("encode response: %v", err)
	}
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func parseID(w http.ResponseWriter, raw string) (int64, bool) {
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id < 1 {
		writeError(w, http.StatusBadRequest, "invalid id")
		return 0, false
	}
	return id, true
}

func scoreIsNegative(score *int) bool {
	return score != nil && *score < 0
}

func normalizeKickoff(value *string) (*string, error) {
	if value == nil || strings.TrimSpace(*value) == "" {
		return nil, nil
	}
	raw := strings.TrimSpace(*value)
	formats := []string{time.RFC3339, "2006-01-02T15:04"}
	for _, format := range formats {
		if parsed, err := time.Parse(format, raw); err == nil {
			normalized := parsed.Format(time.RFC3339)
			return &normalized, nil
		}
	}
	return nil, errors.New("kickoff must be RFC3339 or YYYY-MM-DDTHH:MM")
}

func nullableString(value *string) any {
	if value == nil {
		return nil
	}
	return *value
}

func scheduleStart(raw string) (time.Time, error) {
	if strings.TrimSpace(raw) != "" {
		return time.Parse(time.RFC3339, strings.TrimSpace(raw))
	}
	now := time.Now().UTC()
	days := (int(time.Saturday) - int(now.Weekday()) + 7) % 7
	if days == 0 {
		days = 7
	}
	start := now.AddDate(0, 0, days)
	return time.Date(start.Year(), start.Month(), start.Day(), 15, 0, 0, 0, time.UTC), nil
}

func main() {
	databasePath := envOrDefault("DATABASE_PATH", "./database/database.db")
	if databasePath != ":memory:" {
		if err := os.MkdirAll(filepath.Dir(databasePath), 0o755); err != nil {
			log.Fatal(err)
		}
	}
	db, err := openDatabase(databasePath)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	port := envOrDefault("PORT", "8080")
	server := &http.Server{
		Addr:              ":" + port,
		Handler:           newServer(db),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			log.Printf("shutdown: %v", err)
		}
	}()

	log.Printf("Strat26 running on http://localhost:%s", port)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}

func envOrDefault(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}
