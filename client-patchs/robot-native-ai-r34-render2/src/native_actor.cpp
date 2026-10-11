#include "native_actor.h"
#include "actor_classes.h"
#include <math.h>
#include <algorithm>
#include <vector>

namespace {
uintptr_t& Q(uintptr_t p,size_t off=0) {return *(uintptr_t*)(p+off);}
template<class... A> uintptr_t Call(uintptr_t b,uintptr_t rva,A... args) {
    return ((uintptr_t(__fastcall*)(A...))(b+rva))(args...);
}
template<class... A> uintptr_t Virtual(uintptr_t p,size_t off,A... args) {
    return ((uintptr_t(__fastcall*)(uintptr_t,A...))Q(Q(p),off))(p,args...);
}
void Delete(uintptr_t p) {if(p)Virtual(p,0,uintptr_t(1));}
uintptr_t Alloc(uintptr_t b,size_t n) {return Call(b,0x6e8ba20,n);}
// Native loader returns a std::shared_ptr. The native copy constructor retains
// all required profile references; this temporary loader reference is released.
void ReleaseResource(uintptr_t (&p)[2]) {
    uintptr_t c=p[1];p[0]=p[1]=0;
    if(c&&InterlockedDecrement((volatile LONG*)(c+8))==0) {
        Virtual(c,0);
        if(InterlockedDecrement((volatile LONG*)(c+12))==0)Virtual(c,8);
    }
}
bool IsVector(uintptr_t start,uintptr_t end,size_t stride,size_t max) {
    return end>=start&&(end-start)%stride==0&&(end-start)/stride<=max&&
           (start==end||Readable((void*)start,end-start));
}
// Same job/variant lookup as 142e5d922 and 142e62438. Copy immutable
// configuration instead of registering the ordinary actor in Elite slots.
bool ActionRange(uintptr_t b,uintptr_t a,size_t offset,float (&range)[4]) {
    __try {
        uintptr_t manager=Q(b,0xe638ef8);
        if(!manager||!Readable((void*)(manager+offset),16))return false;
        uint32_t key[2]={(uint32_t)Virtual(a,0x12e0),(uint32_t)Virtual(a,0x12f0)};
        uintptr_t it=0;
        Call(b,0x879f20,manager+offset,(uintptr_t)&it,(uintptr_t)key);
        if(!it||it==Q(manager,offset)||!Readable((void*)(it+0x24),16))return false;
        memcpy(range,(void*)(it+0x24),16);
        for(float value:range)if(!isfinite(value)||value<0)return false;
        return true;
    } __except(EXCEPTION_EXECUTE_HANDLER) {return false;}
}
}

