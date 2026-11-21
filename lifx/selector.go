package lifx

// SelectorType is the kind of LIFX selector, which picks the lights a
// request applies to.
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

// MakeSelector builds a LIFX selector string, e.g.
// MakeSelector(Group, "Living Room") gives "group:Living Room". The name is
// ignored for All.
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
