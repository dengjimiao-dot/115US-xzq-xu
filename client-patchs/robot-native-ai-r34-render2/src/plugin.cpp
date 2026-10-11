#define _CRT_SECURE_NO_WARNINGS
#include "native_actor.h"
#include "crash_trace.h"
#include "ai_policy.h"
#include "skill_policy.h"
#include "natural_policy.h"
#include <stdlib.h>
#include "native_evidence.h"
#include <stdio.h>
#include <intrin.h>
#include <math.h>

static HMODULE gSelf;
static volatile LONG gDiagnosticSession=1;
static uintptr_t gBase;
static FILE* gLog;
static InlineHook gFrameHook,gDestroyHook,gAiHook,gLocalHook,gAttackHook,gRangeHook,gAttackRangeHook,gSkillHook;
static InlineHook gMoveHook,gValueHook,gAwakeningHook,gTickHook,gGoalXHook,gGoalYHook;
using GoalFn=void(__fastcall*)(uintptr_t,int);
static GoalFn gGoalX,gGoalY;
static InlineHook gPartyBuffHook,gChargingHook,gHasDelayHook,gDelayHook,gHasReuseHook,gReuseHook,gHasJumpHook,gJumpHook;
using PolicyFn=uintptr_t(__fastcall*)(uintptr_t,int,uintptr_t);
static PolicyFn gPartyBuff,gCharging,gHasDelay,gDelay,gHasReuse,gReuse,gHasJump,gJump;
static volatile LONG gStarted=0,gEnabled=0;
static DWORD gGameThread;
static NativeRobot gRobots[3];
static AiLease gRooms[3]={},gOffer={};
static AiAck gAck={};
static uint64_t gFailedGeneration[3]={};
static ULONGLONG gLastFrame;
static SRWLOCK gIpcLock=SRWLOCK_INIT;
static wchar_t gOfferPath[1024],gAckPath[1024];
static thread_local bool gInFrame=false;
using FrameFn=void(__fastcall*)(uintptr_t);
using PredFn=uintptr_t(__fastcall*)(uintptr_t);
static FrameFn gFrame,gDestroy;
static InlineHook gCommonDestroyHook;static FrameFn gCommonDestroy;
static InlineHook gClassDestroyHook[6];
static FrameFn gClassDestroy[6];
static PredFn gAi,gLocal,gAttack,gCharacter;
static InlineHook gBaseStatsHook,gResetStatsHook,gRecalculateHook,gPetPreloadCall1Hook,gPetPreloadCall2Hook;
using StatsFn=void(__fastcall*)(uintptr_t,int);
using RecalculateFn=void(__fastcall*)(uintptr_t,unsigned char);
using PetPreloadFn=void(__fastcall*)(uintptr_t,uintptr_t);
static StatsFn gBaseStats,gResetStats;
static RecalculateFn gRecalculate;
static PetPreloadFn gPetPreload;
// Native equipment rebuild may nest and may calculate another actor. Only
// the currently owned ordinary actor suppresses the Elite identity adapter.
static thread_local uintptr_t gOrdinaryStatsActor=0;
struct OrdinaryStatsScope {
    uintptr_t previous;
    explicit OrdinaryStatsScope(uintptr_t actor):previous(gOrdinaryStatsActor) {gOrdinaryStatsActor=actor;}
    ~OrdinaryStatsScope(){gOrdinaryStatsActor=previous;}
};
static InlineHook gCharacterHook,gPartyHudHook,gPartyHudCompactHook,gCoinDrawHook;
static FrameFn gPartyHud,gPartyHudCompact;
using CoinDrawFn=void(__fastcall*)(uintptr_t,unsigned,int);
static CoinDrawFn gCoinDraw;
static thread_local uintptr_t gDrawingPartyHud=0;
static thread_local unsigned gCoinDrawMask=0;
static uint64_t gCoinLoggedGeneration[3]={};
static uint32_t gCoinLoggedDeath[3]={};
using RangeFn=uintptr_t(__fastcall*)(uintptr_t,uintptr_t);
static RangeFn gRange;
using AttackRangeFn=uintptr_t(__fastcall*)(uintptr_t,int,uintptr_t);
static AttackRangeFn gAttackRange;
using SkillFn=void(__fastcall*)(uintptr_t,int,unsigned char);
static SkillFn gSkill;
static InlineHook gSkillLookupHook;
using SkillLookupFn=uintptr_t(__fastcall*)(uintptr_t,int,uintptr_t,uintptr_t);
static SkillLookupFn gSkillLookup;
static InlineHook gBackdropCoordinatesHook;
using CoordinatesFn=int*(__fastcall*)(uintptr_t,int*);
static CoordinatesFn gBackdropCoordinates;
static uint64_t gBackdropLogged[3][5]={};
using MoveFn=void(__fastcall*)(uintptr_t,unsigned);
using ValueFn=float(__fastcall*)(uintptr_t,int);
static MoveFn gMove;
static ValueFn gValue;
static PredFn gAwakening;
static FrameFn gTick;
static thread_local bool gFollowEvaluation=false;
static thread_local float gFollowDistance=0;
static uint64_t Now() {
    FILETIME f;GetSystemTimeAsFileTime(&f);
    return ((((uint64_t)f.dwHighDateTime<<32)|f.dwLowDateTime)/10000)-11644473600000ull;
}
static uintptr_t Q(uintptr_t p,size_t off=0) {return *(uintptr_t*)(p+off);}
template<class... A> static uintptr_t Call(uintptr_t rva,A... a) {
#ifdef ROBOT_NATIVE_TEST
    const uintptr_t words[]={0,(uintptr_t)a...};
    extern uintptr_t TestNativeCall(uintptr_t,const uintptr_t*,size_t);
    return TestNativeCall(rva,words+1,sizeof...(a));
#else
    return ((uintptr_t(__fastcall*)(A...))(gBase+rva))(a...);
#endif
}
static uintptr_t Map(uintptr_t actor) {
    return ((uintptr_t(__fastcall*)(uintptr_t))Q(Q(actor),0x130))(actor);
}
static NativeRobot* Active(uintptr_t pointer,bool ai=false) {
    if(!gEnabled||GetCurrentThreadId()!=gGameThread)return nullptr;
    AiLease current;
    AcquireSRWLockShared(&gIpcLock);current=gOffer;ReleaseSRWLockShared(&gIpcLock);
    if(!ValidAiOffer(current,Now())||current.phase!=2)return nullptr;
    for(unsigned i=0;i<3;i++) {
        auto& r=gRobots[i];
        if(r.active&&SameAiRoom(gRooms[i],current)&&(ai?r.ai:r.actor)==pointer)return &r;
    }
    return nullptr;
}
// The ordinary coin renderer and the F9..F12 handler are independent. The
// latter already resolves a dead party actor and opens native dialog 0x9a9.
// Supply the existing renderer for owned dead companions when the HUD omitted
// it (including after its ordinary ten-second offer). A bot cannot request its
// own revive, so retain this payer-facing entry until revived. Map restrictions,
// coin balance, cooldown and the native confirmation/payment path remain native.
static void __fastcall CoinDraw(uintptr_t hud,unsigned seat,int visualSeat) {
    if(hud==gDrawingPartyHud&&seat<8)gCoinDrawMask|=1u<<seat;
    gCoinDraw(hud,seat,visualSeat);
}
static void SupplementReviveHud(uintptr_t hud) {
    if(!gEnabled||GetCurrentThreadId()!=gGameThread||!Readable((void*)hud,0x35b0))return;
    const uintptr_t world=Q(gBase+0xe66c090),ui=Q(gBase+0xe683c78),party=Q(gBase+0xe683c20);
    if(!world||!ui||!party)return;
    int scene=(int)Call(0x59a90f0,world);
    if(scene!=3&&scene!=10)return;
    // Native renderer dependencies; no lazy construction or fabricated widgets.
    if(!Q(hud,8)||!Q(hud,0x4d0)||!Q(hud,0x870))return;
    AiLease offer;
    AcquireSRWLockShared(&gIpcLock);offer=gOffer;ReleaseSRWLockShared(&gIpcLock);
    if(!ValidAiOffer(offer,Now())||offer.phase!=2||Call(0x5f12d90,party)!=0)return;
    for(unsigned seat=1;seat<=3;seat++) {
        auto& robot=gRobots[seat-1];
        if(!Active(robot.actor)||!robot.lifeDead||!Readable((void*)(robot.actor+0x11502),1)||
           !*(unsigned char*)(robot.actor+0x11502))continue;
        int visualSeat=(int)seat;
        if(Call(0x556cdf0,hud)) {
            const uintptr_t pager=Q(gBase+0xe638ab0);
            if(!pager)continue;
            const int page=(int)Call(0x217d5b0,pager);visualSeat=-1;
            if(page<0||page>1)continue;
            for(int index=0;index<4;index++)if(Call(0x217d600,pager,page*4+index)==seat){visualSeat=index;break;}
            if(visualSeat<0)continue;
        }
        if(!Q(hud,0x648+visualSeat*0x10))continue;
        auto rec=Call(0x5f13700,party,seat);
        if(!rec||Call(0x5effac0,rec)!=robot.actor||Call(0x5f01210,rec)!=offer.party.bots[seat])continue;
        uintptr_t revive=Call(0x667c460,ui),inventory=Q(gBase+0xe683c80);
        if(!revive||!inventory)continue;
        auto token=Call(0x5ad7f80,inventory);
        if(!token||!Readable((void*)token,8)||!Readable((void*)Q(token),0xe8))continue;
        int coins=(int)((PredFn)Q(Q(token),0xe0))(token);
        bool blocked=*(unsigned char*)(gBase+0xe66bb23)!=0||Call(0x5595e50)!=0||
            Call(0x5596120,revive)!=0||Call(0x5595f90,revive)!=0||Call(0x5596080,revive,seat)!=0;
        if(Call(0x5595de0)&&Call(0x5596340,revive,seat))blocked=true;
        const uintptr_t special=Q(gBase+0xe638470);
        if(special&&Call(0x5494680,special))blocked=true;
        const bool drawn=(gCoinDrawMask&(1u<<seat))!=0;
        if(!blocked&&coins>0&&!drawn)gCoinDraw(hud,seat,visualSeat);
        if(gCoinLoggedGeneration[seat-1]!=offer.generation||gCoinLoggedDeath[seat-1]!=robot.deathSequence) {
            gCoinLoggedGeneration[seat-1]=offer.generation;gCoinLoggedDeath[seat-1]=robot.deathSequence;
            fprintf(gLog,"REVIVE_HUD seat=%u actor=%u coins=%d nativeDraw=%d blocked=%d supplement=%d death=%u\n",
                seat,offer.party.bots[seat],coins,drawn,blocked,!blocked&&coins>0&&!drawn,robot.deathSequence);fflush(gLog);
        }
    }
}
static void RenderPartyHud(uintptr_t hud,FrameFn original) {
    const auto previous=gDrawingPartyHud;const unsigned previousMask=gCoinDrawMask;
    gDrawingPartyHud=hud;gCoinDrawMask=0;
    original(hud);SupplementReviveHud(hud);
    gDrawingPartyHud=previous;gCoinDrawMask=previousMask;
}
static void __fastcall PartyHud(uintptr_t hud){RenderPartyHud(hud,gPartyHud);}
static void __fastcall PartyHudCompact(uintptr_t hud){RenderPartyHud(hud,gPartyHudCompact);}
// The ordinary actor is never inserted into the three Elite weak slots.
// Read the same immutable source maps used by 142e5cd00 to populate those slots.
static uintptr_t QueryPolicy(uintptr_t manager,int skill,uintptr_t actor,RobotSkillPolicy kind,PolicyFn original) {
    if(!Active(actor))return original(manager,skill,actor);
    const int job=(int)((PredFn)Q(Q(actor),0x12e0))(actor);
    const int advance=(int)((PredFn)Q(Q(actor),0x12f0))(actor);
    int value=0;
    if(!ReadRobotSkillPolicy(manager,job,advance,skill,kind,value)) {
        static bool reported=false;
        if(!reported){reported=true;fputs("POLICY_REFUSED unreadable source table; native Elite slot lookup suppressed\n",gLog);fflush(gLog);}
        return 0;
    }
    return (uint32_t)value;
}
static uintptr_t __fastcall PartyBuff(uintptr_t m,int s,uintptr_t a){return QueryPolicy(m,s,a,RobotSkillPolicy::PartyBuff,gPartyBuff);}
static uintptr_t __fastcall Charging(uintptr_t m,int s,uintptr_t a){return QueryPolicy(m,s,a,RobotSkillPolicy::Charging,gCharging);}
static uintptr_t __fastcall HasDelay(uintptr_t m,int s,uintptr_t a){return QueryPolicy(m,s,a,RobotSkillPolicy::HasDelay,gHasDelay);}
static uintptr_t __fastcall Delay(uintptr_t m,int s,uintptr_t a){return QueryPolicy(m,s,a,RobotSkillPolicy::Delay,gDelay);}
static uintptr_t __fastcall HasReuse(uintptr_t m,int s,uintptr_t a){return QueryPolicy(m,s,a,RobotSkillPolicy::HasReuse,gHasReuse);}
static uintptr_t __fastcall Reuse(uintptr_t m,int s,uintptr_t a){return QueryPolicy(m,s,a,RobotSkillPolicy::Reuse,gReuse);}
static uintptr_t __fastcall HasJump(uintptr_t m,int s,uintptr_t a){return QueryPolicy(m,s,a,RobotSkillPolicy::HasJump,gHasJump);}
static uintptr_t __fastcall Jump(uintptr_t m,int s,uintptr_t a){return QueryPolicy(m,s,a,RobotSkillPolicy::Jump,gJump);}
static uintptr_t __fastcall Ai(uintptr_t p) {
    return p!=gOrdinaryStatsActor&&Active(p)?1:gAi(p);
}
static void __fastcall BaseStats(uintptr_t actor,int mode) {
    OrdinaryStatsScope scope(Active(actor)?actor:gOrdinaryStatsActor);
    gBaseStats(actor,mode);
}
static void __fastcall ResetStats(uintptr_t actor,int mode) {
    OrdinaryStatsScope scope(Active(actor)?actor:gOrdinaryStatsActor);
    gResetStats(actor,mode);
}
static void __fastcall Recalculate(uintptr_t actor,unsigned char mode) {
    OrdinaryStatsScope scope(Active(actor)?actor:gOrdinaryStatsActor);
    gRecalculate(actor,mode);
}
// This optional pet-skill preload dereferences a resource without a null check.
// Current species 30338 names 490100178, absent from the pinned passive list.
// Preserve the pet/skill and the normal loader; skip only that absent preload.
static void __fastcall PetPreload(uintptr_t pet,uintptr_t index) {
    if(index==490100178u) {
        uintptr_t source=Call(0x21be90,Q(gBase+0xe683b28),index,1);
        if(!source) {
            static volatile LONG reported=0;
            if(!InterlockedExchange(&reported,1)) {
                fputs("PET_PRELOAD missing optional object 490100178; pet retained\n",gLog);fflush(gLog);
            }
            return;
        }
    }
    gPetPreload(pet,index);
}
// v2338 normally selects a clone-owned character record when v90 is true.
// Our AI profile owns input behavior, not a cloned player record. Resolve the
// ordinary actor's current native cache entry by its own identity, every call.
static uintptr_t __fastcall Character(uintptr_t actor) {
    return Active(actor)?Call(0x5ce9040,actor):gCharacter(actor);
}
static uintptr_t __fastcall Local(uintptr_t p) {return Active(p)?1:gLocal(p);}
// Generic AI's attack-mode virtual normally reads the Elite manager. The fake
// has its own explicit attack policy, and never changes that manager's mode.
static uintptr_t __fastcall Attack(uintptr_t p) {return Active(p,true)?1:gAttack(p);}
static uintptr_t __fastcall Range(uintptr_t manager,uintptr_t actor) {
    if(auto r=Active(actor))return (uintptr_t)r->retreatRange;
    return gRange(manager,actor);
}
static uintptr_t __fastcall AttackRange(uintptr_t manager,int skill,uintptr_t actor) {
    if(auto r=Active(actor))return ReadRobotAttackRange(manager,(int)((PredFn)Q(Q(actor),0x12e0))(actor),skill,(uintptr_t)r->attackRange);
    return gAttackRange(manager,skill,actor);
}
static void __fastcall Skill(uintptr_t module,int id,unsigned char mode) {
    // This one native key-stream call treats skill:N as a raw skill ID for
    // ordinary players, but the reused script means slot N. Translate only
    // that call and this adapter's owned module; every other request is native.
    if((uintptr_t)_ReturnAddress()==gBase+0x5e67c2f) {
        for(auto& r:gRobots)if(r.skills==module&&Active(r.actor)) {
            if(id<0||id>=14||r.skillIDs[id]<0)return;
            id=r.skillIDs[id];break;
        }
    }
    gSkill(module,id,mode);
}

