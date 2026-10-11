#pragma once
// Native 14304AF1D..14304B0CE: HUD selection 0 draws fame only.
// Factory enum 5 and font descriptor come from 1459996CB/147044220.
// Live proof: extra font maps to slot 37; original vectors hold slots 0..36.
// Native 14700C156 sets both constructor counts. Reserve through that path.
#include "hud_font_load_trace.h"
static bool gHudCNTrackingRestored;
static void __fastcall HudCNPaths(void* manager,const void* desc,void* key){
 // Native ImportFont 14700BF99..14700C004: US codepage 437 applies
 // -50 per-mille to type 1/2; Chinese codepage 936 leaves Noto name 12 at 0.
 // Change only the import key of our private font before native caches exist.
 if(gHudDiagActive&&desc&&key){
  uint32_t name=0,type=0;int tracking=0;wchar_t path[512]{};
  Read((uintptr_t)desc+0x44,name);Read((uintptr_t)desc+0x40,type);
  Read((uintptr_t)key+0x10,tracking);HudDiagWString((uintptr_t)desc,path,512);
  if(name==12&&type==2&&tracking==-50&&!wcscmp(path,L"Fonts/hud-cn-digits.otf")){
   int nativeChineseTracking=0;memcpy((unsigned char*)key+0x10,&nativeChineseTracking,4);
   gHudCNTrackingRestored=true;
   Log("HUD_CN private import tracking %d -> %d per-mille; native Chinese Noto branch",tracking,nativeChineseTracking);
  }else Log("HUD_CN tracking fingerprint rejected name=%u type=%u tracking=%d",name,type,tracking);
 }
 HudDiagPaths(manager,desc,key);
}
static bool gHudSlotReserved;
static void HudReserveNativeSlotAtAttach(){
 uintptr_t base=(uintptr_t)GetModuleHandleW(nullptr);wchar_t host[MAX_PATH]{};
 if(!GetModuleFileNameW(nullptr,host,MAX_PATH))return;
 const wchar_t* file=wcsrchr(host,L'\\');file=file?file+1:host;if(_wcsicmp(file,L"DFO.exe"))return;
 auto dos=(IMAGE_DOS_HEADER*)base;if(dos->e_magic!=IMAGE_DOS_SIGNATURE)return;
 auto nt=(IMAGE_NT_HEADERS64*)(base+dos->e_lfanew);
 if(nt->Signature!=IMAGE_NT_SIGNATURE||nt->OptionalHeader.Magic!=IMAGE_NT_OPTIONAL_HDR64_MAGIC||nt->OptionalHeader.SizeOfImage<=0xf3bb2c8)return;
 static const unsigned char init[]={0xba,0x25,0,0,0,0x44,0x8b,0xc2,0x89,0x5c,0x24,0x20,0x48,0x8b,0xc8,0x4c,0x8b,0x10,0x44,0x8d,0x4b,0x03,0x41,0xff,0x12};
 static const unsigned char extra[]={0x48,0x3b,0x0d,0xa1,0x03,0x30,0x08,0x0f,0x95,0xc0,0x04,0x24,0xc3};
 if(memcmp((void*)(base+0x700c156),init,sizeof(init))||memcmp((void*)(base+0x6ec0a50),extra,sizeof(extra)))return;
 uintptr_t backend=*(uintptr_t*)(base+0xf3bb2c0);
 if(backend&&(*(uintptr_t*)(backend+8)||*(uintptr_t*)(backend+16)))return;
 // Loader-lock safe: only validate bytes and patch the constructor's count.
 // Both importer and manager vectors are built by the original allocator.
 DWORD old=0;if(!VirtualProtect((void*)(base+0x700c156),5,PAGE_EXECUTE_READWRITE,&old))return;
 *(unsigned char*)(base+0x700c157)=0x26;FlushInstructionCache(GetCurrentProcess(),(void*)(base+0x700c156),5);
 DWORD ignored=0;if(!VirtualProtect((void*)(base+0x700c156),5,old,&ignored)){
  *(unsigned char*)(base+0x700c157)=0x25;FlushInstructionCache(GetCurrentProcess(),(void*)(base+0x700c156),5);return;
 }
 gHudSlotReserved=true;
}
static bool HudSlotAvailable(uintptr_t font){
 uintptr_t manager=0,backend=0,vt=0;unsigned char slot=0;
 if(!Read(gBase+0xf1c0ca0,manager)||!Read(gBase+0xf3bb2c0,backend)||!Read(backend,vt)||vt!=gBase+0xb67e640)return false;
 void* value=((void*(__fastcall*)(void*,const void*))(gBase+0x700b4b0))((void*)(manager+0x138),&font);
 if(!Read((uintptr_t)value,slot)||slot!=37)return false;
 for(unsigned side:{8u,16u}){uintptr_t importer=0,first=0,last=0,mgr=0;uint32_t count=0;
  if(!Read(backend+side,importer)||!Read(importer,mgr)||!Read(importer+0x60,first)||!Read(importer+0x68,last)||
     !Read(mgr+0x28,count)||last<first||((last-first)/8!=38&&(last-first)/8!=40&&(last-first)/8!=41)||count!=(last-first)/8)return false;
  Log("HUD_CN reserved slot=%u side=%u importer_slots=%llu manager_slots=%u",slot,side,(unsigned long long)((last-first)/8),count);
 }return true;
}
using HudApplyFn=void(__fastcall*)(const void*);
using HudDrawFn=void(__fastcall*)(void*,float,float,uint32_t,const wchar_t*,unsigned char);
using HudWidthFn=float(__fastcall*)(void*,const wchar_t*,int,const void*);
static uintptr_t gHudCNFont;
static LONG gHudCNInit,gHudCNObserved;
static __declspec(thread) selectionfont::Style gHudCNStyle;
static __declspec(thread) selectionfont::Style gHudOriginalStyle;
static __declspec(thread) uintptr_t gHudSourceFont;
static __declspec(thread) bool gHudCNReady;
struct HudInkBounds { float left=0,right=0;bool valid=false; };
static HudInkBounds HudNativeDrawBounds(uintptr_t font,const wchar_t* text){
 // 148760A60 measures importer+8 only. Read glyph advances and ink bounds
 // through both original managers; keep the larger envelope for placement.
 HudInkBounds result{};uintptr_t backend=0;
 if(!Read(gBase+0xf3bb2c0,backend)||!text)return result;
 auto raw=((void*(__fastcall*)(void*))(gBase+0x7044920))((void*)font);
 int key[7]{};SIZE_T got=0;
 if(!ReadProcessMemory(GetCurrentProcess(),raw,key,sizeof(key),&got)||got!=sizeof(key)||key[0]!=37)return result;
 for(unsigned side:{8u,16u}){
  uintptr_t importer=0,manager=0;
  if(!Read(backend+side,importer)||!Read(importer,manager))return {};
  auto group=((void*(__fastcall*)(void*,int))(gBase+0x87ab8c0))((void*)importer,key[0]);
  uint32_t family=0;if(!Read((uintptr_t)group+0x10,family))return {};
  alignas(16) unsigned char rendererKey[0x30]{};
  ((void(__fastcall*)(void*,int,int,int,int,int,int,int))(gBase+0x87a3d50))(rendererKey,family,key[1],key[2],key[3],key[4],0,key[5]);
  auto renderer=((void*(__fastcall*)(void*,void*,void*))(gBase+0x87a7f50))((void*)manager,group,rendererKey);
  if(!renderer)return {};
  float pen=0,left=0,right=0;size_t n=0;
  for(;text[n]&&n<32;n++){
   if((text[n]<L'0'||text[n]>L'9')&&text[n]!=L',')return {};
   auto glyph=((void*(__fastcall*)(void*,unsigned short,unsigned char))(gBase+0x87a75f0))(renderer,text[n],0);
   float advance=0,offset=0,width=0;
   if(!Read((uintptr_t)glyph+0x18,advance)||!Read((uintptr_t)glyph+0x20,offset)||!Read((uintptr_t)glyph+0x2c,width)||
      !std::isfinite(advance)||!std::isfinite(offset)||!std::isfinite(width)||advance<=0||width<0)return {};
   left=(std::min)(left,pen+offset);right=(std::max)(right,pen+offset+width);
   pen+=advance;right=(std::max)(right,pen);
  }
  if(!n||n==32)return {};
  result.left=(std::min)(result.left,left);result.right=(std::max)(result.right,right);
 }result.valid=result.right>result.left;return result;
}
static bool HudNativeFont(uintptr_t p){uintptr_t t=0;return Read(p,t)&&t==gBase+0xb1f8498;}
static uintptr_t HudLoadCNFont(uintptr_t source){
 if(gHudCNFont)return gHudCNFont;
 if(InterlockedCompareExchange(&gHudCNInit,1,0)!=0)return 0;
 if(!gHudSlotReserved){Log("HUD_CN native slot not reserved; original HUD retained");return 0;}
 alignas(16) unsigned char desc[0x70]={},original[0x70]={};SIZE_T got=0;
 if(!HudNativeFont(source)||!ReadProcessMemory(GetCurrentProcess(),(void*)(source+0x58),original,sizeof(original),&got)||got!=sizeof(original)){Log("HUD_CN font descriptor rejected");return 0;}
 wchar_t root[MAX_PATH]={};GetModuleFileNameW(gSelf,root,MAX_PATH);wchar_t* slash=wcsrchr(root,L'\\');
 if(!slash)return 0;wcscpy_s(slash+1,MAX_PATH-(slash+1-root),L"Fonts\\hud-cn-digits.otf");
 if(GetFileAttributesW(root)==INVALID_FILE_ATTRIBUTES){Log("HUD_CN private font missing");return 0;}
 // Construct an owned descriptor and resolve the actual registered family.
 // The live HUD source is SimSun (name 14). Resolve the CFF asset as Noto SC.
 ((void*(__fastcall*)(void*))(gBase+0x6ebf9b0))(desc);
 struct OwnedDesc { unsigned char* p;~OwnedDesc(){((void(__fastcall*)(void*))(gBase+0x14c710))(p+0x20);((void(__fastcall*)(void*))(gBase+0x14c710))(p);} } owned{desc};
 memcpy(desc+0x40,original+0x40,0x30);
 static const wchar_t file[]=L"Fonts/hud-cn-digits.otf";
 static const wchar_t family[]=L"Noto Sans SC Regular";
 alignas(16) unsigned char pathString[0x20]={},familyString[0x20]={};
 uintptr_t ptr=(uintptr_t)file;size_t n=wcslen(file);memcpy(pathString,&ptr,8);memcpy(pathString+0x10,&n,8);memcpy(pathString+0x18,&n,8);
 ptr=(uintptr_t)family;n=wcslen(family);memcpy(familyString,&ptr,8);memcpy(familyString+0x10,&n,8);memcpy(familyString+0x18,&n,8);
 ((void(__fastcall*)(void*,const void*))(gBase+0x6ec0eb0))(desc,pathString);
 ((void(__fastcall*)(void*,const void*))(gBase+0x6ec0ed0))(desc,familyString);
 uint32_t nativeName=0,oldName=0;memcpy(&nativeName,desc+0x44,4);memcpy(&oldName,original+0x44,4);
 Log("HUD_CN descriptor family resolved native_name=%u source_name=%u",nativeName,oldName);
 if(nativeName==0x25){Log("HUD_CN registered family missing; no guessed font identity");return 0;}
 uintptr_t factory=0,table=0,create=0,font=0;
 if(!Read(gBase+0xf1ed278,factory)||!Read(factory,table)||!Read(table+8,create)||create<gBase||create>=gBase+0xc000000){Log("HUD_CN native factory rejected");return 0;}
 int result=((int(__fastcall*)(void*,int,uintptr_t*))create)((void*)factory,5,&font);
 if(result<0||!HudNativeFont(font)){Log("HUD_CN native create failed result=%d",result);return 0;}
 if(!HudSlotAvailable(font)){Log("HUD_CN native slot validation failed; original HUD retained");return 0;}
 gHudDiagActive=true;
 result=((int(__fastcall*)(void*,const void*))(gBase+0x7044220))((void*)font,desc);
 gHudDiagActive=false;
 Log("HUD_CN LoadFont result=%08x name=%u",(unsigned)result,nativeName);
 if(result!=1){Log("HUD_CN native font load failed; original HUD retained");return 0;}
 if(!gHudCNTrackingRestored){Log("HUD_CN spacing restoration not validated; original HUD retained");return 0;}
 float oldWidth=((HudWidthFn)(gBase+0x7044c90))((void*)source,L"144,951",-1,nullptr);
 float newWidth=((HudWidthFn)(gBase+0x7044c90))((void*)font,L"144,951",-1,nullptr);
 Log("HUD_CN private font loaded source=%p cn=%p widths=%.3f/%.3f",(void*)source,(void*)font,oldWidth,newWidth);
 if(!std::isfinite(oldWidth)||!std::isfinite(newWidth)||oldWidth<=0||newWidth<=0){Log("HUD_CN font metrics rejected; native HUD retained");return 0;}
 gHudCNFont=font;return font;
}
static void __fastcall HudFameApply(const void* style){
 gHudCNReady=false;gHudSourceFont=0;selectionfont::Style original{};uintptr_t source=0;
 if(style&&Read((uintptr_t)style,original)){memcpy(&source,original.bytes+0x10,8);
  uintptr_t font=HudLoadCNFont(source);
  if(font){gHudOriginalStyle=original;gHudCNStyle=original;memcpy(gHudCNStyle.bytes+0x10,&font,8);
   float smallerSize=11.0f;memcpy(gHudCNStyle.bytes+0x3c,&smallerSize,4);
   gHudSourceFont=source;gHudCNReady=true;}
 }
 ((HudApplyFn)(gBase+0x6ec0510))(gHudCNReady?&gHudCNStyle:style);
}
static void __fastcall HudFameDraw(void* context,float x,float y,uint32_t color,const wchar_t* text,unsigned char flag){
 if(gHudCNReady&&text&&HudNativeFont(gHudSourceFont)&&HudNativeFont(gHudCNFont)){
  // The native HUD centres using the old font. Recompute only its centre offset.
  float oldWidth=((HudWidthFn)(gBase+0x7044c90))((void*)gHudSourceFont,text,-1,nullptr);
  HudInkBounds bounds{};float size=11.0f;
  for(;size>=9.0f;size-=1.0f){
   memcpy(gHudCNStyle.bytes+0x3c,&size,4);
   ((void(__fastcall*)(void*,const void*))(gBase+0x7045040))((void*)gHudCNFont,&gHudCNStyle);
   bounds=HudNativeDrawBounds(gHudCNFont,text);
   // Current CN/US HUD XUI textSize is 51x12; leave 2px on each side.
   if(!bounds.valid||bounds.right-bounds.left<=47.0f)break;
  }
  if(bounds.valid&&size>=9.0f&&std::isfinite(oldWidth)&&oldWidth>0){
   x+=(oldWidth-bounds.left-bounds.right)*0.5f;
   if(InterlockedCompareExchange(&gHudCNObserved,1,0)==0)Log("HUD_CN fame fit size=%.1f source_width=%.3f ink_bounds=%.3f..%.3f max_width=47 x=%.3f y=%.3f color=%08x",size,oldWidth,bounds.left,bounds.right,x,y,color);
  }else{
   // The caller resets the style immediately after drawing; TLS retains it.
   gHudCNStyle=gHudOriginalStyle;((HudApplyFn)(gBase+0x6ec0570))(&gHudCNStyle);
   ((void(__fastcall*)(void*,const void*))(gBase+0x7045040))((void*)gHudSourceFont,&gHudCNStyle);
   if(InterlockedCompareExchange(&gHudCNObserved,1,0)==0)Log("HUD_CN fame fit rejected; source font retained");
  }
 }
 ((HudDrawFn)(gBase+0x6ebded0))(context,x,y,color,text,flag);gHudCNReady=false;
}
static bool InstallHudCNDigits(){
 if(!gHudSlotReserved){Log("HUD_CN early native initialization reserve rejected; original HUD retained");return false;}
 const uintptr_t helpers[]={0x7044920,0x87ab8c0,0x87a3d50,0x87a7f50,0x87a75f0};
 const unsigned char helperBytes[][8]={
  {0x40,0x53,0x48,0x83,0xec,0x20,0x48,0x8b},
  {0x85,0xd2,0x78,0x1c,0x48,0x8b,0x41,0x60},
  {0x40,0x53,0x48,0x83,0xec,0x20,0x48,0x8b},
  {0x40,0x55,0x56,0x57,0x41,0x56,0x41,0x57},
  {0x48,0x89,0x5c,0x24,0x08,0x48,0x89,0x74}
 };
 for(int i=0;i<5;i++)if(memcmp((void*)(gBase+helpers[i]),helperBytes[i],8)){Log("HUD_CN fit helper fingerprint rejected %d",i);return false;}
 const uintptr_t sites[]={0x304b044,0x304b09c,0x87a7d61,0x8afba16,0x700c00f,0x7044533,0x87ababf,0x87abb1a,0x700c01c};
 const uintptr_t targets[]={0x6ec0510,0x6ebded0,0x8af9270,0x8b01530,0x700d3b0,0x700bf40,0x87a7be0,0x87a7be0,0x700b4b0};
 static const unsigned char codepage[]={0xb8,0xb5,0x01,0,0,0xc3};
 if(memcmp((void*)(gBase+0x3531470),codepage,sizeof(codepage))){Log("HUD_CN codepage fingerprint rejected");return false;}
 const uintptr_t replacements[]={(uintptr_t)&HudFameApply,(uintptr_t)&HudFameDraw,(uintptr_t)&HudDiagFace,(uintptr_t)&HudDiagStream,(uintptr_t)&HudCNPaths,(uintptr_t)&HudDiagImport,(uintptr_t)&HudDiagFile,(uintptr_t)&HudDiagFile,(uintptr_t)&HudDiagSlot};
 unsigned char expected[9][5]{};unsigned char* relays[9]{};
 for(int i=0;i<9;i++){expected[i][0]=0xe8;int32_t d=(int32_t)(targets[i]-sites[i]-5);memcpy(expected[i]+1,&d,4);
  if(memcmp((void*)(gBase+sites[i]),expected[i],5)){Log("HUD_CN native callsite rejected %d",i);return false;}}
 for(int i=0;i<9;i++){relays[i]=ChoiceRelay(gBase+sites[i],replacements[i]);if(!relays[i]){for(auto r:relays)if(r)VirtualFree(r,0,MEM_RELEASE);return false;}}
 for(int i=0;i<9;i++){unsigned char call[5]={0xe8};int32_t d=(int32_t)((intptr_t)relays[i]-(intptr_t)(gBase+sites[i]+5));memcpy(call+1,&d,4);
  if(!Patch(gBase+sites[i],call,5)){for(int j=i;j>=0;j--)Patch(gBase+sites[j],expected[j],5);Log("HUD_CN install failed; native calls restored");return false;}}
 Log("HUD_CN fit candidate installed; fame size 11, native two-manager glyph bounds, max ink width 47; native counts 38");return true;
}
