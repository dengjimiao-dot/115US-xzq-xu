#define _CRT_SECURE_NO_WARNINGS
#include "call_hook.h"
#include "route_policy.h"
#include <stdio.h>
#include <stdarg.h>
#include <cmath>

static HMODULE gSelf;
static uintptr_t gBase;
static FILE* gLog;
static SRWLOCK gLock=SRWLOCK_INIT;
static LONG gStarted=0,gEnabled=0,gRouted=0,gMenuRouted=0,gRejected=0,gMenuOutside=0,gKeyKept=0;
static __declspec(thread) unsigned gDepth=0,gMenuDepth=0,gKeyDepth=0;
static headtext::CallHook gHooks[10];
using headtext::Read;
using ActorDraw=void(__fastcall*)(void*,unsigned char,int,int,unsigned char);
using WindowDraw=void(__fastcall*)(void*);
using SetCategory=void(__fastcall*)(void*,int);
using MakeFontNode=void*(__fastcall*)(void*,void*);
static WindowDraw gMenuOriginal;
static uintptr_t gMenuSlot=0,gMenuExpected=0;
static bool gMenuInstalled=false;
static void Log(const char* format,...){
    if(!gLog)return;
    AcquireSRWLockExclusive(&gLock);
    fprintf(gLog,"[%lu] ",GetTickCount());
    va_list args;va_start(args,format);vfprintf(gLog,format,args);va_end(args);
    fputc('\n',gLog);fflush(gLog);ReleaseSRWLockExclusive(&gLock);
}
static void __fastcall Overhead(void* actor,unsigned char visible,int x,int y,unsigned char flag){
    uintptr_t table=*(uintptr_t*)actor;
    ActorDraw original=*(ActorDraw*)(table+0x6a0);
    ++gDepth;
    __try {original(actor,visible,x,y,flag);}
    __finally {--gDepth;}
}
static void __fastcall MenuWindow(void* window){
    uintptr_t table=0;
    bool scoped=Read((uintptr_t)window,table)&&table==gBase+0x977ef80;
    if(scoped)++gMenuDepth;
    // Chain the cumulative proxy's existing private-font wrapper.
    __try {gMenuOriginal(window);}
    __finally {if(scoped)--gMenuDepth;}
}
using KeyDraw=void(__fastcall*)(void*);
static void KeyScope(void* actor,uintptr_t original){
    ++gKeyDepth;
    __try {((KeyDraw)original)(actor);}
    __finally {--gKeyDepth;}
}
static void __fastcall TutorialKeys(void* actor){KeyScope(actor,gBase+0x5cca7a0);}
static void __fastcall TutorialTargetKeys(void* actor){KeyScope(actor,gBase+0x5cc52a0);}
static bool NativeFontStyle(uintptr_t root){
    uintptr_t style=0,font=0,table=0;
    return Read(root+0x148,style)&&style&&Read(style+0x10,font)&&font&&
           Read(font,table)&&table==gBase+0xb1f8498;
}
static bool Views(uintptr_t root,int source){
    if(source<1||source>7)return false;
    uintptr_t native[2]{};
    int layers[2]={source,8};
    for(unsigned i=0;i<2;i++){
        uintptr_t wrapper=0,view=0,payload=0,table=0;
        if(!Read(root+0xbc0+layers[i]*8,wrapper)||!wrapper||
           !Read(wrapper+0x50,view)||!view||!Read(view+0x28,payload)||!payload||
           !Read(payload+0x18,native[i])||!native[i]||!Read(native[i],table)||
           table!=gBase+0xb686060)return false;
    }
    uintptr_t target[2]{};float dimensions[2][2]{};
    for(unsigned i=0;i<2;i++){
        if(!Read(native[i]+0x20,target[i])||!target[i]||
           !headtext::CopyRead(native[i]+0x10,dimensions[i],sizeof(dimensions[i])))return false;
    }
    return target[0]==target[1]&&std::isfinite(dimensions[0][0])&&std::isfinite(dimensions[0][1])&&
           dimensions[0][0]>0&&dimensions[0][1]>0&&
           dimensions[0][0]==dimensions[1][0]&&dimensions[0][1]==dimensions[1][1];
}
static void RouteReadyNode(void* node){
    if(!gEnabled||(!gDepth&&!gMenuDepth))return;
    if(gKeyDepth){
        if(InterlockedIncrement(&gKeyKept)<=8)Log("KEEP tutorial key font in native layer; overhead/menu enhancement remains enabled");
        return;
    }
    uintptr_t root=0,unit=0;int category=-1;unsigned char enabled=0;
    if(!Read(gBase+0xf1c0ca0,root)||!root||!Read(root+0x15c0,enabled)||enabled!=1||
       !Read((uintptr_t)node+0x10,category)||!Read((uintptr_t)node+8,unit)||!unit)return;
    if(gMenuDepth&&category>=8&&InterlockedIncrement(&gMenuOutside)<=4)
        Log("MENU already outside enhancement category=%d; original layer retained",category);
    if(category<1||category>7)return;
    bool style=NativeFontStyle(root),views=style&&Views(root,category);
    int actual=headtext::Category({gDepth+gMenuDepth,category,true,style,views});
    if(actual!=category){
        ((SetCategory)(gBase+0x6f3b760))(node,actual);
        LONG count=gMenuDepth?InterlockedIncrement(&gMenuRouted):InterlockedIncrement(&gRouted);
        if(count<=8)Log("ROUTE %s font category=%d->8 node=%p fully-created-unit=%p; font/position/settings retained",
            gMenuDepth?"menu":"overhead",category,node,(void*)unit);
    }else if(InterlockedIncrement(&gRejected)<=4){
        Log("SKIP scoped font route: native-font/source-overlay target/dimension guard rejected category=%d",category);
    }
}
static void* __fastcall FontNodeReady(void* owner,void* cache){
    // Constructor resets BOTH node category and initial unit. Route after it
    // returns, on first use and cached reuse, without retaining native pointers.
    void* node=((MakeFontNode)(gBase+0x6fab490))(owner,cache);
    RouteReadyNode(node);
    return node;
}
static bool NativeGuard(){
    struct Site {uintptr_t rva;const unsigned char* bytes;size_t size;};
    static const unsigned char set[]{0x88,0x91,0xc0,0x15,0x00,0x00};
    static const unsigned char scope[]{0x49,0x8b,0x06,0x40,0x88,0x74,0x24,0x20};
    static const unsigned char category[]{0x39,0x51,0x10,0x74,0x13,0x89,0x51,0x10,0x48,0x8b,0x49,0x08};
    Site checks[]={{0x6eb7750,set,sizeof(set)},{0x5b85275,scope,sizeof(scope)},
                   {0x6f3b760,category,sizeof(category)}};
    for(const auto& check:checks){
        unsigned char got[16]{};
        if(!headtext::CopyRead(gBase+check.rva,got,check.size)||memcmp(got,check.bytes,check.size)){
            Log("GUARD rejected native RVA=%llx",(unsigned long long)check.rva);return false;
        }
    }
    return true;
}
static bool MenuPrepare(){
    // Installed cumulative proxy (SHA256 5b4a9099...) MenuWindowDraw PE RVA.
    uintptr_t proxy=(uintptr_t)GetModuleHandleW(L"dinput8.dll");
    if(!proxy)return false;
    gMenuSlot=gBase+0x977efc0;gMenuExpected=proxy+0x10970;
    static const unsigned char prefix[]={0x40,0x53,0x55,0x57,0x41,0x56,0x48,0x83,0xec,0x38,0x8b,0x15,0x7c,0xc6,0x0c,0x00};
    unsigned char got[sizeof(prefix)]{};
    if(!headtext::CopyRead(gMenuExpected,got,sizeof(got))||memcmp(got,prefix,sizeof(got)))return false;
    // Let the existing proxy finish first, preserving its private-font setup.
    uintptr_t target=0;
    for(unsigned i=0;i<100;i++){
        if(!Read(gMenuSlot,target))return false;
        if(target==gMenuExpected){gMenuOriginal=(WindowDraw)target;return true;}
        if(target!=gBase+0x67a5e70)return false;
        Sleep(100);
    }
    return false;
}
static bool MenuInstall(){
    uintptr_t target=0,hook=(uintptr_t)&MenuWindow;
    if(!Read(gMenuSlot,target)||target!=gMenuExpected)return false;
    gMenuInstalled=true;
    return headtext::Write(gMenuSlot,&hook,sizeof(hook));
}
static bool MenuRestore(){
    if(!gMenuInstalled)return true;
    if(!headtext::Write(gMenuSlot,&gMenuExpected,sizeof(gMenuExpected)))return false;
    gMenuInstalled=false;return true;
}
extern "C" __declspec(dllexport) const char* WINAPI ModName(){return "Head/menu text outside scene enhancement (candidate r3)";}
extern "C" __declspec(dllexport) DWORD WINAPI ModStart(){
    if(InterlockedCompareExchange(&gStarted,1,0))return 0;
    wchar_t path[MAX_PATH]{};
    if(!GetModuleFileNameW(gSelf,path,MAX_PATH)||wcslen(path)>=MAX_PATH-1)return 1;
    wchar_t* filename=wcsrchr(path,L'\\');if(!filename)return 1;
    wcscpy_s(filename+1,MAX_PATH-(filename+1-path),L"head-text-upscale.disabled");
    bool disabled=GetFileAttributesW(path)!=INVALID_FILE_ATTRIBUTES;
    wcscpy_s(filename+1,MAX_PATH-(filename+1-path),L"head-text-upscale.log");
    _wfopen_s(&gLog,path,L"a");
    Log("START candidate-r3 plugin=%p disabled=%u; manual visual acceptance pending",gSelf,disabled);
    if(disabled)return 0;
    gBase=(uintptr_t)GetModuleHandleW(nullptr);
    if(!NativeGuard()||!MenuPrepare()){Log("SKIP native/proxy menu identity guard rejected; no hooks installed");return 2;}
    const unsigned char indirect[]{0xff,0x90,0xa0,0x06,0,0};
    bool prepared=gHooks[0].Prepare(gBase+0x5b85288,indirect,6,(uintptr_t)&Overhead);
    const uintptr_t calls[]={0x6fab83c,0x6fac4a9,0x6fac59d,0x6fac683,0x6fac730};
    for(unsigned i=0;i<5&&prepared;i++){
        unsigned char direct[5]={0xe8};int32_t relative=(int32_t)(0x6fab490-calls[i]-5);
        memcpy(direct+1,&relative,4);
        prepared=gHooks[i+1].Prepare(gBase+calls[i],direct,5,(uintptr_t)&FontNodeReady);
    }
    const uintptr_t keyCalls[]={0x5cc0e7a,0x62ca4e4,0x5cc0e82,0x62ca512};
    const uintptr_t keyTargets[]={0x5cca7a0,0x5cca7a0,0x5cc52a0,0x5cc52a0};
    for(unsigned i=0;i<4&&prepared;i++){
        unsigned char direct[5]={0xe8};int32_t relative=(int32_t)(keyTargets[i]-keyCalls[i]-5);
        memcpy(direct+1,&relative,4);
        prepared=gHooks[i+6].Prepare(gBase+keyCalls[i],direct,5,
            i<2?(uintptr_t)&TutorialKeys:(uintptr_t)&TutorialTargetKeys);
    }
    if(!prepared){for(auto& hook:gHooks)hook.FreeUnused();Log("SKIP font-node call guard/allocation rejected");return 3;}
    bool installed=true;
    for(unsigned i=1;i<10&&installed;i++)installed=gHooks[i].Install();
    if(installed)installed=MenuInstall();
    if(installed)installed=gHooks[0].Install();
    if(!installed){
        bool restored=MenuRestore();for(auto& hook:gHooks)restored=hook.Restore()&&restored;
        if(restored)for(auto& hook:gHooks)hook.FreeUnused();
        Log("INSTALL failed restored=%u",restored);return 4;
    }
    InterlockedExchange(&gEnabled,1);
    Log("READY ten guarded calls with tutorial-key native-layer exclusion + chained main-menu scope; fully-created font units route to native layer 8 only while enhancement ON; manual acceptance pending");
    return 0;
}
BOOL WINAPI DllMain(HINSTANCE self,DWORD reason,LPVOID){
    if(reason==DLL_PROCESS_ATTACH){gSelf=self;DisableThreadLibraryCalls(self);}return TRUE;
}
