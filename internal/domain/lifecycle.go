package domain

import (
	"encoding/json"
	"math"
	"time"
)

type LifecyclePolicy struct {
	TTLSeconds *int64 `json:"ttlSeconds,omitempty"`
}

func ParseLifecyclePolicy(data json.RawMessage) (LifecyclePolicy, error) {
	if len(data) == 0 || string(data) == "null" {
		return LifecyclePolicy{}, nil
	}
	var policy LifecyclePolicy
	if err := json.Unmarshal(data, &policy); err != nil {
		return LifecyclePolicy{}, err
	}
	return policy, nil
}

func (policy LifecyclePolicy) TTL() (time.Duration, bool) {
	if policy.TTLSeconds == nil || *policy.TTLSeconds <= 0 {
		return 0, false
	}
	if *policy.TTLSeconds > int64(math.MaxInt64/int64(time.Second)) {
		return 0, false
	}
	return time.Duration(*policy.TTLSeconds) * time.Second, true
}

func LifecycleTTL(data json.RawMessage) (time.Duration, bool) {
	policy, err := ParseLifecyclePolicy(data)
	if err != nil {
		return 0, false
	}
	return policy.TTL()
}
