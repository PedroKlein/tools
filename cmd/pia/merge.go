package main

import "maps"

// deepMerge merges overlay into base recursively.
// Objects merge recursively, scalars and arrays are replaced.
func deepMerge(base, overlay map[string]any) map[string]any {
	result := make(map[string]any, len(base))
	maps.Copy(result, base)

	for k, v := range overlay {
		if baseMap, ok := result[k].(map[string]any); ok {
			if overlayMap, ok := v.(map[string]any); ok {
				result[k] = deepMerge(baseMap, overlayMap)
				continue
			}
		}

		result[k] = v
	}

	return result
}
