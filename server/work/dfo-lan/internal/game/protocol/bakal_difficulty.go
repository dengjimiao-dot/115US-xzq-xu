package protocol

import "fmt"

// Both native C656 mode bytes are copied into the owned raid descriptor.
// Complete hard capture20261013: options[1]=options[4]=1; normal both0.
func BakalHardMode(r RaidCreateRequest) (bool, error) {
	if r.Options[1] > 1 || r.Options[4] > 1 || r.Options[1] != r.Options[4] {
		return false, fmt.Errorf("inconsistent native Bakal difficulty flags")
	}
	return r.Options[1] == 1, nil
}
