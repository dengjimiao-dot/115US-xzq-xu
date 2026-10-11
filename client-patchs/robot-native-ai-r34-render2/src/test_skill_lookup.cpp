#define ROBOT_NATIVE_TEST
#include "plugin.cpp"
#include <assert.h>
static constexpr uintptr_t actor=0x123400,module=0x567800;
static int cacheCalls=0,originalCalls=0;
uintptr_t TestNativeCall(uintptr_t rva,const uintptr_t* args,size_t n) {
    assert(rva==0x5ce53a0&&n==3&&args[0]==actor&&args[2]==0);
    cacheCalls++;
    if(args[1]==7)return 0x700000;
    if(args[1]==113)return 0x113000;
    return 0;
}
static uintptr_t __fastcall OriginalLookup(uintptr_t m,int id,uintptr_t a3,uintptr_t a4) {
    assert(m==module||m==0x999999);
    assert(id==7&&a3==33&&a4==44);originalCalls++;return 0x777;
}
int main() {
    gEnabled=1;gGameThread=GetCurrentThreadId();gSkillLookup=OriginalLookup;
    auto& a=gOffer;memcpy(a.party.magic,"TRAUTH16",8);a.party.version=1;a.party.serverPID=123;
    a.party.issued=Now();a.party.epoch=2;a.party.active=1;a.party.owner=2;a.party.count=1;
    a.party.bots[1]=65534;a.party.footer=0x3130763631524154ull;a.generation=17;a.phase=2;
    gRooms[0]=a;auto& r=gRobots[0];r.active=true;r.actor=actor;r.skills=module;
    r.skillIDs[0]=113;
    // Raw ID 7 is learned but absent from all fourteen selected hotbar slots.
    assert(SkillLookup(module,7,33,44)==0x700000);
    assert(SkillLookup(module,113,33,44)==0x113000);
    assert(SkillLookup(module,511,33,44)==0);
    assert(SkillLookup(module,-1,33,44)==0&&SkillLookup(module,512,33,44)==0);
    assert(cacheCalls==3&&originalCalls==0&&r.skillIDs[0]==113);
    assert(SkillLookup(0x999999,7,33,44)==0x777&&originalCalls==1);
    r.active=false;assert(SkillLookup(module,7,33,44)==0x777);r.active=true;
    gOffer.phase=3;assert(SkillLookup(module,7,33,44)==0x777);
    gOffer.phase=2;gOffer.party.issued=0;assert(SkillLookup(module,7,33,44)==0x777);
    assert(originalCalls==4&&cacheCalls==3);
    puts("PASS learned raw ID outside hotbar; bound/missing/range; no slot mutation; foreign/inactive/halt/expired pass-through");
}
