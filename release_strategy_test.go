package main

import (
	"context"
	"encoding/json"
	"os"
	"testing"
)

func TestReleaseStrategyReplays(t *testing.T) {
	for _, example := range []struct {
		fixture string
		mode    Mode
		want    string
	}{
		{"duels-protect-lead", Duels, "right"},
		{"constrictor-contested-passage", Constrictor, "up"},
	} {
		t.Run(example.fixture, func(t *testing.T) {
			data, err := os.ReadFile("testdata/" + example.fixture + ".json")
			if err != nil {
				t.Fatal(err)
			}
			var request MoveRequest
			if err := json.Unmarshal(data, &request); err != nil {
				t.Fatal(err)
			}
			var got string
			if example.mode == Duels {
				got = protectDuelLead(context.Background(), request, "down")
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				if fallback := protectDuelLead(ctx, request, "down"); fallback != "down" {
					t.Fatalf("got %s on cancellation, want down", fallback)
				}
			} else {
				got = chooseMove(example.mode, request)
			}
			if got != example.want {
				t.Fatalf("got %s, want %s", got, example.want)
			}
		})
	}
}
