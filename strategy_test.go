package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func strategyRequest(you Snake, opponents ...Snake) MoveRequest {
	return MoveRequest{
		Board: Board{Width: 11, Height: 11, Snakes: append([]Snake{you}, opponents...)},
		You:   you,
	}
}

func testSnake(id string, health int, body ...Point) Snake {
	return Snake{ID: id, Health: health, Head: body[0], Body: body}
}

func TestVacatedTailIsUsable(t *testing.T) {
	request := strategyRequest(testSnake("us", 1, Point{0, 1}, Point{0, 0}, Point{1, 0}, Point{1, 1}))
	request.Board.Food = []Point{{1, 1}}
	if move := chooseMove(Duels, request); move != "right" {
		t.Fatalf("got %s, want the food on the vacated tail", move)
	}
}

func TestRepeatedTailStaysBlocked(t *testing.T) {
	request := strategyRequest(testSnake("us", 25, Point{0, 1}, Point{0, 0}, Point{1, 0}, Point{1, 1}, Point{1, 1}))
	request.Board.Food = []Point{{1, 1}}
	if move := chooseMove(Duels, request); move != "up" {
		t.Fatalf("got %s, want the only unblocked move", move)
	}
}

func TestEqualHeadThreat(t *testing.T) {
	us := testSnake("us", 1, Point{2, 2}, Point{2, 1}, Point{2, 0})
	them := testSnake("them", 90, Point{4, 2}, Point{4, 1}, Point{4, 0})
	request := strategyRequest(us, them)
	request.You.Health = 2
	request.Board.Food = []Point{{3, 2}, {2, 3}}
	if move := chooseMove(Duels, request); move != "up" {
		t.Fatalf("got %s, want uncontested food", move)
	}
}

func TestShorterHeadDoesNotBlockFood(t *testing.T) {
	us := testSnake("us", 1, Point{2, 2}, Point{2, 1}, Point{2, 0})
	them := testSnake("them", 90, Point{4, 2}, Point{4, 1})
	request := strategyRequest(us, them)
	request.Board.Food = []Point{{3, 2}}
	if move := chooseMove(Duels, request); move != "right" {
		t.Fatalf("got %s, want food contested by shorter snake", move)
	}
}

func TestCriticalHealthRoyaleFood(t *testing.T) {
	request := strategyRequest(testSnake("us", 1, Point{2, 2}, Point{2, 1}, Point{2, 0}))
	request.Board.Food = []Point{{3, 2}}
	request.Board.Hazards = []Point{{3, 2}}
	if move := chooseMove(Royale, request); move != "right" {
		t.Fatalf("got %s, want food in the hazard", move)
	}
}

func TestConstrictorTailNeverVacates(t *testing.T) {
	request := strategyRequest(testSnake("us", 100, Point{0, 1}, Point{0, 0}, Point{1, 0}, Point{1, 1}))
	if move := chooseMove(Constrictor, request); move != "up" {
		t.Fatalf("got %s, want the only available cell", move)
	}
}

func TestNoSafeMoveReturnsDirection(t *testing.T) {
	request := strategyRequest(testSnake("us", 1, Point{0, 0}))
	request.Board.Width, request.Board.Height = 1, 1
	switch move := chooseMove(Duels, request); move {
	case "up", "down", "left", "right":
	default:
		t.Fatalf("invalid fallback %q", move)
	}
}

func TestMoveLatency(t *testing.T) {
	request := strategyRequest(
		testSnake("us", 45, Point{5, 5}, Point{5, 4}, Point{5, 3}, Point{5, 2}, Point{5, 1}),
		testSnake("a", 60, Point{8, 4}, Point{8, 3}, Point{8, 2}, Point{8, 1}),
		testSnake("b", 75, Point{2, 6}, Point{2, 7}, Point{2, 8}, Point{2, 9}),
		testSnake("c", 55, Point{5, 8}, Point{6, 8}, Point{7, 8}, Point{8, 8}),
	)
	request.Board.Food = []Point{{0, 0}, {1, 2}, {10, 10}}
	request.Board.Hazards = []Point{{0, 0}, {0, 1}, {0, 2}, {0, 3}}
	payload, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(newHandler(chooseMove))
	defer server.Close()
	client := server.Client()
	client.Timeout = 500 * time.Millisecond
	for _, mode := range []Mode{Duels, Standard, Royale, Constrictor} {
		var maxLatency time.Duration
		for range 64 {
			start := time.Now()
			response, err := client.Post(server.URL+"/"+string(mode)+"/move", "application/json", bytes.NewReader(payload))
			if err != nil {
				t.Fatal(err)
			}
			var body map[string]string
			err = json.NewDecoder(response.Body).Decode(&body)
			response.Body.Close()
			if err != nil || response.StatusCode != http.StatusOK || body["move"] == "" {
				t.Fatalf("mode %s: status %d, result %v, decode %v", mode, response.StatusCode, body, err)
			}
			maxLatency = max(maxLatency, time.Since(start))
		}
		if maxLatency >= 500*time.Millisecond {
			t.Fatalf("%s max latency %s", mode, maxLatency)
		}
		t.Logf("%s: max HTTP response %s across 64 moves", mode, maxLatency)
	}
}
