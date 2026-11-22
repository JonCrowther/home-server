package nhl

import (
	"context"
	"net/url"
	"time"
)

// scheduledGame is one game in the NHL schedule.
type scheduledGame struct {
	ID           int64     `json:"id"`
	GameState    string    `json:"gameState"`
	StartTimeUTC time.Time `json:"startTimeUTC"`
	AwayTeam     team      `json:"awayTeam"`
	HomeTeam     team      `json:"homeTeam"`
}

// schedule is the NHL's schedule response. GameWeek holds seven days of games
// starting with the requested date.
type schedule struct {
	GameWeek []struct {
		Date  string          `json:"date"`
		Games []scheduledGame `json:"games"`
	} `json:"gameWeek"`
}

// gameOn returns the game teamAbbrev plays on date (YYYY-MM-DD), or false if
// it isn't playing that day.
func gameOn(ctx context.Context, date, teamAbbrev string) (scheduledGame, bool, error) {
	var s schedule
	if err := getJSON(ctx, nhleAPI+"/schedule/"+url.PathEscape(date), &s); err != nil {
		return scheduledGame{}, false, err
	}

	// The requested date should be the first day of GameWeek, but match on
	// the date rather than relying on the order.
	for _, day := range s.GameWeek {
		if day.Date != date {
			continue
		}
		for _, g := range day.Games {
			if g.AwayTeam.Abbrev == teamAbbrev || g.HomeTeam.Abbrev == teamAbbrev {
				return g, true, nil
			}
		}
	}
	return scheduledGame{}, false, nil
}

// gameOver reports whether a game in state has finished: FINAL straight
// after the last whistle, then OFF once the result is official.
func gameOver(state string) bool {
	return state == "FINAL" || state == "OFF"
}

// gameStarted reports whether a game in state has started. FUT is a future
// game and PRE is the pre-game warm-up.
func gameStarted(state string) bool {
	return state != "FUT" && state != "PRE"
}
