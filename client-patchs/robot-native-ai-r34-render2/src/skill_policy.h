#pragma once
#include <stdint.h>
#include <stddef.h>
#include <math.h>

enum class RobotSkillPolicy { PartyBuff, Charging, HasDelay, Delay, HasReuse, Reuse, HasJump, Jump };

// Layouts are pinned to 515fefa1: 142e5cd00 binds outer job maps at
// e8/f8/108/138 and the (job, advancement) map at 128. Its node payload is +28.
// Nested skill maps use key +1c, value +20 (142e5f550/142e601f0).
// Readable is supplied by the existing host memory guard; no manager writes.
namespace RobotPolicyDetail {
template<class T> bool Read(uintptr_t p,T& v) {
    if(!p||!Readable((void*)p,sizeof(T)))return false;
    v=*(const T*)p;return true;
}
inline bool Find(uintptr_t map,int key,int second,bool pair,size_t keyOffset,uintptr_t& found) {
    found=0;uintptr_t head=0,node=0;
    if(!Read(map,head)||!Read(head+8,node))return false;
    for(unsigned depth=0;depth<128;depth++) {
        unsigned char nil=0;int current=0,other=0;
        if(!Read(node+0x19,nil))return false;
        if(nil)return true;
        if(!Read(node+keyOffset,current)||(pair&&!Read(node+keyOffset+4,other)))return false;
        if(current==key&&(!pair||other==second)){found=node;return true;}
        const bool right=current<key||(pair&&current==key&&other<second);
        if(!Read(node+(right?16:0),node))return false;
    }
    return false; // corrupt/cyclic tree, never enter an unchecked native lookup
}
inline bool Query(uintptr_t manager,int job,int advance,int skill,RobotSkillPolicy kind,int& result) {
    result=0;
    const bool vector=kind==RobotSkillPolicy::PartyBuff||kind==RobotSkillPolicy::Charging;
    const bool reuse=kind==RobotSkillPolicy::HasReuse||kind==RobotSkillPolicy::Reuse;
    const bool jump=kind==RobotSkillPolicy::HasJump||kind==RobotSkillPolicy::Jump;
    const size_t offset=kind==RobotSkillPolicy::PartyBuff?0x138:
        kind==RobotSkillPolicy::Charging?0xe8:reuse?0x128:jump?0xf8:0x108;
    uintptr_t node=0;
    if(!Find(manager+offset,job,advance,reuse,0x20,node))return false;
    if(!node)return true; // identical to an unbound native per-member table
    const uintptr_t payload=node+0x28;
    if(vector) {
        uintptr_t begin=0,end=0;
        if(!Read(payload,begin)||!Read(payload+8,end)||end<begin||(end-begin)%4||(end-begin)/4>4096)return false;
        for(uintptr_t p=begin;p<end;p+=4) {
            int id=0;if(!Read(p,id))return false;
            if(id==skill){result=1;return true;}
        }
        return true;
    }
    if(!Find(payload,skill,0,false,0x1c,node))return false;
    if(!node)return true;
    if(kind==RobotSkillPolicy::HasDelay||kind==RobotSkillPolicy::HasJump){result=1;return true;}
    if(!Read(node+0x20,result))return false;
    if(kind==RobotSkillPolicy::HasReuse)result=result>0;
    return true;
}
}
inline bool ReadRobotSkillPolicy(uintptr_t manager,int job,int advance,int skill,RobotSkillPolicy kind,int& result) {
    __try {return RobotPolicyDetail::Query(manager,job,advance,skill,kind,result);}
    __except(1) {result=0;return false;}
}

// 142e5cd00 binds manager+d8's job map payload (+28) to the Elite
// per-member skill-range table (+198). 142e60460 returns node+20 or the
// default range when that skill has no override. Read the same immutable
// source table for an ordinary companion without claiming an Elite seat.
inline uintptr_t ReadRobotAttackRange(uintptr_t manager,int job,int skill,uintptr_t fallback) {
    __try {
        uintptr_t jobNode=0,skillNode=0;
        if(!RobotPolicyDetail::Find(manager+0xd8,job,0,false,0x20,jobNode))return 0;
        uintptr_t result=fallback;
        if(jobNode) {
            if(!RobotPolicyDetail::Find(jobNode+0x28,skill,0,false,0x1c,skillNode))return 0;
            if(skillNode)result=skillNode+0x20;
        }
        float values[4]={};
        if(!result||!Readable((void*)result,sizeof(values)))return 0;
        for(unsigned i=0;i<4;i++) {
            values[i]=((const float*)result)[i];
            if(!isfinite(values[i])||values[i]<0)return 0;
        }
        return result;
    } __except(1) {return 0;}
}