// Native job handlers query raw skill IDs (145e769d0 requests ID 7), while
// the owned hotbar only contains fourteen selected actions. Its 145ef4430
// reverse map therefore omits valid learned prerequisites. Query the ordinary
// actor's source-backed native cache, preserving hotbar order/cooldowns/levels.
// Only this adapter's active owned module uses the bridge; all others pass through.
static uintptr_t __fastcall SkillLookup(uintptr_t module,int id,uintptr_t arg3,uintptr_t arg4) {
    for(auto& r:gRobots)if(r.skills==module&&Active(r.actor)) {
        if(id<0||id>=512)return 0;
        return Call(0x5ce53a0,r.actor,uintptr_t(id),uintptr_t(0));
    }
    return gSkillLookup(module,id,arg3,arg4);
}

// [target distance] is enum 11 in 145e59e80. Only the synchronous
// MoveMethod evaluation below sees the owner distance; attack selection,
// other actors and the shared native condition map retain their own values.
static float __fastcall Value(uintptr_t condition,int key) {
    return gFollowEvaluation&&key==11?gFollowDistance:gValue(condition,key);
}
static unsigned RobotSeat(const NativeRobot* r) {return unsigned(r-gRobots);}
// Destination setters feed the existing walkability/grid path solver. Adjust
// the requested destination, never actor position or the shared owner/target.
static int OwnedGoal(uintptr_t ai,int destination,bool y) {
 if(auto r=Active(ai,true)) {
  if(r->deadInput||r->lifeDead||!r->owner||Map(r->owner)!=Map(r->actor))return destination;
  const bool combat=Call(0x5e6eb00,ai)!=0;
  return OffsetGoal(destination,RobotSeat(r),y,combat);
 }
 return destination;
}
static void __fastcall GoalX(uintptr_t ai,int destination){gGoalX(ai,OwnedGoal(ai,destination,false));}
static void __fastcall GoalY(uintptr_t ai,int destination){gGoalY(ai,OwnedGoal(ai,destination,true));}
static void __fastcall Move(uintptr_t ai,unsigned mode) {
    if(auto r=Active(ai,true)) {
        if(!gFollowEvaluation&&!Call(0x5e6eb00,ai)&&r->owner&&Map(r->owner)==Map(r->actor)) {
            double dx=OffsetGoal((int)Call(0x5b8c7d0,r->owner),RobotSeat(r),false,false)-(double)(int)Call(0x5b8c7d0,r->actor);
            double dy=OffsetGoal((int)Call(0x5b8c890,r->owner),RobotSeat(r),true,false)-(double)(int)Call(0x5b8c890,r->actor);
            gFollowDistance=(float)sqrt(dx*dx+dy*dy);
            gFollowEvaluation=true;
            unsigned nativeMode=(unsigned)((uintptr_t(__fastcall*)(uintptr_t,int))Q(Q(ai),0x28))(ai,3);
            gFollowEvaluation=false;
            if(nativeMode<3)mode=nativeMode;
        }
    }
    gMove(ai,mode);
}
static uintptr_t __fastcall Awakening(uintptr_t ai) {
    if(auto r=Active(ai,true)) {
        if(r->deadInput||Now()<r->awakeningAfter||!Call(0x5e6eb00,ai))return 0;
        // Native usability and cooldown predicates arbitrate every request.
        // Preserve the native buff branch before selecting an awakening.
        for(unsigned slot=9;slot<12;slot++)
            if(r->skillIDs[slot]>=0&&!(Call(0x5e70cc0,ai,slot)&255))return 0;
        for(unsigned slot=13;slot>=12;slot--)
            if(r->skillIDs[slot]>=0&&!(Call(0x5e70cc0,ai,slot)&255))return slot-11;
        return 0;
    }
    return gAwakening(ai);
}
static void __fastcall Tick(uintptr_t actor) {
    if(auto r=Active(actor)) {
        const bool dead=(int)Call(0x5790070,actor)==5;
        if(dead) {
            if(!r->deadInput) {
                Call(0x1b07610,r->input,uintptr_t(0));
                Call(0x5922690,r->input,uintptr_t(0));
                r->deadInput=true;
                fprintf(gLog,"%llu DEATH_RETAIN actor=%p ordinary party entity; Elite AI cleanup suppressed\n",Now(),(void*)actor);fflush(gLog);
            }
            // This native AI tick ends by deleting dead Elite clones. An
            // ordinary party member remains under the ordinary death/revive
            // lifecycle. Its common frame still runs exactly once.
            return;
        }
        if(r->deadInput) {
            Call(0x5922690,r->input,uintptr_t(1));r->deadInput=false;
            fprintf(gLog,"%llu DEATH_STATE_LEFT actor=%p\n",Now(),(void*)actor);fflush(gLog);
        }
        if(Now()<r->driveAfter)return;
    }
    gTick(actor);
}

