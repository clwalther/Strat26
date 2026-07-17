package main

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func testApplication(t *testing.T) (*sql.DB, http.Handler) {
	t.Helper()
	db, err := openDatabase(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db, newServer(db)
}

func performRequest(t *testing.T, server http.Handler, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	recorder := httptest.NewRecorder()
	server.ServeHTTP(recorder, request)
	return recorder
}

func decodeResponse[T any](t *testing.T, response *httptest.ResponseRecorder) T {
	t.Helper()
	var value T
	if err := json.Unmarshal(response.Body.Bytes(), &value); err != nil {
		t.Fatalf("decode response %q: %v", response.Body.String(), err)
	}
	return value
}

func TestGenerateAndSimulateSeason(t *testing.T) {
	_, server := testApplication(t)

	response := performRequest(t, server, http.MethodPost, "/api/schedule/generate",
		`{"startAt":"2026-08-01T15:00:00Z","intervalDays":7}`)
	if response.Code != http.StatusCreated {
		t.Fatalf("generate schedule returned %d: %s", response.Code, response.Body.String())
	}
	generated := decodeResponse[APIState](t, response)
	if len(generated.Games) != 16 {
		t.Fatalf("expected 16 games, got %d", len(generated.Games))
	}
	rounds := map[int]int{}
	pairs := map[[2]int64]bool{}
	for _, game := range generated.Games {
		rounds[game.Round]++
		pair := [2]int64{game.HomeTeamID, game.AwayTeamID}
		if pairs[pair] {
			t.Fatalf("duplicate pairing: %v", pair)
		}
		pairs[pair] = true
	}
	for round := 1; round <= 4; round++ {
		if rounds[round] != 4 {
			t.Fatalf("round %d contains %d games", round, rounds[round])
		}
	}

	response = performRequest(t, server, http.MethodPost, "/api/simulate", `{"seed":2026}`)
	if response.Code != http.StatusOK {
		t.Fatalf("simulate season returned %d: %s", response.Code, response.Body.String())
	}
	simulated := decodeResponse[APIState](t, response)
	for _, game := range simulated.Games {
		if game.Status != "played" || game.HomeScore == nil || game.AwayScore == nil {
			t.Fatalf("game %d was not simulated: %+v", game.ID, game)
		}
	}
	if len(simulated.Standing) != 8 {
		t.Fatalf("expected 8 standings rows, got %d", len(simulated.Standing))
	}
	for _, row := range simulated.Standing {
		if row.Played != 4 {
			t.Fatalf("team %d played %d games, expected 4", row.TeamID, row.Played)
		}
		if row.GoalDifference != row.GoalsFor-row.GoalsAgainst {
			t.Fatalf("invalid goal difference for team %d", row.TeamID)
		}
	}
}

func TestSimulationIsReproducible(t *testing.T) {
	_, server := testApplication(t)
	created := performRequest(t, server, http.MethodPost, "/api/games",
		`{"homeTeamId":1,"awayTeamId":5,"round":1}`)
	if created.Code != http.StatusCreated {
		t.Fatalf("create game returned %d: %s", created.Code, created.Body.String())
	}
	game := decodeResponse[Game](t, created)
	path := "/api/games/" + integerString(game.ID) + "/simulate"

	firstResponse := performRequest(t, server, http.MethodPost, path, `{"seed":42}`)
	if firstResponse.Code != http.StatusOK {
		t.Fatalf("first simulation returned %d: %s", firstResponse.Code, firstResponse.Body.String())
	}
	first := decodeResponse[GameDetails](t, firstResponse)
	secondResponse := performRequest(t, server, http.MethodPost, path, `{"seed":42}`)
	second := decodeResponse[GameDetails](t, secondResponse)
	if *first.Game.HomeScore != *second.Game.HomeScore || *first.Game.AwayScore != *second.Game.AwayScore {
		t.Fatalf("same seed produced different scores: %d:%d vs %d:%d",
			*first.Game.HomeScore, *first.Game.AwayScore,
			*second.Game.HomeScore, *second.Game.AwayScore)
	}
	type comparableEvent struct {
		Minute int
		Kind   string
		TeamID int64
		Player string
	}
	compact := func(events []GameEvent) []comparableEvent {
		result := make([]comparableEvent, len(events))
		for index, event := range events {
			result[index] = comparableEvent{event.Minute, event.Kind, event.TeamID, event.Player}
		}
		return result
	}
	if !reflect.DeepEqual(compact(first.Events), compact(second.Events)) {
		t.Fatal("same seed produced a different event timeline")
	}
}

func TestManualResultsAndValidation(t *testing.T) {
	_, server := testApplication(t)
	tests := []string{
		`{"homeTeamId":1,"awayTeamId":1}`,
		`{"homeTeamId":1,"awayTeamId":99}`,
		`{"homeTeamId":1,"awayTeamId":5,"homeScore":2}`,
		`{"homeTeamId":1,"awayTeamId":5,"homeScore":-1,"awayScore":0}`,
		`{"homeTeamId":1,"awayTeamId":5,"unexpected":true}`,
	}
	for _, body := range tests {
		response := performRequest(t, server, http.MethodPost, "/api/games", body)
		if response.Code != http.StatusBadRequest {
			t.Errorf("invalid request %s returned %d", body, response.Code)
		}
	}

	response := performRequest(t, server, http.MethodPost, "/api/games",
		`{"home":1,"away":5,"homeScore":3,"awayScore":1}`)
	if response.Code != http.StatusCreated {
		t.Fatalf("legacy-compatible create returned %d: %s", response.Code, response.Body.String())
	}
	standingResponse := performRequest(t, server, http.MethodGet, "/api/standings", "")
	standing := decodeResponse[[]Standing](t, standingResponse)
	if standing[0].TeamID != 1 || standing[0].Points != 3 {
		t.Fatalf("manual result was not reflected in standings: %+v", standing[0])
	}
}

func TestLegacyDatabaseMigration(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy.db")
	legacy, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	_, err = legacy.Exec(`
		CREATE TABLE games (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			home INTEGER NOT NULL,
			away INTEGER NOT NULL,
			home_score INTEGER,
			away_score INTEGER,
			created_at TEXT NOT NULL DEFAULT (datetime('now'))
		);
		INSERT INTO games (home, away, home_score, away_score) VALUES (2, 6, 4, 2);`)
	if err != nil {
		t.Fatal(err)
	}
	if err := legacy.Close(); err != nil {
		t.Fatal(err)
	}

	db, err := openDatabase(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	games, err := listGames(db)
	if err != nil {
		t.Fatal(err)
	}
	if len(games) != 1 || games[0].HomeTeamID != 2 || games[0].AwayTeamID != 6 {
		t.Fatalf("legacy game was not migrated: %+v", games)
	}
	if games[0].Status != "played" || *games[0].HomeScore != 4 || *games[0].AwayScore != 2 {
		t.Fatalf("legacy result was not preserved: %+v", games[0])
	}
}

func TestHealthAndSecurityHeaders(t *testing.T) {
	_, server := testApplication(t)
	response := performRequest(t, server, http.MethodGet, "/api/health", "")
	if response.Code != http.StatusOK {
		t.Fatalf("health returned %d", response.Code)
	}
	if response.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatal("security headers are missing")
	}
}

func integerString(value int64) string {
	return strconv.FormatInt(value, 10)
}
