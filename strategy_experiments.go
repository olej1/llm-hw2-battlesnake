package main

import (
	"context"
	"slices"
)

type strategyOptions struct {
	constrictorRollout bool
	duelDepth          int
	duelRadius         int
	phaseWeights       int
	tailSafety         bool
	standardEndgame    bool
}

func tacticalMove(ctx context.Context, mode Mode, request MoveRequest, fallback string, options strategyOptions) string {
	if len(request.Board.Snakes) != 2 || (mode != Duels && !(mode == Standard && options.standardEndgame)) {
		return fallback
	}
	radius := 0
	if mode == Duels {
		radius = options.duelRadius
	}
	depth := duelDepthForPosition(request, tacticalDepth(mode, options.duelDepth), radius)
	if depth <= 3 {
		return searchDuelAtDepth(ctx, request, fallback, depth)
	}
	return searchDeepDuel(ctx, request, fallback, depth)
}

func tacticalDepth(mode Mode, depth int) int {
	if mode == Standard {
		return min(depth, 4)
	}
	return depth
}

func phaseAdjustedWeights(base weights, variant, turn, ours, theirs int) weights {
	if variant == 0 || theirs == 0 {
		return base
	}
	switch {
	case ours <= theirs && turn < 55:
		base.meal *= 1.25 + 0.20*float64(variant-1)
		base.territory *= 0.95
	case ours >= theirs+3:
		base.territory *= 1.10
		base.meal *= 0.92
	case turn >= 80 && ours >= 12:
		base.escape *= 1.15
	}
	return base
}

func hasTailEscape(board Board, you Snake, first Point, horizon int) bool {
	release := make([]int, board.Width*board.Height)
	for _, snake := range board.Snakes {
		if snake.ID == you.ID {
			continue
		}
		for i, point := range snake.Body {
			if insideDuelBoard(board, point) {
				cell := point.Y*board.Width + point.X
				release[cell] = max(release[cell], len(snake.Body)-i)
			}
		}
	}

	remaining := 160
	var visit func(Snake, Point, []Point, int) bool
	visit = func(snake Snake, head Point, food []Point, turn int) bool {
		if !insideDuelBoard(board, head) || release[head.Y*board.Width+head.X] > turn ||
			slices.Contains(snake.Body[:len(snake.Body)-1], head) {
			return false
		}
		meal := slices.Index(food, head)
		snake = advanceDuelSnake(snake, head, meal >= 0)
		if snake.Health <= 0 {
			return false
		}
		if turn == horizon || remaining <= 0 {
			return true
		}
		remaining--
		if meal >= 0 {
			food = append(slices.Clone(food[:meal]), food[meal+1:]...)
		}
		for _, dir := range directions {
			if visit(snake, nextDuelHead(snake, dir), food, turn+1) {
				return true
			}
		}
		return false
	}
	return visit(you, first, board.Food, 1)
}
