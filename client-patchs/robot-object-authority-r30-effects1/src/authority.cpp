#define _CRT_SECURE_NO_WARNINGS
#include "../../elite-companion-pilot/src/inline_hook.h"
#include "authority_policy.h"
#include "native_evidence.h"
#include <stdio.h>
#include <stdlib.h>

static HMODULE gSelf;
static uintptr_t gBase;
static InlineHook gHook;
static FILE* gLog;
static SRWLOCK gLeaseLock=SRWLOCK_INIT;
static AuthorityLease gLease={};
static wchar_t gLeasePath[1024]={};
static volatile LONG gStarted=0,gEnabled=0,gRemaps=0,gFaults=0;
static thread_local bool gInHook=false;
using AuthorityFn=uint64_t(__fastcall*)(uintptr_t);
static AuthorityFn gOriginal;
static uint64_t UnixMillis() {
 FILETIME ft;GetSystemTimeAsFileTime(&ft);
 return (((uint64_t)ft.dwHighDateTime<<32)|ft.dwLowDateTime)/10000-11644473600000ull;
}
template<size_t N> static bool Match(uintptr_t rva,const unsigned char(&b)[N]) {
 return Readable((void*)(gBase+rva),N)&&memcmp((void*)(gBase+rva),b,N)==0;
}
// This runs on the original game thread. The worker never accesses game objects.
static bool Scope(uintptr_t object,int nativeSeat,const AuthorityLease& lease) {
 __try {
  auto manager=*(uintptr_t*)(gBase+0xe66c090);
  if(!manager||!*(uintptr_t*)(gBase+0xe683c20))return false;
  auto scene=((uint32_t(__fastcall*)(uintptr_t))(gBase+kStateRVA))(manager);
  if(scene!=3)return false;
  auto local=((int(__fastcall*)(uintptr_t))(gBase+kLocalSeatRVA))(manager);
  if(local!=0)return false;
  auto identity=(uint32_t(__fastcall*)(uintptr_t))(gBase+kIdentityRVA);
  auto getActor=(uintptr_t(__fastcall*)(uintptr_t,uint32_t))(gBase+kActorRVA);
  uint32_t objectID=identity(object);
  unsigned type=objectID>>16;
  if(type!=3&&type!=4&&type!=17)return false;
  uint32_t actors[8]={};uintptr_t owner=0;
  for(unsigned i=0;i<8;i++) {
   uintptr_t actor=getActor(manager,i);
   if(i==0)owner=actor;
   if(actor)actors[i]=identity(actor);
  }
  if(!owner)return false;
  auto mapOf=[](uintptr_t p)->uintptr_t {
   return ((uintptr_t(__fastcall*)(uintptr_t))(*(uintptr_t*)(*(uintptr_t*)p+0x130)))(p);
  };
  uint32_t effectOwnerID=0;
  if(type==17) {
   // Current PE CPassiveObject v538 is the checked weak-parent getter.
   // Refuse unrelated type17 objects and dead/expired parent references.
   auto vt=*(uintptr_t*)object;
   if(!Readable((void*)(vt+0x538),8)||*(uintptr_t*)(vt+0x538)!=gBase+kEffectOwnerRVA)return false;
   auto parent=((uintptr_t(__fastcall*)(uintptr_t))(gBase+kEffectOwnerRVA))(object);
   if(!parent||parent!=getActor(manager,(uint32_t)nativeSeat))return false;
   effectOwnerID=identity(parent);
  }
  auto map=mapOf(owner);
  return AllowTransfer(lease,UnixMillis(),scene,local,objectID,nativeSeat,map&&mapOf(object)==map,actors,effectOwnerID);
 } __except(EXCEPTION_EXECUTE_HANDLER) {InterlockedIncrement(&gFaults);return false;}
}
static uint64_t __fastcall Authority(uintptr_t object) {
 uint64_t native=gOriginal(object);
 if(!gEnabled||gInHook||!object||(int32_t)native<=0||(int32_t)native>3)return native;
 AuthorityLease lease;
 AcquireSRWLockShared(&gLeaseLock);lease=gLease;ReleaseSRWLockShared(&gLeaseLock);
 if(!ValidLease(lease,UnixMillis()))return native;
 gInHook=true;
 bool remap=Scope(object,(int32_t)native,lease);
 gInHook=false;
 if(!remap)return native;
 InterlockedIncrement(&gRemaps);
 return 0; // owner seat, validated above. No entity/state/door writes.
}
static DWORD WINAPI LeaseWorker(void*) {
 LONG lastRemaps=0,lastFaults=0;DWORD lastLog=GetTickCount();
 for(;;) {
  AuthorityLease next={};DWORD bytes=0;
  HANDLE h=CreateFileW(gLeasePath,GENERIC_READ,FILE_SHARE_READ|FILE_SHARE_WRITE|FILE_SHARE_DELETE,
                        nullptr,OPEN_EXISTING,FILE_ATTRIBUTE_NORMAL,nullptr);
  bool valid=false;
  if(h!=INVALID_HANDLE_VALUE) {
   LARGE_INTEGER size={};
   valid=GetFileSizeEx(h,&size)&&size.QuadPart==sizeof(next)&&ReadFile(h,&next,sizeof(next),&bytes,nullptr)&&bytes==sizeof(next);
   CloseHandle(h);
  }
  valid=valid&&ValidLease(next,UnixMillis());
  if(valid) {
   HANDLE server=OpenProcess(SYNCHRONIZE,FALSE,next.serverPID);
   valid=server&&WaitForSingleObject(server,0)==WAIT_TIMEOUT;
   if(server)CloseHandle(server);
  }
  if(!valid)memset(&next,0,sizeof(next));
  AcquireSRWLockExclusive(&gLeaseLock);gLease=next;ReleaseSRWLockExclusive(&gLeaseLock);
  if(GetTickCount()-lastLog>=5000) {
   LONG remaps=InterlockedCompareExchange(&gRemaps,0,0),faults=InterlockedCompareExchange(&gFaults,0,0);
   if(remaps!=lastRemaps||faults!=lastFaults) {
    fprintf(gLog,"COUNTERS remaps=%ld guard_faults=%ld lease_active=%u epoch=%u\n",remaps,faults,next.active,next.epoch);fflush(gLog);
    lastRemaps=remaps;lastFaults=faults;
   }
   lastLog=GetTickCount();
  }
  Sleep(100);
 }
}
extern "C" __declspec(dllexport) const char* WINAPI ModName(){return "Local robot object authority r30 effects1";}
extern "C" __declspec(dllexport) DWORD WINAPI ModStart() {
 if(InterlockedCompareExchange(&gStarted,1,0))return 0;
 wchar_t path[MAX_PATH]={},opt[64]={};
 DWORD n=GetModuleFileNameW(gSelf,path,_countof(path));if(!n||n>=_countof(path))return 1;
 auto slash=wcsrchr(path,L'\\');if(!slash)return 1;
 wcscpy_s(slash+1,_countof(path)-(slash+1-path),L"robot-object-authority.log");
 gLog=_wfopen(path,L"ab");if(!gLog)return 1;
 if(!GetEnvironmentVariableW(L"DFO_LOCAL_ROBOT_AUTHORITY",opt,_countof(opt))||wcscmp(opt,L"r16-object-authority-v1")) {
  fputs("INACTIVE opt-in absent; no writes\n",gLog);fflush(gLog);return 0;
 }
 gBase=(uintptr_t)GetModuleHandleW(nullptr);
 auto dos=(IMAGE_DOS_HEADER*)gBase;
 if(!Readable(dos,sizeof(*dos))||dos->e_magic!=IMAGE_DOS_SIGNATURE||dos->e_lfanew<0||dos->e_lfanew>0x100000)return 2;
 auto nt=(IMAGE_NT_HEADERS64*)(gBase+dos->e_lfanew);
 if(!Readable(nt,sizeof(*nt))||nt->Signature!=IMAGE_NT_SIGNATURE||nt->FileHeader.TimeDateStamp!=kImageTimestamp||
    nt->OptionalHeader.SizeOfImage!=kImageSize||!Match(kAuthorityRVA,kAuthorityHead)||!Match(kStateRVA,kStateHead)||
    !Match(kLocalSeatRVA,kLocalSeatHead)||!Match(kActorRVA,kActorHead)||!Match(kIdentityRVA,kIdentityHead)||!Match(kEffectOwnerRVA,kEffectOwnerHead)) {
  fputs("DISABLED image/function mismatch; no writes\n",gLog);fflush(gLog);return 2;
 }
 n=GetEnvironmentVariableW(L"DFO_ROBOT_AUTHORITY_LEASE",gLeasePath,_countof(gLeasePath));
 if(n<4||n>=_countof(gLeasePath)||gLeasePath[1]!=L':'||gLeasePath[2]!=L'\\')return 3;
 if(!PrepareHook(gHook,(void*)(gBase+kAuthorityRVA),kAuthorityHead,sizeof(kAuthorityHead),(void*)&Authority))return 4;
 gOriginal=(AuthorityFn)gHook.original();
 if(!PublishHook(gHook))return 5;
 HANDLE worker=CreateThread(nullptr,0,LeaseWorker,nullptr,0,nullptr);
 if(!worker) {fputs("DISABLED worker failed; native return only\n",gLog);fflush(gLog);return 6;}
 CloseHandle(worker);InterlockedExchange(&gEnabled,1);
 fputs("READY r30 effects1 ordinary local fake-only party; type3/4 and verified fake-parent type17 authority adapter; native gate conditions preserved\n",gLog);fflush(gLog);
 return 0;
}
BOOL WINAPI DllMain(HINSTANCE self,DWORD reason,LPVOID) {
 if(reason==DLL_PROCESS_ATTACH){gSelf=self;DisableThreadLibraryCalls(self);}return TRUE;
}
