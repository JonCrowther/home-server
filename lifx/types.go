package lifx

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

type SelectorType int

const (
	Label SelectorType = iota
	ID
	GroupID
	Group
	LocationID
	Location
	SceneID
	All
)

func MakeSelector(selectorType SelectorType, selectorName string) string {
	switch selectorType {
	case All:
		return "all"
	case Label:
		return "label:" + selectorName
	case GroupID:
		return "group_id:" + selectorName
	case Group:
		return "group:" + selectorName
	case LocationID:
		return "location_id:" + selectorName
	case Location:
		return "location:" + selectorName
	case ID:
		return "id:" + selectorName
	case SceneID:
		return "scene_id:" + selectorName
	default:
		return ""
	}
}

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

type LightState struct {
	// The power state you want to set on the selector. on or off
	Power string `json:"power,omitempty"`
	// The colour to set the light to.
	Colour Colour `json:"color"`
	// The brightness level from 0.0 to 1.0. Overrides any brightness set in color (if any).
	Brightness float64 `json:"brightness,omitempty"`
	// How long in seconds you want the power action to take. Range: 0.0 – 3155760000.0 (100 years)
	Duration float64 `json:"duration,omitempty"`
	// The maximum brightness of the infrared channel from 0.0 to 1.0.
	Infrared float64 `json:"infrared,omitempty"`
	// Execute the query fast, without initial state checks and wait for no results.
	Fast bool `json:"fast,omitempty"`
}

// SelectorState pairs a selector with the state to set on it.
type SelectorState struct {
	Selector string `json:"selector"`
	LightState
}

type Light struct {
	ID           string     `json:"id"`
	Name         string     `json:"name"`
	GroupID      string     `json:"group_id"`
	GroupName    string     `json:"group_name"`
	LocationID   string     `json:"location_id"`
	LocationName string     `json:"location_name"`
	MaxKelvin    int        `json:"max_kelvin"`
	MinKelvin    int        `json:"min_kelvin"`
	State        LightState `json:"light_state"`
}

// ToLight converts a map representation of a light into a Light struct.
// It drops some fields that are not needed for the Light struct.
func ToLight(m map[string]any) Light {
	product, _ := m["product"].(map[string]any)
	caps, _ := product["capabilities"].(map[string]any)

	id, _ := m["id"].(string)
	label, _ := m["label"].(string)
	power, _ := m["power"].(string)
	brightness, _ := m["brightness"].(float64)
	location, _ := m["location"].(map[string]any)
	locationID, _ := location["id"].(string)
	locationName, _ := location["name"].(string)
	maxKelvin, _ := caps["max_kelvin"].(float64)
	minKelvin, _ := caps["min_kelvin"].(float64)

	group, _ := m["group"].(map[string]any)
	groupID, _ := group["id"].(string)
	groupName, _ := group["name"].(string)

	color, _ := m["color"].(map[string]any)
	hue, _ := color["hue"].(float64)
	saturation, _ := color["saturation"].(float64)
	kelvin, _ := color["kelvin"].(float64)

	return Light{
		ID:           id,
		Name:         label,
		GroupID:      groupID,
		GroupName:    groupName,
		LocationID:   locationID,
		LocationName: locationName,
		MaxKelvin:    int(maxKelvin),
		MinKelvin:    int(minKelvin),
		State: LightState{
			Power:      power,
			Brightness: brightness,
			Colour: Colour{
				Hue:        hue,
				Saturation: saturation,
				Kelvin:     int(kelvin),
			},
		},
	}
}
