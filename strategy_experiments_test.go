package main

import (
	"context"
	"testing"
)

func TestDeepLeafValuesLengthWhenSpaceIsSymmetric(t *testing.T) {
	us := testSnake("us", 90, Point{1, 1}, Point{0, 1}, Point{0, 0})
	them := testSnake("them", 90, Point{9, 9}, Point{10, 9})
	board := strategyRequest(us, them).Board
	if score := deepLeaf(board, [2]Snake{us, them}); score <= 0 {
		t.Fatalf("got %d, want positive score for longer snake", score)
	}
	if score := deepLeaf(board, [2]Snake{them, us}); score >= 0 {
		t.Fatalf("got %d, want negative score for shorter snake", score)
	}
}

func TestPhaseWeightsIncreaseFoodPriorityWhileBehind(t *testing.T) {
	base := profile(Duels)
	behind := phaseAdjustedWeights(base, 1, 10, 3, 5)
	ahead := phaseAdjustedWeights(base, 1, 10, 6, 4)
	if behind.meal <= base.meal || behind.meal <= ahead.meal {
		t.Fatalf("food weight behind=%v, ahead=%v, base=%v", behind.meal, ahead.meal, base.meal)
	}
	same := phaseAdjustedWeights(base, 0, 10, 3, 5)
	if same != base {
		t.Fatal("disabled phase weights changed the baseline")
	}
}

func TestTailEscapeDetectsGrowthTrap(t *testing.T) {
	us := testSnake("us", 90, Point{0, 0}, Point{0, 1}, Point{1, 1})
	request := strategyRequest(us)
	request.Board.Width, request.Board.Height = 2, 2
	start := Point{1, 0}
	if !hasTailEscape(request.Board, us, start, 5) {
		t.Fatal("a single loop around the square must be possible with a vacating tail")
	}
	request.Board.Food = []Point{{1, 0}, {1, 1}}
	if hasTailEscape(request.Board, us, start, 5) {
		t.Fatal("two meals leave no escape before the duplicated tail can move")
	}
}

func TestStandardUsesSearchOnlyWithOneRemainingOpponent(t *testing.T) {
	us := testSnake("us", 90, Point{2, 2}, Point{2, 3}, Point{3, 3}, Point{4, 3}, Point{4, 4}, Point{3, 4})
	them := testSnake("them", 90, Point{4, 2}, Point{4, 1}, Point{3, 1})
	request := strategyRequest(us, them)
	request.Board.Width, request.Board.Height = 5, 5
	opts := strategyOptions{duelDepth: 3, standardEndgame: true}
	if move := tacticalMove(context.Background(), Standard, request, "left", opts); move != "right" {
		t.Fatalf("got %s, want the forced win to the right", move)
	}
	request.Board.Snakes = append(request.Board.Snakes, testSnake("third", 85, Point{0, 0}))
	if move := tacticalMove(context.Background(), Standard, request, "left", opts); move != "left" {
		t.Fatalf("search must not run with three snakes, got %s", move)
	}
}

func TestStandardEndgameCapsLookahead(t *testing.T) {
	if got := tacticalDepth(Standard, 6); got != 4 {
		t.Fatalf("Standard used %d, want its validated four-turn setting", got)
	}
	if got := tacticalDepth(Duels, 6); got != 6 {
		t.Fatalf("Duels used %d, want its own configured setting", got)
	}
}

func TestDeepSearchOnlyRunsAtCloseRange(t *testing.T) {
	us := testSnake("us", 90, Point{2, 2})
	them := testSnake("them", 90, Point{7, 2})
	request := strategyRequest(us, them)
	if got := duelDepthForPosition(request, 5, 4); got != 3 {
		t.Fatalf("distance five used depth %d, want three", got)
	}
	request.Board.Snakes[1] = testSnake("them", 90, Point{6, 2})
	if got := duelDepthForPosition(request, 5, 4); got != 5 {
		t.Fatalf("distance four used depth %d, want five", got)
	}
}
