#include "../../elite-companion-pilot/src/inline_hook.h"
#include "authority_policy.h"
#include <initializer_list>
#include "native_evidence.h"
#include <assert.h>
#include <stdio.h>
#include <stddef.h>
static uintptr_t Replacement(){return 99;}
int main() {
 assert(offsetof(AuthorityLease,issued)==16&&offsetof(AuthorityLease,bots)==36&&offsetof(AuthorityLease,footer)==56);
 AuthorityLease lease={};memcpy(lease.magic,"TRAUTH16",8);
 lease.version=1;lease.serverPID=42;lease.issued=10000;lease.epoch=3;lease.active=1;lease.owner=2;lease.count=1;
 lease.bots[1]=65534;lease.footer=0x3130763631524154ull;
 uint32_t actors[8]={0x50002,0x5fffe};
 assert(AllowTransfer(lease,11000,3,0,0x40001,1,true,actors));
 assert(AllowTransfer(lease,11000,3,0,0x31001,1,true,actors));
 for(unsigned type=0;type<40;type++)assert(AllowTransfer(lease,11000,3,0,type<<16|1,1,true,actors)==(type==3||type==4));
 for(int scene=0;scene<20;scene++)assert(AllowTransfer(lease,11000,scene,0,0x40001,1,true,actors)==(scene==3));
 assert(!AllowTransfer(lease,11000,3,1,0x40001,1,true,actors));
 assert(!AllowTransfer(lease,11000,3,0,0x40001,0,true,actors));
 assert(!AllowTransfer(lease,11000,3,0,0x40001,-1,true,actors));
 assert(!AllowTransfer(lease,11000,3,0,0x40001,2,true,actors));
 assert(!AllowTransfer(lease,11000,3,0,0x40001,1,false,actors));
 assert(!ValidLease(lease,9999)&&!ValidLease(lease,12501)&&ValidLease(lease,12500));
 actors[1]=0x50003;assert(!AllowTransfer(lease,11000,3,0,0x40001,1,true,actors));actors[1]=0x5fffe;
 actors[2]=0x50003;assert(!AllowTransfer(lease,11000,3,0,0x40001,1,true,actors));actors[2]=0;
 auto copy=lease;copy.active=0;assert(!ValidLease(copy,11000));copy=lease;copy.bots[1]=2;assert(!ValidLease(copy,11000));
 copy=lease;copy.count=2;copy.bots[2]=65534;assert(!ValidLease(copy,11000));
 // Type17 is only delegated when its verified native parent is this fake seat.
 assert(!AllowTransfer(lease,11000,3,0,0x110001,1,true,actors));
 assert(AllowTransfer(lease,11000,3,0,0x110001,1,true,actors,0x5fffe));
 for(auto parent : {0u,0x50002u,0x50003u,0x40001u,0x5ffffu})
  assert(!AllowTransfer(lease,11000,3,0,0x110001,1,true,actors,parent));
 assert(!AllowTransfer(lease,12501,3,0,0x110001,1,true,actors,0x5fffe));
 assert(!AllowTransfer(lease,11000,1,0,0x110001,1,true,actors,0x5fffe));
 assert(!AllowTransfer(lease,11000,3,0,0x110001,1,false,actors,0x5fffe));
 // Execute the exact pinned prologue's pushes/stack effects, relay and restore.
 unsigned char body[]={0xb8,32,0,0,0,0x48,0x83,0xc4,0x40,0x41,0x5f,0x41,0x5e,0x41,0x5d,0x41,0x5c,0x5f,0xc3};
 auto p=(unsigned char*)VirtualAlloc(nullptr,4096,MEM_COMMIT|MEM_RESERVE,PAGE_READWRITE);assert(p);
 memcpy(p,kAuthorityHead,sizeof(kAuthorityHead));memcpy(p+sizeof(kAuthorityHead),body,sizeof(body));
 DWORD old;assert(VirtualProtect(p,4096,PAGE_EXECUTE_READ,&old));auto fn=(uintptr_t(*)())p;assert(fn()==32);
 InlineHook hook;assert(PrepareHook(hook,p,kAuthorityHead,sizeof(kAuthorityHead),(void*)&Replacement));
 assert(((uintptr_t(*)())hook.original())()==32);assert(PublishHook(hook));assert(fn()==99);
 assert(RestoreHook(hook));assert(fn()==32&&!memcmp(p,kAuthorityHead,sizeof(kAuthorityHead)));
 unsigned char bad[16];memcpy(bad,kAuthorityHead,16);bad[0]^=1;InlineHook reject;assert(!PrepareHook(reject,p,bad,16,(void*)&Replacement));
 puts("PASS identity/TTL/scene/entity/map boundaries; exact prologue trampoline, restore and mismatch rejection");
}
