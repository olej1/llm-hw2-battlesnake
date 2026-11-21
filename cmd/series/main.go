package main

import (
	"bufio"
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const candidateName = "candidate"

type MatchResult struct {
	Outcome      string
	Place        int
	MaxLatencyMS int
	Cause        string
}

type replayFrame struct {
	Turn  *int `json:"turn"`
	Board struct {
		Snakes []struct {
			ID      string `json:"id"`
			Name    string `json:"name"`
			Latency string `json:"latency"`
		} `json:"snakes"`
	} `json:"board"`
	WinnerID string `json:"winnerId"`
	IsDraw   *bool  `json:"isDraw"`
}

func summarizeReplay(reader io.Reader, candidateID string) (MatchResult, error) {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 64<<10), 2<<20)
	result := MatchResult{MaxLatencyMS: -1}
	lastSeen := make(map[string]int)
	lastTurn, aliveAtEnd := -1, 0
	var final *replayFrame
	for line := 0; scanner.Scan(); line++ {
		var frame replayFrame
		if err := json.Unmarshal(scanner.Bytes(), &frame); err != nil {
			return MatchResult{}, fmt.Errorf("line %d: %w", line+1, err)
		}
		if final != nil {
			return MatchResult{}, errors.New("unexpected data after the game result")
		}
		if line == 0 && frame.Turn == nil && frame.IsDraw == nil {
			continue
		}
		if frame.IsDraw != nil && frame.Turn == nil {
			final = &frame
			continue
		}
		if frame.Turn == nil || *frame.Turn != lastTurn+1 || frame.Board.Snakes == nil {
			return MatchResult{}, fmt.Errorf("missing or out-of-order frame on line %d", line+1)
		}
		lastTurn = *frame.Turn
		aliveAtEnd = len(frame.Board.Snakes)
		for _, snake := range frame.Board.Snakes {
			lastSeen[snake.ID] = lastTurn
			if lastTurn == 0 && candidateID == "" && snake.Name == candidateName {
				candidateID = snake.ID
			}
			if snake.ID == candidateID {
				if latency, err := strconv.Atoi(snake.Latency); err == nil && latency >= 0 {
					result.MaxLatencyMS = max(result.MaxLatencyMS, latency)
				}
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return MatchResult{}, err
	}
	candidateTurn, found := lastSeen[candidateID]
	if final == nil || lastTurn < 1 || candidateID == "" || !found {
		return MatchResult{}, errors.New("incomplete replay or candidate missing")
	}
	if *final.IsDraw && (final.WinnerID != "" || aliveAtEnd != 0) {
		return MatchResult{}, errors.New("draw requires a final frame without survivors")
	}
	if !*final.IsDraw {
		if turn, found := lastSeen[final.WinnerID]; aliveAtEnd != 1 || final.WinnerID == "" || !found || turn != lastTurn {
			return MatchResult{}, errors.New("winner must be the only survivor in the final frame")
		}
	}

	result.Place = 1
	for _, last := range lastSeen {
		if last > candidateTurn {
			result.Place++
		}
	}
	switch {
	case final.WinnerID == candidateID:
		result.Outcome = "win"
	case *final.IsDraw && candidateTurn == lastTurn-1:
		result.Outcome = "draw"
	default:
		result.Outcome = "loss"
	}
	return result, nil
}

var candidateDeath = regexp.MustCompile(`(?m)^candidate [^\n]*?, Eliminated: ([a-z-]+), Turn: [0-9]+`)

func eliminationCause(log string) string {
	match := candidateDeath.FindStringSubmatch(log)
	if len(match) < 2 {
		return ""
	}
	return match[1]
}

type listFlag []string

func (values *listFlag) String() string { return strings.Join(*values, ", ") }

func (values *listFlag) Set(value string) error {
	*values = append(*values, value)
	return nil
}

type matchConfig struct {
	cli     string
	mode    string
	players []string
	timeCap time.Duration
}

func seriesRules(mode string) (game, board string) {
	switch mode {
	case "constrictor":
		return "constrictor", "empty"
	case "royale":
		return "royale", "royale"
	default:
		return "standard", "standard"
	}
}

func (config matchConfig) play(seed int64, slot int, replay string) (MatchResult, error) {
	game, board := seriesRules(config.mode)
	args := []string{"play", "-W", "11", "-H", "11", "-g", game, "-m", board,
		"-r", strconv.FormatInt(seed, 10), "-t", "500", "-o", replay, "-v"}
	for position := range config.players {
		player := (position + len(config.players) - slot) % len(config.players)
		name := candidateName
		if player != 0 {
			name = fmt.Sprintf("opponent-%d", player)
		}
		args = append(args, "-n", name, "-u", config.players[player])
	}
	ctx, cancel := context.WithTimeout(context.Background(), config.timeCap)
	defer cancel()
	command := exec.CommandContext(ctx, config.cli, args...)
	// Go 1.24+ can otherwise ignore the legacy CLI's call to rand.Seed.
	command.Env = append(os.Environ(), "GODEBUG="+os.Getenv("GODEBUG")+",randseednop=0")
	logs, err := command.CombinedOutput()
	if err != nil {
		return MatchResult{}, fmt.Errorf("CLI: %w: %s", err, logExcerpt(string(logs)))
	}
	for _, line := range strings.Split(string(logs), "\n") {
		if strings.HasPrefix(line, "WARN ") || strings.HasPrefix(line, "ERROR ") {
			return MatchResult{}, fmt.Errorf("engine reported a technical error: %s", line)
		}
	}
	file, err := os.Open(replay)
	if err != nil {
		return MatchResult{}, err
	}
	defer file.Close()
	result, err := summarizeReplay(file, "")
	result.Cause = eliminationCause(string(logs))
	return result, err
}

func logExcerpt(log string) string {
	const limit = 600
	if len(log) > limit {
		log = log[len(log)-limit:]
	}
	return strings.TrimSpace(log)
}

func run() error {
	cli := flag.String("cli", "battlesnake", "path to Battlesnake CLI")
	mode := flag.String("mode", "duels", "duels, standard, royale, or constrictor")
	candidate := flag.String("candidate", "", "candidate base URL")
	seedStart := flag.Int64("seed-start", 1, "first seed")
	seeds := flag.Int("seeds", 11, "number of seeds, each mirrored across all starting slots")
	replays := flag.String("replays", "", "parent directory in which to retain a new directory of replays")
	timeCap := flag.Duration("game-timeout", 30*time.Second, "wall-clock limit for each match")
	var opponents listFlag
	flag.Var(&opponents, "opponent", "opponent base URL, repeat for additional opponents")
	flag.Parse()

	switch *mode {
	case "duels", "standard", "royale", "constrictor":
	default:
		return fmt.Errorf("unknown mode %q", *mode)
	}
	if len(opponents) < 1 || len(opponents) > 3 || (*mode == "duels" && len(opponents) != 1) {
		return errors.New("supply one opponent for duels or one to three for other modes")
	}
	if *seedStart < 0 || *seeds < 1 || *timeCap <= 0 {
		return errors.New("seed-start must be nonnegative; seeds and game-timeout must be positive")
	}
	players := append([]string{*candidate}, opponents...)
	for _, address := range players {
		parsed, err := url.ParseRequestURI(address)
		if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" ||
			parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || strings.Contains(address, ",") {
			return fmt.Errorf("invalid base URL %q", address)
		}
	}
	if *replays != "" {
		if err := os.MkdirAll(*replays, 0o755); err != nil {
			return err
		}
	}
	directory, err := os.MkdirTemp(*replays, "battlesnake-series-")
	if err != nil {
		return err
	}
	if *replays == "" {
		defer os.RemoveAll(directory)
	} else {
		fmt.Fprintln(os.Stderr, "Replays:", directory)
	}

	config := matchConfig{cli: *cli, mode: *mode, players: players, timeCap: *timeCap}
	output := csv.NewWriter(os.Stdout)
	if err := output.Write([]string{"mode", "seed", "slot", "outcome", "place", "max_latency_ms", "cause"}); err != nil {
		return err
	}
	counts := make(map[string]int)
	for offset := range *seeds {
		seed := *seedStart + int64(offset)
		for slot := range players {
			name := fmt.Sprintf("%s-%d-%d.jsonl", *mode, seed, slot+1)
			result, err := config.play(seed, slot, filepath.Join(directory, name))
			if err != nil {
				fmt.Fprintf(os.Stderr, "%s: %v\n", name, err)
				result = MatchResult{Outcome: "technical", MaxLatencyMS: -1}
			}
			counts[result.Outcome]++
			latency, place := "", ""
			if result.MaxLatencyMS >= 0 {
				latency = strconv.Itoa(result.MaxLatencyMS)
			}
			if result.Place > 0 {
				place = strconv.Itoa(result.Place)
			}
			if err := output.Write([]string{*mode, strconv.FormatInt(seed, 10), strconv.Itoa(slot + 1),
				result.Outcome, place, latency, result.Cause}); err != nil {
				return err
			}
			output.Flush()
			if err := output.Error(); err != nil {
				return err
			}
		}
	}
	fmt.Fprintf(os.Stderr, "%s: %d wins, %d losses, %d draws, %d technical failures\n",
		*mode, counts["win"], counts["loss"], counts["draw"], counts["technical"])
	if counts["technical"] > 0 {
		return errors.New("series contains technical failures")
	}
	return nil
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
