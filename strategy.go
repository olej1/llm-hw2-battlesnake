package main

import (
	"context"
	"math"
	"time"
)

type direction struct {
	name string
	dx   int
	dy   int
}

var directions = [...]direction{
	{name: "up", dy: 1},
	{name: "right", dx: 1},
	{name: "down", dy: -1},
	{name: "left", dx: -1},
}

const unavailable = 1 << 20

// Territory can be enabled locally to reproduce its benchmark variants.
const useDuelTerritory = false
const duelSearchDepth = 5
const duelSearchRadius = 4

type position struct {
	board         Board
	mode          Mode
	duelTerritory bool
	options       strategyOptions
	turn          int
	damage        int
	food          []bool
	hazard        []int
	neighbors     [][4]int
	release       []int
	enemyRange    []int
	enemyLength   []int
	threats       []int
}

func chooseMove(mode Mode, request MoveRequest) string {
	return chooseMoveWithOptions(mode, request, strategyOptions{duelDepth: duelSearchDepth, duelRadius: duelSearchRadius, standardEndgame: true})
}

func chooseMoveWithOptions(mode Mode, request MoveRequest, options strategyOptions) string {
	state := newPosition(mode, request)
	state.options, state.turn = options, request.Turn
	state.options.tailSafety = options.tailSafety && isStandardDuel(request)
	you := request.You
	head := state.index(you.Head)
	if head < 0 {
		return directions[0].name
	}

	best := directions[0].name
	bestRisk, bestScore := 4, math.Inf(-1)
	for dir, cell := range state.neighbors[head] {
		risk := state.moveRisk(you, cell)
		if risk > bestRisk {
			continue
		}
		score := 0.0
		if risk <= 1 {
			score = state.score(you, cell)
		}
		if risk < bestRisk || score > bestScore {
			best, bestRisk, bestScore = directions[dir].name, risk, score
		}
	}
	if mode == Constrictor && bestRisk <= 1 {
		move := state.constrictorRouteMove(you, head, best, bestRisk)
		move = state.avoidContestedPassage(request, move)
		if options.constrictorRollout {
			return state.constrictorRolloutMove(request, move)
		}
		return move
	}
	if len(request.Board.Snakes) == 2 && options.duelDepth >= 2 &&
		(mode == Duels || (mode == Standard && options.standardEndgame)) {
		budget := 8 * time.Millisecond
		if options.duelDepth >= 4 {
			budget = 12 * time.Millisecond
		}
		ctx, cancel := context.WithTimeout(context.Background(), budget)
		best = tacticalMove(ctx, mode, request, best, options)
		cancel()
		if mode == Duels && len(you.Body) >= 12 {
			leadCtx, cancelLead := context.WithTimeout(context.Background(), 12*time.Millisecond)
			defer cancelLead()
			return protectDuelLead(leadCtx, request, best)
		}
	}
	return best
}

func newPosition(mode Mode, request MoveRequest) position {
	board := request.Board
	size := board.Width * board.Height
	state := position{
		board: board, mode: mode, damage: 14,
		duelTerritory: useDuelTerritory && mode == Duels && isStandardDuel(request),
		food:          make([]bool, size), hazard: make([]int, size),
		neighbors: make([][4]int, size), release: make([]int, size),
		enemyRange: make([]int, size), enemyLength: make([]int, size), threats: make([]int, size),
	}
	if configured := request.Game.Ruleset.Settings.HazardDamagePerTurn; configured != nil {
		state.damage = max(0, *configured)
	}
	for _, cell := range board.Food {
		if i := state.index(cell); i >= 0 {
			state.food[i] = mode != Constrictor
		}
	}
	for _, cell := range board.Hazards {
		if i := state.index(cell); i >= 0 && mode != Constrictor {
			state.hazard[i]++
		}
	}
	for i := range state.neighbors {
		cell := Point{i % board.Width, i / board.Width}
		for dir, move := range directions {
			state.neighbors[i][dir] = state.index(Point{cell.X + move.dx, cell.Y + move.dy})
		}
		state.enemyRange[i] = unavailable
	}

	state.addBody(request.You)
	for _, snake := range board.Snakes {
		if snake.ID != request.You.ID {
			state.addBody(snake)
		}
	}
	for _, snake := range board.Snakes {
		if snake.ID != request.You.ID {
			state.addThreats(snake)
		}
	}
	return state
}

