#define ROBOT_NATIVE_TEST
#include "plugin.cpp"
#include <assert.h>
#include <vector>
static unsigned char actorBytes[0x13000]={},ownerBytes[0x13000]={},stateBytes[0x500]={},refsBytes[16]={},worldBytes[256]={};
static unsigned char layers[6][0x8b8]={},actorVT[0x1300]={};
static uintptr_t actor=(uintptr_t)actorBytes,owner=(uintptr_t)ownerBytes,state=(uintptr_t)stateBytes;
static uintptr_t currentState=128,partyID=65534;
static int originalCalls=0,centreCalls=0;static bool wrongMap=false;
static uintptr_t __fastcall Job(uintptr_t){return 14;}
static uintptr_t __fastcall ObjectMap(uintptr_t o){return wrongMap&&o==(uintptr_t)layers[0]?0x88888:0x99999;}
static void put(uintptr_t p,size_t offset,uintptr_t value){memcpy((void*)(p+offset),&value,8);}
uintptr_t TestNativeCall(uintptr_t rva,const uintptr_t* args,size_t n) {
    switch(rva) {
    case 0x5790070:assert(n==1&&args[0]==actor);return currentState;
    case 0x5ce5810:assert(n==2&&args[0]==actor&&args[1]==128);return state;
    case 0x59a90f0:return 3;
    case 0x59a9d30:return 0;
    case 0x59a9de0:assert(n==2);return args[1]==0?owner:args[1]==1?actor:0;
    case 0x5b8b0a0:return 0x50000u|(args[0]==owner?2:partyID);
    case 0x5efafb0:return owner;
    case 0x59d7c00:assert(n==1);assert(args[0]==2||args[0]==7);return args[0]==2?1:6;
    case 0xeede60:assert(n==6&&args[2]==(uintptr_t)(*(int*)(gBase+0xdc63f70)==2?1:6)&&args[3]==1&&args[4]==0&&args[5]==0);centreCalls++;return args[1]+(args[2]==1?20:200);
    case 0xeedf70:assert(n==7&&args[2]==0&&args[3]==(uintptr_t)(*(int*)(gBase+0xdc63f70)==2?1:6)&&args[4]==1&&args[5]==0&&args[6]==0);centreCalls++;return args[1]+(args[3]==1?10:100);
    case 0xdcfe20:assert(n==1&&(args[0]==0x777||args[0]==0));return args[0]?11:0;
    case 0x2ad8420:assert(n==1&&(args[0]==0x777||args[0]==0));return args[0]?uintptr_t(-9):0;
    default:assert(false);return 0;
    }
}
static int* __fastcall Original(uintptr_t,int* xy){originalCalls++;xy[0]=-100;xy[1]=250;return xy;}
static void pass(uintptr_t obj){int xy[2]={};int n=centreCalls;assert(BackdropCoordinates(obj,xy)==xy);assert(xy[0]==-100&&xy[1]==250&&centreCalls==n);}
int main(){
    gBase=(uintptr_t)VirtualAlloc(nullptr,0x10000000,MEM_RESERVE,PAGE_NOACCESS);assert(gBase);
    for(uintptr_t rva:{0xe66c090ULL,0xe6839f0ULL,0xe683c20ULL,0xe634238ULL,0xdc6d8f8ULL,0xdc63f70ULL,0x9c4f688ULL})
        assert(VirtualAlloc((void*)((gBase+rva)&~4095ULL),4096,MEM_COMMIT,PAGE_READWRITE));
    put(gBase,0xe66c090,0xabc);put(gBase,0xe6839f0,(uintptr_t)worldBytes);put(gBase,0xe683c20,0xdef);
    put(gBase,0xe634238,0xaaa);put(gBase+0x9c4f688,0x130,(uintptr_t)ObjectMap);
    put((uintptr_t)actorVT,0x130,(uintptr_t)ObjectMap);put((uintptr_t)actorVT,0x12e0,(uintptr_t)Job);
    put(actor,0,(uintptr_t)actorVT);put(owner,0,(uintptr_t)actorVT);
    put(state,0,gBase+0x9a46f30);*(int*)(state+0x90)=121;put(state,0xa0,(uintptr_t)refsBytes);put(state,0xa8,actor+0x30);*(int*)(refsBytes+8)=1;
    for(unsigned i=0;i<6;i++){put((uintptr_t)layers[i],0,gBase+0x9c4f688);put((uintptr_t)layers[i],0x190,(uintptr_t)refsBytes);put((uintptr_t)layers[i],0x198,0x777);}
    for(unsigned i=0;i<5;i++)put(state,0x3e8+i*8,(uintptr_t)layers[i]);
    gEnabled=1;gGameThread=GetCurrentThreadId();gBackdropCoordinates=Original;
    auto& a=gOffer;memcpy(a.party.magic,"TRAUTH16",8);a.party.version=1;a.party.serverPID=123;
    a.party.issued=Now();a.party.epoch=2;a.party.active=1;a.party.owner=2;a.party.count=1;
    a.party.bots[1]=65534;a.party.footer=0x3130763631524154ull;a.generation=17;a.phase=2;
    gRooms[0]=a;auto& r=gRobots[0];r.active=r.ready=true;r.actor=actor;
    std::vector<unsigned char> stateBefore(stateBytes,stateBytes+sizeof(stateBytes)),objectsBefore((unsigned char*)layers,(unsigned char*)layers+sizeof(layers));
    for(auto size: {std::pair<int,int>(1280,720),{1700,1062},{1920,1080},{2560,1440}}){
        *(int*)(gBase+0xdc6d8f8)=size.first;*(int*)(gBase+0xdc6d8fc)=size.second;
        for(unsigned i=0;i<5;i++){*(int*)(gBase+0xdc63f70)=i==0?7:2;int xy[2]={};assert(BackdropCoordinates((uintptr_t)layers[i],xy)==xy);assert(xy[0]==size.first/2+(i==0?211:31)&&xy[1]==size.second/2+(i==0?91:1));}
    }
    pass((uintptr_t)layers[5]);pass(actor);pass(0);
    currentState=0;pass((uintptr_t)layers[0]);currentState=128;
    *(int*)(state+0x90)=120;pass((uintptr_t)layers[0]);*(int*)(state+0x90)=121;
    *(int*)(refsBytes+8)=0;pass((uintptr_t)layers[0]);*(int*)(refsBytes+8)=1;
    put(state,0xa8,owner+0x30);pass((uintptr_t)layers[0]);put(state,0xa8,actor+0x30);
    wrongMap=true;pass((uintptr_t)layers[0]);wrongMap=false;
    partyID=65533;pass((uintptr_t)layers[0]);partyID=65534;
    gRooms[0].generation++;pass((uintptr_t)layers[0]);gRooms[0]=a;
    gOffer.phase=3;pass((uintptr_t)layers[0]);gOffer.phase=2;
    gOffer.party.issued=0;pass((uintptr_t)layers[0]);gOffer.party.issued=Now();
    gEnabled=0;pass((uintptr_t)layers[0]);gEnabled=1;
    gGameThread++;pass((uintptr_t)layers[0]);gGameThread=GetCurrentThreadId();
    *(int*)(gBase+0xdc6d8f8)=0;pass((uintptr_t)layers[0]);
    assert(!memcmp(stateBefore.data(),stateBytes,sizeof(stateBytes))&&!memcmp(objectsBefore.data(),layers,sizeof(layers)));
    assert(centreCalls==40&&originalCalls>=35);
    puts("PASS 5 visual-only layers / 4 viewports; native transform and animation offsets; owner/current state/map/party/generation/TTL/thread gates; foreign pass-through; no entity/flag/timer writes");
}
