#include "head_font_slot.h"
#include <assert.h>
#include <stdio.h>
struct Provider { int height; };
static Provider* Lookup(Provider* const* first,unsigned count,unsigned slot){
 return slot<count?first[slot]:nullptr;
}
int main(){
 const uintptr_t privateFont=0x1000,otherFont=0x2000;
 Provider head{21},hud{11};Provider* first[41]{};
 first[40]=&head;
 // First entry: HUD has never been opened. The native default addresses a
 // null slot, while the private registration/metrics/key identity is 40.
 assert(Lookup(first,41,37)==nullptr);
 for(uintptr_t site:headfontslot::NumberCalls){
  assert(site!=0x7043ba6);
  auto loading=headfontslot::Resolve(privateFont,37,privateFont,0);
  auto ready=headfontslot::Resolve(privateFont,37,0,privateFont);
  assert(loading==40&&ready==40);
  assert(Lookup(first,41,loading)->height==21);
  assert(Lookup(first,41,ready)->height==21);
 }
 // Later HUD creation must neither hide the bug nor redirect head metrics.
 first[37]=&hud;
 assert(Lookup(first,41,headfontslot::Resolve(privateFont,37,0,privateFont))==&head);
 assert(Lookup(first,41,headfontslot::Resolve(otherFont,37,0,privateFont))==&hud);
 // Native fonts, failed initialization, null pointers and nonowned private
 // fonts retain their original identity.
 for(unsigned n=0;n<37;n++)assert(headfontslot::Resolve(privateFont,(unsigned char)n,privateFont,privateFont)==n);
 assert(headfontslot::Resolve(privateFont,37,0,0)==37);
 assert(headfontslot::Resolve(0,37,0,0)==37);
 assert(headfontslot::Resolve(otherFont,37,privateFont,privateFont)==37);
 assert(headfontslot::Resolve(privateFont,38,privateFont,privateFont)==38);
 puts("HEAD SLOT VERIFIED first-entry HUD absent, loaded HUD isolation, loading/ready keys, native/unknown font isolation");
}
