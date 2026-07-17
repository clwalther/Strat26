package main

type SimulationRules struct {
	Model                 string `json:"model"`
	BaseAttacksPerStep    int    `json:"baseAttacksPerStep"`
	ExtraAttackChance     int    `json:"extraAttackChance"`
	MinimumPossession     int    `json:"minimumPossession"`
	MaximumPossession     int    `json:"maximumPossession"`
	MinimumBuildUpChance  int    `json:"minimumBuildUpChance"`
	MaximumBuildUpChance  int    `json:"maximumBuildUpChance"`
	MinimumFinishChance   int    `json:"minimumFinishChance"`
	MaximumFinishChance   int    `json:"maximumFinishChance"`
	HomeAdvantage         int    `json:"homeAdvantage"`
}

type Rules struct {
	FieldPlayers     int `json:"fieldPlayers"`
	MaxSubstitutions int `json:"maxSubstitutions"`
	StepMinutes      int `json:"stepMinutes"`
	SponsorReward    int `json:"sponsorReward"`
	TrainingCost     int `json:"trainingCost"`
	TrainingGain     int `json:"trainingGain"`
	DopingCost       int `json:"dopingCost"`
	DopingGain       int `json:"dopingGain"`
	DopingRisk       int `json:"dopingRisk"`
	Simulation       SimulationRules `json:"simulation"`
}

var gameRules = Rules{
	FieldPlayers: 11, MaxSubstitutions: 5, StepMinutes: 5,
	SponsorReward: 4, TrainingCost: 2, TrainingGain: 2,
	DopingCost: 1, DopingGain: 3, DopingRisk: 30,
	Simulation: SimulationRules{
		Model: "possession-attack-finish-v2", BaseAttacksPerStep: 1, ExtraAttackChance: 35,
		MinimumPossession: 28, MaximumPossession: 72,
		MinimumBuildUpChance: 35, MaximumBuildUpChance: 78,
		MinimumFinishChance: 6, MaximumFinishChance: 32, HomeAdvantage: 2,
	},
}

type CoachTeam struct {
	ID           int64  `json:"id"`
	Name         string `json:"name"`
	Color        string `json:"color"`
	Hymns        int    `json:"hymns"`
	SponsorLevel int    `json:"sponsorLevel"`
}

type SquadGroup struct {
	ID            int64  `json:"id"`
	TeamID        int64  `json:"teamId"`
	Number        int    `json:"number"`
	Name          string `json:"name"`
	FIFAAttention int    `json:"fifaAttention"`
}

type PlayerCard struct {
	ID               int64  `json:"id"`
	TeamID           int64  `json:"teamId"`
	Number           int    `json:"number"`
	Name             string `json:"name"`
	Position         string `json:"position"`
	Attack           int    `json:"attack"`
	Defense          int    `json:"defense"`
	Fitness          int    `json:"fitness"`
	Morale           int    `json:"morale"`
	DopingLevel      int    `json:"dopingLevel"`
	Suspended        bool   `json:"suspended"`
	CustodianGroupID int64  `json:"custodianGroupId"`
}

type CoachMatch struct {
	ID                int64  `json:"id"`
	HomeTeamID        int64  `json:"homeTeamId"`
	AwayTeamID        int64  `json:"awayTeamId"`
	Phase             string `json:"phase"`
	Minute            int    `json:"minute"`
	HomeScore         int    `json:"homeScore"`
	AwayScore         int    `json:"awayScore"`
	HomeSubstitutions int    `json:"homeSubstitutions"`
	AwaySubstitutions int    `json:"awaySubstitutions"`
	Seed              int64  `json:"seed"`
}

type LineupEntry struct {
	PlayerID     int64  `json:"playerId"`
	TeamID       int64  `json:"teamId"`
	OnField      bool   `json:"onField"`
	Slot         string `json:"slot"`
	EnteredAt    int    `json:"enteredAt"`
	LeftAt       *int   `json:"leftAt"`
	PlayerNumber int    `json:"playerNumber"`
	PlayerName   string `json:"playerName"`
	Position     string `json:"position"`
}

type CoachEvent struct {
	ID       int64  `json:"id"`
	MatchID  int64  `json:"matchId"`
	Minute   int    `json:"minute"`
	Kind     string `json:"kind"`
	TeamID   *int64 `json:"teamId"`
	PlayerID *int64 `json:"playerId"`
	Text     string `json:"text"`
}

type CoachState struct {
	Rules   Rules         `json:"rules"`
	Teams   []CoachTeam   `json:"teams"`
	Groups  []SquadGroup  `json:"groups"`
	Players []PlayerCard  `json:"players"`
	Match   CoachMatch    `json:"match"`
	Lineup  []LineupEntry `json:"lineup"`
	Events  []CoachEvent  `json:"events"`
}

type teamRequest struct {
	TeamID int64 `json:"teamId"`
}

type playerActionRequest struct {
	TeamID   int64  `json:"teamId"`
	PlayerID int64  `json:"playerId"`
	Focus    string `json:"focus"`
}

type inspectionRequest struct {
	GroupID int64  `json:"groupId"`
	Seed    *int64 `json:"seed"`
}

type substitutionRequest struct {
	TeamID      int64 `json:"teamId"`
	OutPlayerID int64 `json:"outPlayerId"`
	InPlayerID  int64 `json:"inPlayerId"`
}

type advanceRequest struct {
	Minutes int    `json:"minutes"`
	Seed    *int64 `json:"seed"`
}

type assignGroupRequest struct {
	GroupID int64 `json:"groupId"`
}
