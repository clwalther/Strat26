package main

type Team struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	ShortName string `json:"shortName"`
	Color     string `json:"color"`
	Rating    int    `json:"rating"`
}

type Season struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	Status    string `json:"status"`
	CreatedAt string `json:"createdAt"`
}

type Game struct {
	ID         int64   `json:"id"`
	SeasonID   int64   `json:"seasonId"`
	Round      int     `json:"round"`
	HomeTeamID int64   `json:"homeTeamId"`
	AwayTeamID int64   `json:"awayTeamId"`
	Kickoff    *string `json:"kickoff"`
	Status     string  `json:"status"`
	HomeScore  *int    `json:"homeScore"`
	AwayScore  *int    `json:"awayScore"`
	SimSeed    *int64  `json:"simulationSeed"`
	CreatedAt  string  `json:"createdAt"`
	UpdatedAt  string  `json:"updatedAt"`
}

type GameEvent struct {
	ID       int64  `json:"id"`
	GameID   int64  `json:"gameId"`
	Minute   int    `json:"minute"`
	Kind     string `json:"kind"`
	TeamID   int64  `json:"teamId"`
	Player   string `json:"player"`
	Detail   string `json:"detail"`
}

type GameDetails struct {
	Game   Game        `json:"game"`
	Events []GameEvent `json:"events"`
}

type Standing struct {
	Rank           int    `json:"rank"`
	TeamID         int64  `json:"teamId"`
	TeamName       string `json:"teamName"`
	ShortName      string `json:"shortName"`
	Color          string `json:"color"`
	Played         int    `json:"played"`
	Won            int    `json:"won"`
	Drawn          int    `json:"drawn"`
	Lost           int    `json:"lost"`
	GoalsFor       int    `json:"goalsFor"`
	GoalsAgainst   int    `json:"goalsAgainst"`
	GoalDifference int    `json:"goalDifference"`
	Points         int    `json:"points"`
}

type APIState struct {
	Season   Season     `json:"season"`
	Teams    []Team     `json:"teams"`
	Games    []Game     `json:"games"`
	Standing []Standing `json:"standings"`
}

type createGameRequest struct {
	SeasonID   int64   `json:"seasonId"`
	Round      int     `json:"round"`
	HomeTeamID int64   `json:"homeTeamId"`
	AwayTeamID int64   `json:"awayTeamId"`
	Home       int64   `json:"home"`
	Away       int64   `json:"away"`
	Kickoff    *string `json:"kickoff"`
	HomeScore  *int    `json:"homeScore"`
	AwayScore  *int    `json:"awayScore"`
}

type scoreRequest struct {
	HomeScore *int `json:"homeScore"`
	AwayScore *int `json:"awayScore"`
}

type scheduleRequest struct {
	Replace      bool   `json:"replace"`
	StartAt      string `json:"startAt"`
	IntervalDays int    `json:"intervalDays"`
}

type simulateRequest struct {
	Seed          *int64 `json:"seed"`
	IncludePlayed bool   `json:"includePlayed"`
}
