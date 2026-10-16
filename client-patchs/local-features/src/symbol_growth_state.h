#pragma once
#include "symbol_protocol.h"
#include <algorithm>
namespace symbol_growth_state {
enum class Panel {None,Resonance,Upgrade,Disjoint,Shop,Ticket,Change};
struct State {
    Panel panel=Panel::None;bool special=false,replacement=false,batch=false;
    uint64_t role=0,sequence=UINT64_MAX;
    uint16_t selected=UINT16_MAX;
    std::vector<uint16_t> disjoint;
    void reset(){*this=State{};}
    const symbol_wire::GrowthOption* upgrade(const symbol_wire::View&v)const {
        for(const auto&o:v.growth.options)if(o.slot==selected&&o.operation==(special?5:4)&&(!special||o.argument==uint32_t(replacement)))return &o;
        return nullptr;
    }
    const symbol_wire::GrowthOption* split(const symbol_wire::View&v,uint16_t slot)const {
        for(const auto&o:v.growth.options)if(o.operation==3&&o.slot==slot)return &o;
        return nullptr;
    }
    bool eligible(const symbol_wire::View&v,uint16_t slot)const {
        if(panel==Panel::Disjoint)return split(v,slot)!=nullptr;
        if(panel==Panel::Upgrade)for(const auto&o:v.growth.options)
            if(o.slot==slot&&o.operation==(special?5:4)&&(!special||o.argument==uint32_t(replacement)))return true;
        return panel!=Panel::Upgrade;
    }
    bool select(const symbol_wire::View&v,uint16_t slot){
        if(panel==Panel::Upgrade){for(const auto&o:v.growth.options)if(o.slot==slot&&o.operation==(special?5:4)){selected=slot;return true;}return false;}
        if(panel==Panel::Disjoint&&split(v,slot)){
            auto it=std::find(disjoint.begin(),disjoint.end(),slot);
            if(it!=disjoint.end())disjoint.erase(it);
            else if(disjoint.size()<24)disjoint.push_back(slot);
            else return false;
            return true;
        }
        return false;
    }
    void accept(const symbol_wire::View&v){
        if(role&&role!=v.role){reset();return;}
        role=v.role;sequence=v.sequence;
        if(selected!=UINT16_MAX&&!upgrade(v))selected=UINT16_MAX;
        disjoint.erase(std::remove_if(disjoint.begin(),disjoint.end(),[&](uint16_t slot){return !split(v,slot);}),disjoint.end());
    }
    void all(const symbol_wire::View&v){disjoint.clear();for(const auto&o:v.growth.options)if(o.operation==3&&disjoint.size()<24)disjoint.push_back(o.slot);}
};
}
