package main

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
)

type Mode string

const (
	Duels       Mode = "duels"
	Standard    Mode = "standard"
	Royale      Mode = "royale"
	Constrictor Mode = "constrictor"
)

type Point struct {
	X int `json:"x"`
	Y int `json:"y"`
}

type Snake struct {
	ID     string  `json:"id"`
	Health int     `json:"health"`
	Head   Point   `json:"head"`
	Body   []Point `json:"body"`
}

type Board struct {
	Width   int     `json:"width"`
	Height  int     `json:"height"`
	Food    []Point `json:"food"`
	Hazards []Point `json:"hazards"`
	Snakes  []Snake `json:"snakes"`
}

type RuleSettings struct {
	HazardDamagePerTurn *int `json:"hazardDamagePerTurn"`
}

type Ruleset struct {
	Name     string       `json:"name"`
	Settings RuleSettings `json:"settings"`
}

type Game struct {
	ID      string  `json:"id"`
	Map     string  `json:"map"`
	Ruleset Ruleset `json:"ruleset"`
	Timeout int     `json:"timeout"`
}

type MoveRequest struct {
	Game  Game  `json:"game"`
	Turn  int   `json:"turn"`
	Board Board `json:"board"`
	You   Snake `json:"you"`
}

func modeForGame(request MoveRequest) Mode {
	for _, value := range []string{request.Game.Ruleset.Name, request.Game.Map} {
		switch Mode(value) {
		case Royale, Constrictor:
			return Mode(value)
		}
	}
	if len(request.Board.Snakes) == 2 {
		return Duels
	}
	return Standard
}

func newHandler(choose func(Mode, MoveRequest) string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimRight(r.URL.Path, "/")
		var mode Mode
		for _, profile := range []Mode{Duels, Standard, Royale, Constrictor} {
			prefix := "/" + string(profile)
			if path == prefix || strings.HasPrefix(path, prefix+"/") {
				mode = profile
				path = strings.TrimPrefix(path, prefix)
				break
			}
		}

		var allowed string
		switch path {
		case "":
			allowed = http.MethodGet
		case "/start", "/move", "/end":
			allowed = http.MethodPost
		default:
			http.NotFound(w, r)
			return
		}
		if r.Method != allowed {
			w.Header().Set("Allow", allowed)
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		switch path {
		case "":
			writeJSON(w, map[string]string{
				"apiversion": "1",
				"color":      "#247d6e",
				"head":       "default",
				"tail":       "default",
				"version":    "0.1.0",
			})
		case "/start", "/end":
			w.WriteHeader(http.StatusOK)
		case "/move":
			decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 256<<10))
			var request MoveRequest
			if err := decoder.Decode(&request); err != nil ||
				decoder.Decode(new(any)) != io.EOF || !validMove(request) {
				http.Error(w, "invalid move request", http.StatusBadRequest)
				return
			}
			if mode == "" {
				mode = modeForGame(request)
			}
			writeJSON(w, map[string]string{"move": choose(mode, request)})
		}
	})
}

func validMove(request MoveRequest) bool {
	board, you := request.Board, request.You
	return board.Width > 0 && board.Height > 0 && board.Width <= 32 && board.Height <= 32 &&
		you.ID != "" && len(you.Body) > 0 && you.Body[0] == you.Head &&
		you.Head.X >= 0 && you.Head.X < board.Width && you.Head.Y >= 0 && you.Head.Y < board.Height
}

func writeJSON(w http.ResponseWriter, body map[string]string) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(body)
}
