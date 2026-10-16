#pragma once
#include "../../elite-companion-pilot/src/inline_hook.h"

// Ordinary source-backed player adapter (seventeen proven base classes). Every operation is called on the game thread.
// The actor, learned levels, equipment, identity and party remain owned by DFO.
struct NativeRobot {
    uintptr_t actor=0, owner=0, oldAI=0, input=0, skills=0, profile=0, ai=0;
    uintptr_t resource[2]={};
    float retreatRange[4]={},attackRange[4]={};
    int skillIDs[14]={-1,-1,-1,-1,-1,-1,-1,-1,-1,-1,-1,-1,-1,-1};
    bool deadInput=false;
    bool lifeDead=false;
    uint32_t deathSequence=0;
    uint64_t lifeSample=0;
    uint64_t lastHealthLog=0;
    uint64_t driveAfter=0,awakeningAfter=0;
    int lastState=-1;
    unsigned char timer[32]={};
    unsigned char oldTeam=0;
    bool ready=false, active=false;
};
bool AttachNativeRobot(uintptr_t base,NativeRobot& r,uintptr_t actor,uintptr_t owner);
void DetachNativeRobot(uintptr_t base,NativeRobot& r);
