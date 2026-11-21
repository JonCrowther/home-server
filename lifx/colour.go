package lifx

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
)

// Colour is a LIFX colour. Fields left at zero are left out when it is sent,
// so LIFX keeps the bulb's current value for them.
type Colour struct {
	// The hue of the colour, from 0 to 360.
	Hue float64 `json:"hue,omitempty"`
	// The saturation of the colour, from 0.0 to 1.0.
	Saturation float64 `json:"saturation,omitempty"`
	// The brightness of the colour, from 0.0 to 1.0.
	Brightness float64 `json:"brightness,omitempty"`
	// The colour temperature in Kelvin.
	Kelvin int `json:"kelvin,omitempty"`
}

// Named colours used as effect defaults. Red uses a hue of 360 rather than 0:
// String drops zero fields, and a missing hue would leave the bulb on its
// current hue. 360 is the same point on the colour wheel.
var (
	red  = Colour{Hue: 360, Saturation: 1, Brightness: 1}
	blue = Colour{Hue: 240, Saturation: 1, Brightness: 1}
)

// String formats the colour as a LIFX colour string, e.g.
// "hue:120 saturation:1 brightness:0.5". Zero-valued fields are left out,
// matching the omitempty JSON tags, so a zero Colour gives "".
func (c Colour) String() string {
	var parts []string
	add := func(name string, v float64) {
		if v != 0 {
			parts = append(parts, name+":"+strconv.FormatFloat(v, 'f', -1, 64))
		}
	}
	add("hue", c.Hue)
	add("saturation", c.Saturation)
	add("brightness", c.Brightness)
	add("kelvin", float64(c.Kelvin))
	return strings.Join(parts, " ")
}

// MarshalJSON encodes the colour as its LIFX colour string, which is the form
// LIFX's endpoints expect. This lets option structs holding Colours be sent to
// LIFX as they are. Decoding still uses the struct fields, so request bodies
// can give colours as {"hue": ..., "saturation": ...}.
func (c Colour) MarshalJSON() ([]byte, error) {
	return json.Marshal(c.String())
}

// ColourFromHex converts an RGB hex code such as "#F47A38" to a Colour.
//
// Because String drops zero-valued fields, two cases are adjusted so the
// colour still comes out right on the bulb:
//   - A hue of 0 (pure red) is stored as 360, the same point on the colour
//     wheel, so it isn't dropped and left as the bulb's current hue.
//   - Greys and white have no saturation, so they are expressed as 6500K
//     white instead, which LIFX treats as saturation 0.
//
// Black has a brightness of 0, which a bulb can't show; its String is "".
func ColourFromHex(hex string) (Colour, error) {
	var r, g, b uint8
	if _, err := fmt.Sscanf(strings.TrimPrefix(hex, "#"), "%02x%02x%02x", &r, &g, &b); err != nil {
		return Colour{}, fmt.Errorf("parsing hex colour %q: %w", hex, err)
	}

	rf, gf, bf := float64(r)/255, float64(g)/255, float64(b)/255
	maxC := math.Max(rf, math.Max(gf, bf))
	delta := maxC - math.Min(rf, math.Min(gf, bf))

	if delta == 0 {
		return Colour{Brightness: maxC, Kelvin: 6500}, nil
	}

	var hue float64
	switch maxC {
	case rf:
		hue = 60 * math.Mod((gf-bf)/delta, 6)
	case gf:
		hue = 60 * ((bf-rf)/delta + 2)
	default:
		hue = 60 * ((rf-gf)/delta + 4)
	}
	if hue <= 0 {
		hue += 360
	}

	round := func(v float64) float64 { return math.Round(v*100) / 100 }
	return Colour{
		Hue:        math.Round(hue),
		Saturation: round(delta / maxC),
		Brightness: round(maxC),
	}, nil
}