func (state *position) index(cell Point) int {
	if cell.X < 0 || cell.X >= state.board.Width || cell.Y < 0 || cell.Y >= state.board.Height {
		return -1
	}
	return cell.Y*state.board.Width + cell.X
}

func (state *position) addBody(snake Snake) {
	for segment, point := range snake.Body {
		cell := state.index(point)
		if cell < 0 {
			continue
		}
		turn := len(snake.Body) - segment
		if state.mode == Constrictor {
			turn = unavailable
		}
		state.release[cell] = max(state.release[cell], turn)
	}
}

func (state *position) healthAfter(health, cell int) int {
	if state.mode == Constrictor {
		return 100
	}
	if state.food[cell] {
		return 100
	}
	return health - 1 - state.damage*state.hazard[cell]
}

func (state *position) addThreats(snake Snake) {
	head := state.index(snake.Head)
	if head < 0 || len(snake.Body) == 0 {
		return
	}

	sources := make([]int, 0, len(directions))
	for _, next := range state.neighbors[head] {
		if next >= 0 && state.release[next] <= 1 && state.healthAfter(snake.Health, next) > 0 {
			state.threats[next] = max(state.threats[next], len(snake.Body))
			if state.duelTerritory {
				length := len(snake.Body)
				var delayed []bool
				if state.food[next] {
					length++
					delayed = state.growthDelay(snake)
				}
				state.addEnemyArrivals(state.distances([]int{next}, -1, delayed, 0), length)
			} else {
				sources = append(sources, next)
			}
		}
	}
	if len(sources) > 0 {
		state.addEnemyArrivals(state.distances(sources, -1, nil, 0), len(snake.Body))
	}
}

func (state *position) addEnemyArrivals(arrivals []int, length int) {
	for cell, turn := range arrivals {
		if turn < state.enemyRange[cell] {
			state.enemyRange[cell], state.enemyLength[cell] = turn, length
		} else if turn != unavailable && turn == state.enemyRange[cell] {
			state.enemyLength[cell] = max(state.enemyLength[cell], length)
		}
	}
}

func (state *position) growthDelay(snake Snake) []bool {
	delayed := make([]bool, len(state.release))
	for _, point := range snake.Body[:len(snake.Body)-1] {
		if cell := state.index(point); cell >= 0 {
			delayed[cell] = true
		}
	}
	return delayed
}

func (state *position) moveRisk(you Snake, next int) int {
	switch {
	case next < 0:
		return 3
	case state.release[next] > 1 || state.healthAfter(you.Health, next) <= 0:
		return 2
	case state.threats[next] >= len(you.Body):
		return 1
	default:
		return 0
	}
}

// distances counts the move to each source as turn one. A cell is traversable
// from the turn its last occupying segment has moved away.
func (state *position) distances(sources []int, avoid int, delayed []bool, claimLength int) []int {
	distance := make([]int, len(state.release))
	for i := range distance {
		distance[i] = unavailable
	}
	queue := make([]int, 0, len(distance))
	for _, source := range sources {
		distance[source] = 1
		queue = append(queue, source)
	}
	for cursor := 0; cursor < len(queue); cursor++ {
		cell := queue[cursor]
		arrival := distance[cell] + 1
		for _, next := range state.neighbors[cell] {
			if next < 0 || next == avoid || distance[next] != unavailable {
				continue
			}
			release := state.release[next]
			if delayed != nil && delayed[next] {
				release++
			}
			if claimLength > 0 && (state.enemyRange[next] < arrival ||
				(state.enemyRange[next] == arrival && state.enemyLength[next] >= claimLength)) {
				continue
			}
			if release <= arrival {
				distance[next] = arrival
				queue = append(queue, next)
			}
		}
	}
	return distance
}

type weights struct {
	space, territory, escape, meal float64
}

