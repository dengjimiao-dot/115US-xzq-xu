#include <cstdint>
static uintptr_t testAddress(uintptr_t);
#define SYMBOL_WINDOW_TEST_ADDRESS testAddress
#include "native_symbol_runtime.h"
#include <cassert>
#include <map>
#include <cstdio>
#include "npc_mouse_branch_fixture.h"
namespace nw=native_symbol_window;
namespace ne=native_symbol_entrances;
using L=nw::InfoLayout;
static std::map<uintptr_t,uintptr_t> addresses;
static uintptr_t testAddress(uintptr_t va){auto i=addresses.find(va);assert(i!=addresses.end());return i->second;}
static int managerToken,objects[5],opens,closes,reloads;
static void* context=&managerToken;
static bool defer,refuseClose,reloadOK=true;
static int deferredID;
static std::vector<std::pair<L,bool>> opened;
static bool __fastcall originalOpen(void*,void*,intptr_t){return true;}
static void __fastcall originalClose(void*){}
static void* __fastcall originalDestroy(void*,unsigned){return nullptr;}
static void __fastcall managerClose(void*,void* object,bool){if(!refuseClose){++closes;nw::onClose(object);}}
static void completeOpen(int id){
    auto* entry=nw::infoCache.find(id);assert(entry);
    if(!entry->object)nw::infoCache.created(id,&objects[symbol_window_lifecycle::index(id)],context,nw::infoLayout);
    assert(nw::onOpen(nw::infoCache.find(id)->object,nullptr,0));
}
static void* __fastcall managerOpen(void*,int id,uintptr_t,uintptr_t){++opens;if(defer)deferredID=id;else completeOpen(id);return nullptr;}
static void* __fastcall stringCtor(void* value,const wchar_t*,size_t){return value;}
static bool __fastcall reload(void*,void*){++reloads;return reloadOK;}
static void observed(L l,bool internal){opened.emplace_back(l,internal);}
static void reset(){
    nw::infoCache={};nw::infoPopup=nullptr;nw::infoManager=nullptr;nw::infoRequested=false;nw::reopenInfo=false;
    nw::factoryHook.installed=true;nw::uiThread=GetCurrentThreadId();nw::callbacks.infoOpened=observed;
    defer=false;refuseClose=false;reloadOK=true;opens=closes=reloads=0;opened.clear();
}
struct Node {void* left;void* parent;void* right;unsigned char color=0,nil=0;unsigned char pad[6]{};int key=0;int pad2=0;void* control=nullptr;};
static_assert(offsetof(Node,key)==0x20&&offsetof(Node,control)==0x28,"native tree layout");
static char menu[0x340];
static int visibleIDs[8];
struct DescriptorNode{Node base;unsigned char pad[0x24]{};int kind=4;};
static_assert(offsetof(DescriptorNode,kind)==0x54,"native descriptor category offset");
static Node descriptorHead;
static DescriptorNode descriptorNodes[3];
static void* descriptorHeadPointer;
static int ordinaryDispatches;
static Node head,nodes[3];
static int controls[4];
static bool roles[3],menuClosed,legacyRole;
static void* pressed;
static std::vector<int> added;
static bool __fastcall hasRole(void* npc,int role){assert(npc==&managerToken);if(role==538)return legacyRole;if(role==539)return true;for(int i=0;i<3;++i)if(ne::routes[i].role==role)return roles[i];assert(false);return false;}
static bool __fastcall add(void* owner,int id){assert(owner==menu);added.push_back(id);visibleIDs[added.size()-1]=id;*reinterpret_cast<int**>(menu+0x198)=visibleIDs+added.size();return false;}
static uintptr_t __fastcall ordinaryDispatch(void* owner,int id){assert(owner==menu);++ordinaryDispatches;return uintptr_t(0x12340000+id);}
static bool __fastcall predicate(void* c){return c==pressed;}
static void __fastcall closeMenu(void* owner){assert(owner==menu);menuClosed=true;}
static void setupMenu(){
    memset(menu,0,sizeof menu);head={};head.nil=1;head.parent=&nodes[1];
    for(int i=0;i<3;++i){nodes[i]={};nodes[i].key=ne::routes[i].menu;nodes[i].control=&controls[i];nodes[i].left=nodes[i].right=&head;}
    nodes[1].left=&nodes[0];nodes[1].right=&nodes[2];
    *reinterpret_cast<void**>(menu+0x180)=&head;menuClosed=false;added.clear();ordinaryDispatches=0;
    *reinterpret_cast<int**>(menu+0x190)=visibleIDs;*reinterpret_cast<int**>(menu+0x198)=visibleIDs;
    descriptorHead={};descriptorHead.nil=1;descriptorHead.parent=&descriptorNodes[1];descriptorHeadPointer=&descriptorHead;
    for(int i=0;i<3;++i){descriptorNodes[i]={};descriptorNodes[i].base.key=ne::routes[i].menu;descriptorNodes[i].base.left=descriptorNodes[i].base.right=&descriptorHead;descriptorNodes[i].kind=4;}
    descriptorNodes[1].base.left=&descriptorNodes[0];descriptorNodes[1].base.right=&descriptorNodes[2];
    *reinterpret_cast<void**>(menu+0x178)=&descriptorHeadPointer;
}
static std::vector<uint8_t> noTransport(uint16_t,uint64_t,const std::vector<uint8_t>&){assert(false);return {};}
static unsigned char* makeCall(bool build){
    auto* p=NearPage(reinterpret_cast<uintptr_t>(add));assert(p);
    // Save the native nonvolatile context register and supply shadow space.
    unsigned char code[]={0x41,0x56,0x48,0x83,0xec,0x20,0x4d,0x89,0xc6,0xe8,0,0,0,0,0x48,0x83,0xc4,0x20,0x41,0x5e,0xc3};
    if(build){code[0]=0x40;code[6]=0x4c;code[18]=0x40;} // push/pop RSI, mov RSI,R8
    else{code[6]=code[7]=code[8]=0x90;}
    const uintptr_t original=reinterpret_cast<uintptr_t>(build?reinterpret_cast<void*>(hasRole):reinterpret_cast<void*>(ordinaryDispatch));
    assert(Rel32(code+10,reinterpret_cast<uintptr_t>(p)+14,original));memcpy(p,code,sizeof code);
    DWORD old;assert(VirtualProtect(p,4096,PAGE_EXECUTE_READ,&old));FlushInstructionCache(GetCurrentProcess(),p,4096);return p;
}
static unsigned char* makeNativeMouseBranch(){
    auto* p=NearPage(reinterpret_cast<uintptr_t>(ordinaryDispatch));assert(p);
    // Preserve native RBX/R15 and provide ABI-aligned shadow space; args menu,node.
    unsigned char prefix[]={0x53,0x41,0x57,0x48,0x83,0xec,0x28,0x49,0x89,0xcf,0x48,0x89,0xd3};
    memcpy(p,prefix,sizeof prefix);memcpy(p+sizeof prefix,nativeNpcMouseBranch,sizeof nativeNpcMouseBranch);
    auto* body=p+sizeof prefix;
    assert(body[4]==0xe8&&body[0x73]==0xe8);
    assert(Rel32(body+5,reinterpret_cast<uintptr_t>(body)+9,reinterpret_cast<uintptr_t>(predicate)));
    assert(Rel32(body+0x74,reinterpret_cast<uintptr_t>(body)+0x78,reinterpret_cast<uintptr_t>(ordinaryDispatch)));
    unsigned char suffix[]={0x48,0x83,0xc4,0x28,0x41,0x5f,0x5b,0xc3};memcpy(body+sizeof nativeNpcMouseBranch,suffix,sizeof suffix);
    DWORD old;assert(VirtualProtect(p,4096,PAGE_EXECUTE_READ,&old));FlushInstructionCache(GetCurrentProcess(),p,4096);return p;
}
int main(){
    addresses[0x14e683c78]=reinterpret_cast<uintptr_t>(&context);
    addresses[0x14668c520]=reinterpret_cast<uintptr_t>(managerOpen);
    addresses[0x146671a70]=reinterpret_cast<uintptr_t>(managerClose);
    addresses[0x145f6f4b0]=reinterpret_cast<uintptr_t>(originalOpen);
    addresses[native_symbol_window_binding::methods[0x30/8]]=reinterpret_cast<uintptr_t>(originalClose);
    addresses[0x145f6dca0]=reinterpret_cast<uintptr_t>(originalDestroy);
    addresses[0x14014c8d0]=reinterpret_cast<uintptr_t>(stringCtor);
    addresses[0x145f6f2e0]=reinterpret_cast<uintptr_t>(reload);
    addresses[0x145eaf7e0]=reinterpret_cast<uintptr_t>(hasRole);
    addresses[0x146a24ff0]=reinterpret_cast<uintptr_t>(add);
    addresses[0x146a31860]=reinterpret_cast<uintptr_t>(ordinaryDispatch);
    addresses[0x146a2c070]=reinterpret_cast<uintptr_t>(closeMenu);
    const L layouts[]={L::Summary,L::Resonance,L::Upgrade,L::Disjoint,L::Shop,L::Confirm,L::Change};
    // Execute actual openInfo/onClose/pumpInfo against a manager that caches
    // every closed popup and never calls its destructor, reproducing r4.
    for(auto a:layouts)for(auto b:layouts){
        reset();assert(nw::openInfo(a));void* old=nw::infoPopup;assert(old);
        assert(nw::openInfo(b));nw::pumpInfo();assert(nw::infoPopup&&nw::infoLayout==b&&!nw::reopenInfo);
        void* reused=nw::infoPopup;managerClose(context,reused,false);assert(!nw::infoPopup);
        assert(nw::openInfo(b));assert(nw::infoPopup==reused&&nw::infoLayout==b);
        if(old!=reused){nw::onClose(old);assert(nw::infoPopup==reused);nw::destroy(old,0);assert(nw::infoPopup==reused);}
    }
    reset();nw::openInfo(L::Shop);nw::openInfo(L::Change);reloadOK=false;nw::pumpInfo();assert(!nw::infoPopup&&!nw::infoRequested);assert(reloads==1);
    reloadOK=true;assert(nw::openInfo(L::Change));assert(nw::infoPopup&&nw::infoLayout==L::Change);
    reset();nw::openInfo(L::Resonance);refuseClose=true;nw::openInfo(L::Upgrade);nw::pumpInfo();assert(nw::infoLayout==L::Resonance&&opens==1&&nw::reopenInfo);
    refuseClose=false;nw::pumpInfo();assert(nw::infoLayout==L::Upgrade&&opens==2);
    reset();defer=true;nw::openInfo(L::Resonance,false);nw::openInfo(L::Upgrade,false);nw::openInfo(L::Disjoint,false);nw::pumpInfo();assert(opens==1);
    defer=false;completeOpen(deferredID);nw::pumpInfo();assert(nw::infoLayout==L::Disjoint&&!nw::reopenInfo);assert(!opened.back().second);
    // Exercise the published machine-code CALL adapters inside this process,
    // including register capture, ordinary click passthrough and restoration.
    auto* buildCode=makeCall(true);auto* clickCode=makeNativeMouseBranch();auto* keyCode=makeCall(false);
    addresses[0x146a4d711]=reinterpret_cast<uintptr_t>(buildCode+9);
    addresses[0x146a52498]=reinterpret_cast<uintptr_t>(clickCode+13+0x73);
    addresses[0x146a2a4d7]=reinterpret_cast<uintptr_t>(keyCode+9);
    assert(ne::install());
    assert(ne::routes[0].role==535&&ne::routes[1].role==536&&ne::routes[2].role==537);
    for(int old=0;old<2;++old)for(int mask=0;mask<8;++mask){reset();setupMenu();legacyRole=old!=0;for(int i=0;i<3;++i)roles[i]=(mask&(1<<i))!=0;
        bool result=reinterpret_cast<bool(__fastcall*)(void*,int,void*)>(buildCode)(&managerToken,538,menu);
        assert(result==legacyRole);unsigned count=0;for(int i=0;i<3;++i)if(roles[i]){assert(added[count++]==ne::routes[i].menu);}assert(added.size()==count);
    }
    // Reproduce the live failure: all three source roles exist, legacy 538
    // is absent. Entries must still be added while the native result is false.
    setupMenu();legacyRole=false;roles[0]=roles[1]=roles[2]=true;
    assert(!reinterpret_cast<bool(__fastcall*)(void*,int,void*)>(buildCode)(&managerToken,538,menu));assert(added.size()==3);
    setupMenu();nodes[2].control=nullptr;
    assert(!reinterpret_cast<bool(__fastcall*)(void*,int,void*)>(buildCode)(&managerToken,538,menu));assert(added.size()==2);
    setupMenu();assert(ne::build(&managerToken,539,menu));assert(added.empty());
    assert(!ne::build(nullptr,538,menu));
    for(int i=0;i<3;++i){
        reset();setupMenu();pressed=&controls[i];
        reinterpret_cast<void(__fastcall*)(void*,void*)>(clickCode)(menu,&nodes[i]);
        assert(menuClosed&&nw::infoLayout==ne::routes[i].layout&&nw::infoPopup&&!ordinaryDispatches);assert(!opened.back().second);
        reset();setupMenu();reinterpret_cast<uintptr_t(__fastcall*)(void*,int,void*)>(keyCode)(menu,ne::routes[i].menu,nullptr);
        assert(menuClosed&&nw::infoLayout==ne::routes[i].layout&&nw::infoPopup&&!ordinaryDispatches);
    }
    // Original NPC input/category gates are executed from the current PE bytes.
    reset();setupMenu();pressed=nullptr;reinterpret_cast<void(__fastcall*)(void*,void*)>(clickCode)(menu,&nodes[0]);assert(!menuClosed&&!opens&&!ordinaryDispatches);
    reset();setupMenu();pressed=&controls[0];descriptorNodes[0].kind=2;reinterpret_cast<void(__fastcall*)(void*,void*)>(clickCode)(menu,&nodes[0]);assert(!menuClosed&&!opens&&!ordinaryDispatches);
    reset();setupMenu();assert(reinterpret_cast<uintptr_t(__fastcall*)(void*,int,void*)>(keyCode)(menu,43,nullptr)==0x1234002b);assert(ordinaryDispatches==1&&!menuClosed&&!opens);
    reset();setupMenu();nodes[2].control=nullptr;assert(ne::dispatch(menu,1158)==0x12340486);assert(ordinaryDispatches==1&&!menuClosed&&!opens);
    assert(RestoreHook(ne::keyHook)&&RestoreHook(ne::clickHook)&&RestoreHook(ne::buildHook));
    VirtualFree(buildCode,0,MEM_RELEASE);VirtualFree(clickCode,0,MEM_RELEASE);VirtualFree(keyCode,0,MEM_RELEASE);
    namespace runtime=native_symbol_runtime;
    // A pending worker cannot leave hotkey item-use without a native UI tick.
    // Use a real handle in place of a blocked worker, then deliver its reply.
    for(unsigned char kind:{1,2}){
        reset();defer=true;nw::callbacks.infoOpened=runtime::infoOpened;
        runtime::transport=noTransport;runtime::worker=CreateEventW(nullptr,TRUE,FALSE,nullptr);assert(runtime::worker);
        assert(runtime::itemUse(kind,17,12345));
        assert(nw::infoRequested&&deferredID==4159&&runtime::pendingUseKind==kind&&!runtime::ready);
        const auto generation=runtime::generation;
        auto response=std::make_unique<runtime::Result>();response->generation=generation;response->id=runtime::pendingID;response->kind=3;response->view.role=9;response->view.sequence=77;
        runtime::result=std::move(response);runtime::update();
        assert(runtime::ready&&!runtime::worker&&!runtime::pendingUseKind&&runtime::view.role==9);
        assert(native_symbol_growth::useSlot==17&&native_symbol_growth::useTemplate==12345&&native_symbol_growth::mode);
        assert(native_symbol_growth::state.panel==(kind==1?native_symbol_growth::Panel::Ticket:native_symbol_growth::Panel::Change));
        defer=false;completeOpen(deferredID);assert(runtime::generation==generation);
        // Selecting or leaving the inventory tab must retain an NPC/item page.
        nw::reopenInfo=true;runtime::enter();runtime::leave();
        assert(runtime::active&&runtime::ready&&runtime::generation==generation&&native_symbol_growth::mode);
        runtime::inventoryDestroyed(nullptr);assert(!runtime::active&&!runtime::ready);
    }
    reset();runtime::active=false;runtime::pendingUseKind=1;runtime::generation=200;runtime::pendingID=6;
    auto stale=std::make_unique<runtime::Result>();stale->generation=199;stale->id=6;stale->kind=3;stale->view.role=99;
    runtime::result=std::move(stale);runtime::update();assert(!runtime::ready&&!nw::infoRequested);
    reset();runtime::active=true;runtime::generation=250;runtime::pendingID=7;runtime::pendingUseKind=1;
    auto cancelled=std::make_unique<runtime::Result>();cancelled->generation=250;cancelled->id=7;cancelled->kind=3;cancelled->view.role=9;
    runtime::result=std::move(cancelled);runtime::update();assert(runtime::ready&&!runtime::pendingUseKind&&!native_symbol_growth::mode&&!nw::infoRequested);
    puts("PASS hotkey loading popup, fresh async view, inventory/NPC state preservation, role cleanup, stale reply rejection and close-before-reply cancellation");
    puts("PASS: 49 cached page transitions; native CALL adapters, 16 role masks with legacy result preservation, live NPC30 false-gate regression, missing-control refusal, current PE NPC mouse branch, keyboard dispatch, ordinary dispatch and three hook byte restoration");
}
