package main

import (
	"context"
	"slices"
)

type duelOutcome int

const (
	duelLoss duelOutcome = iota
	duelDraw
	duelContinues
	duelWin
)

type duelTurn struct {
	board  Board
	snakes [2]Snake
	alive  [2]bool
}

func isStandardDuel(request MoveRequest) bool {
	ruleset := request.Game.Ruleset.Name
	boardMap := request.Game.Map
	return len(request.Board.Hazards) == 0 &&
		(ruleset == "" || ruleset == "standard") &&
		(boardMap == "" || boardMap == "standard")
}

// Deeper positions get extra search only inside the configured radius.
func duelDepthForPosition(request MoveRequest, maxDepth, deepRadius int) int {
	depth := min(maxDepth, 2)
	for _, opponent := range request.Board.Snakes {
		if opponent.ID == request.You.ID {
			continue
		}
		head := request.You.Head
		distance := max(head.X-opponent.Head.X, opponent.Head.X-head.X) +
			max(head.Y-opponent.Head.Y, opponent.Head.Y-head.Y)
		if maxDepth > 3 && deepRadius > 0 {
			if distance <= deepRadius {
				depth = maxDepth
			} else if distance <= 6 {
				depth = 3
			}
		} else if distance <= 2*maxDepth {
			depth = maxDepth
		}
	}
	return depth
}

func searchDuelAtDepth(ctx context.Context, request MoveRequest, fallback string, depth int) string {
	snakes, eligible := duelSnakes(request)
	if !eligible || ctx.Err() != nil || depth < 2 {
		return fallback
	}
	// The standard map spawns food away from both heads. Keep the exact
	// tactical horizon at three and avoid simulated starvation.
	depth = min(depth, 3, snakes[0].Health-1, snakes[1].Health-1)

	best := fallback
	for turns := 2; turns <= depth; turns++ {
		move, outcome := bestDuelMove(ctx, request.Board, snakes, best, turns)
		if ctx.Err() != nil {
			break
		}
		best = move
		if outcome == duelWin {
			break
		}
	}
	return best
}

func duelSnakes(request MoveRequest) ([2]Snake, bool) {
	snakes := [2]Snake{request.You}
	if len(request.Board.Snakes) != 2 || !isStandardDuel(request) {
		return snakes, false
	}
	for _, snake := range request.Board.Snakes {
		if snake.ID != request.You.ID {
			snakes[1] = snake
		}
	}
	for _, snake := range snakes {
		if snake.ID == "" || len(snake.Body) == 0 {
			return snakes, false
		}
		for _, part := range snake.Body {
			if !insideDuelBoard(request.Board, part) {
				return snakes, false
			}
		}
	}
	return snakes, true
}

func bestDuelMove(ctx context.Context, board Board, snakes [2]Snake, fallback string, turns int) (string, duelOutcome) {
	best := fallback
	bestOutcome := duelLoss
	for _, move := range directions {
		if move.name == fallback {
			bestOutcome = duelMoveOutcome(ctx, board, snakes, nextDuelHead(snakes[0], move), turns)
			break
		}
	}
	for _, move := range directions {
		if bestOutcome == duelWin || ctx.Err() != nil {
			break
		}
		if move.name == fallback {
			continue
		}
		head := nextDuelHead(snakes[0], move)
		if !insideDuelBoard(board, head) {
			continue
		}
		if outcome := duelMoveOutcome(ctx, board, snakes, head, turns); outcome > bestOutcome {
			best, bestOutcome = move.name, outcome
		}
	}
	return best, bestOutcome
}

func duelMoveOutcome(ctx context.Context, board Board, snakes [2]Snake, ourHead Point, turns int) duelOutcome {
	worst := duelWin
	for _, enemyMove := range directions {
		if ctx.Err() != nil {
			return duelLoss
		}
		heads := [2]Point{ourHead, nextDuelHead(snakes[1], enemyMove)}
		turn := resolveDuelTurn(board, snakes, heads)
		outcome := turn.outcome()
		if outcome == duelContinues && turns > 1 {
			outcome = duelLoss
			for _, ourMove := range directions {
				reply := duelMoveOutcome(ctx, turn.board, turn.snakes, nextDuelHead(turn.snakes[0], ourMove), turns-1)
				outcome = max(outcome, reply)
				if outcome >= worst {
					break
				}
			}
		}
		worst = min(worst, outcome)
		if worst == duelLoss {
			break
		}
	}
	return worst
}

func resolveDuelTurn(board Board, snakes [2]Snake, heads [2]Point) duelTurn {
	turn := duelTurn{board: board}
	var fed [2]bool
	for i, snake := range snakes {
		fed[i] = slices.Contains(board.Food, heads[i])
		snake = advanceDuelSnake(snake, heads[i], fed[i])
		turn.snakes[i] = snake
		turn.alive[i] = insideDuelBoard(board, snake.Head) && snake.Health > 0
	}
	if fed[0] || fed[1] {
		turn.board.Food = make([]Point, 0, len(board.Food))
		for _, food := range board.Food {
			if food != heads[0] && food != heads[1] {
				turn.board.Food = append(turn.board.Food, food)
			}
		}
	}

	// Both snakes move before collision resolution. Snakes already
	// eliminated by health or a wall do not participate in collisions.
	present := turn.alive
	for i, snake := range turn.snakes {
		if !present[i] {
			continue
		}
		other := turn.snakes[1-i]
		bodyCollision := slices.Contains(snake.Body[1:], snake.Head) ||
			(present[1-i] && slices.Contains(other.Body[1:], snake.Head))
		headCollision := present[1-i] && snake.Head == other.Head && len(snake.Body) <= len(other.Body)
		turn.alive[i] = !bodyCollision && !headCollision
	}
	return turn
}

func advanceDuelSnake(snake Snake, head Point, eat bool) Snake {
	length := len(snake.Body)
	body := make([]Point, length, length+1)
	body[0] = head
	copy(body[1:], snake.Body[:length-1])
	snake.Health--
	if eat {
		body = append(body, body[length-1])
		snake.Health = 100
	}
	snake.Head, snake.Body = head, body
	return snake
}

func (turn duelTurn) outcome() duelOutcome {
	switch turn.alive {
	case [2]bool{true, true}:
		return duelContinues
	case [2]bool{true, false}:
		return duelWin
	case [2]bool{false, false}:
		return duelDraw
	default:
		return duelLoss
	}
}

func nextDuelHead(snake Snake, move direction) Point {
	return Point{snake.Head.X + move.dx, snake.Head.Y + move.dy}
}

func insideDuelBoard(board Board, point Point) bool {
	return point.X >= 0 && point.X < board.Width && point.Y >= 0 && point.Y < board.Height
}
