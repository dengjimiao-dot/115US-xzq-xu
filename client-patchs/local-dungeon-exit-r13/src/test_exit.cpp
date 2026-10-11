#include "../../elite-companion-pilot/src/inline_hook.h"
#include "exit_policy.h"
#include "native_evidence.h"
#include <assert.h>
#include <stdio.h>
static uintptr_t Replacement(){return 99;}
int main(){
    assert(!NativeMenuAllowed(0x1234567800ULL));
    assert(NativeMenuAllowed(0x1234567801ULL));
    for(unsigned action=0;action<100;action++)
        assert(AllowLocalExit(true,(uint16_t)action,true,3,0,true,true,true)==(action==22||action==23||action==30));
    for(unsigned state=0;state<30;state++)
        assert(AllowLocalExit(true,22,true,state,0,true,true,true)==(state==3));
    for(unsigned phase=1;phase<20;phase++)assert(!AllowLocalExit(true,23,true,3,phase,true,true,true));
    assert(!AllowLocalExit(false,22,true,3,0,true,true,true));
    assert(!AllowLocalExit(true,22,false,3,0,true,true,true));
    assert(!AllowLocalExit(true,22,true,3,0,false,true,true));
    assert(!AllowLocalExit(true,22,true,3,0,true,false,true));
    assert(!AllowLocalExit(true,22,true,3,0,true,true,false));
    // Exact audited prologue has no relative instruction. Exercise its real
    // stack/register effects and trampoline, restoring all bytes afterwards.
    unsigned char body[]={0xb8,0x20,0,0,0,0x48,0x83,0xc4,0x30,0x41,0x5e,0x5f,0x5e,0xc3};
    auto p=(unsigned char*)VirtualAlloc(nullptr,4096,MEM_RESERVE|MEM_COMMIT,PAGE_READWRITE);
    assert(p);memcpy(p,kMenuHead,sizeof(kMenuHead));memcpy(p+sizeof(kMenuHead),body,sizeof(body));
    DWORD old;assert(VirtualProtect(p,4096,PAGE_EXECUTE_READ,&old));
    auto fn=(uintptr_t(*)())p;assert(fn()==32);
    InlineHook hook;assert(PrepareHook(hook,p,kMenuHead,sizeof(kMenuHead),(void*)&Replacement));
    assert(((uintptr_t(*)())hook.original())()==32);
    assert(PublishHook(hook));assert(fn()==99);assert(RestoreHook(hook));assert(fn()==32);
    assert(!memcmp(p,kMenuHead,sizeof(kMenuHead)));
    unsigned char bad[sizeof(kMenuHead)];memcpy(bad,kMenuHead,sizeof(bad));bad[0]^=1;
    InlineHook rejected;assert(!PrepareHook(rejected,p,bad,sizeof(bad),(void*)&Replacement));
    assert(!Readable((void*)1,8));
    puts("PASS 100 action permissions, 30 states, transition/missing guards, exact native prologue trampoline, restore and byte mismatch");
}
