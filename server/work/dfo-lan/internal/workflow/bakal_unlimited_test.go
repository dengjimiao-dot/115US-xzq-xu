package workflow

import (
	"context"
	"dfolan/internal/database"
	"dfolan/internal/savecontract"
	"encoding/json"
	"fmt"
	"testing"
	"time"
)

func TestBakalUnlimitedDifferentRunsKeepQuotaAndRewardEligibility(t *testing.T) {
	now := time.Date(2026, 10, 10, 12, 0, 0, 0, time.UTC)
	state := json.RawMessage(`{"other":{"keep":42},"bakal_raid_rewards":{"week":"` + bakalWeekOf(now) + `","clears":1,"rewards":1,"plans":{}}}`)
	role := database.Character{ID: 7, AccountID: 2, ConfigVersion: savecontract.Identity(), State: state}
	svc, store := bakalTestService(t, role)
	for i := 0; i < 3; i++ {
		if e := BakalRaidAdmission(svc.Rules, store.role, now); e != nil {
			t.Fatal(e)
		}
		run := fmt.Sprintf("unlimited-%d", i)
		plan, saved, e := svc.Freeze(context.Background(), store.role, run, 3, now)
		if e != nil || !plan.Eligible {
			t.Fatal("new run lost source rewards", e)
		}
		_, saved, e = svc.Claim(context.Background(), saved, run, now)
		if e != nil {
			t.Fatal(e)
		}
		top, p, e := readBakalProgress(saved.State)
		if e != nil || p.Clears != 1 || p.Rewards != 1 || string(top["other"]) != `{"keep":42}` {
			t.Fatal("consumed quota or changed legacy save", e)
		}
		usage, e := BakalWeeklyUsage(saved, now)
		if e != nil || usage != (BakalWeeklyCounters{}) {
			t.Fatal("UI differs from unlimited admission", e)
		}
	}
}