static bool Scope(const AiLease& lease,uintptr_t actor,unsigned& slot,uintptr_t& owner) {
    __try {
        uintptr_t manager=Q(gBase,0xe66c090),world=Q(gBase,0xe6839f0);
        if(!manager||!world||!Q(gBase,0xe683c20)||*(uint32_t*)(world+0xa0)!=0)return false;
        unsigned scene=(unsigned)Call(0x59a90f0,manager);
        unsigned local=(unsigned)Call(0x59a9d30,manager);
        uintptr_t pointers[8]={};uint32_t ids[8]={};
        slot=0;
        for(unsigned i=0;i<8;i++) {
            pointers[i]=Call(0x59a9de0,manager,uintptr_t(i));
            if(pointers[i])ids[i]=(uint32_t)Call(0x5b8b0a0,pointers[i]);
            if(pointers[i]==actor)slot=i;
        }
        owner=pointers[0];
        if(!owner||owner!=Call(0x5efafb0)||!slot)return false;
        uintptr_t map=Map(owner);
        return AllowNativeRobot(lease,Now(),scene,local,slot,map&&Map(actor)==map,ids);
    } __except(EXCEPTION_EXECUTE_HANDLER) {return false;}
}
// 14266cf80 creates five viewport-scaled, non-damaging multi-animation
// layers at the caster's world position. Remote ordinary teammates can be
// off-centre in the local viewport. Change only the two draw outputs, using
// the native centre-conversion branch in 145b8ac50 and its animation offsets.
// The world entity, attack objects, flags, timers and camera are not written.
static bool BackdropScope(uintptr_t object,unsigned& index,unsigned& layer) {
    if(!gEnabled||GetCurrentThreadId()!=gGameThread||!Readable((void*)object,8)||
       Q(object)!=gBase+0x9c4f688)return false;
    __try {
        for(unsigned i=0;i<3;i++) {
            const auto& r=gRobots[i];
            if(!r.ready||!Active(r.actor))continue;
            if((int)Call(0x5790070,r.actor)!=128||
               (int)((PredFn)Q(Q(r.actor),0x12e0))(r.actor)!=14)continue;
            AiLease lease;AcquireSRWLockShared(&gIpcLock);lease=gOffer;ReleaseSRWLockShared(&gIpcLock);
            unsigned slot=0;uintptr_t owner=0;
            if(!Scope(lease,r.actor,slot,owner)||slot!=i+1)continue;
            uintptr_t state=Call(0x5ce5810,r.actor,128);
            if(!state||!Readable((void*)state,0x410)||Q(state)!=gBase+0x9a46f30||
               *(int*)(state+0x90)!=121)continue;
            uintptr_t refs=Q(state,0xa0);
            if(!refs||!Readable((void*)refs,12)||!*(int*)(refs+8)||
               Q(state,0xa8)!=r.actor+0x30)continue;
            for(unsigned j=0;j<5;j++)if(Q(state,0x3e8+j*8)==object) {
                if(!Readable((void*)object,0x1a0)||Map(object)!=Map(r.actor))return false;
                index=i;layer=j;return true;
            }
        }
    } __except(EXCEPTION_EXECUTE_HANDLER) {return false;}
    return false;
}
static bool CentreBackdrop(uintptr_t object,int& x,int& y) {
    __try {
        const uintptr_t camera=Q(gBase,0xe634238);
        int width=*(int*)(gBase+0xdc6d8f8),height=*(int*)(gBase+0xdc6d8fc);
        if(!camera||width<640||width>8192||height<360||height>8192)return false;
        uintptr_t refs=Q(object,0x190),animation=0;
        if(refs&&Readable((void*)refs,12)&&*(int*)(refs+8))animation=Q(object,0x198);
        // Match the current render context in the original centre branch.
        // 14266cf80 uses layer 6 only for the first backdrop; the other four
        // use layer 1. Using 6 for all of them mixes different camera planes.
        int renderLayer=*(int*)(gBase+0xdc63f70);
        int layer=(int)Call(0x59d7c00,renderLayer);
        int cx=(int)Call(0xeede60,camera,width/2,layer,1,uintptr_t(0),(unsigned char)0);
        int cy=(int)Call(0xeedf70,camera,height/2,uintptr_t(0),layer,1,uintptr_t(0),(unsigned char)0);
        int ox=(int)Call(0xdcfe20,animation),oy=(int)Call(0x2ad8420,animation);
        const int64_t nx=int64_t(cx)+ox,ny=int64_t(cy)+oy;
        if(nx<-65536||nx>65536||ny<-65536||ny>65536)return false;
        x=(int)nx;y=(int)ny;return true;
    } __except(EXCEPTION_EXECUTE_HANDLER) {return false;}
}
static int* __fastcall BackdropCoordinates(uintptr_t object,int* xy) {
    int* result=gBackdropCoordinates(object,xy);
    unsigned index=0,layer=0;int x=0,y=0;
    if(result!=xy||!BackdropScope(object,index,layer)||!CentreBackdrop(object,x,y))return result;
    if(gLog&&gBackdropLogged[index][layer]!=gRooms[index].generation) {
        gBackdropLogged[index][layer]=gRooms[index].generation;
        fprintf(gLog,"BACKDROP_CENTER slot=%u layer=%u native=%d,%d centred=%d,%d render=%d generation=%llu\n",
                index+1,layer,xy[0],xy[1],x,y,*(int*)(gBase+0xdc63f70),gRooms[index].generation);fflush(gLog);
    }
    xy[0]=x;xy[1]=y;return result;
}

