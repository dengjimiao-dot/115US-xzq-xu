#pragma once
#include <stdint.h>
namespace headfontslot {
// Native 146EC0930 returns 37 for every dynamically created font. The
// importer registration assigns this private font to 40 instead. Apply the
// same identity to metrics and queued glyph keys, including during LoadFont.
inline unsigned char Resolve(uintptr_t font, unsigned char nativeSlot,
                             uintptr_t loadingFont, uintptr_t readyFont) {
 if(nativeSlot==37&&font&&(font==loadingFont||font==readyFont))return 40;
 return nativeSlot;
}
constexpr uintptr_t NumberCalls[]={0x704466f,0x70447f1,0x704487f,0x7044a37,0x70454d9};
}
