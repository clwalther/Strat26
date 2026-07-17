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
	mux.HandleFunc("GET /api/rules", app.handleRules)
	mux.HandleFunc("POST /api/actions/sponsor", app.handleSponsor)
	mux.HandleFunc("POST /api/actions/train", app.handleTraining)
	mux.HandleFunc("POST /api/actions/dope", app.handleDoping)
	mux.HandleFunc("POST /api/actions/inspect", app.handleInspection)
	mux.HandleFunc("POST /api/match/substitute", app.handleSubstitution)
	mux.HandleFunc("POST /api/match/advance", app.handleAdvance)
	mux.HandleFunc("PATCH /api/players/{id}/group", app.handleAssignGroup)
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
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "station": "zentrale-coach"})
}

func (app *application) handleState(w http.ResponseWriter, _ *http.Request) {
	app.writeState(w, http.StatusOK)
}

func (app *application) handleRules(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, gameRules)
}

func (app *application) handleSponsor(w http.ResponseWriter, r *http.Request) {
	var input teamRequest
	if !app.decode(w, r, &input) {
		return
	}
	if err := collectSponsor(app.db, input.TeamID); err != nil {
		app.writeDomainError(w, err)
		return
	}
	app.writeState(w, http.StatusOK)
}

func (app *application) handleTraining(w http.ResponseWriter, r *http.Request) {
	var input playerActionRequest
	if !app.decode(w, r, &input) {
		return
	}
	if err := trainPlayer(app.db, input); err != nil {
		app.writeDomainError(w, err)
		return
	}
	app.writeState(w, http.StatusOK)
}

func (app *application) handleDoping(w http.ResponseWriter, r *http.Request) {
	var input playerActionRequest
	if !app.decode(w, r, &input) {
		return
	}
	if err := dopePlayer(app.db, input); err != nil {
		app.writeDomainError(w, err)
		return
	}
	app.writeState(w, http.StatusOK)
}

func (app *application) handleInspection(w http.ResponseWriter, r *http.Request) {
	var input inspectionRequest
	if !app.decode(w, r, &input) {
		return
	}
	if err := inspectGroup(app.db, input); err != nil {
		app.writeDomainError(w, err)
		return
	}
	app.writeState(w, http.StatusOK)
}

func (app *application) handleSubstitution(w http.ResponseWriter, r *http.Request) {
	var input substitutionRequest
	if !app.decode(w, r, &input) {
		return
	}
	if err := substitutePlayer(app.db, input); err != nil {
		app.writeDomainError(w, err)
		return
	}
	app.writeState(w, http.StatusOK)
}

func (app *application) handleAdvance(w http.ResponseWriter, r *http.Request) {
	input := advanceRequest{Minutes: gameRules.StepMinutes}
	if !app.decodeOptional(w, r, &input) {
		return
	}
	if err := advanceCoachMatch(app.db, input); err != nil {
		app.writeDomainError(w, err)
		return
	}
	app.writeState(w, http.StatusOK)
}

func (app *application) handleAssignGroup(w http.ResponseWriter, r *http.Request) {
	playerID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || playerID < 1 {
		writeError(w, http.StatusBadRequest, "invalid player id")
		return
	}
	var input assignGroupRequest
	if !app.decode(w, r, &input) {
		return
	}
	if err := assignCardToGroup(app.db, playerID, input.GroupID); err != nil {
		app.writeDomainError(w, err)
		return
	}
	app.writeState(w, http.StatusOK)
}

func (app *application) handleReset(w http.ResponseWriter, _ *http.Request) {
	if err := resetCoachGame(app.db); err != nil {
		app.internalError(w, err)
		return
	}
	app.writeState(w, http.StatusOK)
}

func (app *application) writeState(w http.ResponseWriter, status int) {
	state, err := loadCoachState(app.db)
	if err != nil {
		app.internalError(w, err)
		return
	}
	writeJSON(w, status, state)
}

func (app *application) decode(w http.ResponseWriter, r *http.Request, destination any) bool {
	if err := decodeJSON(w, r, destination); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return false
	}
	return true
}

func (app *application) decodeOptional(w http.ResponseWriter, r *http.Request, destination any) bool {
	if r.Body == nil || r.ContentLength == 0 {
		return true
	}
	return app.decode(w, r, destination)
}

func (app *application) writeDomainError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, sql.ErrNoRows):
		writeError(w, http.StatusNotFound, "resource not found")
	case strings.Contains(err.Error(), "not enough hymns"):
		writeError(w, http.StatusConflict, err.Error())
	default:
		writeError(w, http.StatusBadRequest, err.Error())
	}
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
		shutdownContext, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownContext); err != nil {
			log.Printf("shutdown: %v", err)
		}
	}()

	log.Printf("Strat26 Coach-Zentrale running on http://localhost:%s", port)
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
