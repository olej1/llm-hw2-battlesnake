package main

import (
	"context"
	"math/bits"
	"slices"
	"time"
)

const rolloutWidth = 11
const rolloutSize = rolloutWidth * rolloutWidth

type constrictorBits struct{ low, high uint64 }

var (
	rolloutCells                             [rolloutSize]constrictorBits
	rolloutNeighbors                         [rolloutSize][4]int
	rolloutAll, rolloutNoEast, rolloutNoWest constrictorBits
	rolloutColor                             [2]constrictorBits
)

func init() {
	for cell := range rolloutSize {
		x, y := cell%rolloutWidth, cell/rolloutWidth
		one := constrictorCell(cell)
		rolloutCells[cell] = one
		rolloutAll = rolloutAll.or(one)
		rolloutColor[(x+y)&1] = rolloutColor[(x+y)&1].or(one)
		if x < rolloutWidth-1 {
			rolloutNoEast = rolloutNoEast.or(one)
		}
		if x > 0 {
			rolloutNoWest = rolloutNoWest.or(one)
		}
		for i, d := range directions {
			nx, ny := x+d.dx, y+d.dy
			rolloutNeighbors[cell][i] = -1
			if nx >= 0 && nx < rolloutWidth && ny >= 0 && ny < rolloutWidth {
				rolloutNeighbors[cell][i] = ny*rolloutWidth + nx
			}
		}
	}
}

func constrictorCell(cell int) constrictorBits {
	if cell < 64 {
		return constrictorBits{low: 1 << cell}
	}
	return constrictorBits{high: 1 << (cell - 64)}
}
func (b constrictorBits) or(c constrictorBits) constrictorBits {
	return constrictorBits{b.low | c.low, b.high | c.high}
}
func (b constrictorBits) and(c constrictorBits) constrictorBits {
	return constrictorBits{b.low & c.low, b.high & c.high}
}
func (b constrictorBits) without(c constrictorBits) constrictorBits {
	return constrictorBits{b.low &^ c.low, b.high &^ c.high}
}
func (b constrictorBits) left(n uint) constrictorBits {
	return constrictorBits{b.low << n, b.high<<n | b.low>>(64-n)}
}
func (b constrictorBits) right(n uint) constrictorBits {
	return constrictorBits{b.low>>n | b.high<<(64-n), b.high >> n}
}
func (b constrictorBits) has(cell int) bool { return b.and(rolloutCells[cell]) != (constrictorBits{}) }
func (b constrictorBits) count() int        { return bits.OnesCount64(b.low) + bits.OnesCount64(b.high) }
func (b constrictorBits) spread() constrictorBits {
	return b.and(rolloutNoEast).left(1).or(b.and(rolloutNoWest).right(1)).or(b.left(rolloutWidth)).or(b.right(rolloutWidth))
}
func rolloutRoom(head int, free constrictorBits) constrictorBits {
	reach := rolloutCells[head]
	for {
		next := reach.or(reach.spread().and(free))
		if next == reach {
			return next
		}
		reach = next
	}
}

type constrictorSimulation struct {
	head   [4]int
	length [4]int
	body   [4]constrictorBits
}

func newConstrictorSimulation(request MoveRequest) (constrictorSimulation, bool) {
	game := constrictorSimulation{head: [4]int{-1, -1, -1, -1}}
	board := request.Board
	if board.Width != rolloutWidth || board.Height != rolloutWidth ||
		len(board.Snakes) < 2 || len(board.Snakes) > 4 || len(board.Hazards) > 0 || len(board.Food) > 0 {
		return game, false
	}
	var seats [4]Snake
	next := 1
	for _, snake := range board.Snakes {
		if snake.ID == request.You.ID {
			seats[0] = snake
			continue
		}
		if next == len(seats) {
			return game, false
		}
		seats[next] = snake
		next++
	}
	if len(seats[0].Body) == 0 {
		return game, false
	}
	// Give rollout policies a reproducible seat regardless of API ordering.
	slices.SortFunc(seats[1:next], func(a, b Snake) int {
		return rolloutIndex(a.Head) - rolloutIndex(b.Head)
	})
	for seat, snake := range seats[:next] {
		if len(snake.Body) == 0 || !insideDuelBoard(board, snake.Head) {
			return game, false
		}
		game.head[seat] = rolloutIndex(snake.Head)
		game.length[seat] = len(snake.Body)
		for _, segment := range snake.Body {
			if !insideDuelBoard(board, segment) {
				return game, false
			}
			game.body[seat] = game.body[seat].or(rolloutCells[rolloutIndex(segment)])
		}
	}
	return game, game.head[0] >= 0
}
func rolloutIndex(point Point) int {
	return point.Y*rolloutWidth + point.X
}

func (game *constrictorSimulation) occupied() constrictorBits {
	return game.body[0].or(game.body[1]).or(game.body[2]).or(game.body[3])
}

