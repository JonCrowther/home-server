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

func (p *queryParser) fail(name, want, got string) {
	if p.err == nil {
		p.err = fmt.Errorf("%s must be %s, got %q", name, want, got)
	}
}

// float reads a non-negative number, e.g. period=2.5.
func (p *queryParser) float(name string, dst *float64) {
	v := p.values.Get(name)
	if v == "" {
		return
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil || !(f >= 0) || math.IsInf(f, 0) {
		p.fail(name, "a non-negative number", v)
		return
	}
	*dst = f
}

// seconds reads a positive number of seconds as a duration, e.g. step=0.5.
func (p *queryParser) seconds(name string, dst *time.Duration) {
	v := p.values.Get(name)
	if v == "" {
		return
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil || !(f > 0) || math.IsInf(f, 0) {
		p.fail(name, "a positive number of seconds", v)
		return
	}
	*dst = time.Duration(f * float64(time.Second))
}

// int reads a non-negative whole number, e.g. passes=3.
func (p *queryParser) int(name string, dst *int) {
	v := p.values.Get(name)
	if v == "" {
		return
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 0 {
		p.fail(name, "a non-negative whole number", v)
		return
	}
	*dst = n
}

// bool reads true or false, e.g. persist=true.
func (p *queryParser) bool(name string, dst *bool) {
	v := p.values.Get(name)
	if v == "" {
		return
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		p.fail(name, "true or false", v)
		return
	}
	*dst = b
}

// optionalBool is bool for fields where nil means "use LIFX's default".
func (p *queryParser) optionalBool(name string, dst **bool) {
	var b bool
	before := p.err
	p.bool(name, &b)
	if p.values.Get(name) != "" && p.err == before {
		*dst = &b
	}
}

// colour reads a hex colour, with or without the #, e.g. colour=F47A38. A #
// must be sent as %23 in a URL, so leaving it off is easier.
func (p *queryParser) colour(name string, dst *Colour) {
	v := p.values.Get(name)
	if v == "" {
		return
	}
	c, ok := p.parseColour(name, v)
	if ok {
		*dst = c
	}
}

// colours reads a comma-separated list of hex colours, e.g.
// palette=FF0000,FFA500,800080.
func (p *queryParser) colours(name string, dst *[]Colour) {
	v := p.values.Get(name)
	if v == "" {
		return
	}
	var cs []Colour
	for _, hex := range strings.Split(v, ",") {
		c, ok := p.parseColour(name, strings.TrimSpace(hex))
		if !ok {
			return
		}
		cs = append(cs, c)
	}
	*dst = cs
}

func (p *queryParser) parseColour(name, hex string) (Colour, bool) {
	c, err := ColourFromHex(hex)
	// Black has no brightness, which a bulb can't show and LIFX can't take.
	if err != nil || c.String() == "" {
		p.fail(name, "a hex colour other than black, e.g. F47A38", hex)
		return Colour{}, false
	}
	return c, true
}
