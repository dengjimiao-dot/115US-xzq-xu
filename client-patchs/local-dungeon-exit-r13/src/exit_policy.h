#pragma once
#include <stdint.h>
// Native callers consume AL; the remaining RAX bits may retain prior values.
inline bool NativeMenuAllowed(uintptr_t result) { return (result & 0xff) != 0; }
// Phase 0 is the native completed fade/idle phase. States other than the
// ordinary dungeon and active transitions retain the original permission.
inline bool AllowLocalExit(bool enabled, uint16_t action, bool readable,
                           uint32_t state, uint32_t phase, bool hasDungeon,
                           bool hasPlayer, bool currentObjectMatches) {
    return enabled && (action == 22 || action == 23 || action == 30) && readable && state == 3 &&
        phase == 0 && hasDungeon && hasPlayer && currentObjectMatches;
}
