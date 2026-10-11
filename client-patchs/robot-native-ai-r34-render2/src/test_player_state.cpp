#define ROBOT_NATIVE_TEST
#include <initializer_list>
#include "plugin.cpp"
#include <assert.h>
#include "actor_classes.h"
static constexpr uintptr_t bot=0x123400,other=0x567800;
static uintptr_t fakeActor,fakeToken;
static int coins=147,draws=0,blockedGate=0,visual=1;static bool nativeDraw=false,reordered=false;
static int destroyCalls=0;
static void __fastcall DestroyProbe(uintptr_t actor){assert(actor==0x999999);destroyCalls++;}
static int tickCalls=0,actorState=0,enemyPresent=0,inputCalls=0,lastGoal=0;
static int preloads=0;static bool sourcePresent=false;
uintptr_t TestNativeCall(uintptr_t rva,const uintptr_t* a,size_t n){
    switch(rva){
    case 0x5790070:return actorState;
    case 0x5e6eb00:return enemyPresent;
    case 0x1b07610:case 0x5922690:inputCalls++;return 0;
    case 0x556cdf0:return reordered;
    case 0x217d5b0:return 0;
    case 0x217d600:return (int)a[1]==visual?1:0xffffffff;
    case 0x21be90:return sourcePresent?0x445500:0;
    case 0x59a90f0:return 3;
    case 0x5f12d90:return 0;
    case 0x5f13700:return a[1]==1?0x123:0;
    case 0x5effac0:return fakeActor;
    case 0x5f01210:return 65534;
    case 0x667c460:return 0x124;
    case 0x5ad7f80:return fakeToken;
    case 0x5595e50:return blockedGate==1;
    case 0x5596120:return blockedGate==2;
    case 0x5595f90:return blockedGate==3;
    case 0x5596080:return blockedGate==4;
    case 0x5595de0:return blockedGate==5;
    case 0x5596340:return blockedGate==5;
    default:printf("unexpected native call %llx\n",(unsigned long long)rva);fflush(stdout);assert(false);return 0;
    }
}
static uintptr_t __fastcall CoinCount(uintptr_t){return coins;}
static void __fastcall DrawProbe(uintptr_t,unsigned seat,int shown){assert(seat==1&&shown==visual);draws++;}
static void __fastcall HudProbe(uintptr_t hud){if(nativeDraw)CoinDraw(hud,1,1);}
static void __fastcall PreloadProbe(uintptr_t,uintptr_t){preloads++;}
static void TestReviveHud(){
    auto globals=(unsigned char*)VirtualAlloc(nullptr,0x90000,MEM_COMMIT|MEM_RESERVE,PAGE_READWRITE);
    auto hud=(unsigned char*)VirtualAlloc(nullptr,0x4000,MEM_COMMIT|MEM_RESERVE,PAGE_READWRITE);
    auto actor=(unsigned char*)VirtualAlloc(nullptr,0x20000,MEM_COMMIT|MEM_RESERVE,PAGE_READWRITE);
    assert(globals&&hud&&actor);gBase=(uintptr_t)globals-0xe600000;fakeActor=(uintptr_t)actor;
    uintptr_t table[32]={};table[0xe0/8]=(uintptr_t)CoinCount;uintptr_t token=(uintptr_t)table;fakeToken=(uintptr_t)&token;
    for(auto off:{0xe66c090,0xe683c78,0xe683c20,0xe683c80})*(uintptr_t*)(gBase+off)=0x111;
    for(auto off:{8,0x4d0,0x870,0x658})*(uintptr_t*)(hud+off)=0x222;
    gRobots[0].actor=fakeActor;gRobots[0].lifeDead=true;gRobots[0].deathSequence=1;actor[0x11502]=1;
    gOffer.phase=2;gOffer.party.issued=Now();gRooms[0]=gOffer;gCoinDraw=DrawProbe;gLog=tmpfile();assert(gLog);
    for(int gate=0;gate<=5;gate++){blockedGate=gate;draws=0;RenderPartyHud((uintptr_t)hud,HudProbe);assert(draws==(gate?0:1));}
    blockedGate=0;coins=0;draws=0;RenderPartyHud((uintptr_t)hud,HudProbe);assert(draws==0);
    coins=147;nativeDraw=true;RenderPartyHud((uintptr_t)hud,HudProbe);assert(draws==1);nativeDraw=false;
    reordered=true;visual=2;*(uintptr_t*)(gBase+0xe638ab0)=0x333;*(uintptr_t*)(hud+0x668)=0x222;
    draws=0;RenderPartyHud((uintptr_t)hud,HudProbe);assert(draws==1);
    visual=5;draws=0;RenderPartyHud((uintptr_t)hud,HudProbe);assert(draws==0);reordered=false;visual=1;
    actor[0x11502]=0;draws=0;RenderPartyHud((uintptr_t)hud,HudProbe);assert(draws==0);
    actor[0x11502]=1;gOffer.phase=3;RenderPartyHud((uintptr_t)hud,HudProbe);assert(draws==0);
    assert(gDrawingPartyHud==0&&gCoinDrawMask==0);
    gPetPreload=PreloadProbe;PetPreload(0x555,490100178);assert(preloads==0);
    sourcePresent=true;PetPreload(0x555,490100178);assert(preloads==1);
    sourcePresent=false;PetPreload(0x555,123);assert(preloads==2);
    fclose(gLog);gLog=nullptr;
    VirtualFree(globals,0,MEM_RELEASE);VirtualFree(hud,0,MEM_RELEASE);VirtualFree(actor,0,MEM_RELEASE);
}
static uintptr_t __fastcall MapProbe(uintptr_t){return 0x112233;}
static void __fastcall GoalProbe(uintptr_t,int v){lastGoal=v;}
static void __fastcall TickProbe(uintptr_t){tickCalls++;}
static void TestNaturalBehavior(){
 auto savedOffer=gOffer;
 uintptr_t table[48]={};table[0x130/8]=(uintptr_t)MapProbe;uintptr_t actor=(uintptr_t)table;
 gGoalX=gGoalY=GoalProbe;gTick=TickProbe;gLog=tmpfile();assert(gLog);
 gOffer.phase=2;gOffer.party.count=3;for(unsigned i=0;i<3;i++)gOffer.party.bots[i+1]=65534-i;
 unsigned priorBrain=0,priorAwake=0;
 for(unsigned seat=0;seat<3;seat++){
  auto& r=gRobots[seat];r={};r.actor=(uintptr_t)&actor;r.owner=(uintptr_t)&actor;r.ai=0x1000+seat;r.active=true;
  gOffer.party.bots[seat+1]=65534-seat;gRooms[seat]=gOffer;
  unsigned brain=InitialBrainDelay(seat,gOffer.party.bots[seat+1]),awake=InitialAwakeningDelay(seat,gOffer.party.bots[seat+1]);
  assert(brain>priorBrain&&awake>priorAwake);priorBrain=brain;priorAwake=awake;
  enemyPresent=0;GoalX(r.ai,300);int x=lastGoal;GoalY(r.ai,200);int y=lastGoal;
  if(seat==0)assert(x<300&&y<200);if(seat==1)assert(x==300&&y>200);if(seat==2)assert(x>300&&y==200);
  enemyPresent=1;GoalX(r.ai,300);assert(lastGoal>=282&&lastGoal<=318);GoalY(r.ai,200);assert(lastGoal>=192&&lastGoal<=208);
  r.lifeDead=true;GoalX(r.ai,300);assert(lastGoal==300);r.lifeDead=false;
  gOffer.phase=3;GoalX(r.ai,300);assert(lastGoal==300);gOffer.phase=2;
  r.driveAfter=Now()+10000;actorState=0;int calls=tickCalls;Tick(r.actor);assert(tickCalls==calls);
  actorState=5;Tick(r.actor);assert(r.deadInput&&tickCalls==calls);actorState=0;
  r.driveAfter=0;Tick(r.actor);assert(!r.deadInput&&tickCalls==calls+1);
  r.awakeningAfter=Now()+10000;assert(Awakening(r.ai)==0);
  r.active=false;
 }
 GoalX(other,300);assert(lastGoal==300);int calls=tickCalls;Tick(other);assert(tickCalls==calls+1);
 assert(OffsetGoal(INT_MIN,0,false,false)==INT_MIN&&OffsetGoal(INT_MAX,2,false,false)==INT_MAX);
 for(auto& r:gRobots)r={};gOffer=savedOffer;gOffer.party.count=1;for(unsigned i=2;i<8;i++)gOffer.party.bots[i]=0;gRooms[0]=gOffer;gRobots[0].active=true;gRobots[0].actor=bot;
 fclose(gLog);gLog=nullptr;enemyPresent=0;actorState=0;
 puts("PASS independent native goals/brain/awakening; dead/halt/foreign pass-through; no actor position writes");
}
static uintptr_t __fastcall Ordinary(uintptr_t){return 0;}
static void __fastcall RecalcProbe(uintptr_t actor,unsigned char mode){assert(actor==bot&&mode==7);assert(Ai(bot)==0);}
static void __fastcall ResetProbe(uintptr_t actor,int mode){assert(actor==bot&&mode==9);assert(Ai(bot)==0);Recalculate(bot,7);assert(Ai(bot)==0);}
static void __fastcall BaseProbe(uintptr_t actor,int mode){
    assert(mode==42);assert(Ai(actor)==0);assert(Ai(other)==0);
    if(actor==bot){ResetStats(bot,9);assert(Ai(bot)==0);}
}
int main(){
 for(unsigned i=0;i<17;i++) {assert(RobotActorClassIndex(0x140000000ULL,0x140000000ULL+kRobotActorClasses[i].vtable)==(int)i);assert(kRobotActorClasses[i].size>=0x18e20);}
 assert(RobotActorClassIndex(0x140000000ULL,0)==-1);
 assert(RobotActorClassIndex(0x140000000ULL,0x140000000ULL+0xad21371)==-1);
    gEnabled=1;gGameThread=GetCurrentThreadId();gAi=Ordinary;
    auto& a=gOffer;memcpy(a.party.magic,"TRAUTH16",8);a.party.version=1;a.party.serverPID=123;
    a.party.issued=Now();a.party.epoch=2;a.party.active=1;a.party.owner=2;a.party.count=1;
    a.party.bots[1]=65534;a.party.footer=0x3130763631524154ull;a.generation=17;a.phase=2;
    gRooms[0]=a;gRobots[0].active=true;gRobots[0].actor=bot;
    gBaseStats=BaseProbe;gResetStats=ResetProbe;gRecalculate=RecalcProbe;
    assert(Ai(bot)==1);BaseStats(bot,42);assert(Ai(bot)==1&&gOrdinaryStatsActor==0);
    BaseStats(other,42);assert(Ai(bot)==1&&gOrdinaryStatsActor==0);
    {OrdinaryStatsScope outer(bot);{OrdinaryStatsScope inner(other);assert(Ai(bot)==1);}assert(Ai(bot)==0);}
    assert(Ai(bot)==1);gOffer.phase=3;assert(Ai(bot)==0);
    TestNaturalBehavior();
    TestReviveHud();
    for(auto& r:gRobots)r={};
    for(auto& callback:gClassDestroy)callback=DestroyProbe;
    ClassDestroy<0>(0x999999);ClassDestroy<1>(0x999999);ClassDestroy<2>(0x999999);
    ClassDestroy<3>(0x999999);ClassDestroy<4>(0x999999);ClassDestroy<5>(0x999999);assert(destroyCalls==6);gCommonDestroy=DestroyProbe;CommonDestroy(0x999999);assert(destroyCalls==7);
    puts("PASS compiled stat scope; revive HUD restrictions/deduplication/living/halt; optional pet preload missing/present/unrelated");
}