bool AttachNativeRobot(uintptr_t b,NativeRobot& r,uintptr_t a,uintptr_t owner) {
    // Refuse unknown classes/builds or pre-existing owned modules. Never delete a
    // getter's shared fallback, and never use the Elite clone/stat initializer.
    if(r.actor||!Readable((void*)a,8))return false;
    int job=RobotActorClassIndex(b,Q(a));
    if(job<0||!Readable((void*)a,kRobotActorClasses[job].size)||
       Virtual(a,0x12e0)!=(uintptr_t)job||
       Q(a,0x11ba0)||Q(a,0x11ba8)||Q(a,0x18a18)||Q(a,0x62b8)||
       !Q(a,0x18e18)||!owner)return false;
    // A party record must exist before taking over AI. No borrowed local-player
    // record and no replacement of profile+1530 (which the profile would own).
    uintptr_t character=Call(b,0x5ce9040,a);
    if(!character||!Readable((void*)character,0x72))return false;
    if(!ActionRange(b,a,0xb8,r.retreatRange)||!ActionRange(b,a,0xc8,r.attackRange))return false;
    r.actor=a;r.owner=owner;r.oldAI=Q(a,0x18e18);
    r.oldTeam=(unsigned char)Virtual(a,0x988);
    memcpy(r.timer,(void*)(a+0x18da0),sizeof(r.timer));
    auto fail=[&](){DetachNativeRobot(b,r);return false;};
    r.input=Alloc(b,0xf0);if(!r.input)return fail();
    Call(b,0x6d7eae0,r.input);
    Call(b,0x5922690,r.input,uintptr_t(1));
    Call(b,0x5ceaa10,a,r.input,uintptr_t(1));
    Call(b,0x5c768f0,a,r.input);
    r.skills=Alloc(b,0x190);if(!r.skills)return fail();
    Call(b,0x5ef18a0,r.skills,uintptr_t(512));
    Call(b,0x5c77190,a,r.skills);
    Call(b,0x5eeb220,r.skills,a);
    Call(b,0x2577bb0,r.skills,r.input);
    r.profile=Alloc(b,0x1548);if(!r.profile)return fail();
    Call(b,0x7807d30,r.profile);
    Q(r.profile)=b+0xa951310;
    memset((void*)(r.profile+0x14f0),0,24);
    *(uint32_t*)(r.profile+0x1540)=0;
    Call(b,0x5c68860,r.profile);
    Q(a,0x18a18)=r.profile;
    uintptr_t path=Call(b,0x70909b0,b+0xf3757b0,uintptr_t(99999));
    if(!path||!Q(b,0xe683b90))return fail();
    Call(b,0x1f47e0,Q(b,0xe683b90),(uintptr_t)r.resource,path);
    if(!r.resource[0])return fail();
    Call(b,0x5c648e0,r.profile,r.resource[0]);
    // This candidate supports the proven generic native AI implementation only.
    if(*(unsigned char*)(r.profile+0x1450)!=0)return fail();
    // The inspected template's optional name-to-action map is empty. A changed
    // resource requiring that mapping must be reviewed rather than guessed.
    if(Q(r.profile,0xcf8)!=0)return fail();
    r.ai=Alloc(b,0xa40);if(!r.ai)return fail();
    Call(b,0x3d72330,r.ai);
    Q(a,0x18e18)=r.ai;
    Call(b,0x5e7b340,r.ai,a);
    Call(b,0x5e7b0e0,r.ai,r.profile+0xcd8);
    Call(b,0x5e7a300,r.ai,r.profile+0x808);
    Call(b,0x5e6fad0,r.ai);
    uintptr_t start=Q(r.profile,0xcc0),end=Q(r.profile,0xcc8);
    if(!IsVector(start,end,32,128)||start==end)return fail();
    for(size_t i=0;i<(end-start)/32;i++)Call(b,0x5c65f80,a,i);
    uintptr_t stream=Alloc(b,0xa0);if(!stream)return fail();
    Call(b,0x5e5d890,stream,a);Q(r.profile,0x1508)=stream;
    // Native Elite classification (145c7abe0 / 142e64220), applied to the
    // ordinary actor's already learned objects. No clone or level mutation.
    struct Learned {int id,order,awakening;uintptr_t skill;bool used;};
    std::vector<Learned> learned;
    for(unsigned id=0;id<512;id++) {
        if(!Q(a,0x7d20+id*24+16))continue;
        uintptr_t skill=Call(b,0x5ce53a0,a,uintptr_t(id),uintptr_t(0));
        if(!skill)continue;
        unsigned level=(unsigned)Call(b,0x5ed23a0,skill,uintptr_t(0));
        uintptr_t metadata=Call(b,0xe54e40,skill);
        if(!metadata||!Readable((void*)metadata,0x1314))return fail();
        int kind=-1;Call(b,0x6e920a0,metadata+0x110,(uintptr_t)&kind);
        if(!level||kind!=0)continue;
        learned.push_back({(int)id,(int)Call(b,0x5ed78c0,skill,uintptr_t(1)),
                           *(int*)(metadata+0x1310),skill,false});
    }
    std::stable_sort(learned.begin(),learned.end(),[](auto& x,auto& y){return x.order>y.order;});
    auto bind=[&](unsigned slot,Learned& v) {
        if(!(Call(b,0x5ef94c0,r.skills,uintptr_t(slot),v.skill)&255))return false;
        Call(b,0x5ef9f40,r.skills);
        if(Call(b,0x5ef4300,r.skills,uintptr_t(slot))!=v.skill)return false;
        r.skillIDs[slot]=v.id;v.used=true;return true;
    };
    for(auto& v:learned)if(v.awakening==1||v.awakening==2) {
        unsigned slot=11+v.awakening;
        if(r.skillIDs[slot]<0&&!bind(slot,v))return fail();
        v.used=true;
    }
    uintptr_t manager=Q(b,0xe638ef8);
    uintptr_t priority=Call(b,0x2e606c0,manager,a);
    if(!priority||!Readable((void*)priority,24))return fail();
    uintptr_t first=Q(priority),last=Q(priority,8);
    if(!IsVector(first,last,8,512))return fail();
    unsigned attackSlot=0,buffSlot=9;
    for(uintptr_t entry=first;entry<last;entry+=8) {
        int id=*(int*)entry;bool buff=*(unsigned char*)(entry+4)==1;
        for(auto& v:learned)if(v.id==id&&!v.used) {
            if(buff) {if(buffSlot<12&&!bind(buffSlot++,v))return fail();}
            else {if(attackSlot<9&&!bind(attackSlot++,v))return fail();}
            v.used=true;break;
        }
    }
    // 142e64220 keeps the learned remainder in descending native required-level
    // order, including newer third-awakening attacks absent from the older
    // autoset list. Native excluded movement skills are not attack fallbacks.
    for(auto& v:learned)if(!v.used&&v.id!=0xa9&&v.id!=0xbe&&attackSlot<9) {
        if(!bind(attackSlot++,v))return fail();
    }
    Call(b,0x5c77bb0,a,owner);
    Call(b,0x6e9fbc0,a+0x18da0);
    Call(b,0x6e9fcf0,a+0x18da0,uintptr_t(1));
    Call(b,0x6e9ff10,a+0x18da0,uintptr_t(0),uintptr_t(0));
    ReleaseResource(r.resource);
    r.ready=true;
    return true;
}

void DetachNativeRobot(uintptr_t b,NativeRobot& r) {
    r.active=false;
    if(!r.actor)return;
    uintptr_t a=r.actor;
    if(r.input) {
        Call(b,0x1b07610,r.input,uintptr_t(0));
        Call(b,0x5922690,r.input,uintptr_t(0));
    }
    if(r.ai&&Q(a,0x18e18)==r.ai)Q(a,0x18e18)=r.oldAI;
    if(r.ai)Delete(r.ai);
    if(r.profile)Delete(r.profile);
    if(r.skills) {
        Call(b,0x5ef1b50,r.skills);
        Call(b,0x6e9f3a0,r.skills,uintptr_t(0x190));
    }
    if(r.input)Delete(r.input);
    if(Q(a,0x11ba0)==r.input)Q(a,0x11ba0)=0;
    if(Q(a,0x11ba8)==r.skills)Q(a,0x11ba8)=0;
    if(Q(a,0x18a18)==r.profile)Q(a,0x18a18)=0;
    Call(b,0x5c77bb0,a,uintptr_t(0));
    Virtual(a,0x990,uintptr_t(r.oldTeam));
    memcpy((void*)(a+0x18da0),r.timer,sizeof(r.timer));
    ReleaseResource(r.resource);
    r=NativeRobot{};
}
