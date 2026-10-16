#pragma once
#include <stdint.h>
#include <stddef.h>
// Current 515fefa1 PE, native job factory 14645b320 and seventeen constructors.
// All inherit 145786d20 -> 145c640a0; owned input/AI fields belong to that base.
// These are ABI guards, not a parallel profession or skill content table.
struct RobotActorClass {uintptr_t vtable;size_t size;uintptr_t constructor;uintptr_t destructor;};
static constexpr RobotActorClass kRobotActorClasses[]={
 {0xad21370,0x1a1f8,0x64296f0,0x642a970},
 {0xad04df8,0x19b00,0x63633f0,0x6363da0},
 {0xad09528,0x1ce48,0x63717a0,0x6372ac0},
 {0xad105f0,0x1a590,0x63c3be0,0x63c4ac0},
 {0xad18e58,0x1a090,0x6409160,0x6409b40},
 {0x9229fa8,0x1cdd0,0x33d5b0,0x33ee40},
 {0xad25770,0x19c00,0x64487d0,0x6448c90},
 {0xacfeb58,0x1a640,0x6301f20,0},
 {0xa3c92b8,0x195e8,0x4799810,0},
 {0xa03d270,0x1a358,0x3b0fb90,0},
 {0xa161618,0x19650,0x3e54a00,0},
 {0xa67dfd0,0x19730,0x4e4df80,0},
 {0xa59ddc0,0x19620,0x4bdfc00,0},
 {0xa27e9a0,0x19640,0x4371300,0},
 {0xa0419a0,0x19738,0x3b32fb0,0},
 {0x9e7bf20,0x19650,0x36352a0,0},
 {0x95ad718,0x19780,0x1030b50,0}
};
// Extra subclasses use their proven common 145787110 teardown barrier.
static int RobotActorClassIndex(uintptr_t base,uintptr_t vtable) {
 for(unsigned i=0;i<sizeof(kRobotActorClasses)/sizeof(*kRobotActorClasses);i++)
  if(vtable==base+kRobotActorClasses[i].vtable)return (int)i;
 return -1;
}
