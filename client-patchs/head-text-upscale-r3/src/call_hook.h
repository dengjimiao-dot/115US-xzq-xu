#pragma once
#include <windows.h>
#include <stdint.h>
#include <string.h>
#include <limits.h>

namespace headtext {
inline bool CopyRead(uintptr_t p, void* out, size_t size) {
    if(!p)return false;
    __try { memcpy(out,(const void*)p,size);return true; }
    __except(EXCEPTION_EXECUTE_HANDLER){return false;}
}
template<class T> bool Read(uintptr_t p,T& out){return CopyRead(p,&out,sizeof(out));}
inline bool Write(uintptr_t p,const void* data,size_t size) {
    DWORD old=0;
    if(!VirtualProtect((void*)p,size,PAGE_EXECUTE_READWRITE,&old))return false;
    bool ok=true;
    __try{memcpy((void*)p,data,size);}
    __except(EXCEPTION_EXECUTE_HANDLER){ok=false;}
    FlushInstructionCache(GetCurrentProcess(),(void*)p,size);
    DWORD ignored=0;
    VirtualProtect((void*)p,size,old,&ignored);
    return ok;
}
struct CallHook {
    uintptr_t site=0;
    unsigned char original[6]{},replacement[6]{};
    size_t size=0;
    void* relay=nullptr;
    bool active=false;
    bool Guard()const {
        unsigned char got[6]{};
        return size && CopyRead(site,got,size) && !memcmp(got,original,size);
    }
    bool Prepare(uintptr_t at,const unsigned char* expected,size_t n,uintptr_t wrapper) {
        if(n!=5 && n!=6)return false;
        site=at;size=n;memcpy(original,expected,n);
        if(!Guard())return false;
        SYSTEM_INFO info{};GetSystemInfo(&info);
        uintptr_t gran=info.dwAllocationGranularity;
        uintptr_t aligned=at&~(gran-1);
        for(uintptr_t distance=gran;distance<0x70000000;distance+=gran){
            uintptr_t candidates[2]={aligned+distance,aligned>distance?aligned-distance:0};
            for(uintptr_t candidate:candidates){
                if(!candidate)continue;
                relay=VirtualAlloc((void*)candidate,gran,MEM_COMMIT|MEM_RESERVE,PAGE_READWRITE);
                if(relay)break;
            }
            if(relay)break;
        }
        if(!relay)return false;
        unsigned char jump[14]={0xff,0x25,0,0,0,0};
        memcpy(jump+6,&wrapper,8);memcpy(relay,jump,sizeof(jump));
        DWORD old=0;
        if(!VirtualProtect(relay,gran,PAGE_EXECUTE_READ,&old))return false;
        FlushInstructionCache(GetCurrentProcess(),relay,sizeof(jump));
        intptr_t delta=(intptr_t)relay-(intptr_t)(at+5);
        if(delta<INT32_MIN||delta>INT32_MAX)return false;
        memset(replacement,0x90,n);replacement[0]=0xe8;
        int32_t d=(int32_t)delta;memcpy(replacement+1,&d,4);
        return true;
    }
    bool Install(){
        if(!Guard())return false;
        // Record ownership before a write so partial writes are restored, too.
        active=true;
        if(Write(site,replacement,size))return true;
        Write(site,original,size);return false;
    }
    bool Restore(){
        if(!active)return true;
        if(!Write(site,original,size))return false;
        active=false;return true;
    }
    void FreeUnused(){if(relay&&!active){VirtualFree(relay,0,MEM_RELEASE);relay=nullptr;}}
};
}
