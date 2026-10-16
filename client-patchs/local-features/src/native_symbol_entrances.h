#pragma once
#include "native_symbol_window.h"
#include "symbol_entrance_binding.h"

// Extend the native PopupMenu.etc definitions and its two audited CALL sites.
// The menu itself still creates, owns, positions and scales every button.
namespace native_symbol_entrances {
using namespace native_symbol_window;
static InlineHook buildHook,clickHook,keyHook;
struct Route {int menu,role;InfoLayout layout;};
// CN PopupMenu.etc IDs; role enums are independently verified in 115US.
// Current NPC 30 role map contains 535/536/537. The separate native role
// check at 146a4d711 asks for 538; its result must remain unchanged.
static constexpr Route routes[]={{1156,0x217,InfoLayout::Disjoint},{1157,0x218,InfoLayout::Upgrade},{1158,0x219,InfoLayout::Resonance}};
static void* findControl(void* menu,int id){
    auto head=read<void*>(menu,0x180);if(!head)return nullptr;
    auto node=read<void*>(head,8);
    for(unsigned limit=0;node&&node!=head&&limit<128;++limit){
        if(read<unsigned char>(node,0x19))return nullptr;
        const int key=read<int>(node,0x20);
        if(key==id)return read<void*>(node,0x28);
        node=read<void*>(node,key<id?0x10:0);
    }
    return nullptr;
}
static bool __fastcall build(void* npc,int role,void* menu){
    if(!npc)return false;
    const auto original=reinterpret_cast<bool(__fastcall*)(void*,int)>(address(0x145eaf7e0));
    const bool legacy=original(npc,role);
    if(!menu||role!=0x21a)return legacy;
    unsigned roleMask=0,controlMask=0,addedMask=0;
    for(const auto& r:routes){
        const unsigned bit=1u<<static_cast<unsigned>(&r-routes);
        if(!original(npc,r.role))continue;
        roleMask|=bit;
        // An absent paired resource fails closed; never insert a map node.
        if(!findControl(menu,r.menu))continue;
        controlMask|=bit;
        // Native AddEntry returns false even after pushing the visible ID.
        reinterpret_cast<void(__fastcall*)(void*,int)>(address(0x146a24ff0))(menu,r.menu);
        const auto first=read<int*>(menu,0x190),last=read<int*>(menu,0x198);
        if(first&&last>=first&&reinterpret_cast<uintptr_t>(last)-reinterpret_cast<uintptr_t>(first)<=2048&&Readable(first,reinterpret_cast<uintptr_t>(last)-reinterpret_cast<uintptr_t>(first))){for(auto p=first;p!=last;++p)if(*p==r.menu){addedMask|=bit;break;}}
    }
    static unsigned logged=0;
    if(native_symbol_window::log&&logged++<24){fprintf(native_symbol_window::log,"symbol NPC entrance r7 npc=%u legacy538=%u roles=%u controls=%u added=%u\n",read<unsigned>(npc),unsigned(legacy),roleMask,controlMask,addedMask);fflush(native_symbol_window::log);}
    return legacy;
}
// Both mouse and keyboard NPC paths call this dispatcher with RCX=menu,
// EDX=menu ID, after their native input and category gates.
static uintptr_t __fastcall dispatch(void* menu,int id){
    for(const auto& r:routes){
        if(id!=r.menu||!menu||!findControl(menu,id))continue;
        reinterpret_cast<void(__fastcall*)(void*)>(address(0x146a2c070))(menu);
        const bool opened=openInfo(r.layout,false);
        if(native_symbol_window::log){fprintf(native_symbol_window::log,"symbol NPC dispatch r7 menu=%d opened=%u\n",id,unsigned(opened));fflush(native_symbol_window::log);}
        return 0;
    }
    return reinterpret_cast<uintptr_t(__fastcall*)(void*,int)>(address(0x146a31860))(menu,id);
}
static bool validate(uintptr_t imageBase){
    for(const auto& g:symbol_entrance_binding::guards){
        auto p=reinterpret_cast<void*>(imageBase+g.va-0x140000000ull);
        if(!Readable(p,g.size)||memcmp(p,g.bytes,g.size))return false;
    }
    return true;
}
static bool prepareCall(InlineHook& hook,uintptr_t va,uintptr_t original,const unsigned char* move,size_t size,void* callback){
    unsigned char expected[5]={0xe8};
    if(!Rel32(expected+1,address(va)+5,address(original)))return false;
    if(!PrepareHook(hook,reinterpret_cast<void*>(address(va)),expected,5,callback,1,5))return false;
    unsigned char relay[24]{};if(size)memcpy(relay,move,size);AbsoluteJump(relay+size,callback);
    return WriteCode(hook.page+64,relay,size+14);
}
static bool install(){
    // Role-check CALL runs for every NPC before the legacy conditional branch.
    // RCX=NPC template, EDX=538, RSI=native PopupMenuDraw.
    const unsigned char npcMove[]={0x49,0x89,0xf0}; // mov r8,rsi
    if(!prepareCall(buildHook,0x146a4d711,0x145eaf7e0,npcMove,sizeof npcMove,reinterpret_cast<void*>(build)))return false;
    if(!prepareCall(clickHook,0x146a52498,0x146a31860,nullptr,0,reinterpret_cast<void*>(dispatch)))return false;
    if(!prepareCall(keyHook,0x146a2a4d7,0x146a31860,nullptr,0,reinterpret_cast<void*>(dispatch)))return false;
    if(!PublishHook(buildHook,true))return false;
    if(!PublishHook(clickHook,true)){RestoreHook(buildHook);return false;}
    if(!PublishHook(keyHook,true)){RestoreHook(clickHook);RestoreHook(buildHook);return false;}
    if(native_symbol_window::log){fputs("symbol native NPC menu routes r7 installed; NPC roles 535/536/537; native role 538 preserved\n",native_symbol_window::log);fflush(native_symbol_window::log);}
    return true;
}
}
