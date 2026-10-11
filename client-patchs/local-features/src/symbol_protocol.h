#pragma once
#include "protocol.h"
#include <set>

namespace symbol_wire {
struct Item {uint32_t id=0,expiry=0,flags=0;};
struct Star {bool present=false;uint32_t grade=0,level=0,fame=0;std::string fixed[2],resonance[2],perEmblem[4][2];};
struct Group {std::string name;uint16_t mask=0;std::vector<Item> items;std::string attributes;Star star;};
struct BagItem {uint16_t slot=0,mask=0;uint32_t id=0,amount=0,expiry=0;};
struct UIQuote {uint32_t material=0,amount=0,points=0,rate=0,unit=0;std::vector<std::pair<uint32_t,uint32_t>> results;};
struct GrowthOption {UIQuote ui;uint16_t operation=0,slot=0;uint32_t id=0,argument=0,amount=0;std::string label;};
struct Visual {uint16_t operation=0,outcome=0;uint32_t item=0,amount=0;};
struct Growth {Visual visual;bool present=false,uiPresent=false;uint32_t material=0,failMaterial=0;uint64_t balance=0,failBalance=0;std::vector<uint8_t> colors;std::string materials,result,progress,lights;std::vector<GrowthOption> options;};
struct View {
    uint64_t role=0,sequence=0;std::string source;
    std::vector<Group> groups;std::vector<BagItem> bag;
    std::string attributes,buffAttributes;
    std::vector<std::pair<uint32_t,uint32_t>> powers;
    Growth growth;
    const Group* find(const std::string& name)const {for(auto&g:groups)if(g.name==name)return &g;return nullptr;}
};
inline bool groupName(const std::string&s){if(s.empty()||s.size()>64)return false;for(char c:s)if(c!='_'&&(c<'a'||c>'z'))return false;return true;}
inline View decode(const std::vector<uint8_t>&p){
    features::Reader r{p};View v;v.role=r.num(8);v.sequence=r.num(8);
    for(int i=0;i<64;++i)v.source.push_back(char(r.num(1)));
    auto groups=r.num(2),rows=r.num(2);
    if(!v.role||!features::hex(v.source,64)||groups==0||groups>16||rows>1024)throw std::runtime_error("invalid symbol view");
    std::set<std::string> seen;
    for(size_t i=0;i<groups;++i){
        Group g;g.name=r.text();g.mask=uint16_t(r.num(2));auto n=r.num(2);
        if(!groupName(g.name)||!seen.insert(g.name).second||!g.mask||!n||n>256)throw std::runtime_error("invalid symbol group");
        for(size_t j=0;j<n;++j){Item x{uint32_t(r.num(4)),uint32_t(r.num(4)),uint32_t(r.num(4))};if(x.id==1||(x.flags&~0xffff01u)||(!x.id&&(x.expiry||x.flags)))throw std::runtime_error("invalid socket item");g.items.push_back(x);}
        v.groups.push_back(std::move(g));
    }
    std::set<uint16_t> slots;
    for(size_t i=0;i<rows;++i){BagItem x{uint16_t(r.num(2)),uint16_t(r.num(2)),uint32_t(r.num(4)),uint32_t(r.num(4)),uint32_t(r.num(4))};
        if(!slots.insert(x.slot).second||!x.mask||x.id<2||!x.amount)throw std::runtime_error("invalid emblem inventory");v.bag.push_back(x);}
    if(r.at<p.size()){
        if(r.num(4)!=0x31544153)throw std::runtime_error("invalid attribute trailer");
        v.attributes=r.text();v.buffAttributes=r.text();for(auto&g:v.groups)g.attributes=r.text();
    }
    if(r.at<p.size()){
        if(r.num(4)!=0x31525353)throw std::runtime_error("invalid star trailer");
        auto n=r.num(2);if(n>64)throw std::runtime_error("too many star options");
        for(size_t i=0;i<n;++i){auto a=uint32_t(r.num(4)),b=uint32_t(r.num(4));if(a<2||b<2)throw std::runtime_error("invalid star option");v.powers.push_back({a,b});}
        for(auto&g:v.groups){auto present=r.num(1);if(present>1)throw std::runtime_error("invalid star state");g.star.present=present!=0;if(!present)continue;
            auto&s=g.star;s.grade=uint32_t(r.num(4));s.level=uint32_t(r.num(4));s.fame=uint32_t(r.num(4));
            if(s.grade>3||!s.level||s.level>5)throw std::runtime_error("invalid star progress");
            for(auto&x:s.fixed)x=r.text();for(auto&x:s.resonance)x=r.text();for(auto&pair:s.perEmblem)for(auto&x:pair)x=r.text();
        }
    }
    if(r.at<p.size()){
        if(r.num(4)!=0x31524753)throw std::runtime_error("invalid growth trailer");
        auto&g=v.growth;g.present=true;g.materials=r.text();g.result=r.text();g.progress=r.text();g.lights=r.text();auto n=r.num(2);if(n>1024)throw std::runtime_error("growth option bounds");
        for(size_t i=0;i<n;++i){GrowthOption o;o.operation=uint16_t(r.num(2));o.slot=uint16_t(r.num(2));o.id=uint32_t(r.num(4));o.argument=uint32_t(r.num(4));o.amount=uint32_t(r.num(4));o.label=r.text();if(o.operation<1||o.operation>8)throw std::runtime_error("growth option kind");g.options.push_back(std::move(o));}
    }
    if(r.at+4<=p.size()&&p[r.at]==0x53&&p[r.at+1]==0x47&&p[r.at+2]==0x55&&p[r.at+3]==0x31){
        auto&g=v.growth;if(!g.present||r.num(4)!=0x31554753)throw std::runtime_error("invalid growth UI trailer");
        g.uiPresent=true;g.material=uint32_t(r.num(4));g.failMaterial=uint32_t(r.num(4));g.balance=r.num(8);g.failBalance=r.num(8);
        auto lights=r.num(2);if(g.material<2||g.failMaterial<2||(lights!=0&&lights!=10))throw std::runtime_error("growth UI identity or lights");
        for(size_t i=0;i<lights;++i){auto color=r.num(1);if(color>4)throw std::runtime_error("growth UI light color");g.colors.push_back(uint8_t(color));}
        if(r.num(2)!=g.options.size())throw std::runtime_error("growth UI quote count");
        for(auto&o:g.options){auto&q=o.ui;q.material=uint32_t(r.num(4));q.amount=uint32_t(r.num(4));q.points=uint32_t(r.num(4));q.rate=uint32_t(r.num(4));q.unit=uint32_t(r.num(4));auto n=r.num(2);
            if(q.points>100||n>8||(q.rate&&(!q.unit||q.rate>q.unit)))throw std::runtime_error("growth UI quote bounds");
            for(size_t i=0;i<n;++i){auto grade=uint32_t(r.num(4)),weight=uint32_t(r.num(4));q.results.push_back({grade,weight});}
        }
    }
    if(r.at<p.size()){
        if(!v.growth.present||r.num(4)!=0x31564753)throw std::runtime_error("invalid visual trailer");
        auto&a=v.growth.visual;a.operation=uint16_t(r.num(2));a.outcome=uint16_t(r.num(2));a.item=uint32_t(r.num(4));a.amount=uint32_t(r.num(4));
        if(a.operation<1||a.operation>8||a.outcome<1||a.outcome>4||a.item==1)throw std::runtime_error("visual bounds");
    }
    r.end();return v;
}
inline std::vector<uint8_t> refresh(const View&v){
    if(!v.role||!features::hex(v.source,64))throw std::runtime_error("invalid symbol refresh");
    std::vector<uint8_t> p;features::put(p,v.role,8);features::put(p,v.sequence,8);p.insert(p.end(),v.source.begin(),v.source.end());return p;
}
inline std::vector<uint8_t> grow(const View&v,const GrowthOption&o){
    if(!v.growth.present||o.operation<1||o.operation>8||v.sequence==UINT64_MAX)throw std::runtime_error("growth operation unavailable");auto p=refresh(v);features::put(p,o.operation,2);features::put(p,o.slot,2);features::put(p,o.id,4);features::put(p,o.argument,4);features::put(p,o.amount,4);return p;
}
inline std::vector<uint8_t> exchange(const View&v,const std::string&name,uint16_t socket,const BagItem*incoming){
    auto g=v.find(name);
    if(!v.role||!features::hex(v.source,64)||!g||socket>=g->items.size()||v.sequence==UINT64_MAX||
       (incoming&&(!(incoming->mask&g->mask)||!incoming->amount||incoming->id<2)))throw std::runtime_error("invalid symbol exchange");
    std::vector<uint8_t> p;features::put(p,v.role,8);features::put(p,v.sequence,8);p.insert(p.end(),v.source.begin(),v.source.end());
    features::put(p,name.size(),2);p.insert(p.end(),name.begin(),name.end());features::put(p,socket,2);features::put(p,g->items[socket].id,4);
    features::put(p,incoming?incoming->slot:0,2);features::put(p,incoming?incoming->id:0,4);return p;
}
}
