#pragma once
#include "selection_font_metrics.h"
namespace headfont {
inline bool Adapt(const selectionfont::Style& original,selectionfont::Style& out,uintptr_t font,float defaultSize){
 float explicitSize=0;memcpy(&explicitSize,original.bytes+0x3c,4);
 if(!font||!selectionfont::ValidSize(defaultSize)||!std::isfinite(explicitSize)||explicitSize<0)return false;
 if(explicitSize!=0&&!selectionfont::ValidSize(explicitSize))return false;
 out=original;memcpy(out.bytes+0x10,&font,8);
 if(explicitSize==0)memcpy(out.bytes+0x3c,&defaultSize,4);
 return true;
}
inline bool Center(float oldLeft,float oldWidth,float newWidth,float& newLeft){
 if(!std::isfinite(oldLeft)||!std::isfinite(oldWidth)||!std::isfinite(newWidth)||oldWidth<0||newWidth<0||oldWidth>4096||newWidth>4096)return false;
 newLeft=oldLeft+(oldWidth-newWidth)*0.5f;return std::isfinite(newLeft);
}
}
