#pragma once
#include <stdint.h>
#include <string.h>
struct AuthorityLease {
 char magic[8]; uint32_t version, serverPID; uint64_t issued;
 uint32_t epoch, active; uint16_t owner, count, bots[8], room[2]; uint64_t footer;
};
static_assert(sizeof(AuthorityLease)==64,"IPC layout");
inline bool ValidLease(const AuthorityLease& l,uint64_t now) {
 if(memcmp(l.magic,"TRAUTH16",8)||l.version!=1||!l.serverPID||l.active!=1||
    !l.epoch||!l.owner||!l.count||l.count>3||l.bots[0]||
    l.footer!=0x3130763631524154ull||l.issued>now||now-l.issued>2500) return false;
 for(unsigned i=1;i<8;i++) {
  if(i>l.count) {if(l.bots[i])return false;continue;}
  if(!l.bots[i]||l.bots[i]==l.owner)return false;
  for(unsigned j=1;j<i;j++)if(l.bots[j]==l.bots[i])return false;
 }
 return true;
}
inline bool AllowTransfer(const AuthorityLease& l,uint64_t now,uint32_t scene,int localSeat,
                          uint32_t objectID,int nativeSeat,bool sameMap,const uint32_t (&actors)[8],uint32_t effectOwnerID=0) {
 unsigned type=objectID>>16;
 if(!ValidLease(l,now)||scene!=3||localSeat!=0||!sameMap||(type!=3&&type!=4&&type!=17)||
    nativeSeat<=0||nativeSeat>3||!l.bots[nativeSeat]||actors[0]!=(0x50000u|l.owner))return false;
 if(type==17&&effectOwnerID!=actors[nativeSeat])return false;
 for(unsigned i=1;i<8;i++) {
  uint32_t expected=l.bots[i]?(0x50000u|l.bots[i]):0;
  if(actors[i]!=expected)return false;
 }
 return true;
}
