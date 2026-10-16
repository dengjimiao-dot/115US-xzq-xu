#pragma once
#include "native_symbol_info.h"
#include "symbol_growth_ui_binding.h"
#include "symbol_growth_state.h"
#include "symbol_growth_presentation.h"
#include <sstream>
#include <iomanip>
#include <unordered_map>
namespace native_symbol_growth {
using namespace native_symbol_window;
using symbol_growth_state::Panel;
static symbol_growth_state::State state;
static bool mode=false;
static uint64_t lastGeneration=0;
static std::string error;
static symbol_wire::GrowthOption pending;
static symbol_wire::Visual lastResult;
static symbol_growth_presentation::Timeline timeline;
static std::unordered_map<void*,std::pair<bool,uint64_t>> animationStates;
static std::unordered_map<void*,uint32_t> animationSelections;
static uint8_t selectedColor=0;
static std::vector<uint8_t> submittedLights;
static std::vector<void*> animationControls;
static std::unordered_map<void*,uint32_t> itemTemplates;
static symbol_wire::GrowthOption confirmedChoice;
static int shopRange=-1;
static uint32_t useTemplate=0;
static uint16_t useSlot=0;
// Keep the last authoritative normal quote visible when its slot is taken out.
static uint32_t normalTemplate=0,normalPoints=0;
static bool rightUpgradeWasDown=false;
static void completed(const symbol_wire::View&v){
    if(!mode||v.growth.visual.operation!=pending.operation)return;
    lastResult=v.growth.visual;timeline.completed(pending.operation,GetTickCount64(),symbol_growth_presentation_binding::resultMS(pending.operation));
}
static bool resultActive(uint16_t op){return lastResult.operation==op&&timeline.result(op,GetTickCount64());}
static void bindColorAnimation(void* control,int color,int kind){
    if(!control||color<0||color>4)return;
    if(std::find(animationControls.begin(),animationControls.end(),control)==animationControls.end()){
        for(const auto&m:symbol_growth_ui_binding::colors)animationPath(control,kind==0?m.progress:kind==1?m.fail:m.upgrade);
        animationControls.push_back(control);
    }
    const auto index=1+uint32_t(color);
    auto found=animationSelections.find(control);
    if(found==animationSelections.end()||found->second!=index){animationIndex(control,index);animationSelections[control]=index;}
}
using Submit=void(*)(const symbol_wire::GrowthOption&);
static void reset(){state.reset();mode=false;error.clear();lastGeneration=0;pending={};lastResult={};timeline={};animationStates.clear();animationSelections.clear();submittedLights.clear();animationControls.clear();itemTemplates.clear();confirmedChoice={};shopRange=-1;useTemplate=0;useSlot=0;normalTemplate=normalPoints=0;rightUpgradeWasDown=false;}
static void open(Panel panel=Panel::Resonance){
    reset();state.panel=panel;mode=true;
    openInfo(panel==Panel::Resonance?InfoLayout::Resonance:panel==Panel::Upgrade?InfoLayout::Upgrade:panel==Panel::Disjoint?InfoLayout::Disjoint:InfoLayout::Shop);
}
static bool selecting(){return infoPopup&&mode&&(state.panel==Panel::Upgrade||state.panel==Panel::Disjoint);}
static int dropTarget(){
    if(!selecting())return -1;
    if(state.panel==Panel::Disjoint){Control panel(infoPopup,L"disjoint_panel");return pointed(panel.value)?0:-1;}
    Control bg(infoPopup,L"bg"),radio(bg.value,L"radiobutton",true),button(radio.value,state.special?L"type_button_1":L"type_button_0",true),group(button.value,state.special?L"group_1":L"group_0",true),slot(group.value,state.special?L"slot_0":L"slot",true);
    return pointed(slot.value)?0:-1;
}
static bool select(const symbol_wire::View&v,uint16_t slot){
    if(!selecting()||state.batch||timeline.occupied(GetTickCount64()))return false;
    const bool accepted=state.select(v,slot);
    if(accepted)error.clear();
    else {if(state.panel==Panel::Upgrade)state.selected=UINT16_MAX;error=state.panel==Panel::Upgrade?"该徽章不在当前突破规则中，请使用可突破的徽章":"该徽章不能分解";}
    if(native_symbol_window::log){fprintf(native_symbol_window::log,"symbol growth drop r10 panel=%d slot=%u accepted=%u\n",int(state.panel),slot,unsigned(accepted));fflush(native_symbol_window::log);}
    return accepted;
}
static std::wstring percent(uint32_t rate,uint32_t unit){if(!unit)return L"—";std::wostringstream out;out<<std::fixed<<std::setprecision(2)<<(100.0*rate/unit)<<L"%";return out.str();}
static void put(void* parent,const wchar_t*name,const std::wstring&value,bool child=true){Control c(parent,name,child);text(c.value,value);}
static void animate(void* control,bool value){
    if(!control)return;
    if(read<uintptr_t>(control)==address(0x14b13a9a8)){
        auto& prior=animationStates[control];
        if(value&&(!prior.first||prior.second!=timeline.cycle))animationRestart(control);
        prior={value,timeline.cycle};
    }
    visible(control,value);
}
static void show(void* parent,const wchar_t*name,bool value,bool child=true){Control c(parent,name,child);animate(c.value,value);}
static void setItem(void*control,uint32_t id){if(!control)return;auto it=itemTemplates.find(control);if(it!=itemTemplates.end()&&it->second==id)return;if(widgets.setTemplate(control,id))itemTemplates[control]=id;}
static void item(void*parent,const wchar_t*name,uint32_t id,bool child=true){Control c(parent,name,child);setItem(c.value,id);}
static void resultName(void*parent,void*control,uint32_t amount){
    hideItemName(control);const auto name=itemName(control);
    const auto rows=symbol_text_layout::wrap(name.empty()?L"":name+L" × "+std::to_wstring(amount),34);
    for(size_t i=0;i<3;++i)put(parent,(L"result_name_"+std::to_wstring(i)).c_str(),i<rows.size()?rows[i]:L"");
}
static uint32_t bagTemplate(const symbol_wire::View&v,uint16_t slot){for(const auto&i:v.bag)if(i.slot==slot)return i.id;return 0;}
static void send(const symbol_wire::View&v,const symbol_wire::GrowthOption*o,Submit submit){
    if(!o||!submit||timeline.occupied(GetTickCount64()))return;
    if(symbol_growth_presentation::lacksQuotedMaterial(v,*o)){error="材料不足";timeline.cancel();return;}
    if(o->operation==5){uint64_t amount=0;for(const auto& i:v.bag)if(i.id==o->id)amount+=i.amount;if(amount<2){error="需要两个相同的徽章";return;}}
    error.clear();pending=*o;lastResult={};timeline.start(o->operation,GetTickCount64(),symbol_growth_presentation_binding::progressMS(o->operation));submit(*o);
}
static void animations(void*parent,std::initializer_list<const wchar_t*>names){for(auto name:names)show(parent,name,false);}
static void resonance(const symbol_wire::View&v,bool ready,Submit submit){
    const auto&g=v.growth;Control material(infoPopup,L"Material"),tip(infoPopup,L"tooltip");
    item(material.value,L"item_slot",g.material);put(material.value,L"stack_num",std::to_wstring(g.balance));
    put(infoPopup,L"top_txt",native_symbol_info::wide(error.empty()?g.result:error),false);
    const symbol_wire::GrowthOption*hover=nullptr;
    for(size_t i=0;i<4;++i){
        Control container(infoPopup,(L"symbol_"+std::to_wstring(i)).c_str()),node(container.value,L"node",true),grade(node.value,L"grade",true),level(node.value,L"level",true);
        const auto*group=v.find(std::string(native_symbol_window::groups[i],native_symbol_window::groups[i]+wcslen(native_symbol_window::groups[i])));
        imageIndex(node.value,symbol_growth_ui_binding::colors[i].symbol);
        if(group&&group->star.present){imageIndex(grade.value,6+int(group->star.grade));imageIndex(level.value,636+int(group->star.level));}
        Control up(node.value,L"up_ani",true);bindColorAnimation(up.value,int(i),2);animate(up.value,resultActive(2)&&lastResult.outcome>1&&(selectedColor==4||selectedColor==i));
    }
    for(size_t i=0;i<10;++i){
        Control container(infoPopup,(L"node_"+std::to_wstring(i)).c_str()),node(container.value,L"node",true),normal(node.value,L"normal",true),dodge(node.value,L"dodge",true),over(node.value,L"over",true);
        const auto&lights=(!ready&&pending.operation==2)||resultActive(2)?submittedLights:g.colors;
        const bool present=i<lights.size();visible(normal.value,present);visible(dodge.value,present);
        show(node.value,L"on_ani",present&&resultActive(1));show(node.value,L"off_ani",present&&resultActive(2));

        const symbol_wire::GrowthOption*choice=nullptr;
        if(present){auto color=lights[i];const auto&frames=symbol_growth_ui_binding::colors[color];imageIndex(normal.value,frames.normal);imageIndex(dodge.value,frames.dodge);
            for(const auto&o:g.options)if(o.operation==2&&o.argument<g.colors.size()&&g.colors[o.argument]==color){choice=&o;break;}
        }
        if(present){
            Control progress(node.value,L"progress_ani",true),fail(node.value,L"fail_ani",true);
            bindColorAnimation(progress.value,lights[i],0);bindColorAnimation(fail.value,lights[i],1);
            animate(progress.value,timeline.progress(2,GetTickCount64())&&(selectedColor==4||selectedColor==lights[i]));
            animate(fail.value,resultActive(2)&&lastResult.outcome==1&&(selectedColor==4||selectedColor==lights[i]));
            show(node.value,L"success_ani",resultActive(2)&&lastResult.outcome>1&&(selectedColor==4||selectedColor==lights[i]));
        }
        const bool hot=ready&&!resultActive(2)&&choice&&(hovered(node.value)||hovered(normal.value)||hovered(dodge.value));visible(over.value,hot);
        if(hot)hover=choice;
        if(ready&&!resultActive(2)&&choice&&(clicked(node.value)||clicked(normal.value)||clicked(dodge.value))){submittedLights=g.colors;selectedColor=g.colors[i];send(v,choice,submit);return;}
    }
    visible(tip.value,hover!=nullptr);
    if(hover){followCursor(tip.value,read<void*>(infoPopup,0xd0));const auto&meta=symbol_growth_ui_binding::colors[g.colors[hover->argument]];put(tip.value,L"name_txt",meta.name);put(tip.value,L"item_count_txt",std::to_wstring(hover->ui.amount));Control rate(tip.value,L"success_rate_txt",true);styledText(rate.value,percent(hover->ui.rate,hover->ui.unit));put(tip.value,L"description_txt",meta.description);}
    Control start(infoPopup,L"start_btn");visible(start.value,g.colors.empty());
    if(ready&&clicked(start.value))for(const auto&o:g.options)if(o.operation==1){send(v,&o,submit);return;}
    Control result(infoPopup,L"result");const bool shown=resultActive(2);visible(result.value,shown);
    Control resultText(infoPopup,L"top_txt");position(resultText.value,shown?17.f:18.f,shown?160.f:17.f);
    show(result.value,L"success",shown&&lastResult.outcome==2);show(result.value,L"success_big",shown&&lastResult.outcome==3);show(result.value,L"fail",shown&&lastResult.outcome==1);
    show(infoPopup,L"finish_pannel",false,false);show(infoPopup,L"finish_pannel_1",false,false);
}
static void refreshNormalProgress(const symbol_wire::View&v,const symbol_wire::GrowthOption*choice){
    if(!state.special&&choice){normalTemplate=choice->id;normalPoints=choice->ui.points;}
    if(normalTemplate)for(const auto&o:v.growth.options)if(o.operation==4&&o.id==normalTemplate){normalPoints=o.ui.points;break;}
}
static void upgrade(const symbol_wire::View&v,bool ready,Submit submit){
    Control bg(infoPopup,L"bg"),radio(bg.value,L"radiobutton",true),normal(radio.value,L"type_button_0",true),special(radio.value,L"type_button_1",true);
    if(ready&&clicked(normal.value)){state.special=false;state.selected=UINT16_MAX;state.replacement=false;}
    if(ready&&clicked(special.value)){state.special=true;state.selected=UINT16_MAX;state.replacement=false;}
    Control group0(normal.value,L"group_0",true),group1(special.value,L"group_1",true);visible(group0.value,!state.special);visible(group1.value,state.special);
    Control material(infoPopup,L"material"),replace(material.value,L"replace_material",true);visible(replace.value,state.special);
    if(ready&&state.special&&clicked(replace.value))state.replacement=!state.replacement;
    const bool rightDown=(GetAsyncKeyState(VK_RBUTTON)&0x8000)!=0;
    const bool takeOut=rightDown&&!rightUpgradeWasDown;rightUpgradeWasDown=rightDown;
    Control normalSlot(group0.value,L"slot",true),specialSlot(group1.value,L"slot_0",true);
    if(ready&&takeOut&&pointed(state.special?specialSlot.value:normalSlot.value)){state.selected=UINT16_MAX;error.clear();}
    auto*choice=state.upgrade(v);uint32_t id=choice?choice->id:0;
    refreshNormalProgress(v,choice);
    Control slot(group0.value,L"slot",true),target(group1.value,L"slot_0",true),duplicate(group1.value,L"slot_1",true);
    uint64_t available=0;for(const auto&i:v.bag)if(i.id==id)available+=i.amount;
    item(slot.value,L"upgrade_item_slot",!state.special?id:0);item(target.value,L"target_item_slot",state.special?id:0);item(duplicate.value,L"material_item_slot",state.special&&available>=2?id:0);
    show(group0.value,L"progress_ani",timeline.progress(4,GetTickCount64()));show(group1.value,L"progress_ani",timeline.progress(5,GetTickCount64()));
    show(group0.value,L"gauge_reset_ani",resultActive(4)&&lastResult.outcome==2);
    for(auto name:{L"behind_gauge",L"front_gauge"}){Control c(group0.value,name,true);gauge(c.value,choice?choice->ui.points:normalPoints);}
    item(material.value,L"material_item_slot",choice?choice->ui.material:v.growth.material);
    put(material.value,L"material_count",choice?std::to_wstring(choice->ui.amount):L"—");
    Control probability(infoPopup,L"txt");
    const uint32_t sourceGrades[]={2,3,7,8};
    for(size_t i=0;i<4;++i){std::wstring value=choice?L"0.00%":L"—";if(choice)for(const auto&result:choice->ui.results)if(result.first==sourceGrades[i])value=percent(result.second,choice->ui.unit);put(probability.value,(L"text_probability_"+std::to_wstring(i)).c_str(),value);}
    const std::wstring notice=!error.empty()?native_symbol_info::wide(error):(choice?(L"突破进度 "+std::to_wstring(choice->ui.points)+L"%") : normalTemplate?(L"保留突破进度 "+std::to_wstring(normalPoints)+L"%，请放入徽章"):L"请放入需要突破的徽章");
    put(group0.value,L"guide_text",notice);
    // Special mode hides group_0, so its guide cannot carry transaction errors.
    put(target.value,L"txt",!error.empty()?native_symbol_info::wide(error):L"请放入要进行突破的徽章");
    Control result(infoPopup,L"result");const bool shown=resultActive(4)||resultActive(5);visible(result.value,shown);show(infoPopup,L"idle_ani",!shown&&ready,false);
    Control resultSlot(result.value,L"slot_normal",true);item(resultSlot.value,L"result_item_slot",shown?lastResult.item:0);
    show(resultSlot.value,L"text_0",shown&&lastResult.outcome==2);show(resultSlot.value,L"text_1",shown&&lastResult.outcome!=2);
    show(result.value,L"ani_0",shown&&lastResult.outcome==2);show(result.value,L"ani_1",shown&&lastResult.outcome!=2);
    Control button(infoPopup,L"upgrade_btn");if(ready&&!shown&&choice&&clicked(button.value))send(v,choice,submit);
}
static void disjoint(const symbol_wire::View&v,bool ready,Submit submit){
    Control panel(infoPopup,L"disjoint_panel"),all(infoPopup,L"select_all_btn"),clear(infoPopup,L"clear_all_btn"),confirm(infoPopup,L"disjoint_btn");
    if(ready&&!state.batch&&clicked(all.value))state.all(v);
    if(ready&&!state.batch&&clicked(clear.value))state.disjoint.clear();
    for(size_t i=0;i<24;++i){Control slot(panel.value,(L"disjoint_item_"+std::to_wstring(i)).c_str(),true);setItem(slot.value,i<state.disjoint.size()?bagTemplate(v,state.disjoint[i]):0);if(ready&&!state.batch&&i<state.disjoint.size()&&clicked(slot.value)){state.disjoint.erase(state.disjoint.begin()+i);break;}}
    put(infoPopup,L"itemCount",error.empty()?std::to_wstring(state.disjoint.size()):native_symbol_info::wide(error),false);
    const unsigned stage=symbol_growth_presentation::disjointStage(timeline,GetTickCount64(),state.batch);const bool shown=symbol_growth_presentation::disjointResultTextVisible(timeline,GetTickCount64(),1880);Control resultPanel(infoPopup,L"item_result_panel");visible(resultPanel.value,shown);
    Control reward(resultPanel.value,L"item_slot_1",true);item(reward.value,L"item_1",shown?lastResult.item:0);
    Control rewardItem(reward.value,L"item_1",true);itemResult(rewardItem.value,shown?lastResult.amount:0);resultName(reward.value,rewardItem.value,shown?lastResult.amount:0);
    Control consumed(resultPanel.value,L"item_slot_0",true);item(consumed.value,L"item_0",shown?pending.id:0);
    Control consumedItem(consumed.value,L"item_0",true);itemResult(consumedItem.value,shown?1:0);resultName(consumed.value,consumedItem.value,shown?1:0);
    show(infoPopup,L"state_ani_0",stage==0,false);show(infoPopup,L"state_ani_1",stage==1,false);show(infoPopup,L"state_ani_2",stage==2&&!shown,false);
    if(ready&&!state.batch&&!state.disjoint.empty()&&clicked(confirm.value))state.batch=true;
    if(ready&&state.batch&&!timeline.progress(3,GetTickCount64())){if(state.disjoint.empty()){state.batch=false;return;}auto slot=state.disjoint.front();auto*choice=state.split(v,slot);state.disjoint.erase(state.disjoint.begin());if(!choice){state.batch=false;return;}send(v,choice,submit);}
}
static const symbol_wire::GrowthOption* currentChoice(const symbol_wire::View&v){
    for(const auto&o:v.growth.options)if(o.operation==confirmedChoice.operation&&o.slot==confirmedChoice.slot&&o.id==confirmedChoice.id&&o.argument==confirmedChoice.argument&&o.amount==confirmedChoice.amount)return &o;
    return nullptr;
}
static void shop(const symbol_wire::View&v,bool ready,Submit submit){
    std::vector<const symbol_wire::GrowthOption*> choices;
    for(const auto&o:v.growth.options)if(state.panel==Panel::Ticket?(o.operation==6&&o.argument==1&&(!useTemplate||(o.id==useTemplate&&o.slot==useSlot))):state.panel==Panel::Change?(o.operation==7&&(!useTemplate||o.argument==useTemplate)):((o.operation==6&&o.argument==0)||o.operation==8))choices.push_back(&o);
    for(auto name:{L"20",L"21",L"22",L"23",L"BtnMultiSell",L"BtnGrowthGuide",L"checkbox_storage_use",L"helpUi",L"haveitem"})show(infoPopup,name,false,false);
    Control scroll(infoPopup,L"1001");const int maximum=choices.size()>10?int(choices.size()-10):0;
    if(shopRange!=maximum){scrollRange(scroll.value,maximum);shopRange=maximum;}
    const size_t offset=size_t((std::max)(0,(std::min)(maximum,scrollPosition(scroll.value))));
    for(size_t i=0;i<10;++i){
        const int sourceID=3000+int(i)*10;Control row(infoPopup,std::to_wstring(sourceID).c_str());
        const bool present=offset+i<choices.size();visible(row.value,present||(!i&&choices.empty()));if(!present){if(!i&&choices.empty()){item(row.value,L"34100",useTemplate);put(row.value,L"3002",!ready?L"正在读取角色状态…":error.empty()?L"当前没有符合条件的操作":native_symbol_info::wide(error));}continue;}
        const auto&o=*choices[offset+i];
        item(row.value,(L"3410"+std::to_wstring(i)).c_str(),o.id);
        auto caption=native_symbol_info::wide(o.label);auto divider=caption.find(L"　");auto bracket=caption.find(L"（");
        auto end=(std::min)(divider,bracket);auto name=caption.substr(0,end);
        for(auto prefix:{L"兑换并使用",L"兑换",L"使用",L"变更"})if(name.find(prefix)==0){name.erase(0,wcslen(prefix));break;}
        put(row.value,std::to_wstring(sourceID+2).c_str(),name);
        for(auto name:{L"BtnPreview",L"PvpGradeText",L"BtnLimitDate",L"BtnPurchaseLimit",L"FreeItem"})show(row.value,name,false);
        for(int baseID:{34000,34200,40000,40100})show(row.value,std::to_wstring(baseID+int(i)).c_str(),false);
        show(row.value,std::to_wstring(30000+int(i)*10).c_str(),false);
        for(int j=3;j<=9;++j)show(row.value,std::to_wstring(sourceID+j).c_str(),false);
        Control highlight(row.value,L"highlightSlot",true);visible(highlight.value,hovered(row.value));
        if(ready&&clicked(row.value)){confirmedChoice=o;error.clear();openInfo(o.operation==7?InfoLayout::Change:InfoLayout::Confirm);return;}
    }
}
static void confirm(const symbol_wire::View&v,bool ready,Submit submit){
    const auto*choice=currentChoice(v);Control board(infoPopup,L"Board"),line(board.value,L"ninepatch_board",true),left(line.value,L"itemSlot1",true),right(line.value,L"itemSlot2",true);
    item(left.value,L"item_source",confirmedChoice.ui.material);item(right.value,L"item_target",confirmedChoice.id);
    auto description=native_symbol_info::wide(confirmedChoice.label);auto divider=description.find(L"　");
    put(board.value,L"txt_noti",L"请核对本次操作");put(board.value,L"txt_trade_0",description.substr(0,divider));
    show(board.value,L"txt_trade_1",divider!=std::wstring::npos);if(divider!=std::wstring::npos)put(board.value,L"txt_trade_1",description.substr(divider+1));
    put(board.value,L"txt_noti_decrement",native_symbol_info::wide(error.empty()?v.growth.result:error));show(board.value,L"txt_charac_count",false);
    Control cancel(infoPopup,L"btn_cancel"),button(infoPopup,L"btn_confirm");
    if(ready&&clicked(cancel.value)){openInfo(InfoLayout::Shop);return;}
    if(ready&&choice&&clicked(button.value)){send(v,choice,submit);return;}
}
static void change(const symbol_wire::View&v,bool ready,Submit submit){
    const auto*choice=currentChoice(v);const bool shown=resultActive(7);Control material(infoPopup,L"MaterialInfo"),result(infoPopup,L"base image");
    item(infoPopup,L"Target",shown?0:confirmedChoice.id,false);item(material.value,L"Material",confirmedChoice.argument);
    put(material.value,L"MaterialName",native_symbol_info::wide(confirmedChoice.label));put(infoPopup,L"TargetName",native_symbol_info::wide(error.empty()?confirmedChoice.label:error),false);
    visible(result.value,shown);item(result.value,L"Result",shown?lastResult.item:0);put(result.value,L"ResultName",native_symbol_info::wide(v.growth.result));
    show(infoPopup,L"base image2",false,false);show(infoPopup,L"Confirm2",shown,false);show(infoPopup,L"confirm button",!shown,false);
    Control cancel(infoPopup,L"cancel button"),done(infoPopup,L"Confirm2"),button(infoPopup,L"confirm button");
    if(ready&&(clicked(cancel.value)||clicked(done.value))){openInfo(InfoLayout::Shop);return;}
    if(ready&&!shown&&choice&&clicked(button.value)){send(v,choice,submit);return;}
}
static void renderOverlay(){
    if(infoPopup&&infoLayout==InfoLayout::Resonance&&resultActive(2)){
        Control resultText(infoPopup,L"top_txt");
        if(resultText.value)reinterpret_cast<void(__fastcall*)(void*)>(read<uintptr_t*>(resultText.value)[8])(resultText.value);
    }
}
static void render(const symbol_wire::View&v,bool ready,Submit submit){
    if(!infoPopup||reopenInfo||infoLayout==InfoLayout::Summary)return;
    if(infoGeneration!=lastGeneration){lastGeneration=infoGeneration;state.role=v.role;state.sequence=v.sequence;animationControls.clear();animationStates.clear();animationSelections.clear();itemTemplates.clear();shopRange=-1;}
    if(!v.growth.uiPresent){if(mode&&infoLayout==InfoLayout::Shop)shop(v,false,submit);return;}
    if(!error.empty()){state.batch=false;timeline.cancel();}
    ready=ready&&!timeline.occupied(GetTickCount64());
    if(ready)state.accept(v);
    Control help(infoPopup,L"guide_btn"),tip(help.value,L"guide_tooltip",true);visible(tip.value,hovered(help.value));
    if(infoLayout==InfoLayout::Resonance)resonance(v,ready,submit);
    if(infoLayout==InfoLayout::Upgrade)upgrade(v,ready,submit);
    if(infoLayout==InfoLayout::Disjoint)disjoint(v,ready,submit);
    if(infoLayout==InfoLayout::Shop)shop(v,ready,submit);
    if(infoLayout==InfoLayout::Confirm)confirm(v,ready,submit);
    if(infoLayout==InfoLayout::Change)change(v,ready,submit);
}
}
