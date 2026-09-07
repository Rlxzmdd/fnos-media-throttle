package downloader

import (
	"encoding/json"
	"sort"
	"strconv"
)

func unwrapFNOSMap(response map[string]any) map[string]any {
	data, _ := response["data"].(map[string]any)
	for _, key := range []string{"rsp", "body"} {
		if value, ok := data[key].(map[string]any); ok {
			return value
		}
	}
	if block, ok := data["block"].(map[string]any); ok {
		switch value := block["data"].(type) {
		case map[string]any:
			return value
		case string:
			var decoded map[string]any
			if json.Unmarshal([]byte(value), &decoded) == nil {
				return decoded
			}
		}
	}
	if len(data) > 0 {
		return data
	}
	return response
}

func mergeFNOSMaps(left, right map[string]any) map[string]any {
	result := map[string]any{}
	for key, value := range left {
		result[key] = value
	}
	for key, value := range right {
		result[key] = value
	}
	return result
}

func sortedFNOSKeys(value map[string]any) []string {
	keys := make([]string, 0, len(value))
	for key := range value {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func int64Value(value any) int64 {
	switch number := value.(type) {
	case float64:
		return int64(number)
	case json.Number:
		result, _ := number.Int64()
		return result
	case int64:
		return number
	case int:
		return int64(number)
	case string:
		result, _ := strconv.ParseInt(number, 10, 64)
		return result
	default:
		return 0
	}
}

func stringValue(value any) string {
	text, _ := value.(string)
	return text
}

func boolValue(value any) bool {
	switch item := value.(type) {
	case bool:
		return item
	case string:
		value, _ := strconv.ParseBool(item)
		return value
	case float64:
		return item != 0
	default:
		return false
	}
}
