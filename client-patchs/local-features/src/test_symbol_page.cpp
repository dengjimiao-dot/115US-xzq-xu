#include "symbol_page_state.h"
#include "native_symbol_widgets.h"
#include "native_symbol_window.h"
#include "symbol_protocol.h"
#include "symbol_growth_state.h"
#include <cassert>
#include <cstdio>
using namespace symbol_page;
int main() {
    // Same layout as Go TestSymbolViewBoundsAndFrame, including the sparse slot.
    std::vector<uint8_t> bytes;
    features::put(bytes,2,8);features::put(bytes,0,8);bytes.insert(bytes.end(),64,'a');
    features::put(bytes,1,2);features::put(bytes,1,2);features::put(bytes,10,2);
    const std::string red="red_symbol";bytes.insert(bytes.end(),red.begin(),red.end());
    features::put(bytes,1,2);features::put(bytes,2,2);features::put(bytes,590004573,4);features::put(bytes,0,4);
    features::put(bytes,1,4);features::put(bytes,0,8);features::put(bytes,0,4);features::put(bytes,289,2);features::put(bytes,1,2);features::put(bytes,590004579,4);features::put(bytes,9,4);features::put(bytes,0,4);
    assert(bytes.size()==140);
    auto snapshot=symbol_wire::decode(bytes);
    assert(snapshot.role==2&&snapshot.groups[0].items[0].id==590004573&&snapshot.bag[0].slot==289);
    auto withAttributes=bytes;features::put(withAttributes,0x31544153,4);
    for(const std::string s:{std::string(u8"力量 +60"),std::string(u8"无属性加成"),std::string(u8"力量 +60")}){features::put(withAttributes,s.size(),2);withAttributes.insert(withAttributes.end(),s.begin(),s.end());}
    auto detailed=symbol_wire::decode(withAttributes);assert(detailed.attributes==u8"力量 +60"&&detailed.groups[0].attributes==detailed.attributes);
    for(size_t n=bytes.size()+1;n<withAttributes.size();++n){bool rejected=false;try{symbol_wire::decode({withAttributes.begin(),withAttributes.begin()+n});}catch(...){rejected=true;}assert(rejected);}
    auto withStar=withAttributes;features::put(withStar,0x31525353,4);features::put(withStar,1,2);features::put(withStar,1986000001,4);features::put(withStar,1986000002,4);
    features::put(withStar,1,1);features::put(withStar,0,4);features::put(withStar,1,4);features::put(withStar,12,4);
    for(int i=0;i<12;++i){std::string s=u8"增益量 +10";features::put(withStar,s.size(),2);withStar.insert(withStar.end(),s.begin(),s.end());}
    auto stars=symbol_wire::decode(withStar);assert(stars.powers.size()==1&&stars.groups[0].star.present&&stars.groups[0].star.level==1&&stars.groups[0].star.fame==12);
    auto withGrowth=withStar;features::put(withGrowth,0x31524753,4);
    for(const std::string s:{std::string(u8"五曜星光：20"),std::string(u8"共鸣成功"),std::string(u8"红：稀有2"),std::string(u8"红 黄 蓝")}){features::put(withGrowth,s.size(),2);withGrowth.insert(withGrowth.end(),s.begin(),s.end());}
    features::put(withGrowth,1,2);const size_t optionStart=withGrowth.size();features::put(withGrowth,4,2);features::put(withGrowth,289,2);features::put(withGrowth,590004579,4);features::put(withGrowth,0,4);features::put(withGrowth,1,4);
    std::string label=u8"普通突破，消耗15个五曜星光";features::put(withGrowth,label.size(),2);withGrowth.insert(withGrowth.end(),label.begin(),label.end());
    auto grown=symbol_wire::decode(withGrowth);assert(grown.growth.present&&grown.growth.options.size()==1&&grown.growth.options[0].label==label);auto request=symbol_wire::grow(grown,grown.growth.options[0]);assert(request.size()==96&&request[80]==4&&request[82]==0x21&&request[83]==1);
    auto invalid=withGrowth;invalid[optionStart]=9;bool invalidOperation=false;try{symbol_wire::decode(invalid);}catch(...){invalidOperation=true;}assert(invalidOperation);
    auto ui=withGrowth;features::put(ui,0x31554753,4);features::put(ui,40001,4);features::put(ui,40002,4);features::put(ui,19,8);features::put(ui,8,8);
    features::put(ui,10,2);for(int i=0;i<10;++i)features::put(ui,i%5,1);features::put(ui,1,2);
    features::put(ui,40001,4);features::put(ui,45,4);features::put(ui,71,4);features::put(ui,1200,4);features::put(ui,10000,4);features::put(ui,2,2);
    features::put(ui,3,4);features::put(ui,8800,4);features::put(ui,7,4);features::put(ui,1200,4);
    auto nativeView=symbol_wire::decode(ui);assert(nativeView.growth.uiPresent&&nativeView.growth.balance==19&&nativeView.growth.colors[4]==4&&nativeView.growth.options[0].ui.points==71&&nativeView.growth.options[0].ui.results[1].first==7);
    assert(symbol_wire::grow(nativeView,nativeView.growth.options[0])==request);
    auto visual=ui;features::put(visual,0x31564753,4);features::put(visual,4,2);features::put(visual,4,2);features::put(visual,590004579,4);features::put(visual,1,4);
    auto resultView=symbol_wire::decode(visual);assert(resultView.growth.visual.operation==4&&resultView.growth.visual.outcome==4&&resultView.growth.visual.item==590004579&&symbol_wire::grow(resultView,resultView.growth.options[0])==request);
    for(size_t n=ui.size()+1;n<visual.size();++n){bool rejected=false;try{symbol_wire::decode({visual.begin(),visual.begin()+n});}catch(...){rejected=true;}assert(rejected);}
    auto badVisual=visual;badVisual[ui.size()+6]=5;bool visualRejected=false;try{symbol_wire::decode(badVisual);}catch(...){visualRejected=true;}assert(visualRejected);
    for(size_t n=withGrowth.size()+1;n<ui.size();++n){bool rejected=false;try{symbol_wire::decode({ui.begin(),ui.begin()+n});}catch(...){rejected=true;}assert(rejected);}
    for(auto offset:{withGrowth.size()+30,withGrowth.size()+40,withGrowth.size()+50}){auto bad=ui;bad[offset]=255;bool rejected=false;try{symbol_wire::decode(bad);}catch(...){rejected=true;}assert(rejected);}
    auto missingMaterial=ui;for(size_t i=0;i<4;++i)missingMaterial[withGrowth.size()+4+i]=0;bool missing=false;try{symbol_wire::decode(missingMaterial);}catch(...){missing=true;}assert(missing);
    symbol_growth_state::State selection;selection.panel=symbol_growth_state::Panel::Upgrade;
    selection.accept(nativeView);assert(selection.select(nativeView,289)&&selection.upgrade(nativeView));assert(!selection.select(nativeView,17));
    auto changed=nativeView;changed.growth.options.clear();selection.accept(changed);assert(selection.selected==UINT16_MAX);
    for(uint16_t i=0;i<30;++i){symbol_wire::GrowthOption option;option.operation=3;option.slot=i;changed.growth.options.push_back(option);}
    selection.panel=symbol_growth_state::Panel::Disjoint;selection.all(changed);assert(selection.disjoint.size()==24&&!selection.select(changed,29));assert(selection.select(changed,4)&&selection.disjoint.size()==23);assert(selection.select(changed,29)&&selection.disjoint.size()==24);
    changed.role++;selection.accept(changed);assert(selection.disjoint.empty()&&selection.panel==symbol_growth_state::Panel::None);
    for(size_t n=withStar.size()+1;n<withGrowth.size();++n){bool rejected=false;try{symbol_wire::decode({withGrowth.begin(),withGrowth.begin()+n});}catch(...){rejected=true;}assert(rejected);}
    for(size_t n=withAttributes.size()+1;n<withStar.size();++n){bool rejected=false;try{symbol_wire::decode({withStar.begin(),withStar.begin()+n});}catch(...){rejected=true;}assert(rejected);}
    auto exchange=symbol_wire::exchange(snapshot,red,0,&snapshot.bag[0]);
    assert(exchange.size()==104&&exchange[0]==2&&exchange[80]==10&&exchange[98]==0x21&&exchange[99]==1);
    for(size_t n=0;n<bytes.size();++n){bool rejected=false;try{symbol_wire::decode({bytes.begin(),bytes.begin()+n});}catch(...){rejected=true;}assert(rejected);}
    bytes.push_back(0);bool trailing=false;try{symbol_wire::decode(bytes);}catch(...){trailing=true;}assert(trailing);
    const Identity role{2,10,std::string(64,'a')};
    State page;
    page.enter(role);auto generation=page.generation();
    Ticket first;
    assert(!page.openGroup("red_symbol",4) && !page.begin(first));
    assert(page.snapshot(role,generation,0));
    assert(page.openGroup("red_symbol",4));
    assert(!page.selectSocket(4) && page.selectSocket(3));
    assert(page.begin(first));
    Ticket duplicate;
    assert(!page.begin(duplicate));
    assert(!page.snapshot(role,generation,1));
    assert(!page.accept(first,2));
    assert(page.accept(first,1) && !page.accept(first,1));
    assert(!page.snapshot(role,generation,0));
    assert(page.begin(first));
    assert(page.fail(first) && !page.ready() && !page.begin(duplicate));
    // Lost reply with a server commit: refresh restores the newer sequence.
    assert(page.snapshot(role,generation,2) && page.ready());
    assert(page.begin(first));
    page.closePanel();
    assert(!page.panelOpen() && page.accept(first,3));
    assert(page.ready() && !page.begin(duplicate));
    assert(page.openGroup("multi_symbol",6) && page.selectSocket(5));
    assert(page.begin(first));
    page.close();
    assert(!page.accept(first,4) && !page.snapshot(role,generation,4));
    // Same role after reconnect must not accept the previous connection's view.
    Identity reconnected=role;reconnected.connection++;
    page.enter(reconnected);auto next=page.generation();
    assert(!page.snapshot(role,next,4));
    assert(!page.snapshot(reconnected,generation,4));
    assert(page.snapshot(reconnected,next,3));
    assert(!page.openGroup("red_symbol",0));
    assert(page.openGroup("red_symbol",1) && !page.selectSocket(1));
    assert(!page.accept(first,4));
    // No DFO image is loaded in this mechanism test. Native API stays disabled.
    native_symbol_widgets::API widgets;
    assert(!widgets.bind(0) && !widgets.setTemplate(nullptr,590004573));
    // Rebased synthetic PE pages validate ASLR guards without calling game code.
    auto memory=static_cast<unsigned char*>(VirtualAlloc(nullptr,0xc000000,MEM_RESERVE,PAGE_NOACCESS));
    assert(memory);
    auto base=reinterpret_cast<uintptr_t>(memory);
    auto at=[&](uintptr_t va){return memory+(va-0x140000000ull);};
    auto commit=[&](uintptr_t va){assert(VirtualAlloc(at(va&~uintptr_t(0xfff)),0x1000,MEM_COMMIT,PAGE_READWRITE));};
    for(const auto& guard:native_symbol_widget_binding::guards) {
        commit(guard.va);memcpy(at(guard.va),guard.bytes,guard.size);
    }
    commit(native_symbol_widget_binding::itemVTable);
    uintptr_t methods[4];
    for(unsigned i=0;i<4;++i)methods[i]=base+native_symbol_widget_binding::itemMethods[i]-0x140000000ull;
    memcpy(at(native_symbol_widget_binding::itemVTable),methods,sizeof methods);
    assert(widgets.bind(base));
    auto site=at(native_symbol_widget_binding::guards[0].va);
    *site^=1;assert(!widgets.bind(base));*site^=1;
    assert(!widgets.setTemplate(nullptr,590004573));
    assert(widgets.bind(base));
    methods[0]^=1;memcpy(at(native_symbol_widget_binding::itemVTable),methods,sizeof methods);
    assert(!widgets.bind(base));
    methods[0]^=1;memcpy(at(native_symbol_widget_binding::itemVTable),methods,sizeof methods);
    for(const auto& guard:native_symbol_window_binding::guards){
        commit(guard.va);commit(guard.va+guard.size-1);memcpy(at(guard.va),guard.bytes,guard.size);
    }
    for(const auto& guard:symbol_window_repair_binding::guards){
        commit(guard.va);commit(guard.va+guard.size-1);memcpy(at(guard.va),guard.bytes,guard.size);
    }
    for(const auto& guard:symbol_native_ui_abi::guards){commit(guard.va);if(guard.size==8){uintptr_t expected=0;memcpy(&expected,guard.bytes,8);expected=base+expected-0x140000000ull;memcpy(at(guard.va),&expected,8);}else memcpy(at(guard.va),guard.bytes,guard.size);}
    for(const auto& guard:symbol_growth_presentation_binding::guards){commit(guard.va);commit(guard.va+guard.size-1);memcpy(at(guard.va),guard.bytes,guard.size);}
    for(const auto& guard:symbol_growth_presentation_binding::guards){auto p=at(guard.va);*p^=1;assert(!native_symbol_window::install(base,nullptr,{}));*p^=1;}
    auto uiGuard=at(symbol_native_ui_abi::guards[0].va);*uiGuard^=1;assert(!native_symbol_window::install(base,nullptr,{}));*uiGuard^=1;
    auto windowTable=native_symbol_window_binding::tableVA;
    commit(windowTable-8);
    uintptr_t windowMethods[121]{};
    for(unsigned i=0;i<120;++i)windowMethods[i+1]=base+native_symbol_window_binding::methods[i]-0x140000000ull;
    memcpy(at(windowTable-8),windowMethods,sizeof windowMethods);
    auto branch=at(native_symbol_window_binding::guards[1].va);
    *branch^=1;assert(!native_symbol_window::install(base,nullptr,{}));*branch^=1;
    for(const auto& guard:symbol_window_repair_binding::guards){
        auto p=at(guard.va);*p^=1;assert(!native_symbol_window::install(base,nullptr,{}));*p^=1;
    }
    assert(!native_symbol_window::open("red_symbol"));
    assert(native_symbol_window::install(base,nullptr,{}));
    assert(!native_symbol_window::open("unknown"));
    assert(RestoreHook(native_symbol_window::factoryHook));
    auto& entry=native_symbol_window_binding::guards[0];
    assert(!memcmp(at(entry.va),entry.bytes,entry.size));
    assert(!native_symbol_window::render("red_symbol",{}));
    assert(VirtualFree(native_symbol_window::factoryHook.page,0,MEM_RELEASE));
    assert(VirtualFree(memory,0,MEM_RELEASE));
    puts("symbol page lifecycle and native adapter fail-closed checks passed");
}
