package nhl

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/JonCrowther/home-server/lifx"
)

// Colour is a single named team colour, e.g. {Name: "Orange", Hex: "#F47A38"}.
type Colour struct {
	Name string
	Hex  string
}

const colourPrefix = "Team Color: "

// ldJSONScript matches the JSON-LD blocks teampalettes.com embeds for search
// engines. They are a more stable source than the page's visual markup.
var ldJSONScript = regexp.MustCompile(`(?s)<script type="application/ld\+json"[^>]*>(.*?)</script>`)

// TeamColours fetches a team's colours from teampalettes.com, in the order the
// site lists them. slug is the team's page name, e.g. "anaheim-ducks".
func TeamColours(ctx context.Context, slug string) ([]Colour, error) {
	endpoint := "https://teampalettes.com/team/" + url.PathEscape(slug)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("building team colours request: %w", err)
	}
	req.Header.Set("User-Agent", "home-server")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching team colours for %s: %w", slug, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetching team colours for %s: unexpected status %s", slug, resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading team colours for %s: %w", slug, err)
	}

	for _, match := range ldJSONScript.FindAllSubmatch(body, -1) {
		var doc struct {
			AdditionalProperty []struct {
				Name  string `json:"name"`
				Value string `json:"value"`
			} `json:"additionalProperty"`
		}
		// Other blocks (FAQ, breadcrumbs) are arrays or different schemas;
		// skip anything that doesn't decode into this shape.
		if err := json.Unmarshal(match[1], &doc); err != nil {
			continue
		}

		var colours []Colour
		for _, p := range doc.AdditionalProperty {
			if name, ok := strings.CutPrefix(p.Name, colourPrefix); ok {
				colours = append(colours, Colour{Name: name, Hex: p.Value})
			}
		}
		if len(colours) > 0 {
			return colours, nil
		}
	}

	// The site serves unknown slugs with a 200, so an empty result is the only
	// signal that the team doesn't exist.
	return nil, fmt.Errorf("no team colours found for %s", slug)
}

// goalColours returns the first two of a team's colours that a bulb can show,
// to pulse between when it scores. Colours with no brightness (black) are
// skipped. If the colours can't be found, it logs why and falls back to the
// Oilers' blue and orange.
func goalColours(ctx context.Context, abbrev string) (from, to lifx.Colour) {
	colours, err := goalColoursFor(ctx, abbrev)
	if err != nil {
		slog.Warn("using default goal colours", "team", abbrev, "err", err)
		return lifx.OilersBlue, lifx.OilersOrange
	}
	return colours[0], colours[1]
}

func goalColoursFor(ctx context.Context, abbrev string) ([]lifx.Colour, error) {
	slug, ok := teamSlugs[abbrev]
	if !ok {
		return nil, fmt.Errorf("no teampalettes slug for %s", abbrev)
	}
	teamColours, err := TeamColours(ctx, slug)
	if err != nil {
		return nil, err
	}

	var usable []lifx.Colour
	for _, c := range teamColours {
		lc, err := lifx.ColourFromHex(c.Hex)
		if err != nil {
			return nil, fmt.Errorf("team colour %s: %w", c.Name, err)
		}
		if lc.Brightness == 0 {
			continue
		}
		usable = append(usable, lc)
		if len(usable) == 2 {
			return usable, nil
		}
	}
	return nil, fmt.Errorf("%s has fewer than two colours a bulb can show", slug)
}
