#define _CRT_SECURE_NO_WARNINGS
#include "../../elite-companion-pilot/src/inline_hook.h"
#include "exit_policy.h"
#include "native_evidence.h"
#include <stdio.h>
#include <stdlib.h>

static HMODULE gSelf;
static uintptr_t gBase;
static FILE* gLog;
static InlineHook gHook;
static volatile LONG gStarted=0,gEnabled=0,gLogged=0;
using MenuFn=uintptr_t(__fastcall*)(uint16_t,uintptr_t,uintptr_t,uintptr_t);
static MenuFn gOriginal;
template<class T> static bool Read(uintptr_t p,T& out) {
    if(!Readable((void*)p,sizeof(T)))return false;
    __try {memcpy(&out,(void*)p,sizeof(T));return true;}
    __except(EXCEPTION_EXECUTE_HANDLER){return false;}
}
template<size_t N> static bool Match(uintptr_t rva,const unsigned char(&bytes)[N]) {
    return Readable((void*)(gBase+rva),N)&&memcmp((void*)(gBase+rva),bytes,N)==0;
}
static bool Scope(uint16_t action) {
    if(!gEnabled||(action!=22&&action!=23&&action!=30))return false;
    uintptr_t manager=0,world=0,expected=0;
    uint32_t phase=UINT32_MAX,state=UINT32_MAX;
    bool dungeon=false,player=false;
    if(!Read(gBase+0xe66c090,manager)||!manager||
       !Read(gBase+0xe6839f0,world)||!world)return false;
    __try {
        state=((uint32_t(__fastcall*)(void*))(gBase+kStateRVA))((void*)manager);
        if(state!=3||!Read(manager+0x110+3*8,expected)||expected!=world||
           !Read(world+0xa0,phase))return false;
        dungeon=((void*(__fastcall*)(void*))(gBase+kDungeonRVA))((void*)manager)!=nullptr;
        player=((void*(__fastcall*)())(gBase+kPlayerRVA))()!=nullptr;
    } __except(EXCEPTION_EXECUTE_HANDLER){return false;}
    return AllowLocalExit(true,action,true,state,phase,dungeon,player,true);
}
static uintptr_t __fastcall Menu(uint16_t action,uintptr_t a2,uintptr_t a3,uintptr_t a4) {
    uintptr_t native=gOriginal(action,a2,a3,a4);
    if(NativeMenuAllowed(native)||!Scope(action))return native;
    LONG bit=action==22?1:(action==23?2:4);
    if(!(InterlockedOr(&gLogged,bit)&bit)&&gLog) {
        fprintf(gLog,"ALLOW action=%u state=3 phase=0; native confirmation/action preserved\n",action);
        fflush(gLog);
    }
    return 1;
}
extern "C" __declspec(dllexport) const char* WINAPI ModName(){return "Local dungeon return and end-game permission candidate";}
extern "C" __declspec(dllexport) DWORD WINAPI ModStart(){
    if(InterlockedCompareExchange(&gStarted,1,0))return 0;
    wchar_t path[MAX_PATH]={},opt[64]={};
    DWORD n=GetModuleFileNameW(gSelf,path,MAX_PATH);
    if(!n||n>=MAX_PATH)return 1;
    auto slash=wcsrchr(path,L'\\');if(!slash)return 1;
    wcscpy_s(slash+1,MAX_PATH-(slash+1-path),L"local-dungeon-exit.log");
    gLog=_wfopen(path,L"ab");if(!gLog)return 1;
    if(GetEnvironmentVariableW(L"DFO_LOCAL_DUNGEON_EXIT",opt,_countof(opt))!=17||
       wcscmp(opt,L"r13-local-exit-v1")) {
        fputs("INACTIVE candidate opt-in absent; no writes\n",gLog);fflush(gLog);return 0;
    }
    gBase=(uintptr_t)GetModuleHandleW(nullptr);
    auto dos=(IMAGE_DOS_HEADER*)gBase;
    if(!Readable(dos,sizeof(*dos))||dos->e_magic!=IMAGE_DOS_SIGNATURE||
       dos->e_lfanew<0||dos->e_lfanew>0x100000)return 2;
    auto nt=(IMAGE_NT_HEADERS64*)(gBase+dos->e_lfanew);
    if(!Readable(nt,sizeof(*nt))||nt->Signature!=IMAGE_NT_SIGNATURE||
       nt->FileHeader.TimeDateStamp!=kImageTimestamp||nt->OptionalHeader.SizeOfImage!=kImageSize||
       !Match(kMenuRVA,kMenuHead)||!Match(kStateRVA,kStateHead)||
       !Match(kDungeonRVA,kDungeonHead)||!Match(kPlayerRVA,kPlayerHead)) {
        fputs("DISABLED image/function byte mismatch; no writes\n",gLog);fflush(gLog);return 2;
    }
    if(!PrepareHook(gHook,(void*)(gBase+kMenuRVA),kMenuHead,sizeof(kMenuHead),(void*)&Menu)) {
        fputs("DISABLED preparation failed; no writes\n",gLog);fflush(gLog);return 3;
    }
    gOriginal=(MenuFn)gHook.original();
    if(!PublishHook(gHook)) {
        fputs("DISABLED publication byte recheck failed\n",gLog);fflush(gLog);return 4;
    }
    InterlockedExchange(&gEnabled,1);
    fputs("READY r13 opt-in local-only; actions=22,23,30 ordinary dungeon idle only; gameplay acceptance pending\n",gLog);
    fflush(gLog);return 0;
}
BOOL WINAPI DllMain(HINSTANCE self,DWORD reason,LPVOID) {
    if(reason==DLL_PROCESS_ATTACH){gSelf=self;DisableThreadLibraryCalls(self);}return TRUE;
}
