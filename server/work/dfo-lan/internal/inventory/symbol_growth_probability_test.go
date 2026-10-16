package inventory

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// Controlled entropy verifies the execution boundary, not the native RNG.
func TestSymbolSingleColorProbabilityIncludesClickedLight(t *testing.T) {
	for color := uint32(0); color < 4; color++ {
		for _, tc := range []struct {
			name                                string
			lights, bonus, base, draw, wantRate uint32
			wantSuccess                         bool
		}{
			{"one-light-success", 1, 100, 7000, 7099, 7100, true},
			{"one-light-boundary", 1, 100, 7000, 7100, 7100, false},
			{"three-light-success", 3, 100, 7000, 7299, 7300, true},
			{"three-light-boundary", 3, 100, 7000, 7300, 7300, false},
			{"changed-source-bonus", 3, 250, 7000, 7749, 7750, true},
			{"changed-source-boundary", 3, 250, 7000, 7750, 7750, false},
			{"ten-light-cap", 10, 100, 9900, 9999, 10000, true},
		} {
			t.Run(fmt.Sprintf("color%d/%s", color, tc.name), func(t *testing.T) {
				s, state, counts := growthFixture(t)
				step := s.SymbolSockets.Rules.Steps[SymbolStepKey{0, 1}]
				step.SuccessRate = tc.base
				s.SymbolSockets.Rules.Steps[SymbolStepKey{0, 1}] = step
				s.SymbolSockets.Rules.Tuning.DuplicateButtonBonusRate = tc.bonus
				lights := make([]uint32, 10)
				for i := range lights {
					lights[i] = (color + 1) % 4
					if uint32(i) < tc.lights {
						lights[i] = color
					}
				}
				var fields map[string]json.RawMessage
				require.NoError(t, json.Unmarshal(state, &fields))
				fields["symbol_growth"], _ = json.Marshal(SymbolGrowthState{Lights: lights})
				state, _ = json.Marshal(fields)
				display, err := s.SymbolGrowthDisplay(state, counts, 100)
				require.NoError(t, err)
				var selected *SymbolGrowthOption
				for i := range display.Options {
					option := &display.Options[i]
					if option.Request.Operation == SymbolResonate && option.Request.Argument == 0 {
						selected = option
					}
				}
				require.NotNil(t, selected)
				require.Contains(t, selected.Label, fmt.Sprintf("成功率%.2f%%", float64(tc.wantRate)/100))
				calls := 0
				after, materials, _, err := s.PrepareSymbolGrowth(state, counts, selected.Request, 100, func(unit uint32) (uint32, error) {
					require.Equal(t, uint32(10000), unit)
					calls++
					if calls == 1 {
						return tc.draw, nil
					}
					return unit - 1, nil // No extra stage.
				})
				require.NoError(t, err)
				slots, err := ReadSymbolSockets(after, s.SymbolSockets)
				require.NoError(t, err)
				wantLevel := uint32(1)
				if tc.wantSuccess {
					wantLevel++
				}
				require.Equal(t, wantLevel, slots.Progress[symbolColorName(color)].Level)
				for other := uint32(0); other < 4; other++ {
					if other != color {
						require.Equal(t, uint32(1), slots.Progress[symbolColorName(other)].Level)
					}
				}
				m, err := ReadAccountMaterials(materials)
				require.NoError(t, err)
				require.Equal(t, uint32(97), m.CommonItems[30001])
				wantReward := uint32(1)
				if tc.wantSuccess {
					wantReward = 0
				}
				require.Equal(t, wantReward, m.CommonItems[30002])
				growth, err := ReadSymbolGrowth(after)
				require.NoError(t, err)
				require.Empty(t, growth.Lights)
				require.Equal(t, tc.wantSuccess, strings.Contains(growth.LastResult, "共鸣成功"))
			})
		}
	}
}
