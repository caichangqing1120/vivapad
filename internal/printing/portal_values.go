package printing

import "github.com/godbus/dbus/v5"

func portalValues(values map[string]dbus.Variant) map[string]any {
	result := make(map[string]any, len(values))
	for key, value := range values {
		if nested, ok := value.Value().(map[string]dbus.Variant); ok {
			result[key] = portalValues(nested)
		} else {
			result[key] = value.Value()
		}
	}
	return result
}