static void PublishFrame(const AiLease& offer) {
    AiAck life={};AiLease& ack=life.lease;ack=offer;ack.clientPID=GetCurrentProcessId();
    life.sampledAt=Now();
    bool all=offer.party.count>0&&offer.party.count<=3;
    for(unsigned i=0;i<offer.party.count&&i<3;i++)
        all=all&&gRobots[i].ready&&SameAiRoom(gRooms[i],offer);
    if(!all)ack.phase=0;
    if(all)for(unsigned i=0;i<offer.party.count;i++) {
        const auto& r=gRobots[i];
        life.deaths[i]=r.deathSequence;
        if(r.lifeDead)life.deadMask|=1u<<i;
        if(r.lifeSample<life.sampledAt)life.sampledAt=r.lifeSample;
    }
    for(unsigned i=0;i<3;i++)if(gFailedGeneration[i]==offer.generation&&offer.generation)ack.phase=3;
    AcquireSRWLockExclusive(&gIpcLock);gAck=life;gLastFrame=GetTickCount64();ReleaseSRWLockExclusive(&gIpcLock);
}
static void __fastcall Frame(uintptr_t actor) {
    if(!gEnabled||gInFrame){gFrame(actor);return;}
    if(!gGameThread)gGameThread=GetCurrentThreadId();
    if(gGameThread!=GetCurrentThreadId()){gFrame(actor);return;}
    gInFrame=true;
    AiLease lease;
    AcquireSRWLockShared(&gIpcLock);lease=gOffer;ReleaseSRWLockShared(&gIpcLock);
    unsigned slot=0;uintptr_t owner=0;
    const bool scoped=ValidAiOffer(lease,Now())&&Scope(lease,actor,slot,owner);
    for(unsigned i=0;i<3;i++)if(gRobots[i].actor==actor&&
        (!scoped||slot!=i+1||!SameAiRoom(gRooms[i],lease))) {
        DetachNativeRobot(gBase,gRobots[i]);gRooms[i]={};
        fprintf(gLog,"%llu DETACH slot=%u actor=%p scope=%d valid=%d phase=%u generation=%llu\n",Now(),i+1,(void*)actor,scoped,ValidAiOffer(lease,Now()),lease.phase,lease.generation);fflush(gLog);
    }
    if(scoped&&slot<=3&&gFailedGeneration[slot-1]!=lease.generation) {
        auto& r=gRobots[slot-1];
        // A pointer that has not reached its destructor cannot be silently
        // replaced with another entity. Refuse until the old native lifetime ends.
        if(!r.actor) {
            fprintf(gLog,"ATTACH_BEGIN slot=%u actor=%p generation=%llu\n",slot,(void*)actor,lease.generation);fflush(gLog);
            if(AttachNativeRobot(gBase,r,actor,owner)) {
                gRooms[slot-1]=lease;
                fprintf(gLog,"PREPARED slot=%u owned_input=%p owned_skills=%p\n",slot,(void*)r.input,(void*)r.skills);fflush(gLog);
            } else {
                gFailedGeneration[slot-1]=lease.generation;
                fprintf(gLog,"REFUSED slot=%u native initialization prerequisites\n",slot);fflush(gLog);
            }
        }
        if(r.actor==actor&&r.ready&&SameAiRoom(gRooms[slot-1],lease)) {
            bool drive=lease.phase==2;
            if(drive&&!r.active) {
                r.driveAfter=Now()+InitialBrainDelay(slot-1,lease.party.bots[slot]);
                r.awakeningAfter=r.driveAfter+InitialAwakeningDelay(slot-1,lease.party.bots[slot]);
                fprintf(gLog,"DRIVE slot=%u generation=%llu skills=%d,%d,%d,%d,%d range=%.1f,%.1f,%.1f,%.1f\n",slot,lease.generation,
                    r.skillIDs[0],r.skillIDs[1],r.skillIDs[2],r.skillIDs[3],r.skillIDs[4],
                    r.attackRange[0],r.attackRange[1],r.attackRange[2],r.attackRange[3]);fflush(gLog);
            }
            r.active=drive;
            int state=(int)Call(0x5790070,actor);
            uint64_t now=Now();
            const bool dead=state==5;
            if(dead&&!r.lifeDead&&r.deathSequence<256)++r.deathSequence;
            r.lifeDead=dead;r.lifeSample=now;
            if(state!=r.lastState||now-r.lastHealthLog>=1000) {
                fprintf(gLog,"%llu STATE slot=%u actor=%p state=%d hp=%d maxhp=%d ai=%p input=%p move=%u phase=%u generation=%llu\n",now,slot,(void*)actor,state,
                    (int)Call(0x5c19920,actor),(int)Call(0x5c199c0,actor),(void*)r.ai,(void*)r.input,*(unsigned*)(r.ai+0x730),lease.phase,lease.generation);
                if(r.lastState<0) {fputs("SLOTS",gLog);for(unsigned n=0;n<14;n++)fprintf(gLog," %u:%d",n,r.skillIDs[n]);fputc('\n',gLog);}
                fflush(gLog);r.lastState=state;r.lastHealthLog=now;
            }
        }
    }
    PublishFrame(lease);
    // The original common frame owns movement, input consumption, attacks,
    // skill cooldowns and state transitions. No extra AI tick or coordinate loop.
    gFrame(actor);
    gInFrame=false;
}
static void BeforeDestroy(uintptr_t actor) {
    for(unsigned i=0;i<3;i++)if(gRobots[i].actor==actor) {
        DetachNativeRobot(gBase,gRobots[i]);gRooms[i]={};
        fprintf(gLog,"%llu DESTROY_DETACH slot=%u actor=%p caller=%p\n",Now(),i+1,(void*)actor,_ReturnAddress());fflush(gLog);
    }
}
static void __fastcall Destroy(uintptr_t actor) {BeforeDestroy(actor);gDestroy(actor);}
static void __fastcall CommonDestroy(uintptr_t actor) {BeforeDestroy(actor);gCommonDestroy(actor);}
template<unsigned I> static void __fastcall ClassDestroy(uintptr_t actor) {BeforeDestroy(actor);gClassDestroy[I](actor);}
static bool ReadOffer(AiLease& result) {
    HANDLE h=CreateFileW(gOfferPath,GENERIC_READ,FILE_SHARE_READ|FILE_SHARE_WRITE|FILE_SHARE_DELETE,
                        nullptr,OPEN_EXISTING,FILE_ATTRIBUTE_NORMAL,nullptr);
    if(h==INVALID_HANDLE_VALUE)return false;
    LARGE_INTEGER size={};DWORD got=0;
    bool ok=GetFileSizeEx(h,&size)&&size.QuadPart==sizeof(result)&&
       ReadFile(h,&result,sizeof(result),&got,nullptr)&&got==sizeof(result);
    CloseHandle(h);
    if(!ok||!ValidAiOffer(result,Now()))return false;
    HANDLE server=OpenProcess(SYNCHRONIZE,FALSE,result.party.serverPID);
    ok=server&&WaitForSingleObject(server,0)==WAIT_TIMEOUT;
    if(server)CloseHandle(server);
    return ok;
}
static void WriteAck(const AiAck& ack) {
    wchar_t tmp[1100];swprintf_s(tmp,L"%s.%lu.tmp",gAckPath,GetCurrentProcessId());
    HANDLE h=CreateFileW(tmp,GENERIC_WRITE,0,nullptr,CREATE_ALWAYS,FILE_ATTRIBUTE_NORMAL,nullptr);
    if(h==INVALID_HANDLE_VALUE)return;
    DWORD got=0;bool ok=WriteFile(h,&ack,sizeof(ack),&got,nullptr)&&got==sizeof(ack);
    CloseHandle(h);
    if(!ok||!MoveFileExW(tmp,gAckPath,MOVEFILE_REPLACE_EXISTING|MOVEFILE_WRITE_THROUGH))DeleteFileW(tmp);
}
static DWORD WINAPI IpcWorker(void*) {
    for(;;) {
        AiLease offer={};AiAck life={};if(!ReadOffer(offer))offer={};
        AcquireSRWLockExclusive(&gIpcLock);
        gOffer=offer;life=gAck;
        AiLease& ack=life.lease;
        // Menus may pause actor frames. The worker may retain an already
        // prepared same-generation acknowledgement; it never renews another
        // room or revives a stopped generation from an old snapshot.
        bool fresh=ValidAiOffer(offer,Now())&&SameAiRoom(offer,ack)&&ack.phase;
        ReleaseSRWLockExclusive(&gIpcLock);
        if(fresh){ack.party.issued=Now();WriteAck(life);}
        Sleep(100);
    }
}
extern "C" __declspec(dllexport) const char* WINAPI ModName(){return "Local ordinary robot native AI r34-render2";}
extern "C" __declspec(dllexport) DWORD WINAPI ModStart() {
    if(InterlockedCompareExchange(&gStarted,1,0))return 0;
    wchar_t path[1024]={},opt[64]={};
    DWORD n=GetModuleFileNameW(gSelf,path,_countof(path));if(!n||n>=_countof(path))return 1;
    auto slash=wcsrchr(path,L'\\');if(!slash)return 1;
    wcscpy_s(slash+1,_countof(path)-(slash+1-path),L"robot-native-ai.log");
    gLog=_wfopen(path,L"ab");if(!gLog)return 1;
    if(!GetEnvironmentVariableW(L"DFO_LOCAL_ROBOT_AI",opt,_countof(opt))||wcscmp(opt,L"r23-native-input-v1")) {
        fputs("INACTIVE opt-in absent; no hooks or actor writes\n",gLog);fflush(gLog);return 0;
    }
    n=GetEnvironmentVariableW(L"DFO_ROBOT_AI_OFFER",gOfferPath,_countof(gOfferPath));
    if(n<4||n>=_countof(gOfferPath)||gOfferPath[1]!=L':'||gOfferPath[2]!=L'\\')return 2;
    n=GetEnvironmentVariableW(L"DFO_ROBOT_AI_ACK",gAckPath,_countof(gAckPath));
    if(n<4||n>=_countof(gAckPath)||gAckPath[1]!=L':'||gAckPath[2]!=L'\\')return 2;
    gBase=(uintptr_t)GetModuleHandleW(nullptr);
    if(!VerifyNativeImage(gBase)) {fputs("DISABLED image/function mismatch\n",gLog);fflush(gLog);return 3;}
    wcscpy_s(slash+1,_countof(path)-(slash+1-path),L"robot-native-ai-crash.log");
    if(!CrashTrace::Start(path,&gDiagnosticSession))return 7;
    fprintf(gLog,"CANDIDATE r34-render2 pid=%lu ordinary character cache preserved under AI drive; source skill policy retained; passive AV evidence enabled\n",GetCurrentProcessId());fflush(gLog);
    struct Entry {InlineHook* hook;uintptr_t rva;const unsigned char* bytes;size_t size;void* target;bool call=false;};
    Entry entries[]={
        {&gFrameHook,0x5c73760,kFrameHead,sizeof(kFrameHead),(void*)Frame},
        {&gCommonDestroyHook,0x5787110,kCommonDestroyHead,sizeof(kCommonDestroyHead),(void*)CommonDestroy},
        {&gDestroyHook,0x642a970,kDestroyHead,sizeof(kDestroyHead),(void*)Destroy},
        {&gClassDestroyHook[0],0x6363da0,kDestroyFighterHead,sizeof(kDestroyFighterHead),(void*)ClassDestroy<0>},
        {&gClassDestroyHook[1],0x6372ac0,kDestroyGunnerHead,sizeof(kDestroyGunnerHead),(void*)ClassDestroy<1>},
        {&gClassDestroyHook[2],0x63c4ac0,kDestroyMageHead,sizeof(kDestroyMageHead),(void*)ClassDestroy<2>},
        {&gClassDestroyHook[3],0x6409b40,kDestroyPriestHead,sizeof(kDestroyPriestHead),(void*)ClassDestroy<3>},
        {&gClassDestroyHook[4],0x33ee40,kDestroyFemaleGunnerHead,sizeof(kDestroyFemaleGunnerHead),(void*)ClassDestroy<4>},
        {&gClassDestroyHook[5],0x6448c90,kDestroyThiefHead,sizeof(kDestroyThiefHead),(void*)ClassDestroy<5>},
        {&gAiHook,0x5c70760,kAiHead,sizeof(kAiHead),(void*)Ai},
        {&gLocalHook,0x5c70f30,kLocalHead,sizeof(kLocalHead),(void*)Local},
        {&gAttackHook,0x5e6fd10,kAttackHead,sizeof(kAttackHead),(void*)Attack},
        {&gRangeHook,0x2e60350,kRangeHead,sizeof(kRangeHead),(void*)Range},
        {&gAttackRangeHook,0x2e60460,kAttackRangeHead,sizeof(kAttackRangeHead),(void*)AttackRange},
        {&gSkillHook,0x5ef9270,kSkillHead,sizeof(kSkillHead),(void*)Skill},
        {&gSkillLookupHook,0x5ef4430,kSkillLookupHead,sizeof(kSkillLookupHead),(void*)SkillLookup},
        {&gBackdropCoordinatesHook,0x5b8ac50,kBackdropCoordinatesHead,sizeof(kBackdropCoordinatesHead),(void*)BackdropCoordinates},
        {&gMoveHook,0x5e7b330,kMoveHead,sizeof(kMoveHead),(void*)Move},
        {&gGoalXHook,0x5e7aba0,kGoalXHead,sizeof(kGoalXHead),(void*)GoalX},
        {&gGoalYHook,0x5e7abb0,kGoalYHead,sizeof(kGoalYHead),(void*)GoalY},
        {&gValueHook,0x5e65100,kValueHead,sizeof(kValueHead),(void*)Value},
        {&gAwakeningHook,0x2ed1f00,kAwakeningHead,sizeof(kAwakeningHead),(void*)Awakening},
        {&gTickHook,0x5c72ab0,kTickHead,sizeof(kTickHead),(void*)Tick},
        {&gPartyBuffHook,0x2e60f00,kPartyBuffHead,sizeof(kPartyBuffHead),(void*)PartyBuff},
        {&gChargingHook,0x2e60880,kChargingHead,sizeof(kChargingHead),(void*)Charging},
        {&gHasDelayHook,0x2e60b20,kHasDelayHead,sizeof(kHasDelayHead),(void*)HasDelay},
        {&gDelayHook,0x2e5f550,kDelayHead,sizeof(kDelayHead),(void*)Delay},
        {&gHasReuseHook,0x2e61050,kHasReuseHead,sizeof(kHasReuseHead),(void*)HasReuse},
        {&gReuseHook,0x2e601f0,kReuseHead,sizeof(kReuseHead),(void*)Reuse},
        {&gHasJumpHook,0x2e609d0,kHasJumpHead,sizeof(kHasJumpHead),(void*)HasJump},
        {&gJumpHook,0x2e5ead0,kJumpHead,sizeof(kJumpHead),(void*)Jump},
        {&gCharacterHook,0x5c6dc10,kCharacterHead,sizeof(kCharacterHead),(void*)Character},
        {&gBaseStatsHook,0x5c6b610,kBaseStatsHead,sizeof(kBaseStatsHead),(void*)BaseStats},
        {&gResetStatsHook,0x5c11d20,kResetStatsHead,sizeof(kResetStatsHead),(void*)ResetStats},
        {&gRecalculateHook,0x5cce150,kRecalculateHead,sizeof(kRecalculateHead),(void*)Recalculate},
        {&gPetPreloadCall1Hook,0x5e47a7b,kPetPreloadCall1Head,sizeof(kPetPreloadCall1Head),(void*)PetPreload,true},
        {&gPetPreloadCall2Hook,0x5e47e9b,kPetPreloadCall2Head,sizeof(kPetPreloadCall2Head),(void*)PetPreload,true},
        {&gPartyHudHook,0x5553590,kPartyHudHead,sizeof(kPartyHudHead),(void*)PartyHud},
        {&gPartyHudCompactHook,0x5542930,kPartyHudCompactHead,sizeof(kPartyHudCompactHead),(void*)PartyHudCompact},
        {&gCoinDrawHook,0x5552be0,kCoinDrawHead,sizeof(kCoinDrawHead),(void*)CoinDraw}
    };
    for(auto& e:entries)if(!PrepareHook(*e.hook,(void*)(gBase+e.rva),e.bytes,e.size,e.target,e.call?1:0,e.call?5:0)) {
        fprintf(gLog,"HOOK_PREPARE_FAILED rva=%llx bytes=%zu call=%u error=%lu\n",(unsigned long long)e.rva,e.size,e.call,GetLastError());fflush(gLog);return 4;
    }
    gCommonDestroy=(FrameFn)gCommonDestroyHook.original();
    gFrame=(FrameFn)gFrameHook.original();gDestroy=(FrameFn)gDestroyHook.original();
    for(unsigned i=0;i<6;i++)gClassDestroy[i]=(FrameFn)gClassDestroyHook[i].original();
    gAi=(PredFn)gAiHook.original();gLocal=(PredFn)gLocalHook.original();gAttack=(PredFn)gAttackHook.original();
    gRange=(RangeFn)gRangeHook.original();
    gAttackRange=(AttackRangeFn)gAttackRangeHook.original();
    gSkill=(SkillFn)gSkillHook.original();
    gSkillLookup=(SkillLookupFn)gSkillLookupHook.original();
    gBackdropCoordinates=(CoordinatesFn)gBackdropCoordinatesHook.original();
    gGoalX=(GoalFn)gGoalXHook.original();gGoalY=(GoalFn)gGoalYHook.original();
    gMove=(MoveFn)gMoveHook.original();gValue=(ValueFn)gValueHook.original();
    gAwakening=(PredFn)gAwakeningHook.original();gTick=(FrameFn)gTickHook.original();
    gPartyBuff=(PolicyFn)gPartyBuffHook.original();
    gCharging=(PolicyFn)gChargingHook.original();
    gHasDelay=(PolicyFn)gHasDelayHook.original();
    gDelay=(PolicyFn)gDelayHook.original();
    gHasReuse=(PolicyFn)gHasReuseHook.original();
    gReuse=(PolicyFn)gReuseHook.original();
    gHasJump=(PolicyFn)gHasJumpHook.original();gJump=(PolicyFn)gJumpHook.original();
    gCharacter=(PredFn)gCharacterHook.original();
    gBaseStats=(StatsFn)gBaseStatsHook.original();gResetStats=(StatsFn)gResetStatsHook.original();
    gRecalculate=(RecalculateFn)gRecalculateHook.original();// The existing Elite plugin owns this function entry. Call through it so
    // its scope-specific protection remains intact; never patch or bypass it.
    gPetPreload=(PetPreloadFn)(gBase+0x5e47f00);
    gPartyHud=(FrameFn)gPartyHudHook.original();gPartyHudCompact=(FrameFn)gPartyHudCompactHook.original();
    gCoinDraw=(CoinDrawFn)gCoinDrawHook.original();
    for(auto& e:entries)if(!PublishHook(*e.hook,e.call)) {
        fprintf(gLog,"HOOK_PUBLISH_FAILED rva=%llx call=%u error=%lu\n",(unsigned long long)e.rva,e.call,GetLastError());fflush(gLog);return 5;
    }
    HANDLE worker=CreateThread(nullptr,0,IpcWorker,nullptr,0,nullptr);if(!worker)return 6;CloseHandle(worker);
    InterlockedExchange(&gEnabled,1);
    fputs("READY r34-render2 explicit prepare/drive; seventeen ordinary player classes; native movement/attack/skills; manual acceptance pending\n",gLog);fflush(gLog);
    return 0;
}
BOOL WINAPI DllMain(HINSTANCE self,DWORD reason,LPVOID) {
    if(reason==DLL_PROCESS_ATTACH){gSelf=self;DisableThreadLibraryCalls(self);}return TRUE;
}
