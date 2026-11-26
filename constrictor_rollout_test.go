package main

import (
	"encoding/json"
	"os"
	"testing"
)

func TestConstrictorRolloutKeepsTailAndEliminatesEqualHeads(t *testing.T) {
	game := constrictorSimulation{
		head: [4]int{0, 2, -1, -1}, length: [4]int{3, 3},
		body: [4]constrictorBits{constrictorCell(0), constrictorCell(2)},
	}
	game.advance([4]int{1, 1, -1, -1})
	if game.head[0] != -1 || game.head[1] != -1 || game.occupied().has(0) {
		t.Fatal("equal heads must both die and their bodies must disappear")
	}

	game = constrictorSimulation{
		head: [4]int{0, 2, -1, -1}, length: [4]int{4, 3},
		body: [4]constrictorBits{constrictorCell(0), constrictorCell(2)},
	}
	game.advance([4]int{1, 1, -1, -1})
	if game.head[0] != 1 || game.head[1] != -1 || !game.occupied().has(0) ||
		!game.occupied().has(1) || game.occupied().has(2) || game.length[0] != 5 {
		t.Fatal("the longer snake survives, grows, and retains its earlier cells")
	}
}

func TestConstrictorSimulationIgnoresOpponentRequestOrder(t *testing.T) {
	us := testSnake("us", 100, Point{5, 5})
	first := testSnake("one", 100, Point{1, 1})
	second := testSnake("two", 100, Point{9, 9})
	a, ok := newConstrictorSimulation(strategyRequest(us, first, second))
	b, reverseOK := newConstrictorSimulation(strategyRequest(us, second, first))
	if !ok || !reverseOK || a != b {
		t.Fatal("reordering opponents must leave the simulation unchanged")
	}
}

func TestConstrictorRolloutDoesNotWrapAcrossRows(t *testing.T) {
	left := constrictorCell(10)
	right := constrictorCell(11)
	if got := rolloutRoom(10, left.or(right)); got != left {
		t.Fatal("cells across a row boundary must not be adjacent")
	}
	a, b := constrictorCell(63), constrictorCell(64)
	if got := rolloutRoom(63, a.or(b)); got != a.or(b) {
		t.Fatal("adjacent cells across the machine-word boundary must be connected")
	}
}

func TestConstrictorRolloutFindsAlternativeFromLosingStart(t *testing.T) {
	data, err := os.ReadFile("testdata/constrictor-rollout-choice.json")
	if err != nil {
		t.Fatal(err)
	}
	var request MoveRequest
	if err := json.Unmarshal(data, &request); err != nil {
		t.Fatal(err)
	}
	if move := chooseMoveWithOptions(Constrictor, request, strategyOptions{}); move != "down" {
		t.Fatalf("baseline chose %s, want down", move)
	}
	if move := chooseMoveWithOptions(Constrictor, request, strategyOptions{constrictorRollout: true}); move != "up" {
		t.Fatalf("rollout chose %s, want up", move)
	}
}