func profile(mode Mode) weights {
	switch mode {
	case Duels:
		return weights{0.20, 0.75, 3.5, 13}
	case Royale:
		return weights{0.28, 0.38, 4.0, 10}
	case Constrictor:
		return weights{1.00, 1.15, 5.0, 0}
	default:
		return weights{0.27, 0.52, 4.0, 11}
	}
}

func (state *position) score(you Snake, next int) float64 {
	weight := profile(state.mode)
	length := len(you.Body)
	if state.mode == Duels && state.options.phaseWeights > 0 {
		enemyLength := 0
		for _, opponent := range state.board.Snakes {
			if opponent.ID != you.ID {
				enemyLength = max(enemyLength, len(opponent.Body))
			}
		}
		weight = phaseAdjustedWeights(weight, state.options.phaseWeights, state.turn, length, enemyLength)
	}
	health := state.healthAfter(you.Health, next)
	var delayed []bool
	if state.food[next] {
		length++
		delayed = state.growthDelay(you)
	}

	// The baseline conservatively keeps the old head blocked. The territory
	// experiment allows reachability once that segment has vacated.
	avoid := -1
	if length > 2 && !state.duelTerritory {
		avoid = state.index(you.Head)
	}
	distance := state.distances([]int{next}, avoid, delayed, 0)
	var controlled []int
	if state.duelTerritory {
		controlled = state.distances([]int{next}, -1, delayed, length)
	}
	area, territory, exits := 0, 0.0, 0
	nearestFood, nearestSafe, firstClaimedFood := unavailable, unavailable, unavailable
	for cell, turns := range distance {
		if turns == unavailable {
			continue
		}
		if turns == 2 && state.healthAfter(health, cell) > 0 {
			exits++
		}
		hazard := state.hazard[cell] > 0
		if !hazard {
			nearestSafe = min(nearestSafe, turns)
		}
		if state.food[cell] {
			nearestFood = min(nearestFood, turns)
			if controlled != nil {
				firstClaimedFood = min(firstClaimedFood, controlled[cell])
			}
		}
		area++
		share := 0.0
		switch {
		case turns < state.enemyRange[cell]:
			share = 1
		case turns == state.enemyRange[cell] && length > state.enemyLength[cell]:
			share = 0.85
		case turns == state.enemyRange[cell] && length == state.enemyLength[cell]:
			share = 0.25
		}
		if controlled != nil && controlled[cell] == unavailable {
			share *= 0.5
		}
		if hazard {
			share *= 0.25
		}
		territory += share
	}

	score := weight.space*float64(area) + weight.territory*territory + weight.escape*float64(exits)
	if area < length+2 {
		score -= 7 * float64(length+2-area)
	}
	if exits == 0 {
		score -= 170
	}
	if state.mode == Constrictor {
		return score
	}
	if state.options.tailSafety && state.mode == Duels &&
		length >= 8 && (exits <= 2 || area < 2*length) {
		head := Point{next % state.board.Width, next / state.board.Width}
		if !hasTailEscape(state.board, you, head, 6) {
			score -= 85
		}
	}
	if state.threats[next] > 0 && state.threats[next] < len(you.Body) {
		score += 8
	}

	// Reachable space matters most while there is time to eat later.
	urgency := 1 + float64(max(0, 70-you.Health))/27
	if state.food[next] {
		score += weight.meal * urgency
	} else if controlled != nil && firstClaimedFood != unavailable {
		score += weight.meal * urgency / float64(firstClaimedFood)
	} else if nearestFood != unavailable {
		bonus := weight.meal * urgency / float64(nearestFood)
		if controlled != nil && health >= 18 {
			bonus *= 0.45
		}
		score += bonus
	}
	if health < 21 && !state.food[next] {
		if nearestFood == unavailable || nearestFood-1 > health {
			score -= float64(21-health) * 5
		}
	}
	if state.hazard[next] > 0 && !state.food[next] {
		score -= float64(state.damage*state.hazard[next]) * (1 + float64(max(0, 55-health))/35)
		if nearestSafe > 2 {
			score -= 12
		}
	}
	return score
}
