#include "plugin.cpp"
#include <assert.h>
#include <limits>
static int gMaxDepth=0,gMaxMenuDepth=0,gUnitCalls=0,gMakeCalls=0;
static void* gOwner;
static bool gKeyRaise=false;
static void __fastcall SpyKey(void* actor){
    assert(actor==gOwner&&gKeyDepth>=1);
    if(gKeyRaise)RaiseException(0xe0424242,0,0,nullptr);
    RouteReadyNode(actor);
}
static void KeyException(void* actor){
    __try{KeyScope(actor,(uintptr_t)&SpyKey);assert(false);}
    __except(GetExceptionCode()==0xe0424242?EXCEPTION_EXECUTE_HANDLER:EXCEPTION_CONTINUE_SEARCH){}
    assert(gKeyDepth==0);
}
static void Jump(void* at,uintptr_t to){unsigned char code[14]={0xff,0x25};memcpy(code+6,&to,8);memcpy(at,code,14);}
static void __fastcall SpyActor(void* self,unsigned char visible,int x,int y,unsigned char flag){
    assert(visible==1&&x==123&&y==456);gMaxDepth=(int)(gDepth>gMaxDepth?gDepth:gMaxDepth);
    if(flag==1)Overhead(self,visible,x,y,0);
    if(flag==2)RaiseException(0xe0424242,0,0,nullptr);
}
static void ExceptionScope(void* self){
    __try{Overhead(self,1,123,456,2);assert(false);}
    __except(GetExceptionCode()==0xe0424242?EXCEPTION_EXECUTE_HANDLER:EXCEPTION_CONTINUE_SEARCH){}
    assert(gDepth==0);
}
static void __fastcall SpyUnit(void* unit,int category){++gUnitCalls;*(int*)((unsigned char*)unit+0x30)=category;}
static void* __fastcall NativeConstructor(void* owner,void* cache){
    assert(owner==gOwner);++gMakeCalls;
    auto node=(unsigned char*)cache;
    *(int*)(node+0x10)=2;
    auto unit=*(unsigned char**)(node+8);
    assert(unit);*(int*)(unit+0x30)=2;
    return node;
}
static void __fastcall SpyMenu(void* window){
    gMaxMenuDepth=(int)(gMenuDepth>gMaxMenuDepth?gMenuDepth:gMaxMenuDepth);
    auto state=(int*)((unsigned char*)window+8);
    if(*state==1){*state=0;MenuWindow(window);*state=1;}
    if(*state==2)RaiseException(0xe0424242,0,0,nullptr);
}
static void MenuException(void* window){
    __try{MenuWindow(window);assert(false);}
    __except(GetExceptionCode()==0xe0424242?EXCEPTION_EXECUTE_HANDLER:EXCEPTION_CONTINUE_SEARCH){}
    assert(gMenuDepth==0);
}
static int __fastcall Replacement(){return 22;}
static void PatchRoundtrip(){
    auto mem=(unsigned char*)VirtualAlloc(nullptr,4096,MEM_COMMIT|MEM_RESERVE,PAGE_EXECUTE_READWRITE);assert(mem);
    unsigned char original[5]={0xe8};int32_t d=64-5;memcpy(original+1,&d,4);
    memcpy(mem,original,5);mem[5]=0xc3;
    const unsigned char eleven[]={0xb8,11,0,0,0,0xc3};memcpy(mem+64,eleven,sizeof(eleven));
    auto run=(int(*)())mem;assert(run()==11);
    headtext::CallHook hook;
    unsigned char wrong[5]{};memcpy(wrong,original,5);wrong[4]^=1;
    assert(!hook.Prepare((uintptr_t)mem,wrong,5,(uintptr_t)&Replacement));
    assert(run()==11&&!memcmp(mem,original,5));
    assert(hook.Prepare((uintptr_t)mem,original,5,(uintptr_t)&Replacement));
    assert(hook.Install());assert(run()==22);assert(hook.Restore());
    assert(run()==11&&!memcmp(mem,original,5));hook.FreeUnused();
    const unsigned char indirect[]={0xff,0x90,0xa0,6,0,0};memcpy(mem,indirect,6);
    headtext::CallHook six;
    assert(six.Prepare((uintptr_t)mem,indirect,6,(uintptr_t)&Replacement));
    assert(six.Install()&&mem[5]==0x90);assert(six.Restore());assert(!memcmp(mem,indirect,6));six.FreeUnused();
    uintptr_t slot=(uintptr_t)&SpyMenu;gMenuSlot=(uintptr_t)&slot;gMenuExpected=slot;
    assert(MenuInstall()&&slot==(uintptr_t)&MenuWindow);assert(MenuRestore()&&slot==gMenuExpected);
    slot=0x1234;assert(!MenuInstall()&&slot==0x1234);
    VirtualFree(mem,0,MEM_RELEASE);
}
static void WrapperRoutes(){
    auto image=(unsigned char*)VirtualAlloc(nullptr,0xf1c2000,MEM_RESERVE,PAGE_NOACCESS);assert(image);
    gBase=(uintptr_t)image;
    assert(VirtualAlloc(image+0xf1c0000,0x2000,MEM_COMMIT,PAGE_READWRITE));
    assert(VirtualAlloc(image+0x6f3b000,0x1000,MEM_COMMIT,PAGE_EXECUTE_READWRITE));
    assert(VirtualAlloc(image+0x6fab000,0x1000,MEM_COMMIT,PAGE_EXECUTE_READWRITE));
    assert(VirtualAlloc(image+0x5cca000,0x1000,MEM_COMMIT,PAGE_EXECUTE_READWRITE));
    assert(VirtualAlloc(image+0x5cc5000,0x1000,MEM_COMMIT,PAGE_EXECUTE_READWRITE));
    Jump(image+0x5cca7a0,(uintptr_t)&SpyKey);
    Jump(image+0x5cc52a0,(uintptr_t)&SpyKey);
    // Execute the exact PE native setter including its unit vtable tailcall.
    const unsigned char setter[]={0x39,0x51,0x10,0x74,0x13,0x89,0x51,0x10,0x48,0x8b,0x49,0x08,
        0x48,0x85,0xc9,0x74,0x07,0x48,0x8b,0x01,0xff,0x60,0x28,0xc3};
    memcpy(image+0x6f3b760,setter,sizeof(setter));
    Jump(image+0x6fab490,(uintptr_t)&NativeConstructor);
    alignas(16) unsigned char root[0x1600]{},style[0x78]{},font[0xb0]{},node[0xc0]{},unit[0x80]{};
    uintptr_t unitTable[6]{};unitTable[5]=(uintptr_t)&SpyUnit;*(uintptr_t*)unit=(uintptr_t)unitTable;
    *(uintptr_t*)(node+8)=(uintptr_t)unit;
    alignas(16) unsigned char wrappers[9][0x80]{},views[9][0x48]{},handles[9][0x20]{},native[9][0x40]{};
    *(uintptr_t*)(image+0xf1c0ca0)=(uintptr_t)root;
    *(uintptr_t*)(root+0x148)=(uintptr_t)style;*(uintptr_t*)(style+0x10)=(uintptr_t)font;
    *(uintptr_t*)font=gBase+0xb1f8498;root[0x15c0]=1;
    for(int i=1;i<=8;i++){
        *(uintptr_t*)(root+0xbc0+i*8)=(uintptr_t)wrappers[i];
        *(uintptr_t*)(wrappers[i]+0x50)=(uintptr_t)views[i];
        *(uintptr_t*)(views[i]+0x28)=(uintptr_t)handles[i];
        *(uintptr_t*)(handles[i]+0x18)=(uintptr_t)native[i];
        *(uintptr_t*)native[i]=gBase+0xb686060;
        *(float*)(native[i]+0x10)=960;*(float*)(native[i]+0x14)=600;
        *(uintptr_t*)(native[i]+0x20)=0x12340000;
    }
    gEnabled=1;gDepth=1;gOwner=style;
    unsigned char retained[sizeof(node)];memcpy(retained,node,sizeof(node));
    assert(FontNodeReady(style,node)==node&&*(int*)(node+0x10)==8&&*(int*)(unit+0x30)==8);
    assert(FontNodeReady(style,node)==node&&gUnitCalls==2&&gMakeCalls==2&&*(int*)(unit+0x30)==8);
    // All non-category node bytes (text/position/font/style) retained.
    memcpy(retained+0x10,node+0x10,4);assert(!memcmp(retained,node,sizeof(node)));
    // A tutorial key stays on its original layer inside overhead scope;
    // the next ordinary head/menu text still routes normally on cached reuse.
    FontNodeReady(style,node);*(int*)(node+0x10)=2;*(int*)(unit+0x30)=2;
    gOwner=node;KeyScope(node,(uintptr_t)&SpyKey);
    assert(gKeyDepth==0&&*(int*)(node+0x10)==2&&*(int*)(unit+0x30)==2);
    TutorialKeys(node);TutorialTargetKeys(node);
    assert(gKeyDepth==0&&*(int*)(node+0x10)==2&&*(int*)(unit+0x30)==2);
    gKeyRaise=true;KeyException(node);gKeyRaise=false;gOwner=style;
    FontNodeReady(style,node);assert(*(int*)(node+0x10)==8&&*(int*)(unit+0x30)==8);
    root[0x15c0]=0;FontNodeReady(style,node);assert(*(int*)(node+0x10)==2&&*(int*)(unit+0x30)==2);root[0x15c0]=1;
    gDepth=0;FontNodeReady(style,node);assert(*(int*)(node+0x10)==2);
    gMenuDepth=1;FontNodeReady(style,node);assert(*(int*)(node+0x10)==8);gMenuDepth=0;gDepth=1;
    *(float*)(native[8]+0x10)=961;FontNodeReady(style,node);assert(*(int*)(node+0x10)==2);*(float*)(native[8]+0x10)=960;
    *(uintptr_t*)(native[8]+0x20)=0x12350000;FontNodeReady(style,node);assert(*(int*)(node+0x10)==2);*(uintptr_t*)(native[8]+0x20)=0x12340000;
    *(float*)(native[8]+0x10)=std::numeric_limits<float>::quiet_NaN();FontNodeReady(style,node);assert(*(int*)(node+0x10)==2);*(float*)(native[8]+0x10)=960;
    *(uintptr_t*)font=0;FontNodeReady(style,node);assert(*(int*)(node+0x10)==2);*(uintptr_t*)font=gBase+0xb1f8498;
    for(int category=1;category<=11;category++){
        *(int*)(node+0x10)=category;*(int*)(unit+0x30)=category;
        RouteReadyNode(node);assert(*(int*)(node+0x10)==(category<=7?8:category));
    }
    *(uintptr_t*)(node+8)=0;*(int*)(node+0x10)=2;RouteReadyNode(node);assert(*(int*)(node+0x10)==2);
    uintptr_t window[2]={gBase+0x977ef80,1};gMenuOriginal=&SpyMenu;
    MenuWindow(window);assert(gMenuDepth==0&&gMaxMenuDepth==2);
    window[1]=2;MenuException(window);
    window[0]=0;window[1]=0;gMaxMenuDepth=0;MenuWindow(window);assert(gMaxMenuDepth==0);
    gDepth=0;gEnabled=0;VirtualFree(image,0,MEM_RELEASE);gBase=0;
}
int main(){
    uintptr_t table[0x6a8/8]{};table[0x6a0/8]=(uintptr_t)&SpyActor;
    uintptr_t actor=(uintptr_t)table;
    Overhead(&actor,1,123,456,1);assert(gDepth==0&&gMaxDepth==2);
    ExceptionScope(&actor);PatchRoundtrip();WrapperRoutes();
    puts("PASS r3 tutorial-key scope retention and exception recovery: first/cached unit category, exact native setter, head/menu ABI and nested/exception scopes, OFF/unrelated/overlay/dimension/font gates, pointer/CALL rollback");
}
