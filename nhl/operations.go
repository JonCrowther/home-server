package nhl

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"sync"
	"time"
)

const pollInterval = 10 * time.Second

type team struct {
	Score      int `json:"score"`
	CommonName struct {
		Default string `json:"default"`
	} `json:"commonName"`
}

type boxscore struct {
	GameState string `json:"gameState"`
	AwayTeam  team   `json:"awayTeam"`
	HomeTeam  team   `json:"homeTeam"`
}

// WatchGame returns a handler, e.g. POST /games/{id}/watch, that starts
// watching a game in the background and responds 202 straight away.
//
// ctx should be the server's lifetime context, not the request's: the request
// context is cancelled as soon as the response is written, which would stop
// the watcher immediately.
func WatchGame(ctx context.Context) http.HandlerFunc {
	var (
		mu       sync.Mutex
		watching = map[string]bool{}
	)

	return func(w http.ResponseWriter, r *http.Request) {
		gameID := r.PathValue("id")
		if !validGameID(gameID) {
			http.Error(w, "game id must be 10 digits, e.g. 2026020260", http.StatusBadRequest)
			return
		}

		mu.Lock()
		if watching[gameID] {
			mu.Unlock()
			http.Error(w, "already watching game "+gameID, http.StatusConflict)
			return
		}
		watching[gameID] = true
		mu.Unlock()

		go func() {
			defer func() {
				mu.Lock()
				delete(watching, gameID)
				mu.Unlock()
			}()
			if err := watchGame(ctx, gameID); err != nil && !errors.Is(err, context.Canceled) {
				slog.Error("watching game", "game", gameID, "err", err)
			}
		}()

		w.WriteHeader(http.StatusAccepted)
	}
}

func validGameID(id string) bool {
	if len(id) != 10 {
		return false
	}
	for _, c := range id {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// watchGame polls the boxscore for gameID (e.g. "2026020260") and prints a
// line whenever either team's score changes. It returns nil once the game's
// state is anything other than "LIVE", or ctx.Err() if ctx is cancelled first.
func watchGame(ctx context.Context, gameID string) error {
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	var prev *boxscore
	for {
		cur, err := fetchBoxscore(ctx, gameID)
		fmt.Println(cur)
		if err != nil {
			// Transient failures shouldn't end the watch; try again next tick.
			slog.Error("polling boxscore", "game", gameID, "err", err)
		} else {
			if prev != nil {
				reportChange(prev.AwayTeam, cur.AwayTeam)
				reportChange(prev.HomeTeam, cur.HomeTeam)
			}
			prev = &cur

			// The game is running if either LIVE or CRIT (last 5 minutes)
			if cur.GameState != "LIVE" && cur.GameState != "CRIT" {
				fmt.Printf("game %s is no longer live (state: %s)\n", gameID, cur.GameState)
				return nil
			}
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func reportChange(before, after team) {
	if before.Score != after.Score {
		fmt.Printf("%s score changed: %d -> %d\n", after.CommonName.Default, before.Score, after.Score)
	}
}

func fetchBoxscore(ctx context.Context, gameID string) (boxscore, error) {
	endpoint := "https://api-web.nhle.com/v1/gamecenter/" + url.PathEscape(gameID) + "/boxscore"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return boxscore{}, err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return boxscore{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return boxscore{}, fmt.Errorf("%s", resp.Status)
	}

	var b boxscore
	if err := json.NewDecoder(resp.Body).Decode(&b); err != nil {
		return boxscore{}, err
	}
	return b, nil
}
