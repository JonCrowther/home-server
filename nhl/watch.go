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

	"github.com/JonCrowther/home-server/lifx"
)

const pollInterval = 10 * time.Second

// goalLights is the LIFX selector for the lights that celebrate a goal.
var goalLights = lifx.MakeSelector(lifx.Group, "Living Room")

type team struct {
	Abbrev     string `json:"abbrev"`
	Score      int    `json:"score"`
	CommonName struct {
		Default string `json:"default"`
	} `json:"commonName"`
}

type boxscore struct {
	GameState string `json:"gameState"`
	AwayTeam  team   `json:"awayTeam"`
	HomeTeam  team   `json:"homeTeam"`
}

// team returns whichever side of the game has the given abbreviation.
func (b boxscore) team(abbrev string) (team, bool) {
	switch abbrev {
	case b.AwayTeam.Abbrev:
		return b.AwayTeam, true
	case b.HomeTeam.Abbrev:
		return b.HomeTeam, true
	}
	return team{}, false
}

// WatchGame returns a handler, e.g. POST /games/{id}/watch/{team}, that starts
// watching one team's score in a game in the background and responds 202
// straight away. {team} is the team's abbreviation, e.g. "EDM".
//
// ctx should be the server's lifetime context, not the request's: the request
// context is cancelled as soon as the response is written, which would stop
// the watcher immediately. token is the LIFX token used to celebrate goals.
func WatchGame(ctx context.Context, token string) http.HandlerFunc {
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
		teamAbbrev := r.PathValue("team")
		if !validTeam(teamAbbrev) {
			http.Error(w, "team must be 3 upper case letters, e.g. EDM", http.StatusBadRequest)
			return
		}

		// Keyed on game and team, so both sides of one game can be watched.
		key := gameID + "/" + teamAbbrev
		mu.Lock()
		if watching[key] {
			mu.Unlock()
			http.Error(w, "already watching "+teamAbbrev+" in game "+gameID, http.StatusConflict)
			return
		}
		watching[key] = true
		mu.Unlock()

		go func() {
			defer func() {
				mu.Lock()
				delete(watching, key)
				mu.Unlock()
			}()
			if err := watchGame(ctx, token, gameID, teamAbbrev); err != nil && !errors.Is(err, context.Canceled) {
				slog.Error("watching game", "game", gameID, "team", teamAbbrev, "err", err)
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

func validTeam(abbrev string) bool {
	if len(abbrev) != 3 {
		return false
	}
	for _, c := range abbrev {
		if c < 'A' || c > 'Z' {
			return false
		}
	}
	return true
}

// watchGame polls the boxscore for gameID (e.g. "2026020260") and celebrates
// with the lights whenever teamAbbrev scores. It returns nil once the game is
// no longer live, an error if teamAbbrev isn't playing in the game, or
// ctx.Err() if ctx is cancelled first.
func watchGame(ctx context.Context, token, gameID, teamAbbrev string) error {
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	// Look the colours up once per watch rather than on every goal.
	from, to := goalColours(ctx, teamAbbrev)

	var prev *team
	for {
		cur, err := fetchBoxscore(ctx, gameID)
		fmt.Println(cur)
		if err != nil {
			// Transient failures shouldn't end the watch; try again next tick.
			slog.Error("polling boxscore", "game", gameID, "err", err)
		} else {
			watched, ok := cur.team(teamAbbrev)
			if !ok {
				return fmt.Errorf("team %s is not playing in game %s (%s vs %s)",
					teamAbbrev, gameID, cur.AwayTeam.Abbrev, cur.HomeTeam.Abbrev)
			}
			if prev != nil {
				reportChange(ctx, token, *prev, watched, from, to)
			}
			prev = &watched

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

// reportChange celebrates by pulsing between the from and to colours if the
// team's score went up. A drop (a goal taken back after review) is only logged.
func reportChange(ctx context.Context, token string, before, after team, from, to lifx.Colour) {
	switch {
	case after.Score > before.Score:
		slog.Info("goal", "team", after.Abbrev, "from", before.Score, "to", after.Score)
		if err := lifx.CelebrateGoal(ctx, token, goalLights, from, to); err != nil {
			slog.Error("celebrating goal", "team", after.Abbrev, "err", err)
		}
	case after.Score < before.Score:
		slog.Info("goal taken back", "team", after.Abbrev, "from", before.Score, "to", after.Score)
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
