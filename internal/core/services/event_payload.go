package services

import (
	"encoding/json"
	"fmt"
)

func marshalEventPayload(payload any) (json.RawMessage, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal event payload: %w", err)
	}
	return json.RawMessage(data), nil
}
