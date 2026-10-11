#include "shop115.cpp"
#include <cassert>
static int strongDestroyed,weakDestroyed;
static void Strong(void*){strongDestroyed++;}
static void Weak(void*){weakDestroyed++;}
static uint32_t battleBase[8],battleCover[8];
static uintptr_t __fastcall BaseItem(void*,int slot){return battleBase[slot]?(uintptr_t)&battleBase[slot]:0;}
static uintptr_t __fastcall CoverItem(void*,int slot){return battleCover[slot]?(uintptr_t)&battleCover[slot]:0;}
static bool PlainItemId(uintptr_t item,uint32_t& id){return Read(item,id)&&id>0;}
static beastcontrol::Pair refreshPair{};
static unsigned refreshMotions;
static bool refreshExpectedMounted;
static void __fastcall RefreshMotion(void* actor,void* value){
 uintptr_t position=0;assert(Read((uintptr_t)actor+0x170,position));
 assert(*(unsigned char*)(position+0x138)==0); // diagnostics must not mutate position
 assert(beastcontrol::mounted==refreshExpectedMounted);
 ++refreshMotions;beastcontrol::Release(*(beastcontrol::Pair*)value);
}
static beastcontrol::Pair* __fastcall RefreshNormal(void*,beastcontrol::Pair* value,int){
 *value=refreshPair;InterlockedIncrement((volatile LONG*)(value->control+8));return value;
}
static int __fastcall RefreshHeight(void* actor){return *(short*)((uintptr_t)actor+0x2d4);}
static unsigned vectorCoordinateCalls,bitmapCoordinateCalls;
static const wchar_t coordinateText[]=L"coordinate-native-test";
static void __fastcall NativeVectorCoordinates(void* node,void* style,void* font,const wchar_t* text,float* x,float* y) {
 assert(node==(void*)0x12340&&style==(void*)0x23450&&font==(void*)0x34560&&text==coordinateText);
 assert(*x==25.5f&&*y==40.25f);*x=30.75f;*y=60.5f;++vectorCoordinateCalls;
}
static void __fastcall NativeBitmapCoordinates(void* node,int* x,int* y,const wchar_t* text,void* style) {
 assert(node==(void*)0x12340&&style==(void*)0x23450&&text==coordinateText);
 assert(*x==25&&*y==40);*x=30;*y=60;++bitmapCoordinateCalls;
}
static void TestNativeCoordinatePassThrough(uintptr_t actor){
 uintptr_t prior=beastcontrol::nameScopeActor;
 for(uintptr_t scope:{uintptr_t(0),actor}){
  beastcontrol::nameScopeActor=scope;
  float x=25.5f,y=40.25f;int bx=25,by=40;
  beastcontrol::ObserveVectorCoordinates((void*)0x12340,(void*)0x23450,(void*)0x34560,coordinateText,&x,&y);
  beastcontrol::ObserveBitmapCoordinates((void*)0x12340,&bx,&by,coordinateText,(void*)0x23450);
  assert(x==30.75f&&y==60.5f&&bx==30&&by==60&&beastcontrol::nameScopeActor==scope);
 }
 beastcontrol::coordinateObserveCount=96;beastcontrol::nameScopeActor=actor;assert(!beastcontrol::ObserveCoordinatesNow());
 beastcontrol::nameScopeActor=prior;
}
static unsigned nativeNameCalls;
static void __fastcall ObserveNativeName(float x,float y,void* actor,void* context,const void* pair,unsigned char first,unsigned char second) {
 assert(x==25.5f&&y==100.25f&&context==(void*)0x12340&&pair==(void*)0x56780&&first==1&&second==2);
 assert(beastcontrol::nameScopeActor==(uintptr_t)actor);
 assert(beastcontrol::TownHeight(actor)==(beastcontrol::mounted?163:88));
 ++nativeNameCalls;
}
static void TestStationaryTransitions(){
 auto priorBase=gBase;
 auto reserve=(unsigned char*)VirtualAlloc(nullptr,0xf000000,MEM_RESERVE,PAGE_NOACCESS);assert(reserve);
 gBase=(uintptr_t)reserve;
 auto page=[&](uintptr_t rva){assert(VirtualAlloc(reserve+(rva&~uintptr_t(0xfff)),0x1000,MEM_COMMIT,PAGE_EXECUTE_READWRITE));};
 auto stub=[&](uintptr_t rva,uintptr_t target){page(rva);unsigned char code[]={0x48,0xb8,0,0,0,0,0,0,0,0,0xff,0xe0};memcpy(code+2,&target,8);memcpy(reserve+rva,code,sizeof(code));FlushInstructionCache(GetCurrentProcess(),reserve+rva,sizeof(code));};
 page(0xef2caa0);stub(beastcontrol::NativeMotion,(uintptr_t)&RefreshMotion);
 stub(0x5bd7e80,(uintptr_t)&RefreshNormal);stub(0x5bdaaa0,(uintptr_t)&RefreshHeight);
 stub(0x5bcd540,(uintptr_t)&ObserveNativeName);nativeNameCalls=0;
 stub(0x6fabb50,(uintptr_t)&NativeVectorCoordinates);stub(0x6ff1630,(uintptr_t)&NativeBitmapCoordinates);
 vectorCoordinateCalls=bitmapCoordinateCalls=0;
 std::vector<unsigned char> a(0x2500),info(0x630),position(0x180,0x55);
 uintptr_t methods[]={(uintptr_t)&Strong,(uintptr_t)&Weak};
 struct Control{uintptr_t table;LONG strong,weak;} control{(uintptr_t)methods,10,10};
 auto actor=(uintptr_t)a.data();
 *(uintptr_t*)a.data()=gBase+beastcontrol::TownTable;
 *(uintptr_t*)(a.data()+0x170)=(uintptr_t)position.data();
 *(uintptr_t*)(a.data()+0x1078)=(uintptr_t)&control;*(uintptr_t*)(a.data()+0x1080)=(uintptr_t)info.data();
 *(uintptr_t*)(a.data()+0x190)=(uintptr_t)&control;*(uintptr_t*)(a.data()+0x198)=0x123450;
 *(uintptr_t*)(reserve+0xef2caa0)=(uintptr_t)&control;*(uintptr_t*)(reserve+0xef2caa8)=actor+0x30;
 *(short*)(a.data()+0x2d4)=88;*(int*)(a.data()+0x2490)=-1;*(int*)(a.data()+0x2494)=-1;
 refreshPair={0x10030,(uintptr_t)&control};refreshMotions=0;
 beastcontrol::owner=actor;beastcontrol::ownerScene=0x123450;beastcontrol::disabled=false;
 auto originalPosition=position;
 unsigned initialEpoch=beastcontrol::nameObserveEpoch;
 for(unsigned profile:{15u,17u}){
  beastcontrol::rideAnimations[0]=refreshPair;InterlockedIncrement(&control.strong);
  position[0x138]=0;refreshExpectedMounted=true;
  assert(beastcontrol::BeginRide(actor,beast115::profiles[profile]));
  assert(position[0x138]==0&&beastcontrol::TownHeight((void*)actor)==163);
  assert(*(short*)(a.data()+0x2d4)==88);
  beastcontrol::nameScopeActor=0x98760;
  beastcontrol::ObserveName(25.5f,100.25f,(void*)actor,(void*)0x12340,(void*)0x56780,1,2);
  assert(beastcontrol::nameScopeActor==0x98760);beastcontrol::nameScopeActor=0;
  // Reproduce the native transition: BeginRide has set the visible motion,
  // but 5be4230 has not yet received the carrier pair used by 5bdbe10.
  *(uintptr_t*)(a.data()+0x1b0)=refreshPair.object;
  auto& nativeCarrier=*(beastcontrol::Pair*)(a.data()+0x1f90);
  assert(!nativeCarrier.object&&!nativeCarrier.control);
  beastcontrol::Pair pending{};
  LONG priorRefs=control.strong;
  assert(beastcontrol::DisguiseUpdate((void*)actor,&pending,2,24)==&pending);
  assert(pending.object==refreshPair.object&&pending.control==refreshPair.control);
  assert(control.strong==priorRefs+1&&*(int*)(a.data()+0x1fa0)==0);
  assert(*(uintptr_t*)(a.data()+0x1b0)==refreshPair.object);
  // Native update moves the returned shared pair into its carrier slot.
  nativeCarrier=pending;pending={};priorRefs=control.strong;
  for(unsigned frame=0;frame<5;frame++){
   pending={0x777,0x888};
   beastcontrol::DisguiseUpdate((void*)actor,&pending,2,24);
   assert(!pending.object&&!pending.control&&control.strong==priorRefs);
  }
  // Both halves of the native pair must agree before skipping the update.
  uintptr_t savedControl=nativeCarrier.control;nativeCarrier.control=0;
  beastcontrol::DisguiseUpdate((void*)actor,&pending,2,24);
  assert(pending.object==refreshPair.object&&pending.control==savedControl);
  nativeCarrier.control=savedControl;beastcontrol::Release(pending);
  assert(control.strong==priorRefs);
  // Walking and returning to idle hand off once, then keep each timeline.
  beastcontrol::rideAnimations[1]={0x20030,refreshPair.control};InterlockedIncrement(&control.strong);
  for(int action:{3,0}){
   *(int*)(a.data()+0x1fa0)=action;
   auto expected=beastcontrol::rideAnimations[action==3?1:0];
   beastcontrol::DisguiseUpdate((void*)actor,&pending,2,24);
   assert(pending.object==expected.object&&pending.control==expected.control);
   beastcontrol::Release(nativeCarrier);nativeCarrier=pending;pending={};
   *(uintptr_t*)(a.data()+0x1b0)=expected.object;priorRefs=control.strong;
   beastcontrol::DisguiseUpdate((void*)actor,&pending,2,24);
   assert(!pending.object&&!pending.control&&control.strong==priorRefs);
  }
  position[0x138]=0;refreshExpectedMounted=false;
  beastcontrol::EndRide(actor);
  // Normal native avatar update releases the carrier after disguise removal.
  beastcontrol::Release(nativeCarrier);
  assert(position[0x138]==0&&beastcontrol::TownHeight((void*)actor)==88);
  assert(*(int*)(a.data()+0x2490)==-1&&*(int*)(a.data()+0x2494)==-1);
  beastcontrol::ObserveName(25.5f,100.25f,(void*)actor,(void*)0x12340,(void*)0x56780,1,2);
  assert(beastcontrol::nameScopeActor==0);
  TestNativeCoordinatePassThrough(actor);
  position[0x138]=originalPosition[0x138];assert(position==originalPosition);
 }
 assert(refreshMotions==4&&nativeNameCalls==4&&vectorCoordinateCalls==4&&bitmapCoordinateCalls==4);assert(beastcontrol::nameObserveEpoch==initialEpoch+4);
 position[0x138]=0;*(int*)(info.data()+0x620)=2;
 assert(!beastcontrol::BeginRide(actor,beast115::profiles[15]));
 assert(!position[0x138]&&!beastcontrol::mounted&&refreshMotions==4);
 assert(beastcontrol::nameObserveEpoch==initialEpoch+4);
 beastcontrol::owner=beastcontrol::ownerScene=0;
 gBase=priorBase;VirtualFree(reserve,0,MEM_RELEASE);
 puts("STATIONARY TRANSITIONS VERIFIED: first idle carrier handoff despite matching visible motion; repeated frames preserve timeline/references; walking/idle each hand off once; mount/unmount preserve position bytes and 163/88 height");
}
int main(){
 static_assert(sizeof(beastcontrol::companionMoves)/sizeof(beastcontrol::companionMoves[0])==30);
 for(int action:{-1,0,1,2,4,27,28})assert(beastcontrol::CompanionMotionIndex(action)==0);
 assert(beastcontrol::CompanionMotionIndex(3)==1);
 const int expectedAttributeJobs[]={0,11,16,3,7,1,2,5,8,4,14,6,12,13,15};
 for(unsigned p=0;p<30;++p)for(auto id:beast115::profiles[p].equipment)
  assert(BeastAttributeJob(id)==expectedAttributeJobs[p%15]);
 for(uint32_t id:{0u,1u,38999u,39000u,787002673u,787002675u,787990101u,0xffffffffu})
  assert(BeastAttributeJob(id)==-1);
 for(unsigned n=0;n<30;n++){
  auto worn=beast115::profiles[n].equipment;
  assert(beastcontrol::Eligible(beastcontrol::assets[n].job,worn));
  assert(!beastcontrol::Eligible(-1,worn));
  // Every proper subset, including every possible missing piece, is rejected.
  for(unsigned mask=0;mask<255;mask++){
   worn=beast115::profiles[n].equipment;
   for(unsigned slot=0;slot<8;slot++)if(!(mask&(1u<<slot)))worn[slot]=0;
   assert(!beastcontrol::Eligible(beastcontrol::assets[n].job,worn));
  }
 }
 auto mixed=beast115::profiles[2].equipment;mixed[3]=beast115::profiles[0].equipment[3];assert(!beastcontrol::Eligible(16,mixed));
 assert(beastcontrol::Eligible(9,beast115::profiles[0].equipment));
 assert(beastcontrol::Eligible(10,beast115::profiles[3].equipment));
 assert(!beastcontrol::Eligible(9,beast115::profiles[3].equipment));
 assert(!beastcontrol::Eligible(10,beast115::profiles[0].equipment));
 // Exercise the actual dungeon reader with independent equipment objects.
 // It must reject changed live gear even though the town snapshot stays full.
 std::vector<unsigned char> liveActor(0x7600),liveTable(0x1f10),townActor(0x1260);
 *(uintptr_t*)liveActor.data()=(uintptr_t)liveTable.data();
 *(uintptr_t*)(liveTable.data()+0x15c8)=(uintptr_t)&BaseItem;
 *(uintptr_t*)(liveTable.data()+0x1f08)=(uintptr_t)&CoverItem;
 *(uintptr_t*)townActor.data()=gBase+beastcontrol::TownTable;
 for(unsigned p=0;p<30;p++){
  int job=beastcontrol::assets[p].job;
  *(int*)(liveActor.data()+0x75e8)=*(int*)(townActor.data()+0x108c)=job;
  for(unsigned slot=0;slot<8;slot++){
   battleBase[slot]=beast115::profiles[p].equipment[slot];battleCover[slot]=0;
   *(uint32_t*)(townActor.data()+0x1220+slot*8)=battleBase[slot];
  }
  assert(beastcontrol::Worn((uintptr_t)townActor.data())==&beast115::profiles[p]);
  assert(beastcontrol::BattleWorn((uintptr_t)liveActor.data(),PlainItemId)==&beast115::profiles[p]);
  for(unsigned slot=0;slot<8;slot++){
   auto id=battleBase[slot];battleBase[slot]=0;
   assert(!beastcontrol::BattleWorn((uintptr_t)liveActor.data(),PlainItemId));
   assert(beastcontrol::Worn((uintptr_t)townActor.data())==&beast115::profiles[p]);
   battleBase[slot]=id;
   battleCover[slot]=beast115::profiles[(p+1)%30].equipment[slot];
   assert(!beastcontrol::BattleWorn((uintptr_t)liveActor.data(),PlainItemId));
   battleCover[slot]=0;
  }
  // A matching appearance worn over clone base objects remains supported.
  for(unsigned slot=0;slot<8;slot++){battleCover[slot]=battleBase[slot];battleBase[slot]=1;}
  assert(beastcontrol::BattleWorn((uintptr_t)liveActor.data(),PlainItemId)==&beast115::profiles[p]);
 }
 for(unsigned sharedProfile:{0u,3u,15u,18u}){
  int job=(sharedProfile%15)==0?9:10;
  unsigned p=sharedProfile;*(int*)(liveActor.data()+0x75e8)=job;
  for(unsigned slot=0;slot<8;slot++){battleCover[slot]=beast115::profiles[p].equipment[slot];battleBase[slot]=1;}
  assert(beastcontrol::BattleWorn((uintptr_t)liveActor.data(),PlainItemId)==&beast115::profiles[p]);
 }
 assert(!beastcontrol::BattleWorn(0,PlainItemId));
 std::vector<unsigned char> cloneItem(0x1d18),cloneTable(0x448);
 *(uintptr_t*)cloneItem.data()=(uintptr_t)cloneTable.data();
 *(uintptr_t*)(cloneTable.data()+0x98)=gBase+0xbb1660;
 *(uintptr_t*)(cloneTable.data()+0x440)=gBase+0x603c110;
 uint32_t visibleId=0;
 for(const auto& p:beast115::profiles)for(uint32_t id:p.equipment){
  *(uint32_t*)(cloneItem.data()+0x70)=769900001; // unrelated stats template
  *(uint32_t*)(cloneItem.data()+0x1d10)=id;
  assert(beastcontrol::NativeItemId((uintptr_t)cloneItem.data(),visibleId)&&visibleId==id);
 }
 *(uint32_t*)(cloneItem.data()+0x1d10)=0;
 assert(!beastcontrol::NativeItemId((uintptr_t)cloneItem.data(),visibleId));
 assert(beastcontrol::AssetsFor(0,0)->battleRow==9001);
 assert(beastcontrol::AssetsFor(9,0)->battleRow==9016);
 assert(beastcontrol::AssetsFor(10,3)->battleRow==9017);
 uintptr_t vt[]={(uintptr_t)&Strong,(uintptr_t)&Weak};
 struct Control{uintptr_t vt;LONG strong,weak;} c{(uintptr_t)vt,2,1};
 beastcontrol::Pair pair{0x10030,(uintptr_t)&c};beastcontrol::Release(pair);
 assert(!pair.object&&!pair.control&&c.strong==1&&c.weak==1&&!strongDestroyed);
 pair={0x10030,(uintptr_t)&c};beastcontrol::Release(pair);
 assert(!pair.control&&c.strong==0&&c.weak==0&&strongDestroyed==1&&weakDestroyed==1);
 c.strong=1;beastcontrol::Weak w{0,(uintptr_t)&c,0x20030};assert(beastcontrol::WeakActor(w)==0x20000);
 c.strong=0;assert(!beastcontrol::WeakActor(w));assert(!beastcontrol::WeakActor({}));
 // Native appearance vectors use slot identifiers, including source rows
 // whose entry order differs from hat/hair/face order.
 std::vector<unsigned char> actor(0x5700),manager(0x108),part(0x518),node(0x40);
 int values[18]{};uintptr_t array[1]={(uintptr_t)node.data()+0x38};
 *(uintptr_t*)(actor.data()+0x5698)=(uintptr_t)manager.data();
 *(uintptr_t*)(manager.data()+0xc0)=(uintptr_t)array;*(int*)(manager.data()+0x100)=1;
 *(uintptr_t*)node.data()=(uintptr_t)part.data();
 *(uintptr_t*)(part.data()+0x500)=(uintptr_t)values;
 for(unsigned n=0;n<34;n++){
  const auto& asset=n<30?beastcontrol::assets[n]:n==30?beastcontrol::darkWarriorAssets:n==31?beastcontrol::creatorAssets:n==32?beastcontrol::fourthDarkWarriorAssets:beastcontrol::fourthCreatorAssets;
  for(unsigned j=0;j<asset.count;j++){unsigned slot=asset.count-1-j;values[j*2]=(int)slot;values[j*2+1]=asset.appearance[slot];}
  *(uintptr_t*)(part.data()+0x508)=(uintptr_t)values+asset.count*8;
  assert(beastcontrol::FindAppearance((uintptr_t)actor.data(),asset)==(uintptr_t)part.data());
  values[1]=123;assert(!beastcontrol::FindAppearance((uintptr_t)actor.data(),asset));
 }
 assert(!beastcontrol::Worn(0));assert(!beastcontrol::Current(0,0));assert(!beastcontrol::InstallController());
 std::vector<unsigned char> category(0x28),shopOwner(0xd8),root(0x120),renderer(8),renderControl(16);
 gShopCategory=(uintptr_t)category.data();
 *(uintptr_t*)category.data()=gBase+0x937dc08;*(uintptr_t*)(category.data()+0x18)=(uintptr_t)shopOwner.data();
 *(uintptr_t*)shopOwner.data()=gBase+0x937ed00;*(uintptr_t*)(shopOwner.data()+0xd0)=(uintptr_t)root.data();
 *(uintptr_t*)root.data()=gBase+0xa53ff28;
 assert(!ShopDisplayed());
 *(uintptr_t*)(root.data()+0x110)=(uintptr_t)renderer.data();*(uintptr_t*)(root.data()+0x118)=(uintptr_t)renderControl.data();
 *(uintptr_t*)renderer.data()=gBase+0xa546138;*(uintptr_t*)renderControl.data()=gBase+0xa5466d8;*(int*)(renderControl.data()+8)=2;
 assert(ShopDisplayed());
 *(uintptr_t*)(root.data()+0x110)=*(uintptr_t*)(root.data()+0x118)=0;
 assert(!ShopDisplayed());gShopCategory=0;
 // Scene cleanup must discard display objects without losing the user's mode.
 for(unsigned mode: {1u,2u}){
  beastcontrol::desiredMode=mode;beastcontrol::desiredOwner=0x12340;beastcontrol::desiredProfile=2;
  beastcontrol::summoned=true;beastcontrol::hudVisible=1;
  beastcontrol::ResetTown();
  assert(beastcontrol::desiredMode==mode&&beastcontrol::desiredOwner==0x12340&&beastcontrol::desiredProfile==2);
  assert(!beastcontrol::owner&&!beastcontrol::ownerScene&&!beastcontrol::summoned&&!beastcontrol::hudVisible);
 }
 // Cadence follows the supplied PVF term and emits only on grounded movement.
 for(unsigned term:{250u,400u,800u}){
  beastcontrol::FootTracker f;
  assert(!f.Observe(1,2,15,0,0,0,100,term));
  assert(!f.Observe(1,2,15,0,0,0,200,term));
  assert(f.Observe(1,2,15,1,0,0,201,term));
  assert(!f.Observe(1,2,15,2,0,0,201+term-1,term));
  assert(f.Observe(1,2,15,3,0,0,201+term,term));
  assert(!f.Observe(1,2,15,4,0,1,201+2*term,term));
  assert(!f.Observe(1,2,15,4,0,0,201+2*term+1,term));
  assert(!f.Observe(1,3,15,5,0,0,201+3*term,term));
  assert(f.Observe(1,3,15,6,0,0,201+3*term+1,term));
  assert(!f.Observe(1,3,16,7,0,0,201+4*term,term));
  assert(!f.Observe(0,3,16,8,0,0,201+5*term,term)&&!f.actor);
  assert(!f.Observe(1,2,15,0,0,0,0xffffff00u,term));
  assert(f.Observe(1,2,15,1,0,0,0xffffff01u,term));
  assert(f.Observe(1,2,15,2,0,0,0xffffff01u+term,term));
 }
 for(const auto& p:beast115::profiles){
  beastcontrol::mounted=true;beastcontrol::ownedDisguise=p.disguise;beastcontrol::ownedVariant=p.variant;
  assert(beastcontrol::OwnDisguise(p.disguise,p.variant));
  assert(!beastcontrol::OwnDisguise(p.disguise+1,p.variant));
  assert(!beastcontrol::OwnDisguise(p.disguise,p.variant+1));
  beastcontrol::mounted=false;assert(!beastcontrol::OwnDisguise(p.disguise,p.variant));
 }
 assert(beastcontrol::AssetsFor(9,15)->battleRow==19009);
 assert(beastcontrol::AssetsFor(10,18)->battleRow==19010);
 assert(!beastcontrol::AssetsFor(0,30));

 {
  std::vector<unsigned char> manager(0xe8),r0(0x60),r1(0x60),r2(0x60),p0(0x18),p1(0x18),p2(0x18);
  *(uint32_t*)(p0.data()+0x10)=4100001u;*(uint32_t*)(p1.data()+0x10)=4204001u;*(uint32_t*)(p2.data()+0x10)=3840002u;
  *(uintptr_t*)(r0.data()+0x58)=(uintptr_t)p0.data();*(uintptr_t*)(r1.data()+0x58)=(uintptr_t)p1.data();*(uintptr_t*)(r2.data()+0x58)=(uintptr_t)p2.data();
  uintptr_t entries[3]={(uintptr_t)r0.data(),(uintptr_t)r2.data(),(uintptr_t)r1.data()};
  *(uintptr_t*)(manager.data()+0xd8)=(uintptr_t)entries;*(uintptr_t*)(manager.data()+0xe0)=(uintptr_t)(entries+3);
  Filter(manager.data(),0);assert(entries[0]==(uintptr_t)r1.data()&&entries[1]==(uintptr_t)r0.data()&&entries[2]==(uintptr_t)r2.data());
  assert(*(uintptr_t*)(manager.data()+0xe0)==(uintptr_t)(entries+3));
  Filter(manager.data(),1);assert(entries[0]==(uintptr_t)r1.data()&&entries[1]==(uintptr_t)r0.data());
  assert(*(uintptr_t*)(manager.data()+0xe0)==(uintptr_t)(entries+2));
 }
 TestStationaryTransitions();
 puts("BEAST CONTROLLER VERIFIED 34 job/set mappings; source cadence, time wrap, exact disguise ownership; 7650 incomplete subsets rejected; live dungeon wear changes override stale town snapshots; clone covers supported; scene intent persistence, exactly-once release; native runtime absent rejected");
}
