#pragma once
#include "head_font_slot.h"
// Functional scope: original player head-name style and native pet name apply.
// Preserve SimSun bytes, native load flags, style bytes and whole-line layout.
// Separate both outer renderer/atlas keys and private per-size FreeType faces.
static bool gHeadMonoReserved,gHeadMonoEnabled;
static uintptr_t gHeadMonoFont,gHeadMonoProviders[2];
static uintptr_t gHeadMonoLoadingFont;
static LONG gHeadMonoSlotObserved;
static LONG gHeadMonoInit,gHeadMonoObserved,gHeadMonoGlyphObserved;
static __declspec(thread) selectionfont::Style gHeadMonoStyle;
static __declspec(thread) bool gHeadMonoPlayerScope;
static __declspec(thread) bool gHeadMonoImportActive,gHeadMonoPathsReady;
static __declspec(thread) unsigned gHeadMonoImportKeys,gHeadMonoProviderCreates;
static wchar_t gHeadMonoFile[MAX_PATH];
static uintptr_t gHeadMonoFaces[128];static unsigned gHeadMonoFaceCount;
static SRWLOCK gHeadMonoFaceLock=SRWLOCK_INIT;
static void HeadMonoReserveAtAttach(){
 if(!gMenuSlotsReserved)return;
 uintptr_t base=(uintptr_t)GetModuleHandleW(nullptr);
 static const unsigned char init[]={0xba,0x28,0,0,0,0x44,0x8b,0xc2,0x89,0x5c,0x24,0x20,0x48,0x8b,0xc8,0x4c,0x8b,0x10,0x44,0x8d,0x4b,0x03,0x41,0xff,0x12};
 if(memcmp((void*)(base+0x700c156),init,sizeof(init)))return;
 DWORD old=0;if(!VirtualProtect((void*)(base+0x700c156),5,PAGE_EXECUTE_READWRITE,&old))return;
 *(unsigned char*)(base+0x700c157)=0x29;
 FlushInstructionCache(GetCurrentProcess(),(void*)(base+0x700c156),5);
 DWORD ignored=0;if(!VirtualProtect((void*)(base+0x700c156),5,old,&ignored)){
  *(unsigned char*)(base+0x700c157)=0x28;FlushInstructionCache(GetCurrentProcess(),(void*)(base+0x700c156),5);return;
 }gHeadMonoReserved=true;
}
static bool HeadMonoProvider(uintptr_t p){return p&&(p==gHeadMonoProviders[0]||p==gHeadMonoProviders[1]);}
static unsigned char __fastcall HeadMonoFontNumber(void* font){
 unsigned char nativeSlot=((unsigned char(__fastcall*)(void*))(gBase+0x6ec0930))(font);
 unsigned char slot=headfontslot::Resolve((uintptr_t)font,nativeSlot,gHeadMonoLoadingFont,gHeadMonoFont);
 if(slot!=nativeSlot&&InterlockedIncrement(&gHeadMonoSlotObserved)<=8)
  Log("HEAD_MONO SLOT_FIX font=%llx native=%u registered=%u phase=%s",(unsigned long long)(uintptr_t)font,(unsigned)nativeSlot,(unsigned)slot,gHeadMonoLoadingFont== (uintptr_t)font?"load":"ready");
 return slot;
}
static bool HeadMonoGroupRange(uintptr_t provider,uintptr_t& first,uintptr_t& last){
 return Read(provider+0x28,first)&&Read(provider+0x30,last)&&first&&last>first&&last-first<=128*8&&(last-first)%8==0;
}
static void __fastcall HeadMonoPaths(void* manager,const void* desc,void* key){
 // Preserve the installed menu/HUD import chain. Family14's native path
 // expansion retains the explicit file plus two empty fallback records (US437).
 MenuFontPaths(manager,desc,key);
 if(!gHeadMonoImportActive||!desc||!key)return;
 DWORD error=GetLastError();uint32_t family=0;uintptr_t first=0,last=0;
 wchar_t requested[128]{};HudDiagWString((uintptr_t)desc,requested,128);
 Read((uintptr_t)key,family);
 if(family!=14||wcscmp(requested,L"Fonts/head-name-simsun.ttc")||
    !Read((uintptr_t)key+0x18,first)||!Read((uintptr_t)key+0x20,last)||!first||last-first!=3*0x28){Log("HEAD_MONO path descriptor rejected family=%u file=%ls count_bytes=%llu",family,requested,(unsigned long long)(last-first));SetLastError(error);return;}
 for(uintptr_t p=first;p<last;p+=0x28){
  wchar_t path[MAX_PATH]{};HudDiagWString(p+8,path,MAX_PATH);uint32_t type=~0u,index=~0u;
  if(!Read(p,type)||!Read(p+4,index)){SetLastError(error);return;}
  bool expected=p==first?(!_wcsicmp(path,requested)||!_wcsicmp(path,gHeadMonoFile)):path[0]==0;
  Log("HEAD_MONO expanded item=%u type=%u index=%u path=%ls expected=%u",(unsigned)((p-first)/0x28),type,index,path,(unsigned)expected);
  if(!expected||type!=0||index!=0){SetLastError(error);return;}
 }
 // Retain each native file type and face index; assign only the owned native
 // wstring. The alias contains the exact original bytes, verified at install.
 for(uintptr_t p=first;p<last;p+=0x28)
  ((void*(__fastcall*)(void*,const wchar_t*,size_t))(gBase+0x14c8d0))((void*)(p+8),gHeadMonoFile,wcslen(gHeadMonoFile));
 gHeadMonoPathsReady=true;
 Log("HEAD_MONO private import paths assigned count=3 file=%ls; native file types/indices retained",gHeadMonoFile);
 SetLastError(error);
}
static void* __fastcall HeadMonoImportKey(void* key,int family,int size,int weight,int italic,int tracking){
 // Only the importer map identity changes. 1487AB8F0 copies this key to
 // 1487AB7A0 for lookup/insertion; both native hash and equality include family.
 if(gHeadMonoImportActive&&gHeadMonoPathsReady&&family==14){family=37;++gHeadMonoImportKeys;}
 return ((void*(__fastcall*)(void*,int,int,int,int,int))(gBase+0x87a3ab0))(key,family,size,weight,italic,tracking);
}
static void* __fastcall HeadMonoProviderCreate(void* provider,int index,const void* key,void* adjust){
 alignas(16) unsigned char actual[0x28]{};uint32_t family=0;
 if(gHeadMonoImportActive&&gHeadMonoPathsReady&&Read((uintptr_t)key,actual)){
  memcpy(&family,actual+0x10,4);
  if(family==37){int size=0,weight=0,italic=0,tracking=0;
   memcpy(&size,actual+0x14,4);memcpy(&weight,actual+0x18,4);memcpy(&italic,actual+0x1c,4);memcpy(&tracking,actual+0x20,4);
   ((void*(__fastcall*)(void*,int,int,int,int,int))(gBase+0x87a3ab0))(actual,14,size,weight,italic,tracking);++gHeadMonoProviderCreates;
   // 1487A4100 copies the parameters into the provider. Its family remains14
   // for native array selection; map insertion still uses the caller's key37.
   return ((void*(__fastcall*)(void*,int,const void*,void*))(gBase+0x87a4100))(provider,index,actual,adjust);
  }
 }
 return ((void*(__fastcall*)(void*,int,const void*,void*))(gBase+0x87a4100))(provider,index,key,adjust);
}
static bool HeadMonoValidateProviders(){
 uintptr_t backend=0;if(!Read(gBase+0xf3bb2c0,backend))return false;
 uintptr_t providers[2]{};
 for(unsigned side=0;side<2;side++){
  uintptr_t importer=0,slots=0,last=0,firstGroups=0,lastGroups=0;
  uint32_t family=0;
  if(!Read(backend+(side?16:8),importer)||!Read(importer+0x60,slots)||!Read(importer+0x68,last)||last-slots!=41*8||
     !Read(slots+40*8,providers[side])||!Read(providers[side]+0x10,family)||family!=14||
     !HeadMonoGroupRange(providers[side],firstGroups,lastGroups))return false;
  // Native family IDs 0..36 are reserved. Tag 37 is a private importer/renderer
  // cache identity only; actual provider family remains 14 for all array selection/fallbacks.
  for(unsigned slot=0;slot<40;slot++){
   uintptr_t original=0,of=0,ol=0;if(!Read(slots+slot*8,original))return false;
   if(!original)continue;
   if(!Read(original+0x10,family)||family==37)return false;
   if(!HeadMonoGroupRange(original,of,ol))return false;
   for(uintptr_t a=firstGroups;a<lastGroups;a+=8){uintptr_t privateGroup=0;if(!Read(a,privateGroup)||!privateGroup)return false;
    for(uintptr_t b=of;b<ol;b+=8){uintptr_t other=0;if(!Read(b,other)||privateGroup==other)return false;}
   }
  }
  for(uintptr_t a=firstGroups;a<lastGroups;a+=8){uintptr_t group=0;wchar_t path[MAX_PATH]{};
   if(!Read(a,group))return false;HudDiagWString(group+8,path,MAX_PATH);
   if(_wcsicmp(path,gHeadMonoFile))return false;
  }
 }
 memcpy(gHeadMonoProviders,providers,sizeof(providers));
 Log("HEAD_MONO private provider groups isolated side8=%llx side16=%llx; family=14 cache_tag=37 slot=40",(unsigned long long)providers[0],(unsigned long long)providers[1]);return true;
}
static uintptr_t HeadMonoLoadFont(uintptr_t source){
 if(gHeadMonoFont)return gHeadMonoFont;
 if(!gHeadMonoEnabled||!gHeadMonoReserved||InterlockedCompareExchange(&gHeadMonoInit,1,0)!=0)return 0;
 alignas(16) unsigned char desc[0x70]{},original[0x70]{};SIZE_T got=0;
 uint32_t sourceFamily=0;
 if(!HudNativeFont(source)||!Read(source+0x9c,sourceFamily)||sourceFamily!=14||
    !ReadProcessMemory(GetCurrentProcess(),(void*)(source+0x58),original,sizeof(original),&got)||got!=sizeof(original))return 0;
 const wchar_t* file=L"Fonts/head-name-simsun.ttc";
 wchar_t root[MAX_PATH]{};if(!GetModuleFileNameW(gSelf,root,MAX_PATH))return 0;wchar_t* slash=wcsrchr(root,L'\\');if(!slash)return 0;
 wcscpy_s(slash+1,MAX_PATH-(slash+1-root),file);
 if(GetFileAttributesW(root)==INVALID_FILE_ATTRIBUTES){Log("HEAD_MONO private font file missing; native names retained");return 0;}
 wcscpy_s(gHeadMonoFile,root);
 ((void*(__fastcall*)(void*))(gBase+0x6ebf9b0))(desc);
 struct Owned {unsigned char* p;~Owned(){((void(__fastcall*)(void*))(gBase+0x14c710))(p+0x20);((void(__fastcall*)(void*))(gBase+0x14c710))(p);}} owned{desc};
 memcpy(desc+0x40,original+0x40,0x30);
 alignas(16) unsigned char fs[0x20]{};uintptr_t ptr=(uintptr_t)file;size_t n=wcslen(file);
 memcpy(fs,&ptr,8);memcpy(fs+0x10,&n,8);memcpy(fs+0x18,&n,8);
 ((void(__fastcall*)(void*,const void*))(gBase+0x6ec0eb0))(desc,fs);
 // Copy the actual native family string, including its inline short-string
 // storage. SimSun has 6 characters; an external pointer with capacity6 is invalid.
 ((void(__fastcall*)(void*,const void*))(gBase+0x6ec0ed0))(desc,original+0x20);
 uint32_t name=0;memcpy(&name,desc+0x44,4);if(name!=14){Log("HEAD_MONO native family descriptor rejected name=%u",name);return 0;}
 uintptr_t factory=0,table=0,create=0,font=0;
 if(!Read(gBase+0xf1ed278,factory)||!Read(factory,table)||!Read(table+8,create)||create<gBase||create>=gBase+0xc000000)return 0;
 unsigned char prior=gMenuCreatingSlot;gMenuCreatingSlot=40;
 int result=((int(__fastcall*)(void*,int,uintptr_t*))create)((void*)factory,5,&font);gMenuCreatingSlot=prior;
 if(result<0||!HudNativeFont(font)||!MenuSlotAvailable(font,40)){Log("HEAD_MONO native private slot rejected");return 0;}
 bool previousImport=gHeadMonoImportActive;gHeadMonoImportActive=true;
 gHeadMonoPathsReady=false;gHeadMonoImportKeys=0;gHeadMonoProviderCreates=0;
 gHeadMonoLoadingFont=font;
 result=((int(__fastcall*)(void*,const void*))(gBase+0x7044220))((void*)font,desc);
 gHeadMonoLoadingFont=0;gHeadMonoImportActive=previousImport;
 Log("HEAD_MONO import result=%d private_paths=%u keys=%u providers=%u",result,(unsigned)gHeadMonoPathsReady,gHeadMonoImportKeys,gHeadMonoProviderCreates);
 if(result!=1||!gHeadMonoPathsReady||gHeadMonoImportKeys!=2||gHeadMonoProviderCreates!=2||!HeadMonoValidateProviders()){Log("HEAD_MONO load/isolation rejected result=%d; native names retained",result);return 0;}
 gHeadMonoFont=font;
 // The factory may have built the cached key before the private slot was
 // registered. Rebuild it only after both importer sides have been verified.
 ((void(__fastcall*)(void*))(gBase+0x70454d0))((void*)font);
 int registeredKey=0;
 if(!Read(font+0x20,registeredKey)||registeredKey!=40){gHeadMonoFont=0;Log("HEAD_MONO slot key rejected; native names retained");return 0;}
 Log("HEAD_MONO private SimSun ready source=%llx font=%llx; identical font bytes required by installer",(unsigned long long)source,(unsigned long long)font);return font;
}
static void* __fastcall HeadMonoRenderer(void* manager,void* provider,const void* key){
 if(gHeadMonoEnabled&&HeadMonoProvider((uintptr_t)provider)){
  alignas(16) unsigned char effective[0x30]{};uint32_t family=0;
  if(Read((uintptr_t)key,effective)){memcpy(&family,effective+0x10,4);
   if(family==14){uint32_t identity=37;memcpy(effective+0x10,&identity,4);
    // 1487A3D50 hash excludes family; native equality 1487A4030 includes it.
    // Native glyph source reads provider family 14, not cache tag 37.
    return ((void*(__fastcall*)(void*,void*,const void*))(gBase+0x87a7f50))(manager,provider,effective);
   }
  }
 }
 return ((void*(__fastcall*)(void*,void*,const void*))(gBase+0x87a7f50))(manager,provider,key);
}
static void* __fastcall HeadMonoChooseFace(void* provider,int index,int size){
 void* face=((void*(__fastcall*)(void*,int,int))(gBase+0x87a4160))(provider,index,size);
 if(gHeadMonoEnabled&&face&&HeadMonoProvider((uintptr_t)provider)){
  AcquireSRWLockExclusive(&gHeadMonoFaceLock);bool seen=false;
  for(unsigned i=0;i<gHeadMonoFaceCount;i++)if(gHeadMonoFaces[i]==(uintptr_t)face)seen=true;
  if(!seen&&gHeadMonoFaceCount<128)gHeadMonoFaces[gHeadMonoFaceCount++]=(uintptr_t)face;
  ReleaseSRWLockExclusive(&gHeadMonoFaceLock);
 }return face;
}
static unsigned HeadMonoRenderMode(uintptr_t face,uintptr_t slot,unsigned original){
 if(!gHeadMonoEnabled||!face)return original;bool seen=false;
 AcquireSRWLockShared(&gHeadMonoFaceLock);
 for(unsigned i=0;i<gHeadMonoFaceCount;i++)if(gHeadMonoFaces[i]==face)seen=true;
 ReleaseSRWLockShared(&gHeadMonoFaceLock);
 uintptr_t nativeSlot=0;if(seen&&Read(face+0x78,nativeSlot)&&nativeSlot==slot)return 2;
 return original;
}
static void HeadMonoObserveGlyph(uintptr_t face,uintptr_t slot,unsigned before,unsigned after,int result){
 if(after!=2||before==after||InterlockedIncrement(&gHeadMonoGlyphObserved)>32)return;
 unsigned char pixel=0;int advance=0;Read(slot+0x82,pixel);Read(slot+0x58,advance);
 Log("HEAD_MONO glyph face=%llx render=%u->%u result=%d pixel_mode=%u advance26_6=%d",(unsigned long long)face,before,after,result,(unsigned)pixel,advance);
}
static void __fastcall HeadMonoApplyStyle(const void* style,bool pet){
 if(style&&gHeadMonoEnabled&&(pet||(gHeadMonoPlayerScope&&(uintptr_t)style==gBase+0xe6ba890))){
  selectionfont::Style original{};uintptr_t source=0;
  if(Read((uintptr_t)style,original)){memcpy(&source,original.bytes+0x10,8);uintptr_t font=HeadMonoLoadFont(source);
   if(font){gHeadMonoStyle=original;memcpy(gHeadMonoStyle.bytes+0x10,&font,8);
    if(InterlockedIncrement(&gHeadMonoObserved)<=8)Log("HEAD_MONO apply kind=%s source=%llx private=%llx; native size/outline/alignment preserved",pet?"pet":"player",(unsigned long long)source,(unsigned long long)font);
    ((HudApplyFn)(gBase+0x6ec0510))(&gHeadMonoStyle);return;
   }
  }
 }((HudApplyFn)(gBase+0x6ec0510))(style);
}
static void __fastcall HeadMonoPlayerApply(const void* style){HeadMonoApplyStyle(style,false);}
static void __fastcall HeadMonoPetApply(const void* style){HeadMonoApplyStyle(style,true);}
static HeadNameFn gHeadMonoPlayerOriginal;
static void __fastcall HeadMonoPlayerDrawScope(float x,float y,void* actor,void* context,const void* pair,unsigned char first,unsigned char second){
 bool previous=gHeadMonoPlayerScope;gHeadMonoPlayerScope=HeadCharacter(actor);
 gHeadMonoPlayerOriginal(x,y,actor,context,pair,first,second);gHeadMonoPlayerScope=previous;
}
static bool InstallHeadMonoText(){
 if(!gHeadMonoReserved)return false;
 HeadFontSite sites[32]{};uintptr_t wrappers[32]{};unsigned count=0;
 auto add=[&](uintptr_t site,uintptr_t target,uintptr_t wrapper){sites[count]={site,target,0};wrappers[count++]=wrapper;};
 for(const auto& s:HeadFontSites){
  if(s.kind==0)add(s.site,s.target,(uintptr_t)&HeadMonoPlayerDrawScope);
  if(s.kind==1)add(s.site,s.target,(uintptr_t)&HeadMonoPlayerApply);
 }
 add(0x5e2f5d8,0x6ec0510,(uintptr_t)&HeadMonoPetApply);
 for(uintptr_t site:{0x87605b5u,0x87609c2u,0x8760b1cu,0x87bfe9eu})add(site,0x87a7f50,(uintptr_t)&HeadMonoRenderer);
 for(uintptr_t site:{0x87a8e76u,0x87a8f94u,0x87a957au})add(site,0x87a4160,(uintptr_t)&HeadMonoChooseFace);
 add(0x700c00f,0x700d3b0,(uintptr_t)&HeadMonoPaths);
 add(0x87ab96b,0x87a3ab0,(uintptr_t)&HeadMonoImportKey);
 add(0x87ab852,0x87a4100,(uintptr_t)&HeadMonoProviderCreate);
 // All five non-factory calls to the native font-number getter in this
 // client. 14704487F reaches the crash at 14876063E through text height.
 // Keep MenuFontNumber's existing factory registration call at 147043BA6.
 for(uintptr_t site:headfontslot::NumberCalls)add(site,0x6ec0930,(uintptr_t)&HeadMonoFontNumber);
 unsigned char expected[32][5]{};unsigned char* relays[32]{};
 gHeadMonoPlayerOriginal=(HeadNameFn)(gBase+0x5bcd540);
 for(unsigned i=0;i<count;i++){
  expected[i][0]=0xe8;int32_t d=(int32_t)(sites[i].target-sites[i].site-5);memcpy(expected[i]+1,&d,4);
  if(memcmp((void*)(gBase+sites[i].site),expected[i],5)){
   // Preserve the current dragonrobe name scope and its coordinate observers.
   // Accept only the exact callback compiled into this same DLL.
   unsigned char actual[5]{},prefix[6]{};uintptr_t destination=0;
   if((sites[i].site!=0x5bc590f&&sites[i].site!=0x700c00f)||!Read(gBase+sites[i].site,actual)||actual[0]!=0xe8){Log("HEAD_MONO native call guard rejected index=%u",i);return false;}
   int32_t old=0;memcpy(&old,actual+1,4);uintptr_t relay=gBase+sites[i].site+5+old;
   uintptr_t owned=sites[i].site==0x5bc590f?(uintptr_t)&beastcontrol::ObserveName:(uintptr_t)&MenuFontPaths;
   if(!Read(relay,prefix)||memcmp(prefix,"\xff\x25\x00\x00\x00\x00",6)||!Read(relay+6,destination)||destination!=owned){Log("HEAD_MONO unrelated owned callback rejected index=%u",i);return false;}
   memcpy(expected[i],actual,5);if(sites[i].site==0x5bc590f)gHeadMonoPlayerOriginal=(HeadNameFn)destination;
  }
 }
 for(unsigned i=0;i<count;i++){
  relays[i]=ChoiceRelay(gBase+sites[i].site,wrappers[i]);
  if(!relays[i]){for(auto p:relays)if(p)VirtualFree(p,0,MEM_RELEASE);return false;}
 }
 for(unsigned i=0;i<count;i++){
  unsigned char call[5]={0xe8};int32_t d=(int32_t)((intptr_t)relays[i]-(intptr_t)(gBase+sites[i].site+5));memcpy(call+1,&d,4);
  if(!Patch(gBase+sites[i].site,call,5)){
   for(unsigned j=0;j<=i;j++)Patch(gBase+sites[j].site,expected[j],5);
   Log("HEAD_MONO installation failed; native calls restored");return false;
  }
 }
 gHeadMonoEnabled=true;
 Log("HEAD_MONO functional candidate installed; %u guarded calls; player/pet private SimSun slot40; mono rendering; original NPC/UI unchanged; SLOT_FIX=1 metric_and_key_calls=5",count);return true;
}
