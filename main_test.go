package main

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
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

func getState(t *testing.T, server http.Handler) CoachState {
	t.Helper()
	response := performRequest(t, server, http.MethodGet, "/api/state", "")
	if response.Code != http.StatusOK {
		t.Fatalf("state returned %d: %s", response.Code, response.Body.String())
	}
	return decodeResponse[CoachState](t, response)
}

func TestInitialCoachState(t *testing.T) {
	_, server := testApplication(t)
	state := getState(t, server)
	if len(state.Teams) != 2 {
		t.Fatalf("expected 2 teams, got %d", len(state.Teams))
	}
	if len(state.Groups) != 8 {
		t.Fatalf("expected 8 groups, got %d", len(state.Groups))
	}
	if len(state.Players) != 52 {
		t.Fatalf("expected 52 player cards, got %d", len(state.Players))
	}
	teamCards := map[int64]int{}
	for _, player := range state.Players {
		teamCards[player.TeamID]++
	}
	if teamCards[1] != 26 || teamCards[2] != 26 {
		t.Fatalf("each team must own 26 cards: %+v", teamCards)
	}
	onField := map[int64]int{}
	for _, entry := range state.Lineup {
		if entry.OnField {
			onField[entry.TeamID]++
		}
	}
	if onField[1] != 11 || onField[2] != 11 {
		t.Fatalf("each team must start with 11 players: %+v", onField)
	}
	if state.Match.HomeTeamID != 1 || state.Match.AwayTeamID != 2 {
		t.Fatalf("wrong match pairing: %+v", state.Match)
	}
}

func TestSponsorTrainingAndDoping(t *testing.T) {
	_, server := testApplication(t)
	initial := getState(t, server)
	player := initial.Players[3]
	initialAttack := player.Attack
	initialFitness := player.Fitness

	response := performRequest(t, server, http.MethodPost, "/api/actions/sponsor", `{"teamId":1}`)
	if response.Code != http.StatusOK {
		t.Fatalf("sponsor returned %d: %s", response.Code, response.Body.String())
	}
	afterSponsor := decodeResponse[CoachState](t, response)
	if afterSponsor.Teams[0].Hymns != 8+gameRules.SponsorReward {
		t.Fatalf("sponsor reward missing: %+v", afterSponsor.Teams[0])
	}

	response = performRequest(t, server, http.MethodPost, "/api/actions/train",
		`{"teamId":1,"playerId":4,"focus":"attack"}`)
	if response.Code != http.StatusOK {
		t.Fatalf("training returned %d: %s", response.Code, response.Body.String())
	}
	afterTraining := decodeResponse[CoachState](t, response)
	trained := findPlayer(afterTraining.Players, 4)
	if trained.Attack != minInt(99, initialAttack+gameRules.TrainingGain) {
		t.Fatalf("training gain missing: %d -> %d", initialAttack, trained.Attack)
	}

	response = performRequest(t, server, http.MethodPost, "/api/actions/dope", `{"teamId":1,"playerId":4}`)
	if response.Code != http.StatusOK {
		t.Fatalf("doping returned %d: %s", response.Code, response.Body.String())
	}
	afterDoping := decodeResponse[CoachState](t, response)
	doped := findPlayer(afterDoping.Players, 4)
	if doped.Attack != minInt(99, trained.Attack+gameRules.DopingGain) || doped.Fitness != minInt(99, initialFitness+gameRules.DopingGain) {
		t.Fatalf("doping gain missing: %+v", doped)
	}
	if doped.DopingLevel != gameRules.DopingRisk {
		t.Fatalf("doping risk missing: %+v", doped)
	}
	group := findGroup(afterDoping.Groups, doped.CustodianGroupID)
	if group.FIFAAttention == 0 {
		t.Fatal("doping must increase FIFA attention for the responsible group")
	}
}

