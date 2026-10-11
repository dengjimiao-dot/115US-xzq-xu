#include "ai_policy.h"
#include <assert.h>
#include <stdio.h>
int main() {
    AiLease a={};memcpy(a.party.magic,"TRAUTH16",8);a.party.version=1;a.party.serverPID=123;
    a.party.issued=1000;a.party.epoch=2;a.party.active=1;a.party.owner=2;a.party.count=1;
    a.party.bots[1]=65534;a.party.footer=0x3130763631524154ull;a.generation=17;a.phase=1;
    uint32_t actors[8]={0x50002,0x5fffe};
    assert(AllowNativeRobot(a,1001,3,0,1,true,actors));
    assert(!AllowNativeRobot(a,3501,3,0,1,true,actors));
    assert(!AllowNativeRobot(a,999,3,0,1,true,actors));
    assert(!AllowNativeRobot(a,1001,1,0,1,true,actors));
    assert(!AllowNativeRobot(a,1001,3,1,1,true,actors));
    assert(!AllowNativeRobot(a,1001,3,0,0,true,actors));
    assert(!AllowNativeRobot(a,1001,3,0,1,false,actors));
    auto old=a;a.generation++;assert(!SameAiRoom(a,old));a=old;
    a.party.epoch++;assert(!SameAiRoom(a,old));a=old;
    a.party.room[0]++;assert(!SameAiRoom(a,old));a=old;
    actors[2]=0x50007;assert(!AllowNativeRobot(a,1001,3,0,1,true,actors));actors[2]=0;
    a.phase=3;assert(!AllowNativeRobot(a,1001,3,0,1,true,actors));a=old;
    a.clientPID=44;assert(!ValidAiOffer(a,1001));a=old;
    a.party.bots[1]=2;assert(!ValidAiOffer(a,1001));
    puts("PASS native AI scope: expiry, room/identity, local owner, extra human, halt and stale generation");
}
