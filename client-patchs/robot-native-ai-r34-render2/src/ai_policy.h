#pragma once
#include "authority_lease.h"
// Separate local IPC, never transmitted as a game packet. A new room receives
// a new random generation, even when actor IDs or the room coordinates repeat.
struct AiLease {
    AuthorityLease party;
    uint64_t generation;
    uint32_t phase, clientPID; // offer 1=prepare, 2=drive, 3=halt
};
static_assert(sizeof(AiLease)==80,"AI IPC layout");
inline bool ValidAiOffer(const AiLease& a,uint64_t now) {
    return ValidLease(a.party,now)&&a.generation&&a.clientPID==0&&a.phase>=1&&a.phase<=3;
}
inline bool SameAiRoom(const AiLease& a,const AiLease& b) {
    return a.generation==b.generation&&a.party.serverPID==b.party.serverPID&&
      a.party.epoch==b.party.epoch&&a.party.owner==b.party.owner&&a.party.count==b.party.count&&
      !memcmp(a.party.bots,b.party.bots,sizeof(a.party.bots))&&
      !memcmp(a.party.room,b.party.room,sizeof(a.party.room));
}
inline bool AllowNativeRobot(const AiLease& l,uint64_t now,uint32_t scene,unsigned localSeat,
                            unsigned slot,bool sameMap,const uint32_t (&actors)[8]) {
    if(!ValidAiOffer(l,now)||l.phase==3||scene!=3||localSeat!=0||!sameMap||
       slot<1||slot>l.party.count||actors[0]!=(0x50000u|l.party.owner))return false;
    for(unsigned i=1;i<8;i++) {
        const uint32_t want=l.party.bots[i]?(0x50000u|l.party.bots[i]):0;
        if(actors[i]!=want)return false;
    }
    return true;
}

// r22 ack extension. The worker renews lease.issued, never sampledAt.
struct AiAck {
 AiLease lease;
 uint64_t sampledAt;
 uint32_t deaths[3], deadMask;
};
static_assert(sizeof(AiAck)==104,"AI life acknowledgement layout");
