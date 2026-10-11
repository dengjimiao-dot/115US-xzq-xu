#pragma once
#include "profiles.h"
#include "beast_assets.h"
#include "companion_moves.h"
#include "beast_visuals.h"
#include <intrin.h>

// Native calls must stay on the actor update thread. The HUD only queues actions.
namespace beastcontrol {
constexpr uintptr_t TownTable=0xa946458, BattleTable=0x95ad718;
constexpr uintptr_t TownUpdate=0x5b910e0, BattleUpdate=0x5d7bb10;
constexpr uintptr_t NativeMotion=0x5b95700, NativeCreate=0x5b5f2b0;
using MotionFn=void(__fastcall*)(void*,void*);
using UpdateFn=void(__fastcall*)(void*,uintptr_t,uintptr_t,uintptr_t);
using CreateFn=void*(__fastcall*)(void*,const wchar_t*,int,int,int,int,int);
using CoordFn=int(__fastcall*)(void*);
struct Pair {uintptr_t object,control;};
struct Weak {uintptr_t reserved,control,value;};
static_assert(sizeof(Pair)==16&&sizeof(Weak)==24,"Native shared/weak ABI");
static Weak companion{};
static Pair rideAnimations[2]{};
static Pair companionAnimations[2]{};
static int companionMotion=-1;
static uintptr_t owner=0,ownerScene=0,battleOwner=0,ownedPart=0;
static int savedDisguise=-1,savedVariant=-1;
static unsigned ownedDisguise=0,ownedVariant=0,ownerProfile=~0u,battleProfile=~0u;
static uintptr_t carrierDescription=0;
static int savedGroup=0,savedType=0;
static bool summoned=false,mounted=false,disabled=false,keys[2]{};
// User intent survives scene-owned objects being destroyed during a dungeon trip.
static unsigned desiredMode=0,desiredProfile=~0u;
static uintptr_t desiredOwner=0;
static DWORD actorThread=0;
static LONG pending=0,hudVisible=0,hudMounted=0,hudSummoned=0;
static LONG townDrawTime=0;
static LONG battleTickTime=0;
static thread_local bool inside=false;

// Default cadence is projected from the source ETC. Dash-state consumption is
// pending native evidence; do not infer an unverified flag.
struct FootTracker {
 uintptr_t actor=0,scene=0;unsigned profile=~0u,last=0;int x=0,y=0;bool emitted=false;
 bool Observe(uintptr_t a,uintptr_t s,unsigned p,int nx,int ny,int z,unsigned now,unsigned term){
  if(!a||!s||!term){*this={};return false;}
  if(actor!=a||scene!=s||profile!=p){*this={a,s,p,now,nx,ny,false};return false;}
  bool moving=x!=nx||y!=ny;x=nx;y=ny;
  if(z!=0||!moving||(emitted&&now-last<term))return false;
  last=now;emitted=true;return true;
 }
};
static FootTracker townFoot{},battleFoot{};

inline bool Eligible(int job,const std::array<uint32_t,8>& worn) {
 const auto* p=beast115::match(worn);return p&&AssetsFor(job,p->id);
}
static uintptr_t Current(uintptr_t controlRva,uintptr_t valueRva) {
 uintptr_t c=0,v=0;int count=0;
 if(!Read(gBase+controlRva,c)||!Read(gBase+valueRva,v)||!Read(c+8,count)||count<=0||v<0x10030)return 0;
 return v-0x30;
}
static uintptr_t Town(){return Current(0xef2caa0,0xef2caa8);}
static uintptr_t Battle(){uintptr_t a=Current(0xef2ca88,0xef2ca90);return a?a:Current(0xef2ca70,0xef2ca78);}
static const beast115::Profile* Worn(uintptr_t actor) {
 uintptr_t vt=0;int job=-1;std::array<uint32_t,8> worn{};
 if(!Read(actor,vt)||vt!=gBase+TownTable||!Read(actor+0x108c,job))return nullptr;
 for(size_t n=0;n<8;n++)if(!Read(actor+0x1220+n*8,worn[n]))return nullptr;
 const auto* p=beast115::match(worn);return Eligible(job,worn)?p:nullptr;
}
// Read the dungeon's actual worn objects, never its cached town mannequin.
// The native transformation appendage also uses these getters when restoring
// ordinary clothes. Prefer the clone's visible cover when one is present.
using WornItemFn=uintptr_t(__fastcall*)(void*,int);
using ItemIdFn=bool(*)(uintptr_t,uint32_t&);
static bool NativeItemId(uintptr_t item,uint32_t& id) {
 uintptr_t vt=0,getter=0,cloneGetter=0;
 if(!Read(item,vt)||!Read(vt+0x98,getter)||getter!=gBase+0xbb1660)return false;
 // Clone::GetAppearanceDefinition (603c110) resolves this current cover ID.
 // The clone's own template is its stats carrier, not its visible clothes.
 if(Read(vt+0x440,cloneGetter)&&cloneGetter==gBase+0x603c110)
  return Read(item+0x1d10,id)&&id>0;
 uintptr_t info=((uintptr_t(__fastcall*)(void*))getter)((void*)item);
 if(!info||!Writable(info+0x18,8))return false;
 // Same checked native decoder used by the equipment's own definition getter.
 ((void(__fastcall*)(void*,int,unsigned char,unsigned char,void*))(gBase+0x80a6620))
  ((void*)(info+0x18),4,0x32,1,(void*)(info+0x1c));
 return Read(info+0x18,id)&&id>0;
}
static const beast115::Profile* BattleWorn(uintptr_t a,ItemIdFn itemId=NativeItemId) {
 uintptr_t vt=0,coverGetter=0,baseGetter=0;int job=-1;
 std::array<uint32_t,8> worn{};
 if(!Read(a,vt)||!Read(a+0x75e8,job)||
    !Read(vt+0x1f08,coverGetter)||!coverGetter||
    !Read(vt+0x15c8,baseGetter)||!baseGetter)return nullptr;
 for(int slot=0;slot<8;slot++){
  uintptr_t item=((WornItemFn)coverGetter)((void*)a,slot);
  if(!item)item=((WornItemFn)baseGetter)((void*)a,slot);
  if(!item||!itemId(item,worn[slot]))return nullptr;
 }
 return Eligible(job,worn)?beast115::match(worn):nullptr;
}
static uintptr_t Scene(uintptr_t a) {
 uintptr_t c=0,s=0;int n=0;
 return Read(a+0x190,c)&&Read(c+8,n)&&n>0&&Read(a+0x198,s)?s:0;
}
static void Release(Pair& p) {
 uintptr_t c=p.control;p={};if(!c)return;
 if(InterlockedDecrement((volatile LONG*)(c+8))==0){
  auto vt=*(uintptr_t*)c;((void(__fastcall*)(void*))*(uintptr_t*)vt)((void*)c);
  if(InterlockedDecrement((volatile LONG*)(c+12))==0)((void(__fastcall*)(void*))*(uintptr_t*)(vt+8))((void*)c);
 }
}
static uintptr_t WeakActor(const Weak& w) {
 int n=0;return w.control&&w.value>0x10030&&Read(w.control+8,n)&&n>0?w.value-0x30:0;
}
static void DestroyCompanion() {
 if(companion.control)((uintptr_t(__fastcall*)(Weak*))(gBase+0x5bb2450))(&companion);
 companion={};
 for(auto& animation:companionAnimations)Release(animation);
 companionMotion=-1;
}
static bool RetainCompanion(uintptr_t a) {
 if(!a)return false;
 // Created effects are scene-owned and have no cached weak reference at 20/28.
 // The same native constructor used by Scene::Add derives it from the interface.
 Weak next{};((Weak*(__fastcall*)(Weak*,void*))(gBase+0x6ea47f0))(&next,(void*)(a+0x30));
 companion=next;
 if(WeakActor(companion)!=a){DestroyCompanion();return false;}
 return true;
}
static bool OwnDisguise(int id,int variant){return mounted&&id==(int)ownedDisguise&&variant==(int)ownedVariant;}
static void RestoreDisguise() {
 if(owner&&Town()==owner&&mounted){
  int id=-1,var=-1;
  if(Read(owner+0x2490,id)&&Read(owner+0x2494,var)&&OwnDisguise(id,var)){
   *(int*)(owner+0x2490)=savedDisguise;*(int*)(owner+0x2494)=savedVariant;
  }
  uintptr_t info=0;int group=0,type=0;
  if(Read(owner+0x1080,info)&&info+0x80==carrierDescription&&Read(carrierDescription+0x5a0,group)&&Read(carrierDescription+0x5a4,type)&&group==2&&type==24){
   *(int*)(carrierDescription+0x5a0)=savedGroup;*(int*)(carrierDescription+0x5a4)=savedType;
  }
 }
 mounted=false;carrierDescription=0;ownedDisguise=ownedVariant=0;
 for(auto& animation:rideAnimations)Release(animation);
}
static void ResetTown() {
 RestoreDisguise();DestroyCompanion();summoned=false;owner=ownerScene=0;ownerProfile=~0u;townFoot={};
 InterlockedExchange(&hudVisible,0);InterlockedExchange(&hudMounted,0);InterlockedExchange(&hudSummoned,0);
}
static bool Geometry(uintptr_t a,int& x,int& y,int& z,int& face) {
 x=((CoordFn)(gBase+0x5b8c7e0))((void*)a);y=((CoordFn)(gBase+0x5b8c8b0))((void*)a);
 z=((CoordFn)(gBase+0x5b8c8e0))((void*)a);face=((CoordFn)(gBase+0x5b89d90))((void*)a);
 return x>=-1000000&&x<=1000000&&y>=-1000000&&y<=1000000&&z>=-1000000&&z<=1000000&&(face==0||face==1);
}
static void Transient(uintptr_t a,const wchar_t* path,bool beside=false){
 int x,y,z,f;if(!path||!Scene(a)||!Geometry(a,x,y,z,f))return;
 // The existing native creator registers effects with the scene. Finite ANI
 // transitions need no retained handle and expire with their animation.
 ((CreateFn)(gBase+NativeCreate))((void*)a,path,x+(beside?(f?65:-65):0),y-(beside?12:0),0,f,1);
}
static void Footprints(uintptr_t a,const beast115::Profile& p,FootTracker& tracker){
 const auto* visual=VisualsFor(p.id);if(!visual){tracker={};return;}
 int x,y,z,f;uintptr_t scene=Scene(a);if(!scene||!Geometry(a,x,y,z,f)){tracker={};return;}
 if(!tracker.Observe(a,scene,p.id,x,y,z,GetTickCount(),visual->term))return;
 for(auto path:visual->footprints)if(path)
  ((CreateFn)(gBase+NativeCreate))((void*)a,path,x+(f?visual->footX:-visual->footX),y,0,f,1);
}
static bool Spawn(uintptr_t a,const beast115::Profile& p) {
 int x,y,z,f;if(!Geometry(a,x,y,z,f))return false;
 uintptr_t effect=(uintptr_t)((CreateFn)(gBase+NativeCreate))((void*)a,assets[p.id].idle,x+(f?65:-65),y-12,0,f,1);
 if(!RetainCompanion(effect))return false;
 // Keep a strong reference to the actual per-effect idle animation. Loading a
 // second idle instance here would unnecessarily reset its timeline on spawn.
 Pair initial{};
 if(!Read(effect+0x1b0,initial.object)||!Read(effect+0x1b8,initial.control)||!initial.object||!initial.control){DestroyCompanion();return false;}
 InterlockedIncrement((volatile LONG*)(initial.control+8));companionAnimations[0]=initial;companionMotion=0;
 Log("BEAST summon owner=%p companion=%p profile=%u",(void*)a,(void*)effect,p.id);return true;
}
static void Follow(uintptr_t a,const beast115::Profile& p) {
 uintptr_t effect=WeakActor(companion);if(!effect)return;
 int x,y,z,f;if(!Geometry(a,x,y,z,f))return;
 ((void(__fastcall*)(void*,int,int,int))(gBase+0x5b960d0))((void*)effect,x+(f?65:-65),y-12,0);
 ((void(__fastcall*)(void*,int))(gBase+0x5b95f90))((void*)effect,f);
 int action=0;if(!Read(a+0x1fa0,action))return;
 unsigned index=p.id>=15?0:CompanionMotionIndex(action);
 if(companionMotion==(int)index)return;
 Pair& cached=companionAnimations[index];
 if(!cached.object||!cached.control){
  ((Pair*(__fastcall*)(Pair*,const wchar_t*,int,int))(gBase+0x4724390))(&cached,companionMoves[p.id],1,0);
  if(!cached.object||!cached.control){Release(cached);return;}
 }
 // Change motion once per state transition; applying it every update freezes
 // the animation on its first frame. NativeMotion consumes this extra reference.
 Pair next=cached;InterlockedIncrement((volatile LONG*)(next.control+8));
 ((MotionFn)(gBase+NativeMotion))((void*)effect,&next);
 companionMotion=(int)index;
 Log("BEAST companion motion profile=%u moving=%u effect=%p animation=%p",p.id,index,(void*)effect,(void*)cached.object);
}
static bool RidePair(uintptr_t a,Pair& pair) {
 int action=-1;if(!Read(a+0x1fa0,action))return false;
 // The town controller can enter additional idle/transition actions. Returning an
 // empty pair for these actions hides the mount when the character stops.
 unsigned index=action==3?1:0;Pair& cached=rideAnimations[index];
 if(!cached.object||!cached.control){
  ((Pair*(__fastcall*)(void*,Pair*,int))(gBase+0x5bd7b00))((void*)a,&cached,index?28:27);
  if(!cached.object||!cached.control){Release(cached);return false;}
  Log("BEAST ride animation cached action=%d slot=%u object=%p",action,index,(void*)cached.object);
 }
 pair=cached;InterlockedIncrement((volatile LONG*)(pair.control+8));
 return pair.object&&pair.control;
}
// Diagnostic-only attempt 2: observe the native name draw and height consumer.
// No position-manager writes or synthetic movement. Bounded to each transition.
static DWORD nameObserveStart=0,nameObserveLast=0,heightObserveLast=0;
static unsigned nameObserveEpoch=0,nameObserveCount=0,heightObserveCount=0,coordinateObserveCount=0;
static DWORD coordinateObserveLast=0;
static int nameObserveAction=-1;
static thread_local uintptr_t nameScopeActor=0;
static void BeginNameObservation() {
 nameObserveStart=GetTickCount();nameObserveLast=heightObserveLast=0;
 ++nameObserveEpoch;nameObserveCount=heightObserveCount=coordinateObserveCount=0;coordinateObserveLast=0;nameObserveAction=-1;
}
using NameFn=void(__fastcall*)(float,float,void*,void*,const void*,unsigned char,unsigned char);
static void __fastcall ObserveName(float x,float y,void* a,void* context,const void* pair,unsigned char first,unsigned char second) {
 uintptr_t prior=nameScopeActor;nameScopeActor=(uintptr_t)a;
 DWORD now=GetTickCount();int action=-1;Read((uintptr_t)a+0x1fa0,action);
 if(nameObserveEpoch&&now-nameObserveStart<=15000&&(uintptr_t)a==Town()&&nameObserveCount<64&&
    (nameObserveCount<4||action!=nameObserveAction||now-nameObserveLast>=500)) {
  ++nameObserveCount;nameObserveAction=action;nameObserveLast=now;
  Log("BEAST name observation epoch=%u ms=%lu n=%u actor=%p owner=%p action=%d mounted=%u input_xy=%.3f/%.3f scene_match=%u flags=%u/%u context=%p pair=%p",
      nameObserveEpoch,now-nameObserveStart,nameObserveCount,a,(void*)owner,action,mounted?1u:0u,x,y,
      Scene((uintptr_t)a)==ownerScene?1u:0u,(unsigned)first,(unsigned)second,context,pair);
 }
 ((NameFn)(gBase+0x5bcd540))(x,y,a,context,pair,first,second);
 nameScopeActor=prior;
}
static bool ObserveCoordinatesNow() {
 DWORD now=GetTickCount();
 if(!nameScopeActor||nameScopeActor!=owner||!nameObserveEpoch||now-nameObserveStart>15000||coordinateObserveCount>=96)return false;
 if(coordinateObserveCount<12||now-coordinateObserveLast>=500){++coordinateObserveCount;coordinateObserveLast=now;return true;}
 return false;
}
static void LogNameCoordinates(const char* backend,uintptr_t node,const wchar_t* text,float beforeX,float beforeY,float afterX,float afterY) {
 wchar_t safe[65]{};for(unsigned n=0;n<64;n++){if(!Read((uintptr_t)text+n*2,safe[n])||!safe[n])break;}safe[64]=0;
 char utf8[260]{};WideCharToMultiByte(CP_UTF8,0,safe,-1,utf8,sizeof(utf8),nullptr,nullptr);
 int action=-1;Read(owner+0x1fa0,action);
 Log("BEAST text coordinates epoch=%u ms=%lu backend=%s node=%p actor=%p action=%d mounted=%u text=%s before=%.3f/%.3f after=%.3f/%.3f",
     nameObserveEpoch,GetTickCount()-nameObserveStart,backend,(void*)node,(void*)nameScopeActor,action,mounted?1u:0u,utf8,beforeX,beforeY,afterX,afterY);
}
using VectorCoordinateFn=void(__fastcall*)(void*,void*,void*,const wchar_t*,float*,float*);
static void __fastcall ObserveVectorCoordinates(void* node,void* style,void* font,const wchar_t* text,float* x,float* y) {
 bool sample=ObserveCoordinatesNow();float oldX=0,oldY=0;
 if(sample&&(!Read((uintptr_t)x,oldX)||!Read((uintptr_t)y,oldY)))sample=false;
 ((VectorCoordinateFn)(gBase+0x6fabb50))(node,style,font,text,x,y);
 float newX=0,newY=0;if(sample&&Read((uintptr_t)x,newX)&&Read((uintptr_t)y,newY))LogNameCoordinates("vector",(uintptr_t)node,text,oldX,oldY,newX,newY);
}
using BitmapCoordinateFn=void(__fastcall*)(void*,int*,int*,const wchar_t*,void*);
static void __fastcall ObserveBitmapCoordinates(void* node,int* x,int* y,const wchar_t* text,void* style) {
 bool sample=ObserveCoordinatesNow();int oldX=0,oldY=0;
 if(sample&&(!Read((uintptr_t)x,oldX)||!Read((uintptr_t)y,oldY)))sample=false;
 ((BitmapCoordinateFn)(gBase+0x6ff1630))(node,x,y,text,style);
 int newX=0,newY=0;if(sample&&Read((uintptr_t)x,newX)&&Read((uintptr_t)y,newY))LogNameCoordinates("bitmap",(uintptr_t)node,text,(float)oldX,(float)oldY,(float)newX,(float)newY);
}
static bool BeginRide(uintptr_t a,const beast115::Profile& p) {
 uintptr_t info=0,control=0;int refs=0;
 if(!Read(a+0x1078,control)||!Read(control+8,refs)||refs<=0||!Read(a+0x1080,info)||!info)return false;
 uintptr_t desc=info+0x80;
 if(!Read(desc+0x5a0,savedGroup)||!Read(desc+0x5a4,savedType)||savedGroup!=0||!Writable(desc+0x5a0,8))return false;
 if(!Read(a+0x2490,savedDisguise)||!Read(a+0x2494,savedVariant))return false;
 *(int*)(a+0x2490)=(int)p.disguise;*(int*)(a+0x2494)=(int)p.variant;
 Pair pair{};bool ready=RidePair(a,pair);
 if(!ready){Release(pair);*(int*)(a+0x2490)=savedDisguise;*(int*)(a+0x2494)=savedVariant;return false;}
 carrierDescription=desc;*(int*)(desc+0x5a0)=2;*(int*)(desc+0x5a4)=24;
 ownedDisguise=p.disguise;ownedVariant=p.variant;
 mounted=true;DestroyCompanion();((MotionFn)(gBase+NativeMotion))((void*)a,&pair);
 BeginNameObservation();
 Log("BEAST mount owner=%p profile=%u native=%u/%u",(void*)a,p.id,p.disguise,p.variant);return true;
}
static void EndRide(uintptr_t a) {
 RestoreDisguise();
 // Original normal animation lookup restores the town renderer immediately.
 Pair pair{};int action=0;Read(a+0x1fa0,action);
 ((Pair*(__fastcall*)(void*,Pair*,int))(gBase+0x5bd7e80))((void*)a,&pair,action==3?3:0);
 if(pair.object&&pair.control)((MotionFn)(gBase+NativeMotion))((void*)a,&pair);else Release(pair);
 BeginNameObservation();
 Log("BEAST unmount owner=%p",(void*)a);
}
static uintptr_t FindOwned(uintptr_t a) {
 uintptr_t m=0,array=0;int count=0;
 if(!Read(a+0x5698,m)||!Read(m+0x100,count)||count<0||count>512||!Read(m+0xc0,array))return 0;
 for(int n=0;n<count;n++){
  uintptr_t node=0,p=0;if(!Read(array+n*8,node)||node<0x10038||!Read(node-0x38,p)||!p)continue;
  if(p==ownedPart)return p;
 }
 return 0;
}
static void RemoveOwned(uintptr_t a) {
 uintptr_t p=FindOwned(a),m=0,vt=0;if(p&&Read(a+0x5698,m)&&Read(p,vt)){
  ((void(__fastcall*)(void*,uintptr_t))*(uintptr_t*)(vt+0x1b0))((void*)p,0);
  ((void(__fastcall*)(void*,void*))(gBase+0x5f491d0))((void*)m,(void*)p);
  Log("BEAST dungeon appearance removed actor=%p part=%p",(void*)a,(void*)p);
 }
 ownedPart=0;
}
static uintptr_t FindAppearance(uintptr_t a,const Assets& asset) {
 uintptr_t m=0,array=0;int count=0;
 if(!Read(a+0x5698,m)||!Read(m+0x100,count)||count<0||count>512||!Read(m+0xc0,array))return 0;
 for(int n=0;n<count;n++){
  uintptr_t node=0,p=0,first=0,last=0;
  if(!Read(array+n*8,node)||node<0x10038||!Read(node-0x38,p)||!p||!Read(p+0x500,first)||!Read(p+0x508,last)||last-first!=asset.count*8)continue;
  int values[18]{};SIZE_T got=0;
  if(!ReadProcessMemory(GetCurrentProcess(),(void*)first,values,asset.count*8,&got)||got!=asset.count*8)continue;
  bool match=true;unsigned seen=0;for(unsigned j=0;j<asset.count;j++){int slot=values[j*2];if(slot<0||slot>=9||(seen&(1u<<slot))||values[j*2+1]!=asset.appearance[slot]){match=false;break;}seen|=1u<<slot;}
  if(match)return p;
 }
 return 0;
}
static bool ApplyHiddenAppearance(uintptr_t a,const Assets& asset) {
 // 115's avatar apply routine rejects slots that the native transformation part
 // has just suppressed. Keep that suppression for normal gear, but clear it for
 // the synchronous application of the part's replacement item only.
 struct ItemReference {uintptr_t table,object,control;};
 for(unsigned slot=0;slot<9;slot++){
  int id=asset.appearance[slot];if(id<=0)continue;
  uintptr_t item=(uintptr_t)((void*(__fastcall*)(void*,unsigned,int))(gBase+0x5d31c30))((void*)a,slot,id);
  uintptr_t table=0,control=0;int count=0;unsigned char hidden=0;
  if(!Read(item,table)||table!=gBase+0xa924448||!Read(item+0x10,control)||!Read(control+8,count)||count<=0||
     !Read(a+0x69a9+slot,hidden)||hidden>1||!Writable(a+0x69a9+slot,1))return false;
  Pair retained{item,control};InterlockedIncrement((volatile LONG*)(control+8));
  ItemReference reference{gBase+0x9229ee8,item,control};
  __try {
   *(unsigned char*)(a+0x69a9+slot)=0;
   ((void(__fastcall*)(void*,ItemReference*,int,int,int))(gBase+0x5c994b0))((void*)a,&reference,slot,1,0);
  } __finally {*(unsigned char*)(a+0x69a9+slot)=hidden;Release(retained);}
  Log("BEAST replacement applied actor=%p slot=%u item=%d original_suppression=%u",(void*)a,slot,id,hidden);
 }
 return true;
}
static void DungeonTick(uintptr_t a) {
 if(a!=Battle())return;
 InterlockedExchange(&battleTickTime,(LONG)GetTickCount());
 ResetTown();int job=-1;if(!Read(a+0x75e8,job))return;
 const auto* p=BattleWorn(a);
 const auto* asset=p?AssetsFor(job,p->id):nullptr;
 if(!asset){
  if(a==battleOwner&&FindOwned(a)){RemoveOwned(a);Log("BEAST dungeon transformation removed: current worn set incomplete or mixed");}
  battleFoot={};return;
 }
 if(battleOwner!=a){battleOwner=a;ownedPart=0;battleProfile=~0u;}
 if(battleProfile!=p->id){if(FindOwned(a))RemoveOwned(a);battleProfile=p->id;}
 Footprints(a,*p,battleFoot);
 if(FindOwned(a))return;
 int ids[2]={job,asset->battleRow};struct Vector {int* first;int* last;int* end;} v{ids,ids+2,ids+2};
 // Zero expires the appearance part on the next update and causes repeated replacement.
 ((void(__fastcall*)(void*,Vector*,int))(gBase+0x5c05c10))((void*)a,&v,3600000);
 ownedPart=FindAppearance(a,*asset);
 if(!ownedPart){disabled=true;Log("BEAST dungeon appearance verification failed; controller disabled");return;}
 if(!ApplyHiddenAppearance(a,*asset)){disabled=true;Log("BEAST dungeon replacement item verification failed; controller disabled");return;}
 Log("BEAST dungeon transformation actor=%p row=%d part=%p",(void*)a,ids[1],(void*)ownedPart);
}
static void TownTick(uintptr_t a) {
 if(a!=Town())return;
 DWORD battleTime=(DWORD)InterlockedCompareExchange(&battleTickTime,0,0);
 if(battleTime&&GetTickCount()-battleTime<=350)return;
 uintptr_t scene=Scene(a);const auto* p=Worn(a);
 if(!scene){ResetTown();return;}
 if(!p){desiredMode=0;desiredOwner=0;desiredProfile=~0u;ResetTown();return;}
 if(desiredOwner!=a||desiredProfile!=p->id){desiredMode=0;desiredOwner=a;desiredProfile=p->id;}
 battleOwner=ownedPart=0;battleProfile=~0u;battleFoot={}; // Dungeon scene owns the old part.
 if(owner!=a||ownerScene!=scene||ownerProfile!=p->id){
  if(owner==a&&mounted)EndRide(a);
  ResetTown();owner=a;ownerScene=scene;ownerProfile=p->id;
  if(desiredMode==2){if(BeginRide(a,*p))summoned=true;}
  else if(desiredMode==1)summoned=Spawn(a,*p);
  if(desiredMode)Log("BEAST town state restored mode=%u summoned=%u mounted=%u",desiredMode,summoned,mounted);
 }
 InterlockedExchange(&hudVisible,1);
 int command=(int)InterlockedExchange(&pending,0);
 bool now[2]={(GetAsyncKeyState(VK_F9)&0x8000)!=0,(GetAsyncKeyState(VK_F10)&0x8000)!=0};
 if(now[0]&&!keys[0])command=1;if(now[1]&&!keys[1])command=2;keys[0]=now[0];keys[1]=now[1];
 const auto* visual=VisualsFor(p->id);
 if(command==1){if(mounted)EndRide(a);if(summoned){if(visual)Transient(a,visual->out,true);DestroyCompanion();summoned=false;}else{summoned=Spawn(a,*p);if(summoned&&visual)Transient(a,visual->call,true);}}
 if(command==2){if(mounted){EndRide(a);summoned=Spawn(a,*p);if(visual)Transient(a,visual->change);}else if(BeginRide(a,*p)){summoned=true;if(visual)Transient(a,visual->change);}}
 if(command){desiredMode=mounted?2:summoned?1:0;desiredOwner=a;desiredProfile=p->id;}
 if(summoned&&!mounted){if(!WeakActor(companion)){summoned=false;DestroyCompanion();Log("BEAST companion expired; no automatic duplicate spawn");}else Follow(a,*p);}
 Footprints(a,*p,townFoot);
 InterlockedExchange(&hudMounted,mounted?1:0);InterlockedExchange(&hudSummoned,summoned?1:0);
}
static void Tick(uintptr_t a,bool dungeon) {
 if(disabled||inside)return;
 // The shop uses cached town/battle actors for its preview, not a dungeon trip.
 if(ShopDisplayed())return;
 DWORD thread=GetCurrentThreadId();if(actorThread&&actorThread!=thread)return;
 if((dungeon?a!=Battle():a!=Town()))return;if(!actorThread)actorThread=thread;
 inside=true;
 __try{if(dungeon)DungeonTick(a);else TownTick(a);}
 __except(EXCEPTION_EXECUTE_HANDLER){disabled=true;InterlockedExchange(&hudVisible,0);Log("BEAST controller disabled exception=%08lx",GetExceptionCode());}
 inside=false;
}
static void __fastcall UpdateTown(void* a,uintptr_t b,uintptr_t c,uintptr_t d){((UpdateFn)(gBase+TownUpdate))(a,b,c,d);Tick((uintptr_t)a,false);}
static void __fastcall UpdateBattle(void* a,uintptr_t b,uintptr_t c,uintptr_t d){((UpdateFn)(gBase+BattleUpdate))(a,b,c,d);Tick((uintptr_t)a,true);}
static void __fastcall DrawTown(void* a) {
 ((void(__fastcall*)(void*))(gBase+0x5bc36f0))(a);
 if((uintptr_t)a==Town()&&Scene((uintptr_t)a))InterlockedExchange(&townDrawTime,(LONG)GetTickCount());
}
static int __fastcall TownHeight(void* a) {
 int height=((int(__fastcall*)(void*))(gBase+0x5bdaaa0))(a);
 // The native town height getter already uses 163 for tall transformation carriers.
 // Beast carrier 2/24 needs the same name/title clearance while mounted.
 int effective=height;
 if(mounted&&!disabled&&(uintptr_t)a==owner&&Town()==owner&&Scene(owner)==ownerScene&&height<163)effective=163;
 DWORD now=GetTickCount();
 if(nameObserveEpoch&&(uintptr_t)a==owner&&now-nameObserveStart<=15000&&heightObserveCount<64&&
    (heightObserveCount<4||now-heightObserveLast>=500)) {
  ++heightObserveCount;heightObserveLast=now;int action=-1;Read((uintptr_t)a+0x1fa0,action);
  Log("BEAST height observation epoch=%u ms=%lu actor=%p name_actor=%p owner=%p action=%d mounted=%u native=%d effective=%d caller=%p scene_match=%u disabled=%u",
      nameObserveEpoch,now-nameObserveStart,a,(void*)nameScopeActor,(void*)owner,action,mounted?1u:0u,height,effective,_ReturnAddress(),
      Scene((uintptr_t)a)==ownerScene?1u:0u,disabled?1u:0u);
 }
 return effective;
}
static void __fastcall Motion(void* a,Pair* input) {
 if(mounted&&!disabled&&!inside&&(uintptr_t)a==owner&&Town()==owner&&Scene(owner)==ownerScene){
  Pair replacement{};bool ready=false;
  __try{ready=RidePair(owner,replacement);if(ready)Release(*input);}
  __except(EXCEPTION_EXECUTE_HANDLER){disabled=true;Log("BEAST ride motion exception=%08lx",GetExceptionCode());}
  if(ready){((MotionFn)(gBase+NativeMotion))(a,&replacement);return;}Release(replacement);
 }
 ((MotionFn)(gBase+NativeMotion))(a,input);
}
static Pair* __fastcall DisguiseUpdate(void* a,Pair* out,uintptr_t group,int type) {
 if(mounted&&!disabled&&(uintptr_t)a==owner&&Town()==owner&&group==2&&type==24){
  // Native update returns an empty pair when the current motion has not changed.
  // Returning the cached pair each frame resets its timeline and freezes idle.
  *out={};__try{
   int action=0;uintptr_t active=0;Pair nativeCarrier{};
   Read(owner+0x1fa0,action);Read(owner+0x1b0,active);
   bool carrierRead=Read(owner+0x1f90,nativeCarrier);
   auto& cached=rideAnimations[action==3?1:0];
   // Native 5be4230 installs a nonempty result in actor+1f90/1f98 before
   // changing motion. Name-height 5bdbe10 consumes that pair, not just 1b0.
   // BeginRide has already changed 1b0; its first idle update must still hand
   // the pair to the native owner. Once both agree, preserve the timeline.
   if(active&&active==cached.object&&carrierRead&&
      nativeCarrier.object==cached.object&&nativeCarrier.control==cached.control)return out;
   if(RidePair(owner,*out)){
    *(int*)(owner+0x1fa4)=action;
    if(active==out->object&&carrierRead&&!nativeCarrier.object)
     Log("BEAST stationary carrier handoff owner=%p action=%d animation=%p",(void*)owner,action,(void*)out->object);
    return out;
   }Release(*out);
  }__except(EXCEPTION_EXECUTE_HANDLER){disabled=true;Log("BEAST disguise update exception=%08lx",GetExceptionCode());}
  return out;
 }
 return ((Pair*(__fastcall*)(void*,Pair*,uintptr_t,int))(gBase+0x5bd8720))(a,out,group,type);
}
static Pair* __fastcall DisguiseCreate(void* a,Pair* out,uintptr_t group,int type) {
 if(mounted&&!disabled&&(uintptr_t)a==owner&&Town()==owner&&group==2&&type==24){
  *out={};__try{if(RidePair(owner,*out))return out;Release(*out);}__except(EXCEPTION_EXECUTE_HANDLER){disabled=true;Log("BEAST disguise create exception=%08lx",GetExceptionCode());}
  return out;
 }
 return ((Pair*(__fastcall*)(void*,Pair*,uintptr_t,int))(gBase+0x5bd84d0))(a,out,group,type);
}
}
