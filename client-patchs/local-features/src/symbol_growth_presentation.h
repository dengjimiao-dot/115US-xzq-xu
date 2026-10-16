#pragma once
#include <cstdint>
#include "symbol_protocol.h"
namespace symbol_growth_presentation {
// Presentation only; the server remains responsible for every transaction.
struct Timeline {
    uint16_t operation=0;
    uint64_t cycle=0,progressUntil=0,resultFrom=0,resultUntil=0;
    void start(uint16_t op,uint64_t now,uint32_t duration){operation=op;++cycle;progressUntil=now+duration;resultFrom=resultUntil=0;}
    void completed(uint16_t op,uint64_t now,uint32_t duration){if(op!=operation)return;resultFrom=now>progressUntil?now:progressUntil;resultUntil=resultFrom+duration;}
    bool progress(uint16_t op,uint64_t now)const{return operation==op&&now<progressUntil;}
    bool result(uint16_t op,uint64_t now)const{return operation==op&&resultFrom&&now>=resultFrom&&now<resultUntil;}
    bool occupied(uint64_t now)const{return now<progressUntil||(resultUntil&&now<resultUntil);}
    void cancel(){progressUntil=resultFrom=resultUntil=0;}
};
// Batch submission must not suppress a completed native result animation.
inline unsigned disjointStage(const Timeline& t,uint64_t now,bool batch){
    return t.progress(3,now)?1:t.result(3,now)?2:batch?3:0;
}
inline bool disjointResultTextVisible(const Timeline& t,uint64_t now,uint32_t animationMS){
    return t.result(3,now)&&now>=t.resultFrom+animationMS;
}
inline bool lacksQuotedMaterial(const symbol_wire::View& v,const symbol_wire::GrowthOption& o){
    if(!o.ui.material||!o.ui.amount)return false;
    if(o.ui.material==v.growth.material)return v.growth.balance<o.ui.amount;
    if(o.ui.material==v.growth.failMaterial)return v.growth.failBalance<o.ui.amount;
    // Other material balances are not in this display snapshot. Defer to the server.
    return false;
}
}
