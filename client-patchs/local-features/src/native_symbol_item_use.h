#pragma once
#include "native_symbol_window.h"
#include "symbol_item_use_binding.h"
#include "symbol_item_routing.h"
namespace native_symbol_item_use {
using namespace native_symbol_window;
using Use=bool(__fastcall*)(void*,int,int);
using Request=bool(*)(unsigned char,uint16_t,uint32_t);
static InlineHook hook;
static Request request;
// Same container bounds, item indirection, virtual base-data getter and
// native field decoder used by 145af2740. This only reads the logical template;
// the native decoder maintains its own wrapper. No item is consumed here.
static uint32_t itemTemplate(void* container,int slot){
    if(!Readable(container,0xb0)||slot<0||slot>UINT16_MAX||slot>=read<int>(container,0x58))return 0;
    auto cells=read<void**>(container,0xa8);
    if(!cells||!Readable(cells+slot,sizeof(void*)))return 0;
    auto cell=read<void*>(cells+slot);
    if(!cell||reinterpret_cast<uintptr_t>(cell)<0x40)return 0;
    auto item=read<void*>(static_cast<char*>(cell)-0x40);
    auto vt=read<uintptr_t*>(item);
    if(!item||!Readable(vt,0xa0)||!Readable(reinterpret_cast<void*>(vt[0x98/8]),1))return 0;
    auto data=reinterpret_cast<void*(__fastcall*)(void*)>(vt[0x98/8])(item);
    if(!Readable(data,0x20))return 0;
    auto field=static_cast<char*>(data)+0x18;
    reinterpret_cast<void(__fastcall*)(void*,unsigned int,unsigned char,unsigned char,void*)>(address(0x1480a6620))(field,4,0x32,1,field+4);
    return read<uint32_t>(field);
}
static bool __fastcall use(void* container,int slot,int flags){
    if(request&&(!uiThread||uiThread==GetCurrentThreadId())){
        const uint32_t id=itemTemplate(container,slot);
        const auto kind=symbol_item_routing::find(id);
        if(kind&&request(kind,uint16_t(slot),id))return true;
    }
    return reinterpret_cast<Use>(hook.original())(container,slot,flags);
}
static bool validate(uintptr_t imageBase){
    for(const auto& g:symbol_item_use_binding::guards){
        auto p=reinterpret_cast<void*>(imageBase+g.va-0x140000000ull);
        if(!Readable(p,g.size)||memcmp(p,g.bytes,g.size))return false;
    }
    return true;
}
static bool install(Request handler){
    request=handler;
    // 14 bytes are seven complete pushes preceded by mov rax,rsp; no RIP fixup.
    const auto& g=symbol_item_use_binding::guards[0];
    if(!PrepareHook(hook,reinterpret_cast<void*>(address(g.va)),g.bytes,14,reinterpret_cast<void*>(use)))return false;
    if(!PublishHook(hook))return false;
    if(native_symbol_window::log){fputs("symbol source-bound item-use route installed\n",native_symbol_window::log);fflush(native_symbol_window::log);}
    return true;
}
}
