#include <windows.h>
#include <bcrypt.h>
#include <string>
#include <vector>
#include <algorithm>
#include <cstdint>
static uintptr_t testAddress(uintptr_t);
static HWND fakeForeground;
#define SYMBOL_WINDOW_TEST_FOREGROUND() fakeForeground
#define SYMBOL_WINDOW_TEST_ADDRESS testAddress
#define NATIVE_INVENTORY_TEST_ADDRESS testAddress
#include "native_inventory.h"
#include "native_symbol_growth.h"
#include <cassert>
#include <map>
#include <cstdio>
static std::map<uintptr_t,uintptr_t> addresses;
static uintptr_t testAddress(uintptr_t va){auto i=addresses.find(va);assert(i!=addresses.end());return i->second;}
static int resets,loads,draws,replays;
static void __fastcall replay(void*){++replays;}
static int submissions;
static void submit(const symbol_wire::GrowthOption&){++submissions;}
static float positionedX,positionedY;
static void __fastcall position(void* c,const float* point){positionedX=point[0];positionedY=point[1];memcpy(static_cast<char*>(c)+0x90,point,8);}
static std::wstring replacedStyle;
static bool __fastcall replaceStyle(void*,const std::wstring* value,bool clearFormatting){assert(!clearFormatting);replacedStyle=*value;return true;}
static void __fastcall namePos(void* c,int x,int y){assert(x==36&&y==8);*reinterpret_cast<int*>(static_cast<char*>(c)+0x3a8)=x;*reinterpret_cast<int*>(static_cast<char*>(c)+0x3ac)=y;}
static void __fastcall nameVisible(void* c,bool v){*reinterpret_cast<bool*>(static_cast<char*>(c)+0x3b0)=v;}
static void __fastcall stackVisible(void* c,bool v){*reinterpret_cast<bool*>(static_cast<char*>(c)+0x57a)=v;}
static void __fastcall stackAmount(void* c,uint32_t amount){*reinterpret_cast<uint32_t*>(static_cast<char*>(c)+0x57c)=amount;}
static bool visibleValue;
static void __fastcall clear(void*){++resets;}
static void __fastcall load(void*,bool){++loads;}
static void __fastcall visible(void*,bool v){visibleValue=v;}
static int __fastcall selected(void*){return 4;}
static int __fastcall rowSlot(void*,int row){return row==7?290:-1;}
static int dragging(){return 290;}
static uintptr_t itemTable[8];static uintptr_t* item=itemTable;
static void* __fastcall rowItem(void*,int){return &item;}
static uint64_t __fastcall cursor(void*){return uint64_t(400)|(uint64_t(300)<<32);}
static int __fastcall anchor(void*){return 0;}
static float __fastcall convert(void*,int,float f){return f;}
static void __fastcall draw(void*,float x,float y,const void* options){assert(x==386.f&&y==286.f);assert(static_cast<const unsigned char*>(options)[12]==0);++draws;}
int main(){
    using namespace symbol_growth_presentation;
    fakeForeground=CreateWindowExW(0,L"STATIC",L"Hidden badge UI test",WS_POPUP,0,0,800,600,nullptr,nullptr,GetModuleHandleW(nullptr),nullptr);assert(fakeForeground);
    const auto testWindow=fakeForeground;
    auto joined=[](const std::vector<std::wstring>& rows){std::wstring all;for(const auto& row:rows)all+=row;return all;};
    const std::wstring badge=L"华丽的红色徽章 [力量+移动速度] × 7";
    auto wrapped=symbol_text_layout::wrap(badge,34);assert(wrapped.size()>1&&wrapped.size()<=3&&joined(wrapped)==badge);
    const std::wstring skill=L"技能攻击力+1.2%";auto skillRows=symbol_text_layout::resonance(skill,14);assert(skillRows.size()==2&&skillRows[0]==L"技能攻击力"&&skillRows[1]==L"+1.2%"&&joined(skillRows)==skill);
    const std::wstring full=L"所有属性强化+123";assert(joined(symbol_text_layout::resonance(full,14))==full);
    Timeline t;t.start(3,100,1280);t.completed(3,101,1880);
    assert(t.progress(3,1379)&&!t.result(3,1379));assert(!t.progress(3,1380)&&t.result(3,1380));assert(t.occupied(3259)&&!t.occupied(3260));
    assert(disjointStage(t,1379,true)==1);assert(disjointStage(t,1380,true)==2);assert(disjointStage(t,3259,true)==2);assert(disjointStage(t,3260,true)==3);assert(disjointStage(t,3260,false)==0);
    Timeline d;d.start(3,100,1280);d.completed(3,101,3280);
    assert(!disjointResultTextVisible(d,3259,1880));assert(disjointResultTextVisible(d,3260,1880));assert(disjointResultTextVisible(d,4659,1880));assert(!disjointResultTextVisible(d,4660,1880));
    t.start(2,4000,1100);t.completed(2,8000,1700);assert(t.result(2,8000));t.cancel();assert(!t.occupied(8001)&&!t.result(2,8001));
    symbol_wire::View v;v.growth.material=123;v.growth.balance=90;
    symbol_wire::GrowthOption o;o.operation=2;o.ui.material=123;o.ui.amount=375;
    assert(lacksQuotedMaterial(v,o));o.ui.amount=90;assert(!lacksQuotedMaterial(v,o));o.ui.amount=0;assert(!lacksQuotedMaterial(v,o));o.ui.material=999;o.ui.amount=999;assert(!lacksQuotedMaterial(v,o));
    uintptr_t table[96]{};table[2]=reinterpret_cast<uintptr_t>(visible);uintptr_t* object=table;
    addresses[0x14b13a9a8]=reinterpret_cast<uintptr_t>(table);
    addresses[0x146af0000]=reinterpret_cast<uintptr_t>(replay);
    addresses[0x146af06e0]=reinterpret_cast<uintptr_t>(clear);addresses[0x146af04c0]=reinterpret_cast<uintptr_t>(load);
    native_symbol_growth::timeline.start(2,1,1100);
    for(int i=0;i<100;++i)native_symbol_growth::animate(&object,true);
    assert(resets==1&&loads==1&&replays==1&&visibleValue);
    native_symbol_growth::timeline.start(2,4000,1100);native_symbol_growth::animate(&object,true);assert(resets==2&&replays==2);
    native_symbol_growth::animate(&object,false);native_symbol_growth::animate(&object,true);assert(resets==3&&replays==3);
    native_symbol_growth::timeline={};o.ui.material=123;o.ui.amount=375;
    native_symbol_growth::send(v,&o,submit);assert(submissions==0&&native_symbol_growth::timeline.cycle==0&&native_symbol_growth::error=="材料不足");
    v.growth.balance=375;native_symbol_growth::send(v,&o,submit);assert(submissions==1&&native_symbol_growth::timeline.cycle==1);
    native_symbol_growth::send(v,&o,submit);assert(submissions==1);
    native_symbol_growth::timeline.cancel();native_symbol_growth::send(v,&o,submit);assert(submissions==2&&native_symbol_growth::timeline.cycle==2);
    native_symbol_growth::timeline.cancel();native_symbol_growth::mode=true;native_symbol_growth::state.panel=symbol_growth_state::Panel::Upgrade;native_symbol_window::infoPopup=&object;
    assert(!native_symbol_growth::select(v,290)&&!native_symbol_growth::error.empty());
    o.operation=4;o.slot=290;v.growth.options.push_back(o);assert(native_symbol_growth::select(v,290)&&native_symbol_growth::error.empty());
    native_symbol_growth::state.special=false;v.growth.options[0].ui.points=15;v.growth.options[0].id=2500018;
    native_symbol_growth::refreshNormalProgress(v,&v.growth.options[0]);assert(native_symbol_growth::normalPoints==15);
    native_symbol_growth::state.selected=UINT16_MAX;native_symbol_growth::refreshNormalProgress(v,nullptr);assert(native_symbol_growth::normalPoints==15);
    v.growth.options[0].ui.points=30;native_symbol_growth::refreshNormalProgress(v,nullptr);assert(native_symbol_growth::normalPoints==30);
    native_symbol_growth::reset();assert(native_symbol_growth::normalPoints==0&&native_symbol_growth::normalTemplate==0);
    // Restore selection fixtures for the eligibility and drag checks below.
    o.operation=4;o.id=2500018;v.growth.options[0]=o;
    // Reproduce the old missing-ghost guard: no cell redraw for more than 100ms.
    // Resolve the item from the current renderer and verify its slot every draw.
    char window[0x700]{},avatar[0x500]{},grid[0x300]{};int group,layoutToken;void* layout=&layoutToken;
    *reinterpret_cast<void**>(window+0x618)=&group;*reinterpret_cast<void**>(avatar+0x40)=grid;
    addresses[0x146f50350]=reinterpret_cast<uintptr_t>(selected);addresses[0x1458bf660]=reinterpret_cast<uintptr_t>(rowSlot);addresses[0x1458c2130]=reinterpret_cast<uintptr_t>(rowItem);
    addresses[0x14e6343e0]=reinterpret_cast<uintptr_t>(&layout);addresses[0x146ec45d0]=reinterpret_cast<uintptr_t>(cursor);addresses[0x140738710]=reinterpret_cast<uintptr_t>(anchor);addresses[0x146ec6b00]=reinterpret_cast<uintptr_t>(convert);addresses[0x146ec6c90]=reinterpret_cast<uintptr_t>(convert);
    char tooltip[0x300]{},parent[0x300]{};*reinterpret_cast<void**>(tooltip+0x190)=reinterpret_cast<void*>(1);*reinterpret_cast<float*>(parent+0xcc)=200.f;*reinterpret_cast<float*>(parent+0xd0)=100.f;
    addresses[0x146ecc720]=reinterpret_cast<uintptr_t>(position);
    *reinterpret_cast<float*>(tooltip+0x98)=180.f;*reinterpret_cast<float*>(tooltip+0x9c)=75.f;
    // Actual game fixture: the generic tooltip has no HWND while DFO is foreground.
    *reinterpret_cast<void**>(tooltip+0x190)=nullptr;
    native_symbol_window::followCursor(tooltip,parent);assert(positionedX==216.f&&positionedY==216.f);
    fakeForeground=GetDesktopWindow();assert(!native_symbol_window::gameWindow(tooltip));
    *reinterpret_cast<HWND*>(tooltip+0x190)=testWindow;assert(native_symbol_window::gameWindow(tooltip)==testWindow);
    *reinterpret_cast<void**>(tooltip+0x190)=nullptr;fakeForeground=testWindow;
    assert(*reinterpret_cast<float*>(tooltip+0x98)==180.f&&*reinterpret_cast<float*>(tooltip+0x9c)==75.f);
    uintptr_t styledTable[96]{};uintptr_t* styled=styledTable;
    addresses[0x14b1d61f0]=reinterpret_cast<uintptr_t>(styledTable);addresses[0x146f19350]=reinterpret_cast<uintptr_t>(replaceStyle);
    native_symbol_window::styledText(&styled,L"88.00%");native_symbol_window::styledText(&styled,L"77.00%");assert(replacedStyle==L"77.00%");
    char resultItem[0x6b8]{};uintptr_t resultTable[96]{};*reinterpret_cast<uintptr_t**>(resultItem)=resultTable;
    addresses[0x14a6cf678]=reinterpret_cast<uintptr_t>(resultTable);addresses[0x14501c260]=reinterpret_cast<uintptr_t>(namePos);addresses[0x14501c2d0]=reinterpret_cast<uintptr_t>(nameVisible);addresses[0x14501c4e0]=reinterpret_cast<uintptr_t>(stackVisible);addresses[0x14501c490]=reinterpret_cast<uintptr_t>(stackAmount);
    native_symbol_window::itemResult(resultItem,37);assert(*reinterpret_cast<uint32_t*>(resultItem+0x57c)==37&&resultItem[0x57a]&&resultItem[0x3b0]);
    native_symbol_window::itemResult(resultItem,5);assert(*reinterpret_cast<uint32_t*>(resultItem+0x57c)==5);
    std::wstring nativeName=badge;memcpy(resultItem+0x380,&nativeName,sizeof nativeName);assert(native_symbol_window::itemName(resultItem)==badge);
    *reinterpret_cast<size_t*>(resultItem+0x390)=4097;assert(native_symbol_window::itemName(resultItem).empty());
    *reinterpret_cast<size_t*>(resultItem+0x390)=nativeName.size();native_symbol_window::hideItemName(resultItem);assert(!resultItem[0x3b0]);
    symbol_growth_state::State filter;filter.panel=symbol_growth_state::Panel::Upgrade;
    assert(filter.eligible(v,290)&&!filter.eligible(v,291));filter.special=true;assert(!filter.eligible(v,290));
    o.operation=5;v.growth.options.push_back(o);assert(filter.eligible(v,290));filter.replacement=true;assert(!filter.eligible(v,290));
    filter.panel=symbol_growth_state::Panel::Disjoint;assert(!filter.eligible(v,290));o.operation=3;v.growth.options.push_back(o);assert(filter.eligible(v,290));
    itemTable[7]=reinterpret_cast<uintptr_t>(draw);native_inventory::active=window;native_inventory::borrowedAvatar=avatar;native_inventory::callbacks.draggedSlot=dragging;native_inventory::draggedRow=7;native_inventory::draggedRowAt=0;
    native_inventory::drawDraggedItem();assert(draws==1);
    native_inventory::draggedRow=8;native_inventory::drawDraggedItem();assert(draws==1);
    native_inventory::borrowedAvatar=nullptr;native_inventory::drawDraggedItem();assert(draws==1);
    DestroyWindow(testWindow);
    puts("PASS presentation: fast/slow replies, batch result visibility, quoted material, one replay per cycle, stale-cell ghost, position preserving size, style replacement, actual result quantity/name layout, mode eligibility, null-HWND owned-window fallback and foreign-process rejection, full native names, lossless Chinese wrap and identity/destruction guards");
}
