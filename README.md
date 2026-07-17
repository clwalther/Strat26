# Strat26

Spielplan-App für Team Grün und Team Blau (Gruppen 1–8): Spiele anlegen,
Ergebnisse festhalten und über die Seitenleiste nach Gruppe filtern.

## Entwicklung

```sh
go run main.go
```

Die App läuft danach auf <http://localhost:8080>. Die Spiele werden in einer
SQLite-Datenbank unter `database/database.db` gespeichert (wird beim ersten
Start automatisch angelegt).

Zum Testen:

```sh
go test ./...
```

## API

| Methode  | Pfad              | Beschreibung                          |
| -------- | ----------------- | ------------------------------------- |
| `GET`    | `/api/games`      | Alle Spiele als JSON                  |
| `POST`   | `/api/games`      | Spiel anlegen (siehe Beispiel unten)  |
| `DELETE` | `/api/games/{id}` | Ein Spiel löschen                     |
| `DELETE` | `/api/games`      | Alle Spiele löschen                   |

```sh
curl -X POST localhost:8080/api/games \
	-d '{"home": 2, "away": 6, "homeScore": 3, "awayScore": 1}'
```

`homeScore`/`awayScore` sind optional (Spiel ohne Ergebnis = geplant).
Gültige Gruppen sind 1–8, Heim- und Gastgruppe müssen sich unterscheiden.

## Deployment ohne Server

Ist die API nicht erreichbar — zum Beispiel beim statischen Deployment des
`source/`-Verzeichnisses auf Cloudflare Workers — speichert das Frontend die
Spiele stattdessen im `localStorage` des Browsers.
