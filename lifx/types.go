package lifx

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

// Scene is a trimmed-down LIFX scene. The per-light colour data LIFX returns
// (the "chain" of zone colours) is dropped, since it is large and not needed
// to list or activate a scene.
type Scene struct {
	UUID   string       `json:"uuid"`
	Name   string       `json:"name"`
	States []SceneState `json:"states"`
}

// SceneState is the light a scene applies to and the power it sets.
type SceneState struct {
	Selector string `json:"selector"`
	Power    string `json:"power"`
}

// SceneOptions are the optional settings when activating a scene.
type SceneOptions struct {
	// Time in seconds to transition from the current light state to the scene
	// state. A pointer so an explicit 0 (instant) is distinguishable from
	// "not set", which LIFX treats as 1.
	Duration *float64 `json:"duration,omitempty"`
	// Properties that should not be changed on devices when applying the
	// scene. Any of "power", "infrared", "duration", "intensity", "hue",
	// "saturation", "brightness" or "kelvin".
	Ignore []string `json:"ignore,omitempty"`
	// Execute the query fast, without initial state checks and wait for no results.
	Fast bool `json:"fast,omitempty"`
}