func (game *constrictorSimulation) advance(moves [4]int) {
	occupied := game.occupied()
	for i, cell := range moves {
		alive := game.head[i] >= 0 && cell >= 0 && !occupied.has(cell)
		if alive {
			for j, other := range moves {
				if j != i && cell == other && game.length[j] >= game.length[i] {
					alive = false
					break
				}
			}
		}
		if alive {
			game.body[i] = game.body[i].or(rolloutCells[cell])
		} else {
			cell = -1
			game.body[i] = constrictorBits{}
		}
		game.head[i] = cell
	}
	for i, head := range game.head {
		if head >= 0 {
			game.length[i]++
		}
	}
}

func (game *constrictorSimulation) threats(seat int, free constrictorBits) constrictorBits {
	var danger constrictorBits
	for i, head := range game.head {
		if head >= 0 && i != seat && game.length[i] >= game.length[seat] {
			danger = danger.or(rolloutCells[head].spread().and(free))
		}
	}
	return danger
}

func rolloutNoise(seed, turn, seat, cell int) int {
	x := uint32(seed*1597 + turn*5059 + seat*9337 + cell*7919)
	x = (x ^ (x >> 16)) * 0x7feb352d
	x = (x ^ (x >> 15)) * 0x846ca68b
	return int((x ^ (x >> 16)) % 11)
}

func (game *constrictorSimulation) choose(seat, seed, turn int, free constrictorBits) int {
	head := game.head[seat]
	if head < 0 {
		return -1
	}
	best, bestScore := -1, -100000
	danger := game.threats(seat, free)
	style := (seed + 3*seat) % 4
	for _, cell := range rolloutNeighbors[head] {
		if cell < 0 || !free.has(cell) {
			continue
		}
		room := rolloutRoom(cell, free)
		exits := rolloutCells[cell].spread().and(free).count()
		same := room.and(rolloutColor[(cell%rolloutWidth+cell/rolloutWidth)&1]).count()
		other := room.count() - same
		parityBound := 2*min(same, other) + max(0, min(1, same-other))
		score := parityBound*7 + rolloutNoise(seed, turn, seat, cell)
		switch style {
		case 0:
			score += exits * 12
		case 1:
			score -= exits * 10
		case 2:
			x, y := cell%rolloutWidth, cell/rolloutWidth
			score += exits*4 - 3*(max(x-5, 5-x)+max(y-5, 5-y))
		case 3:
			score += (free.without(room).count() / 4) - exits*4
		}
		if danger.has(cell) {
			score -= 2000
		}
		if score > bestScore {
			best, bestScore = cell, score
		}
	}
	return best
}

// play returns a score for finish position, plus a small survival tiebreak.
func (game constrictorSimulation) play(ctx context.Context, first, seed int) (int, bool) {
	for turn := 0; turn < rolloutSize*2; turn++ {
		if ctx.Err() != nil {
			return 0, false
		}
		free := rolloutAll.without(game.occupied())
		var moves [4]int
		for seat := range moves {
			if seat == 0 && turn == 0 {
				moves[seat] = first
			} else {
				moves[seat] = game.choose(seat, seed, turn, free)
			}
		}
		game.advance(moves)
		survivors := 0
		for _, head := range game.head[1:] {
			if head >= 0 {
				survivors++
			}
		}
		if game.head[0] < 0 {
			return 60 - 20*survivors + min(turn, 64)/8, true
		}
		if survivors == 0 {
			return 94, true
		}
	}
	return 0, false
}

func (state *position) constrictorRolloutMove(request MoveRequest, fallback string) string {
	game, ok := newConstrictorSimulation(request)
	if !ok {
		return fallback
	}
	baseIndex := -1
	for i, d := range directions {
		if d.name == fallback {
			baseIndex = i
		}
	}
	if baseIndex < 0 {
		return fallback
	}
	neighbors := rolloutNeighbors[game.head[0]]
	baseRisk := state.moveRisk(request.You, neighbors[baseIndex])
	if baseRisk > 1 {
		return fallback
	}
	var candidates [4]bool
	count := 0
	for i, cell := range neighbors {
		candidates[i] = state.moveRisk(request.You, cell) <= baseRisk
		if candidates[i] {
			count++
		}
	}
	if count < 2 {
		return fallback
	}

	ctx, cancel := context.WithTimeout(context.Background(), 24*time.Millisecond)
	defer cancel()
	var sums, worst [4]int
	for i := range worst {
		worst[i] = 100
	}
	completed := 0
rounds:
	for seed := 0; seed < 16; seed++ {
		var scores [4]int
		for i, accepted := range candidates {
			if !accepted {
				continue
			}
			score, ok := game.play(ctx, neighbors[i], seed)
			if !ok {
				break rounds
			}
			scores[i] = score
		}
		for i, accepted := range candidates {
			if accepted {
				sums[i] += scores[i]
				worst[i] = min(worst[i], scores[i])
			}
		}
		completed++
	}
	if completed < 4 {
		return fallback
	}
	best := baseIndex
	value := func(i int) int { return sums[i] + completed*worst[i]/5 }
	for i, accepted := range candidates {
		if accepted && value(i) > value(best) && value(i)-value(baseIndex) >= 3*completed {
			best = i
		}
	}
	return directions[best].name
}
