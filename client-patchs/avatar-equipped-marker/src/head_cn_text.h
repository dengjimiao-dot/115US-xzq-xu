#pragma once
#include "head_font_metrics.h"
#include "head_font_sites.h"
// Native free-function ABI: XMM0/XMM1 x/y, R8 actor, R9 context,
// stack +28 shared pair, +30/+38 native byte flags. Exact outer call 145BC590F.
using HeadNameFn=void(__fastcall*)(float,float,void*,void*,const void*,unsigned char,unsigned char);
static __declspec(thread) bool gHeadInside,gHeadActive;
static __declspec(thread) uintptr_t gHeadSource,gHeadCN;
static __declspec(thread) selectionfont::Style gHeadOriginal,gHeadEffective;
static LONG gHeadObserved,gHeadDrawObserved;
static bool HeadCharacter(void* actor){
 uintptr_t vt=0;if(!Read((uintptr_t)actor,vt))return false;
 if(vt==gBase+beastcontrol::TownTable)return true;
 for(auto rva:beastcontrol::BattleTables)if(vt==gBase+rva)return true;
 return false;
}
static void __fastcall HeadScope(float x,float y,void* actor,void* context,const void* pair,unsigned char first,unsigned char second){
 bool prior=gHeadInside;
 if(!prior){gHeadInside=HeadCharacter(actor);gHeadActive=false;}
 ((HeadNameFn)(gBase+0x5bcd540))(x,y,actor,context,pair,first,second);
 if(!prior){gHeadInside=false;gHeadActive=false;}
}
static void __fastcall HeadApply(const void* style){
 gHeadActive=false;
 if(gHeadInside&&(uintptr_t)style==gBase+0xe6ba890&&Read((uintptr_t)style,gHeadOriginal)){
  uintptr_t source=0;uint32_t family=0;float defaultSize=0;
  memcpy(&source,gHeadOriginal.bytes+0x10,8);
  if(HudNativeFont(source)&&Read(source+0x9c,family)&&family==14&&Read(source+0x4c,defaultSize)){
   // Reuse the exact national regular asset already loaded by the HUD font
   // factory. No shared original font/style, native slot or menu path changes.
   uintptr_t font=HudLoadCNFont(source);
   if(font&&headfont::Adapt(gHeadOriginal,gHeadEffective,font,defaultSize)){
    gHeadSource=source;gHeadCN=font;gHeadActive=true;
    if(InterlockedCompareExchange(&gHeadObserved,1,0)==0)Log("HEAD_CN attempt 1/3 active source=SimSun cn=NotoSansSC-Regular national_asset=hud-cn-digits.otf native_default_size=%.3f; colors/outline preserved",defaultSize);
   }
  }
 }
 ((HudApplyFn)(gBase+0x6ec0510))(gHeadActive?&gHeadEffective:style);
}
static void __fastcall HeadDraw(void* context,float x,float y,uint32_t color,const wchar_t* text,unsigned char flag){
 if(gHeadInside&&gHeadActive&&text&&HudNativeFont(gHeadSource)&&HudNativeFont(gHeadCN)){
  float oldWidth=((HudWidthFn)(gBase+0x7044c90))((void*)gHeadSource,text,-1,nullptr);
  float newWidth=((HudWidthFn)(gBase+0x7044c90))((void*)gHeadCN,text,-1,nullptr),corrected=x;
  if(headfont::Center(x,oldWidth,newWidth,corrected)){
   x=corrected;
   LONG n=InterlockedIncrement(&gHeadDrawObserved);
   if(n<=6){char utf8[512]{};WideCharToMultiByte(CP_UTF8,0,text,-1,utf8,sizeof(utf8),nullptr,nullptr);
    Log("HEAD_CN draw n=%ld text=%s old_width=%.3f cn_width=%.3f adjusted_x=%.3f y=%.3f color=%08x native_flag=%u",n,utf8,oldWidth,newWidth,x,y,color,(unsigned)flag);}
  }else{
   ((HudApplyFn)(gBase+0x6ec0570))(&gHeadOriginal);
   ((void(__fastcall*)(void*,const void*))(gBase+0x7045040))((void*)gHeadSource,&gHeadOriginal);
   gHeadActive=false;Log("HEAD_CN invalid metrics; native text retained");
  }
 }
 ((HudDrawFn)(gBase+0x6ebded0))(context,x,y,color,text,flag);
}
static void __fastcall HeadReset(){
 ((void(__fastcall*)())(gBase+0x6ec0580))();gHeadActive=false;
}
static bool InstallHeadCNText(){
 if(!gHudSlotReserved)return false;
 constexpr size_t n=sizeof(HeadFontSites)/sizeof(HeadFontSites[0]);
 unsigned char expected[n][5]{};unsigned char* relay[n]{};
 const uintptr_t wrappers[]={(uintptr_t)&HeadScope,(uintptr_t)&HeadApply,(uintptr_t)&HeadDraw,(uintptr_t)&HeadReset};
 for(size_t i=0;i<n;i++){
  const auto& s=HeadFontSites[i];expected[i][0]=0xe8;int32_t d=(int32_t)(s.target-s.site-5);memcpy(expected[i]+1,&d,4);
  if(memcmp((void*)(gBase+s.site),expected[i],5)){Log("HEAD_CN rejected native callsite index=%llu",(unsigned long long)i);return false;}
 }
 for(size_t i=0;i<n;i++){
  relay[i]=ChoiceRelay(gBase+HeadFontSites[i].site,wrappers[HeadFontSites[i].kind]);
  if(!relay[i]){for(auto p:relay)if(p)VirtualFree(p,0,MEM_RELEASE);return false;}
 }
 for(size_t i=0;i<n;i++){
  unsigned char call[5]={0xe8};int32_t d=(int32_t)((intptr_t)relay[i]-(intptr_t)(gBase+HeadFontSites[i].site+5));memcpy(call+1,&d,4);
  if(!Patch(gBase+HeadFontSites[i].site,call,5)){
   for(size_t j=0;j<=i;j++)Patch(gBase+HeadFontSites[j].site,expected[j],5);
   Log("HEAD_CN install failed; native sites restored");return false;
  }
 }
 Log("HEAD_CN attempt 1/3 installed; character-only outer scope; %llu native sites; national font with native semantic sizes",(unsigned long long)n);return true;
}
