package main

import (
	"strings"
	"testing"
)

func TestSummarizeReplay(t *testing.T) {
	const opening = `{"id":"match"}
{"turn":0,"board":{"snakes":[{"id":"a","name":"candidate","latency":"0"},{"id":"b","name":"opponent-1"},{"id":"c","name":"opponent-2"}]}}
{"turn":1,"board":{"snakes":[{"id":"a","name":"candidate","latency":"7"},{"id":"b","name":"opponent-1"}]}}
`
	tests := []struct {
		name, finalFrame, result, outcome string
		place                             int
	}{
		{"win", `[{"id":"a","latency":"2"}]`, `{"winnerId":"a","isDraw":false}`, "win", 1},
		{"loss", `[{"id":"b"}]`, `{"winnerId":"b","isDraw":false}`, "loss", 2},
		{"draw", `[]`, `{"winnerId":"","isDraw":true}`, "draw", 1},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			replay := opening + `{"turn":2,"board":{"snakes":` + test.finalFrame + `}}` + "\n" + test.result + "\n"
			result, err := summarizeReplay(strings.NewReader(replay), "")
			if err != nil || result.Outcome != test.outcome || result.Place != test.place || result.MaxLatencyMS != 7 {
				t.Fatalf("got %+v, %v", result, err)
			}
		})
	}
	t.Run("missing result", func(t *testing.T) {
		if _, err := summarizeReplay(strings.NewReader(opening), "a"); err == nil {
			t.Fatal("an unfinished replay must fail")
		}
	})
	t.Run("eliminated before final draw", func(t *testing.T) {
		const replay = `{"id":"game"}
{"turn":0,"board":{"snakes":[{"id":"a"},{"id":"b"},{"id":"c"}]}}
{"turn":1,"board":{"snakes":[{"id":"b"},{"id":"c"}]}}
{"turn":2,"board":{"snakes":[]}}
{"winnerId":"","isDraw":true}
`
		result, err := summarizeReplay(strings.NewReader(replay), "a")
		if err != nil || result.Outcome != "loss" || result.Place != 3 {
			t.Fatalf("got %+v, %v", result, err)
		}
	})
}

func TestEliminationCause(t *testing.T) {
	log := "opponent-1 B: Health: 31, Eliminated: snake-collision, Turn: 12\ncandidate A: Health: 0, Eliminated: out-of-health, Turn: 20\n"
	if got := eliminationCause(log); got != "out-of-health" {
		t.Fatalf("got %q, want out-of-health", got)
	}
}

func TestConstrictorUsesFoodlessMap(t *testing.T) {
	game, board := seriesRules("constrictor")
	if game != "constrictor" || board != "empty" {
		t.Fatalf("got game %q and map %q, want constrictor rules on the empty map", game, board)
	}
}

func TestRejectsOutcomeWithMissingTerminalFrame(t *testing.T) {
	const replay = `{"id":"match"}
{"turn":0,"board":{"snakes":[{"id":"a"},{"id":"b"}]}}
{"turn":1,"board":{"snakes":[{"id":"a"},{"id":"b"}]}}
`
	for _, result := range []string{`{"winnerId":"a","isDraw":false}`, `{"winnerId":"","isDraw":true}`} {
		if _, err := summarizeReplay(strings.NewReader(replay+result), "a"); err == nil {
			t.Errorf("accepted %s even though two snakes were still alive", result)
		}
	}
}
