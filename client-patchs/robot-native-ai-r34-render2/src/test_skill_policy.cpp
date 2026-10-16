#include "../../elite-companion-pilot/src/inline_hook.h"
#include "skill_policy.h"
#include <assert.h>
#include <stdio.h>
#include <string.h>
static uintptr_t P(void* p){return (uintptr_t)p;}
static void Q(void* p,size_t o,uintptr_t v){memcpy((char*)p+o,&v,8);}
static void I(void* p,size_t o,int v){memcpy((char*)p+o,&v,4);}
extern "C" __declspec(dllexport) uintptr_t ProbeRobotAttackRange(uintptr_t manager,int job,int skill,uintptr_t fallback){
    return ReadRobotAttackRange(manager,job,skill,fallback);
}
// Offline oracle loads this test-only DLL into Python, never the game.
extern "C" __declspec(dllexport) int ProbeRobotSkillPolicy(uintptr_t manager,int job,int advance,int skill,int kind,int* value){
    return ReadRobotSkillPolicy(manager,job,advance,skill,(RobotSkillPolicy)kind,*value);
}
struct Tree {
    alignas(8) unsigned char head[64]={},node[80]={};
    Tree(int key,size_t keyOffset=0x20) {
        head[0x19]=1;Q(head,8,P(node));Q(node,0,P(head));Q(node,8,P(head));Q(node,16,P(head));I(node,keyOffset,key);
    }
};
int main(){
    alignas(8) unsigned char manager[0x300]={};
    Tree job(0),skill(40,0x1c),pair(0);
    I(pair.node,0x24,3);I(skill.node,0x20,6000);
    Q(job.node,0x28,P(skill.head));Q(pair.node,0x28,P(skill.head));
    Q(manager,0x108,P(job.head));Q(manager,0x128,P(pair.head));
    int v=0;
    Tree ranges(0),rangeSkill(40,0x1c);
    float normal[4]={100,40,30,20},overrideRange[4]={400,200,100,80};
    Q(manager,0xd8,P(ranges.head));Q(ranges.node,0x28,P(rangeSkill.head));
    memcpy(rangeSkill.node+0x20,overrideRange,16);
    assert(ReadRobotAttackRange(P(manager),0,40,P(normal))==P(rangeSkill.node+0x20));
    assert(ReadRobotAttackRange(P(manager),0,238,P(normal))==P(normal));
    assert(ReadRobotAttackRange(P(manager),0,-1,P(normal))==P(normal));
    assert(ReadRobotAttackRange(P(manager),17,40,P(normal))==P(normal));
    I(rangeSkill.node,0x20,0x7fc00000);
    assert(!ReadRobotAttackRange(P(manager),0,40,P(normal)));
    memcpy(rangeSkill.node+0x20,overrideRange,16);
    assert(!ReadRobotAttackRange(1,0,40,P(normal)));
    assert(ReadRobotSkillPolicy(P(manager),0,3,40,RobotSkillPolicy::HasDelay,v)&&v==1);
    assert(ReadRobotSkillPolicy(P(manager),0,3,40,RobotSkillPolicy::Delay,v)&&v==6000);
    assert(ReadRobotSkillPolicy(P(manager),0,3,40,RobotSkillPolicy::Reuse,v)&&v==6000);
    assert(ReadRobotSkillPolicy(P(manager),0,3,40,RobotSkillPolicy::HasReuse,v)&&v==1);
    I(skill.node,0x20,0);
    assert(ReadRobotSkillPolicy(P(manager),0,3,40,RobotSkillPolicy::HasDelay,v)&&v==1);
    assert(ReadRobotSkillPolicy(P(manager),0,3,40,RobotSkillPolicy::HasReuse,v)&&v==0);
    I(skill.node,0x20,1234);
    assert(ReadRobotSkillPolicy(P(manager),0,3,40,RobotSkillPolicy::Delay,v)&&v==1234);
    assert(ReadRobotSkillPolicy(P(manager),0,4,40,RobotSkillPolicy::Reuse,v)&&v==0);
    assert(ReadRobotSkillPolicy(P(manager),1,3,40,RobotSkillPolicy::Delay,v)&&v==0);
    assert(ReadRobotSkillPolicy(P(manager),0,3,41,RobotSkillPolicy::Delay,v)&&v==0);
    Tree list(0);int ids[]={-1,40,85};
    Q(list.node,0x28,P(ids));Q(list.node,0x30,P(ids+3));
    Q(manager,0x138,P(list.head));Q(manager,0xe8,P(list.head));
    unsigned char before[sizeof(manager)];memcpy(before,manager,sizeof(manager));
    assert(ReadRobotSkillPolicy(P(manager),0,3,40,RobotSkillPolicy::PartyBuff,v)&&v==1);
    assert(ReadRobotSkillPolicy(P(manager),0,3,85,RobotSkillPolicy::Charging,v)&&v==1);
    assert(ReadRobotSkillPolicy(P(manager),0,3,86,RobotSkillPolicy::Charging,v)&&v==0);
    assert(!memcmp(before,manager,sizeof(manager)));
    Q(list.node,0x30,P(ids)-4);
    assert(!ReadRobotSkillPolicy(P(manager),0,3,40,RobotSkillPolicy::PartyBuff,v));
    Q(job.node,0,P(job.node));
    assert(!ReadRobotSkillPolicy(P(manager),-1,3,40,RobotSkillPolicy::Delay,v));
    assert(!ReadRobotSkillPolicy(1,0,3,40,RobotSkillPolicy::Delay,v));
    puts("PASS source policy layouts, source mutation, missing keys, zero-delay semantics, corrupt input and no manager writes");
}
