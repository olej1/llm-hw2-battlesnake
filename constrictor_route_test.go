package main

import (
	"encoding/json"
	"os"
	"testing"
)

func TestConstrictorChoosesLongerRouteThroughEqualSizedArea(t *testing.T) {
	body, err := os.ReadFile("testdata/constrictor-27101-slot-3-turn-17.json")
	if err != nil {
		t.Fatal(err)
	}
	var request MoveRequest
	if err := json.Unmarshal(body, &request); err != nil {
		t.Fatal(err)
	}
	if move := chooseMove(Constrictor, request); move != "right" {
		t.Fatalf("got %q, want the longer route to the right", move)
	}
}

func TestLongestConstrictorRoutes(t *testing.T) {
	const width, height = 5, 4
	state := newPosition(Constrictor, MoveRequest{Board: Board{Width: width, Height: height}})
	tests := []struct {
		name  string
		shape string
		want  int
	}{
		{"full rectangle", "S...................", 20},
		{"corridor", "S....###############", 5},
		{"two components", "..S..##.####.####.##", 4},
		{"two articulation points", "S..###...#.#..######", 7},
		{"parity-limited fork", "#..##S..###.########", 5},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			available := make([]bool, width*height)
			start := -1
			for i, cell := range test.shape {
				available[i] = cell != '#'
				if cell == 'S' {
					start = i
				}
			}
			lower, upper := state.longestConstrictorRoute(start, available, constrictorRouteNodes)
			if lower != test.want || upper != test.want {
				t.Fatalf("bounds [%d,%d], want [%d,%d]", lower, upper, test.want, test.want)
			}
			lower, upper = state.longestConstrictorRoute(start, available, 1)
			if lower > test.want || upper < test.want {
				t.Fatalf("bounds [%d,%d] must include %d", lower, upper, test.want)
			}
		})
	}
}