func TestFIFAInspectionCanSuspendCards(t *testing.T) {
	db, server := testApplication(t)
	if _, err := db.Exec(`UPDATE player_cards SET doping_level = 100 WHERE id = 1`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE coach_groups SET fifa_attention = 100 WHERE id = 1`); err != nil {
		t.Fatal(err)
	}
	response := performRequest(t, server, http.MethodPost, "/api/actions/inspect", `{"groupId":1,"seed":1}`)
	if response.Code != http.StatusOK {
		t.Fatalf("inspection returned %d: %s", response.Code, response.Body.String())
	}
	state := decodeResponse[CoachState](t, response)
	player := findPlayer(state.Players, 1)
	if !player.Suspended || player.DopingLevel != 0 {
		t.Fatalf("detected card was not suspended: %+v", player)
	}
	if findGroup(state.Groups, 1).FIFAAttention != 0 {
		t.Fatal("inspection must resolve the current attention value")
	}
	onField := 0
	for _, entry := range state.Lineup {
		if entry.TeamID == 1 && entry.OnField {
			onField++
		}
	}
	if onField != 11 {
		t.Fatalf("a suspended starter must be replaced automatically, got %d field players", onField)
	}
}

func TestLineupAndSubstitutionLimit(t *testing.T) {
	_, server := testApplication(t)
	response := performRequest(t, server, http.MethodPost, "/api/match/substitute",
		`{"teamId":1,"outPlayerId":4,"inPlayerId":8}`)
	if response.Code != http.StatusOK {
		t.Fatalf("pre-match lineup change returned %d: %s", response.Code, response.Body.String())
	}
	state := decodeResponse[CoachState](t, response)
	if state.Match.HomeSubstitutions != 0 {
		t.Fatal("pre-match lineup changes must not consume a substitution")
	}
	if lineupEntry(state.Lineup, 4).OnField || !lineupEntry(state.Lineup, 8).OnField {
		t.Fatal("lineup was not swapped")
	}

	advance := performRequest(t, server, http.MethodPost, "/api/match/advance", `{"minutes":5,"seed":9}`)
	if advance.Code != http.StatusOK {
		t.Fatalf("advance returned %d: %s", advance.Code, advance.Body.String())
	}
	response = performRequest(t, server, http.MethodPost, "/api/match/substitute",
		`{"teamId":1,"outPlayerId":5,"inPlayerId":9}`)
	state = decodeResponse[CoachState](t, response)
	if state.Match.HomeSubstitutions != 1 {
		t.Fatalf("in-match substitution was not counted: %+v", state.Match)
	}
}

func TestMatchCanRunToFullTime(t *testing.T) {
	_, server := testApplication(t)
	for step := 0; step < 18; step++ {
		response := performRequest(t, server, http.MethodPost, "/api/match/advance", `{"minutes":5,"seed":2026}`)
		if response.Code != http.StatusOK {
			t.Fatalf("step %d returned %d: %s", step, response.Code, response.Body.String())
		}
	}
	state := getState(t, server)
	if state.Match.Minute != 90 || state.Match.Phase != "finished" {
		t.Fatalf("match did not finish: %+v", state.Match)
	}
	if len(state.Events) < 4 {
		t.Fatalf("expected match timeline, got %d events", len(state.Events))
	}
}

func TestCardResponsibilityCanMoveWithinTeam(t *testing.T) {
	_, server := testApplication(t)
	response := performRequest(t, server, http.MethodPatch, "/api/players/1/group", `{"groupId":2}`)
	if response.Code != http.StatusOK {
		t.Fatalf("assign group returned %d: %s", response.Code, response.Body.String())
	}
	state := decodeResponse[CoachState](t, response)
	if findPlayer(state.Players, 1).CustodianGroupID != 2 {
		t.Fatal("card responsibility was not moved")
	}
	response = performRequest(t, server, http.MethodPatch, "/api/players/1/group", `{"groupId":5}`)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("cross-team assignment returned %d", response.Code)
	}
}

func TestHealthAndLegacyTablesCoexist(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy.db")
	legacy, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := legacy.Exec(`CREATE TABLE games (id INTEGER PRIMARY KEY, home INTEGER, away INTEGER); INSERT INTO games VALUES (1, 1, 5)`); err != nil {
		t.Fatal(err)
	}
	legacy.Close()
	db, err := openDatabase(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	server := newServer(db)
	response := performRequest(t, server, http.MethodGet, "/api/health", "")
	if response.Code != http.StatusOK || response.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("health/security check failed: %d", response.Code)
	}
	var legacyCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM games`).Scan(&legacyCount); err != nil || legacyCount != 1 {
		t.Fatal("coach schema must not destroy existing tables")
	}
}

func findPlayer(players []PlayerCard, id int64) PlayerCard {
	for _, player := range players {
		if player.ID == id {
			return player
		}
	}
	return PlayerCard{}
}

func findGroup(groups []SquadGroup, id int64) SquadGroup {
	for _, group := range groups {
		if group.ID == id {
			return group
		}
	}
	return SquadGroup{}
}

func lineupEntry(entries []LineupEntry, playerID int64) LineupEntry {
	for _, entry := range entries {
		if entry.PlayerID == playerID {
			return entry
		}
	}
	return LineupEntry{}
}
