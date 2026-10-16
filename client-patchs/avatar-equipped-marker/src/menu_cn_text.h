#pragma once
#include "menu_font_scope.h"
// Live: main menu window vtable 14977EF80, root +D0; widget parent +110.
// CNUIControlText vtable 14B1C9490, DrawText +2B8, owned style +358.
static bool gMenuSlotsReserved;
static __declspec(thread) uintptr_t gMenuRoot;
static __declspec(thread) unsigned char gMenuCreatingSlot;
static __declspec(thread) bool gMenuImportActive;
static uintptr_t gMenuFonts[2];static LONG gMenuFontInit[2];
static uintptr_t gMenuObserved[128];static unsigned gMenuObservedCount;
static void MenuReserveNativeSlotsAtAttach(){
 if(!gHudSlotReserved)return;
 uintptr_t base=(uintptr_t)GetModuleHandleW(nullptr);
 static const unsigned char init[]={0xba,0x26,0,0,0,0x44,0x8b,0xc2,0x89,0x5c,0x24,0x20,0x48,0x8b,0xc8,0x4c,0x8b,0x10,0x44,0x8d,0x4b,0x03,0x41,0xff,0x12};
 if(memcmp((void*)(base+0x700c156),init,sizeof(init)))return;
 DWORD old=0;if(!VirtualProtect((void*)(base+0x700c156),5,PAGE_EXECUTE_READWRITE,&old))return;
 // Native allocator reserves 0..36, HUD 37, menu Sans 38 and Serif 39.
 *(unsigned char*)(base+0x700c157)=0x28;
 FlushInstructionCache(GetCurrentProcess(),(void*)(base+0x700c156),5);
 DWORD ignored=0;if(!VirtualProtect((void*)(base+0x700c156),5,old,&ignored)){
  *(unsigned char*)(base+0x700c157)=0x26;FlushInstructionCache(GetCurrentProcess(),(void*)(base+0x700c156),5);return;
 }gMenuSlotsReserved=true;
}
static unsigned char __fastcall MenuFontNumber(void* font){
 unsigned char original=((unsigned char(__fastcall*)(void*))(gBase+0x6ec0930))(font);
 if(gMenuCreatingSlot>=38&&gMenuCreatingSlot<=40&&original==37)return gMenuCreatingSlot;
 return original;
}
static void __fastcall MenuFontPaths(void* manager,const void* desc,void* key){
 if(gMenuImportActive&&desc&&key){
  uint32_t name=0,type=0;int tracking=0;wchar_t path[512]{};
  Read((uintptr_t)desc+0x44,name);Read((uintptr_t)desc+0x40,type);
  Read((uintptr_t)key+0x10,tracking);HudDiagWString((uintptr_t)desc,path,512);
  if(name==12&&type==2&&tracking==-50&&!wcscmp(path,L"Fonts/menu-cn-regular.otf")){
   int nativeChineseTracking=0;memcpy((unsigned char*)key+0x10,&nativeChineseTracking,4);
   Log("MENU_CN private Sans tracking -50 -> 0; Chinese native import branch");
  }
 }
 // Retain the existing HUD path wrapper and diagnostics without changing it.
 HudCNPaths(manager,desc,key);
}
static bool MenuSlotAvailable(uintptr_t font,unsigned char expected){
 uintptr_t manager=0,backend=0,vt=0;unsigned char slot=0;
 if(!Read(gBase+0xf1c0ca0,manager)||!Read(gBase+0xf3bb2c0,backend)||!Read(backend,vt)||vt!=gBase+0xb67e640)return false;
 void* value=((void*(__fastcall*)(void*,const void*))(gBase+0x700b4b0))((void*)(manager+0x138),&font);
 if(!Read((uintptr_t)value,slot)||slot!=expected)return false;
 for(unsigned side:{8u,16u}){uintptr_t importer=0,first=0,last=0,mgr=0;uint32_t count=0;
  if(!Read(backend+side,importer)||!Read(importer,mgr)||!Read(importer+0x60,first)||!Read(importer+0x68,last)||
     !Read(mgr+0x28,count)||last<first||((last-first)/8!=40&&(last-first)/8!=41)||count!=(last-first)/8)return false;
 }Log("MENU_CN dedicated font slot=%u both native capacities validated",slot);return true;
}
static uintptr_t MenuLoadCNFont(uintptr_t source,unsigned which){
 if(which>1||!gMenuSlotsReserved)return 0;if(gMenuFonts[which])return gMenuFonts[which];
 if(InterlockedCompareExchange(&gMenuFontInit[which],1,0)!=0)return 0;
 alignas(16) unsigned char desc[0x70]{},original[0x70]{};SIZE_T got=0;
 if(!HudNativeFont(source)||!ReadProcessMemory(GetCurrentProcess(),(void*)(source+0x58),original,sizeof(original),&got)||got!=sizeof(original))return 0;
 const wchar_t* file=which?L"Fonts/menu-cn-serifbold.otf":L"Fonts/menu-cn-regular.otf";
 const wchar_t* family=which?L"Noto SerifSC Bold":L"Noto Sans SC Regular";
 wchar_t root[MAX_PATH]{};if(!GetModuleFileNameW(gSelf,root,MAX_PATH))return 0;wchar_t* slash=wcsrchr(root,L'\\');if(!slash)return 0;
 wcscpy_s(slash+1,MAX_PATH-(slash+1-root),file);
 if(GetFileAttributesW(root)==INVALID_FILE_ATTRIBUTES){Log("MENU_CN private font missing index=%u",which);return 0;}
 ((void*(__fastcall*)(void*))(gBase+0x6ebf9b0))(desc);
 struct Owned { unsigned char* p;~Owned(){((void(__fastcall*)(void*))(gBase+0x14c710))(p+0x20);((void(__fastcall*)(void*))(gBase+0x14c710))(p);} } owned{desc};
 memcpy(desc+0x40,original+0x40,0x30);
 alignas(16) unsigned char fs[0x20]{},ns[0x20]{};uintptr_t ptr=(uintptr_t)file;size_t n=wcslen(file);
 memcpy(fs,&ptr,8);memcpy(fs+0x10,&n,8);memcpy(fs+0x18,&n,8);
 ptr=(uintptr_t)family;n=wcslen(family);memcpy(ns,&ptr,8);memcpy(ns+0x10,&n,8);memcpy(ns+0x18,&n,8);
 ((void(__fastcall*)(void*,const void*))(gBase+0x6ec0eb0))(desc,fs);
 ((void(__fastcall*)(void*,const void*))(gBase+0x6ec0ed0))(desc,ns);
 uint32_t name=0;memcpy(&name,desc+0x44,4);if(name!=(which?16u:12u)){Log("MENU_CN family rejected index=%u name=%u",which,name);return 0;}
 uintptr_t factory=0,table=0,create=0,font=0;
 if(!Read(gBase+0xf1ed278,factory)||!Read(factory,table)||!Read(table+8,create)||create<gBase||create>=gBase+0xc000000)return 0;
 gMenuCreatingSlot=(unsigned char)(38+which);
 int result=((int(__fastcall*)(void*,int,uintptr_t*))create)((void*)factory,5,&font);
 gMenuCreatingSlot=0;
 if(result<0||!HudNativeFont(font)||!MenuSlotAvailable(font,(unsigned char)(38+which))){Log("MENU_CN font factory/slot rejected index=%u",which);return 0;}
 gMenuImportActive=true;result=((int(__fastcall*)(void*,const void*))(gBase+0x7044220))((void*)font,desc);gMenuImportActive=false;
 Log("MENU_CN LoadFont index=%u name=%u result=%08x",which,name,(unsigned)result);
 if(result!=1)return 0;
 float width=((HudWidthFn)(gBase+0x7044c90))((void*)font,L"\u4efb\u52a1",-1,nullptr);
 if(!std::isfinite(width)||width<=0){Log("MENU_CN private font metrics rejected index=%u",which);return 0;}
 Log("MENU_CN private font ready index=%u width=%.3f",which,width);gMenuFonts[which]=font;return font;
}
static void __fastcall MenuWindowDraw(void* window){
 uintptr_t vt=0,root=0,previous=gMenuRoot;
 if(Read((uintptr_t)window,vt)&&vt==gBase+0x977ef80&&Read((uintptr_t)window+0xd0,root))gMenuRoot=root;
 ((void(__fastcall*)(void*))(gBase+0x67a5e70))(window);gMenuRoot=previous;
}
static void __fastcall MenuTextDraw(void* widget){
 uintptr_t obj=(uintptr_t)widget,vt=0;
 if(!gMenuRoot||!Read(obj,vt)||vt!=gBase+0xb1c9490||!menufont::Within(obj,gMenuRoot,[](uintptr_t p,uintptr_t& parent){return Read(p+0x110,parent);})){((void(__fastcall*)(void*))(gBase+0x6ef8550))(widget);return;}
 selectionfont::Style original{},effective{};uintptr_t source=0;uint32_t name=0;
 if(!Read(obj+0x358,original)){((void(__fastcall*)(void*))(gBase+0x6ef8550))(widget);return;}
 memcpy(&source,original.bytes+0x10,8);
 if(!HudNativeFont(source)||!Read(source+0x9c,name)){((void(__fastcall*)(void*))(gBase+0x6ef8550))(widget);return;}
 // Match observed native menu families. All other families retain native text.
 if(name!=14&&name!=12&&name!=16){((void(__fastcall*)(void*))(gBase+0x6ef8550))(widget);return;}
 uintptr_t font=MenuLoadCNFont(source,name==16?1:0);
 if(!font){((void(__fastcall*)(void*))(gBase+0x6ef8550))(widget);return;}
 menufont::Adapt(&original,&effective,font);
 if(!Writable(obj+0x358,sizeof(effective))){((void(__fastcall*)(void*))(gBase+0x6ef8550))(widget);return;}
 bool observed=false;for(unsigned i=0;i<gMenuObservedCount;i++)if(gMenuObserved[i]==obj)observed=true;
 if(!observed&&gMenuObservedCount<128){gMenuObserved[gMenuObservedCount++]=obj;float size=0;memcpy(&size,original.bytes+0x3c,4);wchar_t text[128]{};HudDiagWString(obj+0x3d0,text,128);
  Log("MENU_CN menu-only text font=%u->%u size=%.3f text=%ls; other style bytes preserved",name,name==16?16u:12u,size,text);
 }
 // The native draw measures and renders using the same scoped font. Restore
 // this widget's exact style after Reset; no shared native font is modified.
 memcpy((void*)(obj+0x358),&effective,sizeof(effective));
 ((void(__fastcall*)(void*))(gBase+0x6ef8550))(widget);
 memcpy((void*)(obj+0x358),&original,sizeof(original));
}
static bool InstallMenuCNText(){
 if(!gMenuSlotsReserved){Log("MENU_CN extra native slots unavailable; menu retained");return false;}
 uintptr_t windowSlot=gBase+0x977ef80+0x40,textSlot=gBase+0xb1c9490+0x2b8;
 uintptr_t windowTarget=gBase+0x67a5e70,textTarget=gBase+0x6ef8550;
 uintptr_t sites[]={0x7043ba6};unsigned char expected[5]={0xe8};int32_t d=(int32_t)(0x6ec0930-sites[0]-5);memcpy(expected+1,&d,4);
 if(*(uintptr_t*)windowSlot!=windowTarget||*(uintptr_t*)textSlot!=textTarget||memcmp((void*)(gBase+sites[0]),expected,5))return false;
 unsigned char* relay=ChoiceRelay(gBase+sites[0],(uintptr_t)&MenuFontNumber);if(!relay)return false;
 unsigned char call[5]={0xe8};d=(int32_t)((intptr_t)relay-(intptr_t)(gBase+sites[0]+5));memcpy(call+1,&d,4);
 // Replace only the already-installed HUD import CALL relay with a delegating
 // wrapper, preserving its behavior for every non-menu load.
 uintptr_t pathSite=gBase+0x700c00f;unsigned char beforePath[5]{};memcpy(beforePath,(void*)pathSite,5);
 if(beforePath[0]!=0xe8){VirtualFree(relay,0,MEM_RELEASE);return false;}
 unsigned char* pathRelay=ChoiceRelay(pathSite,(uintptr_t)&MenuFontPaths);if(!pathRelay){VirtualFree(relay,0,MEM_RELEASE);return false;}
 unsigned char pathCall[5]={0xe8};d=(int32_t)((intptr_t)pathRelay-(intptr_t)(pathSite+5));memcpy(pathCall+1,&d,4);
 uintptr_t newWindow=(uintptr_t)&MenuWindowDraw,newText=(uintptr_t)&MenuTextDraw;
 if(!Patch(gBase+sites[0],call,5)||!Patch(pathSite,pathCall,5)||!Patch(windowSlot,&newWindow,8)||!Patch(textSlot,&newText,8)){
  Patch(gBase+sites[0],expected,5);Patch(pathSite,beforePath,5);Patch(windowSlot,&windowTarget,8);Patch(textSlot,&textTarget,8);Log("MENU_CN install failed; menu sites restored");return false;
 }Log("MENU_CN attempt 2/3 installed; main menu scoped; private font slots 38/39; native text sizes preserved");return true;
}
