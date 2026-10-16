#pragma once
#include <stdint.h>
namespace avatarmarker {
// Instance identity is the native avatar bag slot, never the template ID.
inline bool IsCurrentSource(int slot,int source,int part,int capacity,uintptr_t item,uintptr_t atSlot) {
 return part>=0&&part<12&&capacity>0&&capacity<=8192&&slot>=0&&slot<capacity&&
        source==slot&&item!=0&&item==atSlot;
}
}
