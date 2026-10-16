#include <cstdint>
static uintptr_t testAddress(uintptr_t);
#define SYMBOL_WINDOW_TEST_ADDRESS testAddress
#include "native_symbol_item_use.h"
#include <cassert>
#include <cstdio>
namespace use=native_symbol_item_use;
static unsigned char data[0x20],container[0xb0],cell[0x48];
static void* cells[3];static uintptr_t vt[20];static uintptr_t* item=vt;
static int requests;static bool accept=true;
static unsigned char expectedKind;static uint32_t expectedTemplate;
static void* __fastcall baseData(void* p){assert(p==&item);return data;}
static void __fastcall decode(void* field,unsigned n,unsigned char type,unsigned char flag,void* checksum){assert(field==data+0x18&&n==4&&type==0x32&&flag==1&&checksum==data+0x1c);}
static uintptr_t testAddress(uintptr_t va){assert(va==0x1480a6620);return reinterpret_cast<uintptr_t>(decode);}
static bool requested(unsigned char kind,uint16_t slot,uint32_t id){assert(kind==expectedKind&&id==expectedTemplate&&slot==1);++requests;return accept;}
int main(){
    auto* image=static_cast<unsigned char*>(VirtualAlloc(nullptr,0x8100000,MEM_RESERVE,PAGE_NOACCESS));assert(image);
    const auto imageBase=reinterpret_cast<uintptr_t>(image)-0x140000000ull+0x140000000ull;
    for(const auto& g:symbol_item_use_binding::guards){auto offset=g.va-0x140000000ull;auto page=offset&~uintptr_t(4095);assert(VirtualAlloc(image+page,4096,MEM_COMMIT,PAGE_READWRITE));memcpy(image+offset,g.bytes,g.size);}
    assert(use::validate(imageBase));
    for(const auto& g:symbol_item_use_binding::guards){auto* p=image+g.va-0x140000000ull;*p^=1;assert(!use::validate(imageBase));*p^=1;}
    VirtualFree(image,0,MEM_RELEASE);
    vt[0x98/8]=reinterpret_cast<uintptr_t>(baseData);*reinterpret_cast<void**>(cell)=&item;
    cells[1]=cell+0x40;*reinterpret_cast<int*>(container+0x58)=3;*reinterpret_cast<void**>(container+0xa8)=cells;
    const auto& guard=symbol_item_use_binding::guards[0];
    auto* machine=NearPage(reinterpret_cast<uintptr_t>(use::use));assert(machine);
    memcpy(machine,guard.bytes,14);
    // Original records its arguments and returns flags, then restores all pushes.
    unsigned char tail[]={0x89,0x51,0x60,0x44,0x89,0x41,0x64,0x44,0x89,0xc0,0x41,0x5f,0x41,0x5e,0x41,0x5d,0x41,0x5c,0x5f,0x5e,0x5d,0xc3};
    memcpy(machine+14,tail,sizeof tail);DWORD old;assert(VirtualProtect(machine,4096,PAGE_EXECUTE_READ,&old));
    assert(PrepareHook(use::hook,machine,guard.bytes,14,reinterpret_cast<void*>(use::use)));assert(PublishHook(use::hook));
    use::request=requested;auto native=reinterpret_cast<use::Use>(machine);
    for(auto& route:symbol_item_routing::entries){
        expectedKind=route.kind;expectedTemplate=route.id;*reinterpret_cast<uint32_t*>(data+0x18)=route.id;
        accept=true;const int before=requests;assert(native(container,1,0));assert(requests==before+1);
        accept=false;assert(!native(container,1,0));assert(native(container,1,1));
        assert(*reinterpret_cast<int*>(container+0x60)==1&&*reinterpret_cast<int*>(container+0x64)==1);
        native_symbol_window::uiThread=GetCurrentThreadId()+1;assert(!native(container,1,0));native_symbol_window::uiThread=0;
    }
    *reinterpret_cast<uint32_t*>(data+0x18)=42;const int before=requests;assert(!native(container,1,0));assert(requests==before);
    assert(use::itemTemplate(container,-1)==0&&use::itemTemplate(container,3)==0&&use::itemTemplate(nullptr,1)==0);
    assert(use::itemTemplate(container,0)==0);assert(RestoreHook(use::hook));assert(!memcmp(machine,guard.bytes,14));
    assert(native(container,2,1));assert(*reinterpret_cast<int*>(container+0x60)==2);
    puts("PASS source-bound native item use: guarded adapter, template/slot, ordinary forwarding, thread rejection, byte-exact restore");
}
