package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

func testServer(t *testing.T) http.Handler {
	t.Helper()

	db, err := openDatabase(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })

	return newServer(db)
}

func request(t *testing.T, server http.Handler, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()

	recorder := httptest.NewRecorder()
	server.ServeHTTP(recorder, httptest.NewRequest(method, path, strings.NewReader(body)))
	return recorder
}

func listGames(t *testing.T, server http.Handler) []Game {
	t.Helper()

	response := request(t, server, "GET", "/api/games", "")
	if response.Code != http.StatusOK {
		t.Fatalf("GET /api/games returned %d", response.Code)
	}

	var games []Game
	if err := json.Unmarshal(response.Body.Bytes(), &games); err != nil {
		t.Fatal(err)
	}
	return games
}

func TestCreateListDelete(t *testing.T) {
	server := testServer(t)

	if games := listGames(t, server); len(games) != 0 {
		t.Fatalf("expected empty list, got %d games", len(games))
	}

	response := request(t, server, "POST", "/api/games",
		`{"home": 2, "away": 6, "homeScore": 3, "awayScore": 1}`)
	if response.Code != http.StatusCreated {
		t.Fatalf("POST returned %d: %s", response.Code, response.Body)
	}

	var created Game
	if err := json.Unmarshal(response.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if created.ID == 0 {
		t.Fatal("created game has no id")
	}

	response = request(t, server, "POST", "/api/games", `{"home": 3, "away": 5}`)
	if response.Code != http.StatusCreated {
		t.Fatalf("POST without score returned %d: %s", response.Code, response.Body)
	}

	games := listGames(t, server)
	if len(games) != 2 {
		t.Fatalf("expected 2 games, got %d", len(games))
	}
	if games[0].HomeScore == nil || *games[0].HomeScore != 3 {
		t.Fatalf("expected home score 3, got %v", games[0].HomeScore)
	}
	if games[1].HomeScore != nil {
		t.Fatalf("expected nil home score, got %d", *games[1].HomeScore)
	}

	response = request(t, server, "DELETE", "/api/games/"+strconv.FormatInt(created.ID, 10), "")
	if response.Code != http.StatusNoContent {
		t.Fatalf("DELETE returned %d: %s", response.Code, response.Body)
	}
	if games := listGames(t, server); len(games) != 1 {
		t.Fatalf("expected 1 game after delete, got %d", len(games))
	}
}

func TestDeleteAll(t *testing.T) {
	server := testServer(t)

	request(t, server, "POST", "/api/games", `{"home": 1, "away": 5}`)
	request(t, server, "POST", "/api/games", `{"home": 2, "away": 6}`)

	response := request(t, server, "DELETE", "/api/games", "")
	if response.Code != http.StatusNoContent {
		t.Fatalf("DELETE all returned %d: %s", response.Code, response.Body)
	}
	if games := listGames(t, server); len(games) != 0 {
		t.Fatalf("expected empty list after delete all, got %d games", len(games))
	}
}

func TestValidation(t *testing.T) {
	server := testServer(t)

	invalid := []string{
		`{"home": 3, "away": 3}`,                  // identical groups
		`{"home": 0, "away": 5}`,                  // group out of range
		`{"home": 1, "away": 9}`,                  // group out of range
		`{"home": 1, "away": 5, "homeScore": -1}`, // negative score
		`not json`,
	}

	for _, body := range invalid {
		if response := request(t, server, "POST", "/api/games", body); response.Code != http.StatusBadRequest {
			t.Errorf("POST %s returned %d, expected 400", body, response.Code)
		}
	}

	if games := listGames(t, server); len(games) != 0 {
		t.Fatalf("invalid games were stored: %d", len(games))
	}

	if response := request(t, server, "DELETE", "/api/games/999", ""); response.Code != http.StatusNotFound {
		t.Errorf("DELETE of unknown id returned %d, expected 404", response.Code)
	}
	if response := request(t, server, "DELETE", "/api/games/abc", ""); response.Code != http.StatusBadRequest {
		t.Errorf("DELETE with invalid id returned %d, expected 400", response.Code)
	}
}
