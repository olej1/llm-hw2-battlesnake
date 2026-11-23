package main

import "context"

const deepWin = 10_000
const deepInfinity = 20_000

func searchDeepDuel(ctx context.Context, request MoveRequest, fallback string, depth int) string {
	best := searchDuelAtDepth(ctx, request, fallback, 3)
	snakes, eligible := duelSnakes(request)
	if !eligible || ctx.Err() != nil || request.Board.Width > 11 || request.Board.Height > 11 {
		return best
	}
	depth = min(depth, snakes[0].Health-1, snakes[1].Health-1)
	if depth < 4 {
		return best
	}
	for _, move := range directions {
		if move.name == best && duelMoveOutcome(ctx, request.Board, snakes, nextDuelHead(snakes[0], move), 3) == duelWin {
			return best
		}
	}
	for turns := 4; turns <= depth; turns++ {
		move := bestDeepMove(ctx, request.Board, snakes, best, turns)
		if ctx.Err() != nil {
			break
		}
		best = move
	}
	return best
}

func bestDeepMove(ctx context.Context, board Board, snakes [2]Snake, preferred string, turns int) string {
	best, value := preferred, -deepInfinity
	for _, dir := range directions {
		if dir.name == preferred {
			value = deepMoveValue(ctx, board, snakes, nextDuelHead(snakes[0], dir), turns, -deepInfinity, deepInfinity)
			break
		}
	}
	for _, dir := range directions {
		head := nextDuelHead(snakes[0], dir)
		if ctx.Err() != nil || value >= deepWin {
			break
		}
		if dir.name != preferred && insideDuelBoard(board, head) {
			if score := deepMoveValue(ctx, board, snakes, head, turns, value, deepInfinity); score > value {
				best, value = dir.name, score
			}
		}
	}
	return best
}

// Evaluate our committed move against every opponent reply before maximizing
// our next move. The repetition of max then min preserves simultaneous turns.
func deepMoveValue(ctx context.Context, board Board, snakes [2]Snake, head Point, turns, alpha, beta int) int {
	worst := deepInfinity
	for _, enemyMove := range directions {
		if ctx.Err() != nil {
			return 0
		}
		heads := [2]Point{head, nextDuelHead(snakes[1], enemyMove)}
		next := resolveDuelTurn(board, snakes, heads)
		var value int
		switch next.outcome() {
		case duelWin:
			value = deepWin + turns
		case duelLoss:
			value = -deepWin - turns
		case duelContinues:
			if turns == 1 {
				value = deepLeaf(next.board, next.snakes)
			} else {
				value = deepPositionValue(ctx, next, turns-1, alpha, min(beta, worst))
			}
		}
		worst = min(worst, value)
		if worst <= alpha {
			break
		}
	}
	return worst
}

func deepPositionValue(ctx context.Context, turn duelTurn, turns, alpha, beta int) int {
	best := -deepInfinity
	for _, dir := range directions {
		head := nextDuelHead(turn.snakes[0], dir)
		value := deepMoveValue(ctx, turn.board, turn.snakes, head, turns, max(alpha, best), beta)
		best = max(best, value)
		if ctx.Err() != nil || best >= beta {
			break
		}
	}
	return best
}

func deepLeaf(board Board, snakes [2]Snake) int {
	var blocked, food [121]bool
	for _, snake := range snakes {
		for _, p := range snake.Body[:len(snake.Body)-1] {
			blocked[p.Y*board.Width+p.X] = true
		}
	}
	for _, p := range board.Food {
		if insideDuelBoard(board, p) {
			food[p.Y*board.Width+p.X] = true
		}
	}

	score := [2]int{}
	for i, snake := range snakes {
		head := snake.Head.Y*board.Width + snake.Head.X
		var queue [25]int
		var seen [121]bool
		queue[0], seen[head] = head, true
		nearFood, exits, end := false, 0, 1
		for cursor := 0; cursor < end; cursor++ {
			cell := queue[cursor]
			nearFood = nearFood || food[cell]
			p := Point{cell % board.Width, cell / board.Width}
			for _, dir := range directions {
				next := Point{p.X + dir.dx, p.Y + dir.dy}
				if !insideDuelBoard(board, next) {
					continue
				}
				index := next.Y*board.Width + next.X
				if blocked[index] || seen[index] || end == len(queue) {
					continue
				}
				if cursor == 0 {
					exits++
				}
				queue[end], seen[index] = index, true
				end++
			}
		}
		score[i] = 14*len(snake.Body) + 3*end + 5*exits + snake.Health/8
		if nearFood {
			score[i] += 3 + (100-snake.Health)/10
		}
	}
	return score[0] - score[1]
}
