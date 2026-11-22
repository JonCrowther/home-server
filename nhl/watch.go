package nhl

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"

	"github.com/JonCrowther/home-server/lifx"
)

const (
	// pollInterval is how often a running game's score is checked.
	pollInterval = time.Second
	// pregamePollInterval is how often a game that hasn't started is checked.
	// Slower, since a watch can be started hours before the game.
	pregamePollInterval = time.Minute
)

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

// scoreLine formats the score, e.g. "EDM score: 2, CAR score: 1".
func (b boxscore) scoreLine() string {
	return fmt.Sprintf("%s score: %d, %s score: %d",
		b.AwayTeam.Abbrev, b.AwayTeam.Score, b.HomeTeam.Abbrev, b.HomeTeam.Score)
}

// name returns the team's common name, e.g. "Oilers", or its abbreviation if
// the name is missing.
func (t team) name() string {
	if t.CommonName.Default != "" {
		return t.CommonName.Default
	}
	return t.Abbrev
}

// Watcher runs game watches in the background and tracks which are running.
// Both watch handlers share one, so the same team in the same game can't be
// watched twice through either.
type Watcher struct {
	ctx   context.Context
	token string

	mu sync.Mutex
	// watching holds the running watches, keyed by "gameID/team" so both
	// sides of one game can be watched.
	watching map[string]*watch
}

// watch is one running watch.
type watch struct {
	team   string
	cancel context.CancelFunc
}

// NewWatcher returns a Watcher that celebrates goals with the LIFX token.
//
// ctx should be the server's lifetime context, not a request's: the request
// context is cancelled as soon as the response is written, which would stop
// every watch immediately.
func NewWatcher(ctx context.Context, token string) *Watcher {
	return &Watcher{ctx: ctx, token: token, watching: map[string]*watch{}}
}

// start begins watching teamAbbrev in gameID in the background. It returns
// false if that watch is already running.
func (wt *Watcher) start(gameID, teamAbbrev string) bool {
	key := gameID + "/" + teamAbbrev
	wt.mu.Lock()
	defer wt.mu.Unlock()
	if wt.watching[key] != nil {
		return false
	}

	// Each watch gets its own context so stop can cancel it alone; it is
	// still cancelled with the server's.
	ctx, cancel := context.WithCancel(wt.ctx)
	w := &watch{team: teamAbbrev, cancel: cancel}
	wt.watching[key] = w

	go func() {
		defer func() {
			cancel()
			wt.mu.Lock()
			// stop removes the entry itself, and a new watch may have taken
			// the key since, so only remove it if it's still this one.
			if wt.watching[key] == w {
				delete(wt.watching, key)
			}
			wt.mu.Unlock()
		}()
		if err := watchGame(ctx, wt.token, gameID, teamAbbrev); err != nil && !errors.Is(err, context.Canceled) {
			slog.Error("watching game", "game", gameID, "team", teamAbbrev, "err", err)
		}
	}()
	return true
}

// stop cancels every running watch on teamAbbrev and returns how many it
// stopped. Usually there is at most one, since a team plays one game a day.
func (wt *Watcher) stop(teamAbbrev string) int {
	wt.mu.Lock()
	defer wt.mu.Unlock()

	stopped := 0
	for key, w := range wt.watching {
		if w.team == teamAbbrev {
			w.cancel()
			// Removed here rather than when the goroutine exits, so the team
			// can be watched again straight away.
			delete(wt.watching, key)
			stopped++
		}
	}
	return stopped
}

// StopWatch handles DELETE /nhl/watch/{team}: it stops watching {team} (e.g.
// "EDM") in any game. Responds 204 once stopped, or 404 if the team wasn't
// being watched.
func (wt *Watcher) StopWatch(w http.ResponseWriter, r *http.Request) {
	teamAbbrev := r.PathValue("team")
	if !validTeam(teamAbbrev) {
		http.Error(w, "team must be 3 upper case letters, e.g. EDM", http.StatusBadRequest)
		return
	}

	if wt.stop(teamAbbrev) == 0 {
		http.Error(w, "not watching "+teamAbbrev, http.StatusNotFound)
		return
	}
	slog.Info("stopped watch", "team", teamAbbrev)
	w.WriteHeader(http.StatusNoContent)
}

// WatchGame handles POST /games/{id}/watch/{team}: it starts watching one
// team's score in a game and responds 202 straight away. {team} is the
// team's abbreviation, e.g. "EDM".
func (wt *Watcher) WatchGame(w http.ResponseWriter, r *http.Request) {
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

	if !wt.start(gameID, teamAbbrev) {
		http.Error(w, "already watching "+teamAbbrev+" in game "+gameID, http.StatusConflict)
		return
	}
	w.WriteHeader(http.StatusAccepted)
}

