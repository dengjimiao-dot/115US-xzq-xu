#pragma once
#include "profiles.h"

// Diagnostic only: finite exit animation, no retained actor, ride or disguise writes.
namespace beastprobe {
constexpr uintptr_t kTownTable=0xa946458, kMotion=0x5b95700, kCreate=0x5b5f2b0;
using MotionFn=void(__fastcall*)(void*,void*);
using CoordinateFn=int(__fastcall*)(void*);
using SceneFn=void*(__fastcall*)(void*);
using CreateFn=void*(__fastcall*)(void*,const wchar_t*,int,int,int,int,int);
static DWORD mainThread;
static unsigned attempts;
static ULONGLONG lastAttempt;
static bool disabled;
static thread_local bool reentering;

inline bool eligible(int job,const std::array<uint32_t,8>& equipment) {
 const auto* profile=beast115::match(equipment);
 return job==16 && profile && profile->id==2;
}

static bool CurrentArcher(uintptr_t actor) {
 uintptr_t control=0,value=0,table=0,sceneControl=0,scene=0;
 int count=0,sceneCount=0,job=-1;
 if(!Read(gBase+0xef2caa0,control)||!Read(gBase+0xef2caa8,value)||
    !Read(control+8,count)||count<=0||value<0x10030||value-0x30!=actor||
    !Read(actor,table)||table!=gBase+kTownTable||!Read(actor+0x108c,job))return false;
 if(!Read(actor+0x190,sceneControl)||!Read(sceneControl+8,sceneCount)||sceneCount<=0||
    !Read(actor+0x198,scene)||!scene)return false;
 std::array<uint32_t,8> equipment{};
 for(size_t i=0;i<8;i++)if(!Read(actor+0x1220+i*8,equipment[i]))return false;
 return eligible(job,equipment);
}

static void SpawnFiniteAnimation(void* actor) {
 // Executed only from the actor's original motion dispatch thread.
 __try {
  int x=((CoordinateFn)(gBase+0x5b8c7e0))(actor);
  int y=((CoordinateFn)(gBase+0x5b8c8b0))(actor);
  int z=((CoordinateFn)(gBase+0x5b8c8e0))(actor);
  int facing=((CoordinateFn)(gBase+0x5b89d90))(actor);
  float scale=0;uintptr_t parent=0;
  if(x < -1000000 || x > 1000000 || y < -1000000 || y > 1000000 ||
     z < -1000000 || z > 1000000 || (facing!=0 && facing!=1) ||
     !Read((uintptr_t)actor+0x4d8,scale)||!(scale>=0.1f&&scale<=10.0f)||
     !Read((uintptr_t)actor+0x198,parent)||
     ((SceneFn)(gBase+0x5b8b120))(actor)!=(void*)parent) {
   Log("BEAST diagnostic rejected geometry/scene");return;
  }
  const wchar_t* path=L"uniquebeast/archer/animation/unique_1st/01_out/archerout_03.ani";
  void* result=((CreateFn)(gBase+kCreate))(actor,path,x,y,z,facing,1);
  Log("BEAST diagnostic finite-animation attempt=%u actor=%p result=%p xyz=%d,%d,%d facing=%d thread=%lu",attempts,actor,result,x,y,z,facing,GetCurrentThreadId());
 } __except(EXCEPTION_EXECUTE_HANDLER) {
  disabled=true;
  Log("BEAST diagnostic disabled exception=%08lx",GetExceptionCode());
 }
}

static void __fastcall Motion(void* actor,void* animation) {
 ((MotionFn)(gBase+kMotion))(actor,animation);
 if(reentering || disabled || attempts>=3 || !CurrentArcher((uintptr_t)actor))return;
 DWORD thread=GetCurrentThreadId();
 if(!mainThread){mainThread=thread;Log("BEAST archer full-set ready actor=%p thread=%lu; hold F9 and move for finite-animation test",actor,thread);}
 if(thread!=mainThread || !(GetAsyncKeyState(VK_F9)&0x8000))return;
 ULONGLONG now=GetTickCount64();if(lastAttempt&&now-lastAttempt<5000)return;
 lastAttempt=now;attempts++;reentering=true;
 SpawnFiniteAnimation(actor);
 reentering=false;
}

static bool Install() {
 struct Slot {uintptr_t offset,rva;};
 constexpr Slot slots[]={{0x130,0x5b8b120},{0x1c8,0x5b8baa0},{0x1d0,0x5b8c7e0},
  {0x1d8,0x5b8c8b0},{0x1e0,0x5b8c8e0},{0x2e0,0x5b89d90},{0x380,kMotion}};
 for(const auto& slot:slots){uintptr_t value=0;if(!Read(gBase+kTownTable+slot.offset,value)||value!=gBase+slot.rva){Log("BEAST rejected town vtable offset=%zx",slot.offset);return false;}}
 const unsigned char createHead[]={0x48,0x8b,0xc4,0x41,0x56,0x48,0x81,0xec,0x80,0,0,0,0x48,0xc7,0x40,0xa8};
 const unsigned char motionHead[]={0x48,0x89,0x54,0x24,0x10,0x55,0x56,0x57,0x41,0x54,0x41,0x55,0x41,0x56,0x41,0x57};
 if(memcmp((void*)(gBase+kCreate),createHead,sizeof(createHead))||memcmp((void*)(gBase+kMotion),motionHead,sizeof(motionHead))){Log("BEAST rejected native function fingerprints");return false;}
 uintptr_t replacement=(uintptr_t)&Motion;
 if(!Patch(gBase+kTownTable+0x380,&replacement,sizeof(replacement)))return false;
 Log("BEAST diagnostic installed: archer 8-piece only; F9+move; max 3 finite animations; riding/transformation pending");return true;
}
}
