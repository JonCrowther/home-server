package lifx

import (
	"fmt"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// queryParser reads effect options from a request's query string. Each method
// leaves its destination unchanged when the parameter is absent, so handlers
// set defaults first and let the query override them. It keeps the first
// error, so a handler can parse every option and check err once.
type queryParser struct {
	values url.Values
	err    error
}

func newQueryParser(r *http.Request) *queryParser {
	return &queryParser{values: r.URL.Query()}
}

// param reads the query parameter name into dst using parse, which reports
// whether the value is valid. If it isn't, p.err describes it with want, the
// kind of value expected. Generic methods aren't allowed in Go, so this is a
// function rather than a method.
func param[T any](p *queryParser, name, want string, dst *T, parse func(string) (T, bool)) {
	v := p.values.Get(name)
	if v == "" {
		return
	}
	parsed, ok := parse(v)
	if !ok {
		if p.err == nil {
			p.err = fmt.Errorf("%s must be %s, got %q", name, want, v)
		}
		return
	}
	*dst = parsed
}

// float reads a non-negative number, e.g. period=2.5.
func (p *queryParser) float(name string, dst *float64) {
	param(p, name, "a non-negative number", dst, func(v string) (float64, bool) {
		f, ok := parseFinite(v)
		return f, ok && f >= 0
	})
}

// seconds reads a positive number of seconds as a duration, e.g. step=0.5.
func (p *queryParser) seconds(name string, dst *time.Duration) {
	param(p, name, "a positive number of seconds", dst, func(v string) (time.Duration, bool) {
		f, ok := parseFinite(v)
		return time.Duration(f * float64(time.Second)), ok && f > 0
	})
}

// int reads a non-negative whole number, e.g. passes=3.
func (p *queryParser) int(name string, dst *int) {
	param(p, name, "a non-negative whole number", dst, func(v string) (int, bool) {
		n, err := strconv.Atoi(v)
		return n, err == nil && n >= 0
	})
}

// bool reads true or false, e.g. persist=true.
func (p *queryParser) bool(name string, dst *bool) {
	param(p, name, "true or false", dst, parseBool)
}

// optionalBool is bool for fields where nil means "use LIFX's default".
func (p *queryParser) optionalBool(name string, dst **bool) {
	param(p, name, "true or false", dst, func(v string) (*bool, bool) {
		b, ok := parseBool(v)
		return &b, ok
	})
}

// colour reads a hex colour, with or without the #, e.g. colour=F47A38. A #
// must be sent as %23 in a URL, so leaving it off is easier.
func (p *queryParser) colour(name string, dst *Colour) {
	param(p, name, hexColourWant, dst, parseHexColour)
}

// colours reads a comma-separated list of hex colours, e.g.
// palette=FF0000,FFA500,800080.
func (p *queryParser) colours(name string, dst *[]Colour) {
	param(p, name, "a comma-separated list of "+hexColourWant+"s", dst, func(v string) ([]Colour, bool) {
		var cs []Colour
		for _, hex := range strings.Split(v, ",") {
			c, ok := parseHexColour(strings.TrimSpace(hex))
			if !ok {
				return nil, false
			}
			cs = append(cs, c)
		}
		return cs, true
	})
}

// parseFinite parses a number, rejecting NaN and infinities, which
// strconv.ParseFloat accepts.
func parseFinite(v string) (float64, bool) {
	f, err := strconv.ParseFloat(v, 64)
	return f, err == nil && !math.IsNaN(f) && !math.IsInf(f, 0)
}

func parseBool(v string) (bool, bool) {
	b, err := strconv.ParseBool(v)
	return b, err == nil
}

const hexColourWant = "hex colour other than black, e.g. F47A38"

// parseHexColour parses a hex colour, rejecting black: it has no brightness,
// which a bulb can't show and LIFX can't take.
func parseHexColour(hex string) (Colour, bool) {
	c, err := ColourFromHex(hex)
	return c, err == nil && c.String() != ""
}
