#pragma once
#include <cmath>
#include <cstring>
#include <cstdint>
namespace selectionfont {
struct Style { unsigned char bytes[0x78]; };
static_assert(sizeof(Style)==0x78,"current native style size");
struct Upper { uintptr_t owner=0,effect=0;float baseSize=0,y=0;bool ready=false; };
inline bool ValidSize(float size){return std::isfinite(size)&&size>0;}
inline bool ForLower(const Upper& upper,uintptr_t owner,uintptr_t effect,float y,const Style& original,Style& result){
 if(!upper.ready||upper.owner!=owner||upper.effect!=effect||!ValidSize(upper.baseSize)||!std::isfinite(y)||!std::isfinite(upper.y)||y<=upper.y)return false;
 result=original;
 // Native 146ea9350 reads the font size at +0x3c; preserve every other byte.
 memcpy(result.bytes+0x3c,&upper.baseSize,sizeof(upper.baseSize));return true;
}
}
