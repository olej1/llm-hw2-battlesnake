package main

import (
	"flag"
	"log"
	"net"
	"net/http"
	"os"
	"time"
)

func main() {
	var options strategyOptions
	flag.BoolVar(&options.constrictorRollout, "constrictor-rollout", true, "simulate alternative routes in Constrictor")
	flag.IntVar(&options.duelDepth, "duel-depth", duelSearchDepth, "maximum duel search: 0 or 2 to 6")
	flag.IntVar(&options.duelRadius, "duel-search-radius", duelSearchRadius, "Manhattan distance for search past three turns in Duels; 0 uses twice the depth")
	flag.IntVar(&options.phaseWeights, "phase-weights", 0, "phase-based food and territory weights: 0, 1, or 2")
	flag.BoolVar(&options.tailSafety, "tail-safety", false, "look for an escape from tight corridors in Duels")
	flag.BoolVar(&options.standardEndgame, "standard-endgame", true, "enable duel search once Standard has two snakes")
	flag.Parse()
	if options.duelDepth < 0 || options.duelDepth == 1 || options.duelDepth > 6 ||
		options.duelRadius < 0 || options.phaseWeights < 0 || options.phaseWeights > 2 {
		log.Fatal("duel-depth must be 0 or 2 to 6; duel-search-radius must be nonnegative; phase-weights must be 0 to 2")
	}
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	server := &http.Server{
		Addr: net.JoinHostPort(os.Getenv("HOST"), port),
		Handler: newHandler(func(mode Mode, request MoveRequest) string {
			return chooseMoveWithOptions(mode, request, options)
		}),
		ReadHeaderTimeout: 2 * time.Second,
		ReadTimeout:       3 * time.Second,
		WriteTimeout:      3 * time.Second,
		IdleTimeout:       30 * time.Second,
	}
	log.Printf("Listening on %s", server.Addr)
	log.Fatal(server.ListenAndServe())
}
