#pragma once
#include "selection_font_metrics.h"
// Current native Apply 147045040 and Reset 147045330, proved by live vtable.
// Capture only the font's active base size; never copy the upper style block.
static __declspec(thread) selectionfont::Upper gSelectionUpper;
static LONG gSelectionUpperObserved,gSelectionLowerObserved;
using SelectionDrawFn=void(__fastcall*)(void*,int,const void*,int,int,int,unsigned char,const void*,const void*,const void*,unsigned char);
static bool SelectionFontRead(const void* style,selectionfont::Style& out,uintptr_t& effect,float& activeSize){
 if(!style||!Read((uintptr_t)style,out))return false;
 memcpy(&effect,out.bytes+0x10,sizeof(effect));uintptr_t table=0,apply=0,reset=0;
 if(!Read(effect,table)||table!=gBase+0xb1f8498||!Read(table+0x30,apply)||apply!=gBase+0x7045040||!Read(table+0x38,reset)||reset!=gBase+0x7045330)return false;
 return Read(effect+0xa8,activeSize)&&selectionfont::ValidSize(activeSize);
}
static void __fastcall SelectionSelectedDraw(void* owner,int level,const void* name,int job,int grow,int awake,unsigned char flag,const void* row,const void* style,const void* position,unsigned char alpha){
 selectionfont::Style observed{};uintptr_t effect=0;float size=0,xy[2]={};SIZE_T got=0;
 gSelectionUpper.ready=false;
 if(SelectionFontRead(style,observed,effect,size)&&position&&ReadProcessMemory(GetCurrentProcess(),position,xy,sizeof(xy),&got)&&got==sizeof(xy)&&std::isfinite(xy[1])){
  gSelectionUpper={(uintptr_t)owner,effect,size,xy[1],true};
  if(InterlockedCompareExchange(&gSelectionUpperObserved,1,0)==0)Log("SELECTION_FONT upper native_style_preserved=1 actual_base_size=%.3f y=%.3f",size,xy[1]);
 }
 // Exact original pointer and all native arguments: upper draw stays native.
 ((SelectionDrawFn)(gBase+0x2090c0))(owner,level,name,job,grow,awake,flag,row,style,position,alpha);
}
static void __fastcall SelectionLowerDraw(void* owner,int level,const void* name,int job,int grow,int awake,unsigned char flag,const void* row,const void* style,const void* position,unsigned char alpha){
 selectionfont::Style original{},adapted{};uintptr_t effect=0;float size=0,xy[2]={};SIZE_T got=0;const void* effective=style;
 if(SelectionFontRead(style,original,effect,size)&&position&&ReadProcessMemory(GetCurrentProcess(),position,xy,sizeof(xy),&got)&&got==sizeof(xy)&&selectionfont::ForLower(gSelectionUpper,(uintptr_t)owner,effect,xy[1],original,adapted)){
  effective=&adapted;
  if(InterlockedCompareExchange(&gSelectionLowerObserved,1,0)==0){float before=0;memcpy(&before,original.bytes+0x3c,4);Log("SELECTION_FONT lower font_size=%.3f -> %.3f; upper untouched; position/color/outline/alpha and other native bytes preserved",before,gSelectionUpper.baseSize);}
 }
 ((SelectionDrawFn)(gBase+0x2090c0))(owner,level,name,job,grow,awake,flag,row,effective,position,alpha);
}
static bool InstallSelectionTextStyle(){
 const uintptr_t sites[]={0x20ff80,0x212859,0x212941};unsigned char expected[3][5];
 for(int i=0;i<3;i++){expected[i][0]=0xe8;int32_t delta=(int32_t)(0x2090c0-(sites[i]+5));memcpy(expected[i]+1,&delta,4);if(memcmp((void*)(gBase+sites[i]),expected[i],5)){Log("SELECTION_FONT rejected native callsite %d",i);return false;}}
 unsigned char* relays[3]={};
 for(int i=0;i<3;i++){relays[i]=ChoiceRelay(gBase+sites[i],i==0?(uintptr_t)&SelectionSelectedDraw:(uintptr_t)&SelectionLowerDraw);if(!relays[i]){for(auto p:relays)if(p)VirtualFree(p,0,MEM_RELEASE);Log("SELECTION_FONT relay allocation failed");return false;}}
 for(int i=0;i<3;i++){unsigned char call[5]={0xe8};int32_t delta=(int32_t)((intptr_t)relays[i]-(intptr_t)(gBase+sites[i]+5));memcpy(call+1,&delta,4);if(!Patch(gBase+sites[i],call,5)){for(int j=i;j>=0;j--)Patch(gBase+sites[j],expected[j],5);Log("SELECTION_FONT patch failed; rollback attempted");return false;}}
 Log("SELECTION_FONT installed attempt 3/3; read-only upper, lower explicit source font size only");return true;
}
