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
