package main

const constrictorRouteNodes = 384

type constrictorMove struct {
	dir, lower, upper int
}

func (state *position) constrictorRouteMove(you Snake, head int, fallback string, bestRisk int) string {
	var moves [4]constrictorMove
	count, fallbackIndex := 0, -1
	for dir, cell := range state.neighbors[head] {
		if state.moveRisk(you, cell) != bestRisk {
			continue
		}
		// An opponent can enter this branch while we are planning a route.
		if state.enemyRange[cell] != unavailable {
			return fallback
		}
		if directions[dir].name == fallback {
			fallbackIndex = count
		}
		moves[count].dir = dir
		count++
	}
	if count < 2 || fallbackIndex < 0 {
		return fallback
	}

	for i := range moves[:count] {
		move := &moves[i]
		cell := state.neighbors[head][move.dir]
		distances := state.distances([]int{cell}, head, nil, 0)
		available := make([]bool, len(distances))
		for point, arrival := range distances {
			available[point] = arrival != unavailable
		}
		move.lower, move.upper = state.longestConstrictorRoute(cell, available, constrictorRouteNodes)
	}

	best := fallbackIndex
	for i, move := range moves[:count] {
		if move.lower > moves[fallbackIndex].upper && move.lower > moves[best].lower {
			best = i
		}
	}
	return directions[moves[best].dir].name
}

// Route lengths include the first move. The search returns the longest
// route it found and an upper bound on every route it could not finish.
func (state *position) longestConstrictorRoute(start int, available []bool, budget int) (int, int) {
	solver := routeSolver{
		state:     state,
		available: available,
		marks:     make([]int, len(available)),
		queue:     make([]int, 0, len(available)),
		budget:    budget,
	}
	available[start] = false
	upper := solver.search(start, 1)
	available[start] = true
	return solver.best, max(solver.best, upper)
}

type routeSolver struct {
	state     *position
	available []bool
	marks     []int
	queue     []int
	mark      int
	budget    int
	best      int
}

func (solver *routeSolver) search(cell, length int) int {
	solver.best = max(solver.best, length)
	limit := length + solver.remainingBound(cell)
	if limit <= solver.best || solver.budget == 0 {
		return limit
	}
	solver.budget--

	var nextCells, degrees [4]int
	count := 0
	for _, next := range solver.state.neighbors[cell] {
		if next < 0 || !solver.available[next] {
			continue
		}
		degree := 0
		for _, adjacent := range solver.state.neighbors[next] {
			if adjacent >= 0 && solver.available[adjacent] {
				degree++
			}
		}
		// Earlier visits to low-degree cells preserve more continuation options.
		index := count
		for index > 0 && degrees[index-1] > degree {
			nextCells[index], degrees[index] = nextCells[index-1], degrees[index-1]
			index--
		}
		nextCells[index], degrees[index] = next, degree
		count++
	}

	upper := length
	for _, next := range nextCells[:count] {
		solver.available[next] = false
		upper = max(upper, solver.search(next, length+1))
		solver.available[next] = true
		if solver.best >= limit {
			return limit
		}
	}
	return upper
}

// The path must choose one connected component when leaving the current
// cell. Inside it, it must alternate checkerboard colors.
func (solver *routeSolver) remainingBound(cell int) int {
	solver.mark++
	mark := solver.mark
	width := solver.state.board.Width
	color := (cell%width + cell/width) & 1
	limit := 0
	for _, start := range solver.state.neighbors[cell] {
		if start < 0 || !solver.available[start] || solver.marks[start] == mark {
			continue
		}
		solver.queue = append(solver.queue[:0], start)
		solver.marks[start] = mark
		same, other := 0, 0
		for i := 0; i < len(solver.queue); i++ {
			point := solver.queue[i]
			if (point%width+point/width)&1 == color {
				same++
			} else {
				other++
			}
			for _, adjacent := range solver.state.neighbors[point] {
				if adjacent >= 0 && solver.available[adjacent] && solver.marks[adjacent] != mark {
					solver.marks[adjacent] = mark
					solver.queue = append(solver.queue, adjacent)
				}
			}
		}
		extra := 0
		if other > same {
			extra = 1
		}
		limit = max(limit, 2*min(same, other)+extra)
	}
	return limit
}

// avoidContestedPassage compares routes before the opponent can claim a cell.
func (state *position) avoidContestedPassage(r MoveRequest, fallback string) string {
	if len(r.Board.Snakes) < 2 || r.Turn < 5 {
		return fallback
	}
	current := -1
	for _, d := range directions {
		if d.name == fallback {
			current = state.index(nextDuelHead(r.You, d))
		}
	}
	if current < 0 || state.enemyRange[current] == unavailable {
		return fallback
	}
	risk := state.moveRisk(r.You, current)
	if risk > 1 {
		return fallback
	}
	baseline := state.constrictorBeforeCutoff(r.You, current)
	if baseline >= 3 {
		return fallback
	}
	best, bestHorizon, bestScore := fallback, baseline, -1e99
	for _, d := range directions {
		cell := state.index(nextDuelHead(r.You, d))
		if cell < 0 || cell == current || state.moveRisk(r.You, cell) > risk {
			continue
		}
		horizon := state.constrictorBeforeCutoff(r.You, cell)
		if horizon < baseline+2 {
			continue
		}
		score := state.score(r.You, cell)
		if horizon > bestHorizon || (horizon == bestHorizon && score > bestScore) {
			best, bestHorizon, bestScore = d.name, horizon, score
		}
	}
	return best
}

func (state *position) constrictorBeforeCutoff(you Snake, start int) int {
	visited := make([]bool, len(state.release))
	var walk func(int, int) int
	walk = func(cell, turn int) int {
		if state.release[cell] > 0 || visited[cell] || state.enemyRange[cell] < turn ||
			(state.enemyRange[cell] == turn && state.enemyLength[cell] >= len(you.Body)) {
			return turn - 1
		}
		if turn == 4 {
			return turn
		}
		visited[cell] = true
		best := turn
		for _, next := range state.neighbors[cell] {
			if next < 0 {
				continue
			}
			best = max(best, walk(next, turn+1))
			if best == 4 {
				break
			}
		}
		visited[cell] = false
		return best
	}
	return walk(start, 1)
}
