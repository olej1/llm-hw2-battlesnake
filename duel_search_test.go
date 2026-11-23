package main

import (
	"context"
	"testing"
)

func TestDuelSearchAvoidsBlockedExit(t *testing.T) {
	us := testSnake("us", 90, Point{1, 1}, Point{1, 0}, Point{2, 0}, Point{3, 0}, Point{4, 0})
	them := testSnake("them", 90, Point{4, 2}, Point{4, 3}, Point{3, 3}, Point{2, 3}, Point{2, 2}, Point{1, 2}, Point{0, 2})
	request := strategyRequest(us, them)
	request.Board.Width, request.Board.Height = 5, 4
	if move := searchDuelAtDepth(context.Background(), request, "right", 3); move != "left" {
		t.Fatalf("got %s, want left: the opponent can close the only exit to the right", move)
	}
}

func TestDuelSearchExpiredDeadlineKeepsFallback(t *testing.T) {
	us := testSnake("us", 90, Point{1, 1}, Point{1, 0}, Point{2, 0})
	them := testSnake("them", 90, Point{4, 2}, Point{4, 3}, Point{3, 3})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if move := searchDuelAtDepth(ctx, strategyRequest(us, them), "right", 3); move != "right" {
		t.Fatalf("got %s, want fallback after cancellation", move)
	}
}

func TestDuelTurnFoodGrowthAndHeadCollision(t *testing.T) {
	shorter := testSnake("short", 1, Point{1, 1}, Point{0, 1}, Point{0, 0})
	longer := testSnake("long", 1, Point{3, 1}, Point{4, 1}, Point{4, 0}, Point{3, 0})
	request := strategyRequest(shorter, longer)
	request.Board.Food = []Point{{2, 1}}
	result := resolveDuelTurn(request.Board, [2]Snake{shorter, longer}, [2]Point{{2, 1}, {2, 1}})
	if result.alive != [2]bool{false, true} || result.snakes[1].Health != 100 || len(result.snakes[1].Body) != 5 {
		t.Fatalf("unexpected result %+v", result)
	}
	if tail := result.snakes[1].Body[4]; tail != (Point{4, 0}) || result.snakes[1].Body[3] != tail {
		t.Fatalf("growing must duplicate the new tail, got %v", result.snakes[1].Body)
	}
}

func TestDuelSearchTakesForcedWin(t *testing.T) {
	us := testSnake("us", 90, Point{2, 2}, Point{2, 3}, Point{3, 3}, Point{4, 3}, Point{4, 4}, Point{3, 4})
	them := testSnake("them", 90, Point{4, 2}, Point{4, 1}, Point{3, 1})
	request := strategyRequest(us, them)
	request.Board.Width, request.Board.Height = 5, 5
	if move := searchDuelAtDepth(context.Background(), request, "left", 3); move != "right" {
		t.Fatalf("got %s, want right to cover the opponent's only exit", move)
	}
	for _, game := range []Game{
		{Ruleset: Ruleset{Name: "wrapped"}},
		{Ruleset: Ruleset{Name: "standard"}, Map: "hz_spiral"},
	} {
		request.Game = game
		if move := searchDuelAtDepth(context.Background(), request, "left", 3); move != "left" {
			t.Errorf("unexpected simulated move %s for ruleset %s, map %s", move, game.Ruleset.Name, game.Map)
		}
	}
}

func TestDuelSearchDoesNotDiscardCompletedIterationOnTimeout(t *testing.T) {
	us := testSnake("us", 90, Point{1, 1}, Point{1, 0}, Point{2, 0}, Point{3, 0}, Point{4, 0})
	them := testSnake("them", 90, Point{4, 2}, Point{4, 3}, Point{3, 3}, Point{2, 3}, Point{2, 2}, Point{1, 2}, Point{0, 2})
	request := strategyRequest(us, them)
	request.Board.Width, request.Board.Height = 5, 4
	ctx := &stopAfterChecks{Context: context.Background(), remaining: 125}
	if move := searchDuelAtDepth(ctx, request, "right", 3); move != "left" || ctx.remaining > 0 {
		t.Fatalf("got %s after %d checks remaining, want the completed two-turn move after timeout", move, ctx.remaining)
	}
}

type stopAfterChecks struct {
	context.Context
	remaining int
}

func (ctx *stopAfterChecks) Err() error {
	ctx.remaining--
	if ctx.remaining <= 0 {
		return context.DeadlineExceeded
	}
	return nil
}

func TestThirdTurnFindsTacticFromReplay(t *testing.T) {
	us := testSnake("us", 99, Point{1, 7}, Point{2, 7}, Point{2, 8}, Point{2, 9}, Point{3, 9}, Point{4, 9}, Point{5, 9})
	them := testSnake("them", 96, Point{1, 9}, Point{1, 10}, Point{2, 10}, Point{3, 10}, Point{4, 10}, Point{5, 10})
	request := strategyRequest(us, them)
	request.Board.Food = []Point{{0, 4}, {9, 3}}
	if move := searchDuelAtDepth(context.Background(), request, "down", 2); move != "down" {
		t.Fatalf("two-turn baseline changed to %s, want down", move)
	}
	if move := searchDuelAtDepth(context.Background(), request, "down", 3); move != "up" {
		t.Fatalf("three-turn search chose %s, want up", move)
	}
}