// watchResponse describes the game a WatchTeam request started watching.
type watchResponse struct {
	GameID    string    `json:"game_id"`
	AwayTeam  string    `json:"away_team"`
	HomeTeam  string    `json:"home_team"`
	StartTime time.Time `json:"start_time"`
	GameState string    `json:"game_state"`
}

// WatchTeam handles POST /nhl/watch/{team}: it finds the game {team} (e.g.
// "EDM") plays today and starts watching it, responding 202 with the game.
// The watch can start before the game does, and ends when the game is over.
//
// "Today" is the date in the server's local time zone, set with the TZ
// environment variable. Responds 404 if the team isn't playing today, 409 if
// today's game is already over or already being watched, or 502 if the
// schedule can't be fetched.
func (wt *Watcher) WatchTeam(w http.ResponseWriter, r *http.Request) {
	teamAbbrev := r.PathValue("team")
	if !validTeam(teamAbbrev) {
		http.Error(w, "team must be 3 upper case letters, e.g. EDM", http.StatusBadRequest)
		return
	}

	today := time.Now().Format(time.DateOnly)
	game, ok, err := gameOn(r.Context(), today, teamAbbrev)
	if err != nil {
		http.Error(w, "fetching schedule: "+err.Error(), http.StatusBadGateway)
		return
	}
	if !ok {
		http.Error(w, teamAbbrev+" isn't playing today ("+today+")", http.StatusNotFound)
		return
	}

	gameID := strconv.FormatInt(game.ID, 10)
	if gameOver(game.GameState) {
		http.Error(w, teamAbbrev+"'s game today ("+gameID+") is already over", http.StatusConflict)
		return
	}
	if !wt.start(gameID, teamAbbrev) {
		http.Error(w, "already watching "+teamAbbrev+" in game "+gameID, http.StatusConflict)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	if err := json.NewEncoder(w).Encode(watchResponse{
		GameID:    gameID,
		AwayTeam:  game.AwayTeam.Abbrev,
		HomeTeam:  game.HomeTeam.Abbrev,
		StartTime: game.StartTimeUTC,
		GameState: game.GameState,
	}); err != nil {
		slog.Error("encoding watch response", "err", err)
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
// with the lights whenever teamAbbrev scores. It waits through a game that
// hasn't started yet, and returns nil once the game is over, an error if
// teamAbbrev isn't playing in the game, or ctx.Err() if ctx is cancelled
// first.
func watchGame(ctx context.Context, token, gameID, teamAbbrev string) error {
	// Look the colours up once per watch rather than on every goal.
	from, to := goalColours(ctx, teamAbbrev)

	slog.Info("watching game", "game", gameID, "team", teamAbbrev)

	var prev *team
	for {
		wait := pollInterval

		cur, err := fetchBoxscore(ctx, gameID)
		if err != nil {
			// Transient failures shouldn't end the watch; try again next time.
			slog.Error("polling boxscore", "game", gameID, "err", err)
		} else {
			slog.Debug(cur.scoreLine(), "game", gameID, "state", cur.GameState)

			watched, ok := cur.team(teamAbbrev)
			if !ok {
				return fmt.Errorf("team %s is not playing in game %s (%s vs %s)",
					teamAbbrev, gameID, cur.AwayTeam.Abbrev, cur.HomeTeam.Abbrev)
			}
			if prev != nil {
				reportChange(ctx, token, *prev, watched, from, to)
			}
			prev = &watched

			if gameOver(cur.GameState) {
				slog.Info("game over", "game", gameID, "team", teamAbbrev, "state", cur.GameState)
				return nil
			}
			if !gameStarted(cur.GameState) {
				wait = pregamePollInterval
			}
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(wait):
		}
	}
}

// reportChange celebrates by pulsing between the from and to colours if the
// team's score went up. A drop (a goal taken back after review) is only logged.
func reportChange(ctx context.Context, token string, before, after team, from, to lifx.Colour) {
	switch {
	case after.Score > before.Score:
		slog.Info(after.name()+" goal!", "team", after.Abbrev, "from", before.Score, "to", after.Score)
		if err := lifx.CelebrateGoal(ctx, token, goalLights, from, to); err != nil {
			slog.Error("celebrating goal", "team", after.Abbrev, "err", err)
		}
	case after.Score < before.Score:
		slog.Info("goal taken back", "team", after.Abbrev, "from", before.Score, "to", after.Score)
	}
}

func fetchBoxscore(ctx context.Context, gameID string) (boxscore, error) {
	var b boxscore
	err := getJSON(ctx, nhleAPI+"/gamecenter/"+url.PathEscape(gameID)+"/boxscore", &b)
	return b, err
}
