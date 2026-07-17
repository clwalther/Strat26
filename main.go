package main

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strconv"

	_ "modernc.org/sqlite"
)

type Game struct {
	ID        int64 `json:"id"`
	Home      int   `json:"home"`
	Away      int   `json:"away"`
	HomeScore *int  `json:"homeScore"`
	AwayScore *int  `json:"awayScore"`
}

const schema = `
CREATE TABLE IF NOT EXISTS games (
	id         INTEGER PRIMARY KEY AUTOINCREMENT,
	home       INTEGER NOT NULL,
	away       INTEGER NOT NULL,
	home_score INTEGER,
	away_score INTEGER,
	created_at TEXT NOT NULL DEFAULT (datetime('now')),

	CHECK (home BETWEEN 1 AND 8),
	CHECK (away BETWEEN 1 AND 8),
	CHECK (away <> home),
	CHECK (home_score IS NULL OR home_score >= 0),
	CHECK (away_score IS NULL OR away_score >= 0)
);`

// validate mirrors the schema checks so invalid games are rejected with a
// helpful message instead of a bare constraint error.
func validate(game Game) string {
	switch {
	case game.Home < 1 || game.Home > 8 || game.Away < 1 || game.Away > 8:
		return "group must be between 1 and 8"
	case game.Home == game.Away:
		return "home and away group must differ"
	case game.HomeScore != nil && *game.HomeScore < 0,
		game.AwayScore != nil && *game.AwayScore < 0:
		return "scores must not be negative"
	}
	return ""
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if err := json.NewEncoder(w).Encode(value); err != nil {
		log.Println(err)
	}
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func newServer(db *sql.DB) http.Handler {
	mux := http.NewServeMux()

	mux.Handle("GET /", http.FileServer(http.Dir("./source")))

	mux.HandleFunc("GET /api/games", func(w http.ResponseWriter, r *http.Request) {
		rows, err := db.Query(
			`SELECT id, home, away, home_score, away_score FROM games ORDER BY id`)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		defer rows.Close()

		games := []Game{}

		for rows.Next() {
			var game Game
			var homeScore, awayScore sql.NullInt64

			if err := rows.Scan(&game.ID, &game.Home, &game.Away,
				&homeScore, &awayScore); err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}

			if homeScore.Valid {
				score := int(homeScore.Int64)
				game.HomeScore = &score
			}
			if awayScore.Valid {
				score := int(awayScore.Int64)
				game.AwayScore = &score
			}

			games = append(games, game)
		}

		if err := rows.Err(); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}

		writeJSON(w, http.StatusOK, games)
	})

	mux.HandleFunc("POST /api/games", func(w http.ResponseWriter, r *http.Request) {
		var game Game

		if err := json.NewDecoder(r.Body).Decode(&game); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
			return
		}

		if message := validate(game); message != "" {
			writeError(w, http.StatusBadRequest, message)
			return
		}

		result, err := db.Exec(
			`INSERT INTO games (home, away, home_score, away_score) VALUES (?, ?, ?, ?)`,
			game.Home, game.Away, game.HomeScore, game.AwayScore)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}

		game.ID, err = result.LastInsertId()
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}

		writeJSON(w, http.StatusCreated, game)
	})

	mux.HandleFunc("DELETE /api/games/{id}", func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid id")
			return
		}

		result, err := db.Exec(`DELETE FROM games WHERE id = ?`, id)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}

		if affected, _ := result.RowsAffected(); affected == 0 {
			writeError(w, http.StatusNotFound, "no such game")
			return
		}

		w.WriteHeader(http.StatusNoContent)
	})

	mux.HandleFunc("DELETE /api/games", func(w http.ResponseWriter, r *http.Request) {
		if _, err := db.Exec(`DELETE FROM games`); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}

		w.WriteHeader(http.StatusNoContent)
	})

	return mux
}

func openDatabase(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}

	// sqlite handles one writer at a time; a single connection avoids
	// "database is locked" errors under concurrent requests
	db.SetMaxOpenConns(1)

	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, err
	}

	return db, nil
}

func main() {
	// init database
	if err := os.MkdirAll("./database", 0o755); err != nil {
		log.Fatalln(err)
	}

	db, err := openDatabase("./database/database.db")
	if err != nil {
		log.Fatalln(err)
	}
	defer db.Close()

	// start server
	log.Println("Strat26 running on http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", newServer(db)))
}
