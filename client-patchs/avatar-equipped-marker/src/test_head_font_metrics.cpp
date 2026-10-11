#include <cassert>
#include <cstdio>
#include "head_font_metrics.h"
int main(){
 selectionfont::Style original{},effective{};
 for(size_t i=0;i<sizeof(original.bytes);i++)original.bytes[i]=(unsigned char)(i*17+3);
 float zero=0;memcpy(original.bytes+0x3c,&zero,4);auto saved=original;
 assert(headfont::Adapt(original,effective,0x12345678,12));
 float size=0;memcpy(&size,effective.bytes+0x3c,4);assert(size==12);
 assert(!memcmp(&saved,&original,sizeof(saved)));
 for(size_t i=0;i<sizeof(original.bytes);i++)if(!(i>=0x10&&i<0x18)&&!(i>=0x3c&&i<0x40))assert(effective.bytes[i]==original.bytes[i]);
 float explicitSize=14.5f;memcpy(original.bytes+0x3c,&explicitSize,4);
 assert(headfont::Adapt(original,effective,0x12345678,12));memcpy(&size,effective.bytes+0x3c,4);assert(size==14.5f);
 assert(!headfont::Adapt(original,effective,0,12));assert(!headfont::Adapt(original,effective,1,NAN));
 float x=0;assert(headfont::Center(80,40,30,x));assert(x+15==100); // Original centre 80+20 stays 100.
 assert(headfont::Center(81,40,30,x));assert(x==86); // The outline's +1px pass retains that offset.
 assert(!headfont::Center(0,NAN,30,x));assert(!headfont::Center(0,-1,30,x));assert(!headfont::Center(0,40,INFINITY,x));
 puts("HEAD FONT VERIFIED default/explicit size semantics, native colors/outline unchanged, centre and outline offset preserved; invalid metrics rejected");
}
