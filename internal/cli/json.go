package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
)

func WriteJSON(w io.Writer, value any) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}

func ReadJSONObject(raw string) (json.RawMessage, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	var data json.RawMessage
	if strings.HasPrefix(raw, "@") {
		path := strings.TrimPrefix(raw, "@")
		fileData, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		data = json.RawMessage(fileData)
	} else {
		data = json.RawMessage(raw)
	}
	var object map[string]any
	if err := json.Unmarshal(data, &object); err != nil {
		return nil, fmt.Errorf("metadata must be a JSON object: %w", err)
	}
	return data, nil
}

func ParseStringList(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		clean := strings.TrimSpace(part)
		if clean != "" {
			out = append(out, clean)
		}
	}
	return out
}

func SetNonEmpty(payload map[string]any, key string, value string) {
	if strings.TrimSpace(value) != "" {
		payload[key] = value
	}
}

func SetRaw(payload map[string]any, key string, value json.RawMessage) {
	if len(value) > 0 {
		payload[key] = value
	}
}
