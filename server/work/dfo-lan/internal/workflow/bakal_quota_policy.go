package workflow

// BakalUnlimitedWeeklyRuns is the owner-requested server policy (2026-10-10):
// do not consume Bakal entry/reward quota. PVF limits remain source data;
// reward eligibility, quantities, ownership and per-run event keys are unchanged.
// Historical counters/plans remain compatible and are not deleted by projection.
const BakalUnlimitedWeeklyRuns = true
