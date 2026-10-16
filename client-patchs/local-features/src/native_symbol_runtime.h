#pragma once
#include <mutex>
#include <memory>
#include "symbol_protocol.h"
#include "native_symbol_window.h"
#include "native_symbol_info.h"
#include "symbol_effect_sync.h"
#include "native_symbol_growth.h"
#include "native_symbol_entrances.h"
#include "native_symbol_item_use.h"

namespace native_symbol_runtime {
using Exchange=std::vector<uint8_t>(*)(uint16_t,uint64_t,const std::vector<uint8_t>&);
struct Job {uint64_t generation,id;uint16_t kind;std::vector<uint8_t> body;};
struct Result {uint64_t generation,id;uint16_t kind;symbol_wire::View view;std::string error;};
static Exchange transport;
static void (*inventoryNeeded)()=nullptr;
static FILE* log;
static std::mutex mutex;
static std::unique_ptr<Result> result;
static HANDLE worker;
static uint64_t generation=0,nextID=0,pendingID=0;
static bool active=false,ready=false;
static symbol_wire::View view;
static std::string desiredGroup;
static uint16_t selectedSocket=0;
static ULONGLONG retryAfter=0;
static bool gesture=false;
static uint16_t gestureSlot=0;
static POINT gestureOrigin{};
static ULONGLONG lastRelease=0;
static unsigned char pendingUseKind=0;
static uint16_t pendingUseSlot=0;
static uint32_t pendingUseTemplate=0;
static void enter();

static void report(const char* text){if(log){fprintf(log,"symbol: %s\n",text);fflush(log);}}
static DWORD WINAPI run(void*arg){
    std::unique_ptr<Job> j(static_cast<Job*>(arg));
    auto r=std::make_unique<Result>();r->generation=j->generation;r->id=j->id;r->kind=j->kind;
    try {r->view=symbol_wire::decode(transport(j->kind,j->id,j->body));if(j->kind==4||j->kind==6)symbol_effect_sync::changed();}
    catch(const std::exception&e){r->error=e.what();}
    catch(...){r->error="unexpected transport failure";}
    std::lock_guard<std::mutex> lock(mutex);result=std::move(r);return 0;
}
static void begin(uint16_t kind,std::vector<uint8_t> body={}){
    if(worker||!transport||!active)return;
    auto j=std::make_unique<Job>();j->generation=generation;j->id=++nextID;j->kind=kind;j->body=std::move(body);
    pendingID=j->id;ready=false;
    worker=CreateThread(nullptr,0,run,j.get(),0,nullptr);
    if(worker)j.release();else {pendingID=0;retryAfter=GetTickCount64()+2000;report("worker creation failed");}
}
static void render(){
    auto g=view.find(desiredGroup);
    if(!ready||!g)return;
    std::vector<uint32_t> items;for(auto&i:g->items)items.push_back(i.id);
    if(!native_symbol_window::render(desiredGroup,items))report("popup/source slot agreement rejected");
}
static void closed(){if(native_symbol_window::requestedGroup.empty()){desiredGroup.clear();selectedSocket=0;}}
static void opened(const std::string&name){
    if(!active||!ready||name!=desiredGroup){native_symbol_window::close();return;}
    render();
}
static void star(const std::string&name){
    if(!active||!ready||!view.find(name))return;
    desiredGroup=name;selectedSocket=0;
    native_symbol_info::flash(name);
    if(!native_symbol_window::open(name)){desiredGroup.clear();report("native manager refused popup");}
}
static void socket(const std::string&name,uint16_t index){
    auto g=view.find(name);
    if(!ready||name!=desiredGroup||!g||index>=g->items.size())return;
    selectedSocket=index;
    native_symbol_window::flash(index);
}
static void remove(const std::string&name,uint16_t index){
    auto g=view.find(name);
    if(!ready||name!=desiredGroup||!g||index>=g->items.size()||!g->items[index].id)return;
    selectedSocket=index;
    try{begin(4,symbol_wire::exchange(view,name,index,nullptr));}catch(const std::exception&e){report(e.what());}
}
static void equip(uint16_t slot){
    if(!ready||!active)return;
    const symbol_wire::BagItem* incoming=nullptr;
    for(auto&i:view.bag)if(i.slot==slot){incoming=&i;break;}
    if(!incoming)return;
    auto g=view.find(desiredGroup);
    if(!g||!(g->mask&incoming->mask))return;
    if(selectedSocket>=g->items.size())return;
    if(g->items[selectedSocket].id==incoming->id)return;
    try{begin(4,symbol_wire::exchange(view,desiredGroup,selectedSocket,incoming));}catch(const std::exception&e){report(e.what());}
}
static void bag(uint16_t slot){
    if(!ready||!active||GetTickCount64()-lastRelease<120)return;
    if(GetAsyncKeyState(VK_LBUTTON)&0x8000){
        if(!gesture){gesture=true;gestureSlot=slot;GetCursorPos(&gestureOrigin);if(log){fprintf(log,"symbol drag begin bag=%u\n",slot);fflush(log);}}
    }else if(!gesture){if(native_symbol_growth::selecting())native_symbol_growth::select(view,slot);else equip(slot);lastRelease=GetTickCount64();}
}
static bool incompatible(uint16_t slot){
    if(!active)return false;
    if(native_symbol_growth::selecting())return !native_symbol_growth::state.eligible(view,slot);
    if(desiredGroup.empty()||!native_symbol_window::popup)return false;
    auto g=view.find(desiredGroup);if(!g)return true;
    for(auto&i:view.bag)if(i.slot==slot)return !(i.mask&g->mask);
    return true;
}
static int draggedSlot(){
    if(!active||!ready||!gesture||!(GetAsyncKeyState(VK_LBUTTON)&0x8000))return -1;
    DWORD pid=0;GetWindowThreadProcessId(GetForegroundWindow(),&pid);
    if(pid!=GetCurrentProcessId())return -1;
    POINT at{};GetCursorPos(&at);
    if(abs(at.x-gestureOrigin.x)<GetSystemMetrics(SM_CXDRAG)&&abs(at.y-gestureOrigin.y)<GetSystemMetrics(SM_CYDRAG))return -1;
    return gestureSlot;
}
static void growth(const symbol_wire::GrowthOption&option){if(!active||!ready)return;try{begin(6,symbol_wire::grow(view,option));}catch(const std::exception&e){native_symbol_growth::error=e.what();report(e.what());}}
static void header(void* tab){
    native_symbol_window::pumpInfo();
    native_symbol_window::Control detail(tab,L"detail_info_Button",true);
    if(native_symbol_window::clicked(detail.value)){native_symbol_growth::reset();}
    native_symbol_info::header(tab,view,ready);

}
static void update(){
    std::unique_ptr<Result> r;
    {std::lock_guard<std::mutex> lock(mutex);r=std::move(result);}
    if(r){
        if(worker){CloseHandle(worker);worker=nullptr;}
        if(active&&r->generation==generation&&r->id==pendingID){
            pendingID=0;
            if(!r->error.empty()){
                native_symbol_growth::timeline.cancel();native_symbol_growth::error=r->error;report(r->error.c_str());ready=false;retryAfter=GetTickCount64()+1500;
            }else if((r->kind==4||r->kind==6)&&(r->view.role!=view.role||r->view.source!=view.source||r->view.sequence!=view.sequence+1)){
                report("exchange identity/sequence mismatch");ready=false;retryAfter=GetTickCount64()+1500;
            }else{
                if(r->kind==6)native_symbol_growth::completed(r->view);
                view=std::move(r->view);ready=true;retryAfter=0;
                if(pendingUseKind){
                    const auto kind=pendingUseKind;pendingUseKind=0;
                    native_symbol_growth::reset();
                    // Closing the loading page cancels its pending UI action.
                    // Keep the fresh snapshot without reopening the page.
                    if(native_symbol_window::infoPopup||native_symbol_window::infoRequested||native_symbol_window::reopenInfo){
                    native_symbol_growth::mode=true;
                    native_symbol_growth::state.panel=kind==1?native_symbol_growth::Panel::Ticket:native_symbol_growth::Panel::Change;
                    native_symbol_growth::useSlot=pendingUseSlot;native_symbol_growth::useTemplate=pendingUseTemplate;
                    native_symbol_window::openInfo(native_symbol_window::InfoLayout::Shop);
                    }
                }
                if(r->kind==4){native_symbol_window::flash(selectedSocket);native_symbol_info::flash(desiredGroup);}
                if(!desiredGroup.empty())render();
            }
        }
    }
    if(gesture&&!(GetAsyncKeyState(VK_LBUTTON)&0x8000)){
        gesture=false;lastRelease=GetTickCount64();
        DWORD pid=0;GetWindowThreadProcessId(GetForegroundWindow(),&pid);
        POINT end{};GetCursorPos(&end);
        const bool moved=abs(end.x-gestureOrigin.x)>=GetSystemMetrics(SM_CXDRAG)||abs(end.y-gestureOrigin.y)>=GetSystemMetrics(SM_CYDRAG);
        const int target=native_symbol_growth::selecting()?native_symbol_growth::dropTarget():native_symbol_window::dropTarget();
        if(log){fprintf(log,"symbol drag release moved=%d target=%d ready=%d foreground=%d\n",int(moved),target,int(ready),int(pid==GetCurrentProcessId()));fflush(log);}
        if(active&&ready&&pid==GetCurrentProcessId()&&(!moved||target>=0)){
            if(target>=0&&!native_symbol_growth::selecting()){selectedSocket=uint16_t(target);native_symbol_window::highlight(selectedSocket);}
            if(native_symbol_growth::selecting())native_symbol_growth::select(view,gestureSlot);else equip(gestureSlot);
        }
    }
    native_symbol_info::summary(view,ready);
    native_symbol_growth::render(view,ready,growth);
    if(active&&!ready&&!worker&&GetTickCount64()>=retryAfter)begin(3);
}
static void enter(){if(active&&(native_symbol_window::infoPopup||native_symbol_window::infoRequested||native_symbol_window::reopenInfo)){update();return;}++generation;active=true;ready=false;gesture=false;pendingUseKind=0;native_symbol_info::reset();native_symbol_growth::reset();view={};desiredGroup.clear();selectedSocket=0;retryAfter=0;update();}
static void leave(){gesture=false;const bool preserve=native_symbol_window::infoPopup||native_symbol_window::infoRequested||native_symbol_window::reopenInfo;native_symbol_window::close(!preserve);desiredGroup.clear();selectedSocket=0;if(preserve)return;++generation;active=false;ready=false;pendingID=0;pendingUseKind=0;native_symbol_info::reset();native_symbol_growth::reset();view={};}
static void inventoryUpdated(void*){if(!active)enter();else update();}
static void inventoryDestroyed(void*){++generation;active=false;ready=false;pendingID=0;pendingUseKind=0;gesture=false;view={};desiredGroup.clear();native_symbol_info::reset();native_symbol_growth::reset();}
static bool itemUse(unsigned char kind,uint16_t slot,uint32_t id){
    if(!transport||kind<1||kind>2)return false;
    ++generation;active=true;ready=false;pendingID=0;gesture=false;view={};retryAfter=0;
    pendingUseKind=kind;pendingUseSlot=slot;pendingUseTemplate=id;
    native_symbol_growth::reset();native_symbol_window::close(false);desiredGroup.clear();selectedSocket=0;
    native_symbol_growth::mode=true;native_symbol_growth::state.panel=kind==1?native_symbol_growth::Panel::Ticket:native_symbol_growth::Panel::Change;
    native_symbol_growth::useSlot=slot;native_symbol_growth::useTemplate=id;
    // A hotkey can use the item while inventory is closed. Open a native
    // loading window now so its UI tick also consumes the asynchronous reply.
    if(!native_symbol_window::openInfo(native_symbol_window::InfoLayout::Shop)){pendingUseKind=0;report("native item-use popup refused");return true;}
    update();return true; // Opening never submits or consumes the item.
}
static void infoOpened(native_symbol_window::InfoLayout layout,bool requested){
    if(requested)return; // Internal confirmation transitions retain their selection.
    ++generation;active=true;ready=false;pendingID=0;pendingUseKind=0;view={};retryAfter=0;
    begin(3);
    native_symbol_growth::reset();
    using L=native_symbol_window::InfoLayout;using P=native_symbol_growth::Panel;
    if(layout!=L::Summary){
        native_symbol_growth::mode=true;
        native_symbol_growth::state.panel=layout==L::Resonance?P::Resonance:layout==L::Upgrade?P::Upgrade:layout==L::Disjoint?P::Disjoint:P::Shop;
    }
    if((layout==L::Upgrade||layout==L::Disjoint)&&inventoryNeeded)inventoryNeeded();
}
static bool install(uintptr_t base,FILE* sink,Exchange exchange,void (*inventoryOpen)()=nullptr){
    inventoryNeeded=inventoryOpen;
    transport=exchange;log=sink;
    if(!native_symbol_entrances::validate(base))return false;
    if(!native_symbol_item_use::validate(base))return false;
    if(!native_symbol_window::install(base,sink,{infoOpened,opened,socket,closed,update,remove}))return false;
    if(native_symbol_entrances::install()&&native_symbol_item_use::install(itemUse))return true;
    RestoreHook(native_symbol_entrances::buildHook);RestoreHook(native_symbol_entrances::clickHook);
    RestoreHook(native_symbol_window::factoryHook);
    return false;
}
}
