#define _CRT_SECURE_NO_WARNINGS
#include "native_actor.h"
#include <stdlib.h>
#include "native_evidence.h"
#include <assert.h>
#include <stdio.h>
using Fn=uintptr_t(__fastcall*)(uintptr_t,uintptr_t);
static Fn originalEntry,sharedEntry;
static unsigned eliteCalls=0,guardCalls=0;
static uintptr_t __fastcall Elite(uintptr_t self,uintptr_t id){++eliteCalls;printf("ELITE original=%p self=%llx id=%llu\n",(void*)originalEntry,(unsigned long long)self,(unsigned long long)id);fflush(stdout);return originalEntry(self,id);}
static uintptr_t __fastcall Guard(uintptr_t self,uintptr_t id){++guardCalls;printf("GUARD self=%llx id=%llu shared=%p\n",(unsigned long long)self,(unsigned long long)id,(void*)sharedEntry);fflush(stdout);assert(self==0x12345678);if(id==490100178)return 666;return sharedEntry(self,id);}
static uintptr_t __fastcall Unused(uintptr_t,uintptr_t){puts("UNEXPECTED other hook called");fflush(stdout);return 777;}
int main(){
    auto base=(unsigned char*)VirtualAlloc((void*)0x140000000ULL,0x10000000,MEM_RESERVE,PAGE_NOACCESS);assert(base);
    for(auto& e:kAllNativeHooks){assert(VirtualAlloc(base+(e.rva&~4095ULL),4096,MEM_COMMIT,PAGE_EXECUTE_READWRITE));memcpy(base+e.rva,e.bytes,e.size);}
    const unsigned char head[]={0x48,0x8b,0xc4,0x55,0x41,0x54};
    auto entry=base+0x5e47f00;assert(VirtualAlloc(base+0x5e47000,4096,MEM_COMMIT,PAGE_EXECUTE_READWRITE));memcpy(entry,head,6);
    const unsigned char tail[]={0x41,0x5c,0x5d,0xb8,42,0,0,0,0xc3};memcpy(entry+6,tail,sizeof(tail));
    for(auto& e:kAllNativeHooks)if(e.call){auto at=base+e.rva;const unsigned char before[]={0x48,0x83,0xec,0x28};const unsigned char after[]={0x48,0x83,0xc4,0x28,0xc3};memcpy(at-4,before,4);memcpy(at+5,after,5);}
    InlineHook elite;assert(PrepareHook(elite,entry,head,6,(void*)Elite));originalEntry=(Fn)elite.original();sharedEntry=(Fn)entry;assert(PublishHook(elite));
    InlineHook oldConflict;assert(!PrepareHook(oldConflict,entry,head,6,(void*)Guard));
    InlineHook hooks[_countof(kAllNativeHooks)];
    for(size_t i=0;i<_countof(hooks);++i){auto& e=kAllNativeHooks[i];assert(PrepareHook(hooks[i],base+e.rva,e.bytes,e.size,e.call?(void*)Guard:(void*)Unused,e.call?1:0,e.call?5:0));}
    for(size_t i=0;i<_countof(hooks);++i)assert(PublishHook(hooks[i],kAllNativeHooks[i].call));
    for(size_t i=0;i<_countof(hooks);++i){auto& e=kAllNativeHooks[i];if(!e.call)continue;auto fn=(Fn)(base+e.rva-4);printf("CALL site=%llx target=%p elitePage=%p relay=%p\n",(unsigned long long)e.rva,(void*)fn,(void*)elite.page,(void*)hooks[i].page);fflush(stdout);unsigned before=eliteCalls;auto result=fn(0x12345678,490100178);printf("missing result=%llu elite=%u guard=%u\n",(unsigned long long)result,eliteCalls,guardCalls);fflush(stdout);assert(result==666);assert(eliteCalls==before);assert(fn(0x12345678,7)==42);assert(eliteCalls==before+1);}
    assert(guardCalls==4&&eliteCalls==2);
    for(size_t i=0;i<_countof(hooks);++i){assert(RestoreHook(hooks[i]));auto& e=kAllNativeHooks[i];assert(!memcmp(base+e.rva,e.bytes,e.size));}
    assert(sharedEntry(0x12345678,7)==42&&eliteCalls==3);assert(RestoreHook(elite));assert(!memcmp(entry,head,6));
    printf("PASS %zu native bindings prepared/published/restored; old shared-entry conflict reproduced; both preload CALLs preserve Elite chain, arguments, return and missing-resource bypass\n",_countof(hooks));
}
