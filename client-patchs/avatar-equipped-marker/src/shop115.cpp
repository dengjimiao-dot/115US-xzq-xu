#define WIN32_LEAN_AND_MEAN
#define DIRECTINPUT_VERSION 0x0800
#include <windows.h>
#include <dinput.h>
#include <stdint.h>
#include <stdio.h>
#include <stdarg.h>
#include <string.h>
#include <wchar.h>
#include <vector>
#include "shop_filter.h"
#include "preview_leaf.h"
#include "preview_choices.h"
#include "scroll_geometry.h"

static HMODULE gSelf,gReal;
static INIT_ONCE gOnce=INIT_ONCE_STATIC_INIT;
static DWORD gTls=TLS_OUT_OF_INDEXES;
static uintptr_t gBase;
static wchar_t gLogPath[MAX_PATH];
static LONG gLogCount;
static uintptr_t gShopCategory;
struct ChoiceState {void* owner;const PreviewChoices* choices;uint32_t index;DWORD thread;};
static ChoiceState gChoice{};
static void Log(const char* fmt,...) {
 if(InterlockedIncrement(&gLogCount)>2000)return;
 char text[1024];va_list a;va_start(a,fmt);int n=vsnprintf(text,sizeof(text)-3,fmt,a);va_end(a);
 if(n<0)n=(int)strlen(text);if(n>(int)sizeof(text)-3)n=sizeof(text)-3;text[n++]='\r';text[n++]='\n';
 HANDLE f=CreateFileW(gLogPath,FILE_APPEND_DATA,FILE_SHARE_READ|FILE_SHARE_WRITE,nullptr,OPEN_ALWAYS,FILE_ATTRIBUTE_NORMAL,nullptr);
 if(f!=INVALID_HANDLE_VALUE){DWORD written;WriteFile(f,text,n,&written,nullptr);CloseHandle(f);}
}
template<class T> static bool Read(uintptr_t p,T& out) {SIZE_T got=0;return p&&ReadProcessMemory(GetCurrentProcess(),(void*)p,&out,sizeof(out),&got)&&got==sizeof(out);}
static bool Writable(uintptr_t p,size_t n) {
 MEMORY_BASIC_INFORMATION m{};if(!VirtualQuery((void*)p,&m,sizeof(m))||m.State!=MEM_COMMIT||m.Protect&(PAGE_GUARD|PAGE_NOACCESS))return false;
 DWORD protection=m.Protect&0xff;return (protection==PAGE_READWRITE||protection==PAGE_WRITECOPY||protection==PAGE_EXECUTE_READWRITE||protection==PAGE_EXECUTE_WRITECOPY)&&p+n>=p&&p+n<=(uintptr_t)m.BaseAddress+m.RegionSize;
}
using InitFn=bool(__fastcall*)(void*,uintptr_t,uintptr_t,uintptr_t);
using BuildFn=void(__fastcall*)(void*);
using RefreshFn=void(__fastcall*)(void*,bool);
using AssignFn=void(__fastcall*)(void*,void*);
static bool ShopDisplayed() {
 // The category's cached open event survives closing the shop. Its main root
 // owns this rendering pair only while displayed (verified open/closed/open).
 uintptr_t table=0,owner=0,root=0,object=0,control=0;int count=0;
 if(!Read(gShopCategory,table)||table!=gBase+0x937dc08||!Read(gShopCategory+0x18,owner)||
    !Read(owner,table)||table!=gBase+0x937ed00||!Read(owner+0xd0,root)||
    !Read(root,table)||table!=gBase+0xa53ff28)return false;
 return Read(root+0x110,object)&&Read(root+0x118,control)&&
        Read(object,table)&&table==gBase+0xa546138&&Read(control,table)&&table==gBase+0xa5466d8&&
        Read(control+8,count)&&count>0;
}
static bool __fastcall Init(void* self,uintptr_t a2,uintptr_t a3,uintptr_t a4) {
 bool ok=((InitFn)(gBase+0x8a4280))(self,a2,a3,a4);
 if(ok){gShopCategory=(uintptr_t)self;*(int*)((char*)self+0x30)=21;*(int*)((char*)self+0x34)=30;}
 Log("INIT self=%p parent=%u ok=%u",self,*(uint32_t*)((char*)self+0xc),ok);return ok;
}
static uint32_t __fastcall Lowest(void*) {return 21;}
static int* __fastcall ListingCategory(void*,int* out) {out[0]=11;out[1]=0;return out;}
static bool __fastcall ValidCategory(void*,uint32_t,int category) {return category==9||(category>=21&&category<=30);}
struct NativeWString {wchar_t text[8];size_t length;size_t capacity;};
static_assert(sizeof(NativeWString)==32,"115us native SSO string layout");
static void __fastcall Build(void* self) {
 *(int*)((char*)self+0x30)=21;*(int*)((char*)self+0x34)=30;
 int& selected=*(int*)((char*)self+0x10);if(selected<21||selected>30)selected=21;
 ((BuildFn)(gBase+0x8a3950))(self);
 const wchar_t* names[]={L"全部",L"时装",L"宠物",L"光环",L"称号",L"武器",L"伤害数字",L"头像队伍",L"觉醒插画",L"表情"};
 using GetNodeFn=void*(__fastcall*)(void*,int);using LabelFn=void(__fastcall*)(void*,const NativeWString*);
 unsigned labels=0,enabled=0;
 for(int i=0;i<10;i++) {
  void* node=((GetNodeFn)(gBase+0x8a0a70))(self,21+i);if(!node)continue;
  NativeWString label{};label.length=wcslen(names[i]);label.capacity=7;memcpy(label.text,names[i],label.length*sizeof(wchar_t));
  ((LabelFn)(gBase+0x855a50))(node,&label);labels++;enabled+=*((unsigned char*)node+0x50)!=0;
 }
 Log("BUILD self=%p labels=%u enabled=%u range=21..30 selected=%d",self,labels,enabled,selected);
}
static void Filter(void* manager,uint32_t mask) {
 uintptr_t begin=0,end=0;
 if(!Read((uintptr_t)manager+0xd8,begin)||!Read((uintptr_t)manager+0xe0,end)||end<begin||(end-begin)%8||(end-begin)/8>32768||!Writable((uintptr_t)manager+0xe0,8)) {Log("FILTER skipped invalid vector");return;}
 size_t count=(end-begin)/8;if(!count)return;
 std::vector<uintptr_t> rows(count);std::vector<uint32_t> ids(count);SIZE_T got=0;
 if(!Writable(begin,count*8)||!ReadProcessMemory(GetCurrentProcess(),(void*)begin,rows.data(),count*8,&got)||got!=count*8){Log("FILTER skipped inaccessible vector");return;}
 size_t recognized=0;
 for(size_t i=0;i<count;i++) {
  uintptr_t product=0;
  if(!Read(rows[i]+0x58,product)||!Read(product+0x10,ids[i])||ids[i]==0||ids[i]>1000000000) {Log("FILTER skipped product layout row=%zu",i);return;}
  if(CategoryFor(ids[i]))recognized++;
 }
 if(!recognized){Log("FILTER skipped no recognized products first=%u count=%zu",ids[0],count);return;}
 size_t kept=ArrangeProducts((uintptr_t*)begin,rows.data(),ids.data(),count,mask);
 *(uintptr_t*)((char*)manager+0xe0)=begin+kept*8;
 Log("FILTER mask=%u rows=%zu kept=%zu recognized=%zu first=%u",mask,count,kept,recognized,ids[0]);
}
static void UpdateGeometry(void* manager) {
 uintptr_t p=(uintptr_t)manager,begin=0,end=0;uint32_t columns=0,rows=0;float gap=0,height=0;
 if(!Read(p+0xd8,begin)||!Read(p+0xe0,end)||end<begin||(end-begin)%8||(end-begin)/8>32768||
    !Read(p+0xa8,columns)||!Read(p+0xac,rows)||columns<1||columns>16||rows<1||rows>32||
    !Read(p+0x4c,gap)||!Read(p+0xb4,height)||!isfinite(gap)||!isfinite(height)||gap<0||height<=0||
    !Writable(p+0x5c,4)||!Writable(p+0x68,4)||!Writable(p+0xf0,8))return;
 auto geometry=ShopGeometry((uint32_t)((end-begin)/8),columns,rows,gap,height);
 uintptr_t viewport=0;int viewportHeight=0;float padding=0;
 if(Read(p+0x18,viewport)&&Read(viewport+0x9c,viewportHeight)&&viewportHeight>0&&viewportHeight<=8192&&Read(p+0x54,padding)&&isfinite(padding)&&padding>=0)
  geometry.maximum=ShopMaximumSteps(geometry.extent,padding,viewportHeight,gap+height);
 *(float*)(p+0x5c)=geometry.extent;*(uint32_t*)(p+0x68)=geometry.maximum;
 // Enable native bottom alignment against the viewport, including partial rows.
 *(unsigned char*)(p+0xf0)=1;*(int*)(p+0xf4)=0;
 Log("SCROLL count=%zu columns=%u visibleRows=%u extent=%.1f maximum=%u",(end-begin)/8,columns,rows,geometry.extent,geometry.maximum);
}
static void AlignCurrent(void* manager) {
 uintptr_t p=(uintptr_t)manager,viewport=0;uint32_t current=0,maximum=0;int viewportHeight=0;float extent=0,padding=0,step=0;
 if(!Read(p+0x18,viewport)||!Read(viewport+0x9c,viewportHeight)||viewportHeight<1||viewportHeight>32768||
    !Read(p+0x64,current)||!Read(p+0x68,maximum)||!Read(p+0x5c,extent)||!Read(p+0x54,padding)||!Read(p+0x7c,step)||
    !isfinite(extent)||!isfinite(padding)||!isfinite(step)||step<=0||!Writable(p+0x64,4)||!Writable(p+0x6c,12))return;
 if(current>maximum)current=maximum;
 float bottom=ShopBottomOffset(extent,padding,viewportHeight);
 float offset=current*step;if(maximum&&current==maximum)offset=bottom;if(offset>bottom)offset=bottom;
 *(uint32_t*)(p+0x64)=current;*(float*)(p+0x6c)=offset;*(float*)(p+0x70)=offset;*(float*)(p+0x74)=offset;
 ((void(__fastcall*)(void*,bool))(gBase+0x856c70))(manager,false);
}
static void __fastcall Assign(void* manager,void* data) {
 ((AssignFn)(gBase+0x855af0))(manager,data);
 uint32_t mask=(uint32_t)(uintptr_t)TlsGetValue(gTls);
 try {Filter(manager,mask);if(data)UpdateGeometry(manager);}catch(...){Log("FILTER skipped allocation failure");}
}
static void __fastcall Refresh(void* self,bool reset) {
 void* prior=TlsGetValue(gTls);int selected=*(int*)((char*)self+0x10);uint32_t mask=SelectedMask(selected);
 Log("REFRESH self=%p selected=%d mask=%u",self,selected,mask);TlsSetValue(gTls,(void*)(uintptr_t)mask);
 __try {((RefreshFn)(gBase+0x8a33d0))(self,reset);uintptr_t manager=0;if(Read((uintptr_t)self+0x108,manager)&&manager)AlignCurrent((void*)manager);} __finally {TlsSetValue(gTls,prior);}
}
static bool Patch(uintptr_t p,const void* data,size_t n) {
 DWORD old;if(!VirtualProtect((void*)p,n,PAGE_EXECUTE_READWRITE,&old))return false;
 memcpy((void*)p,data,n);FlushInstructionCache(GetCurrentProcess(),(void*)p,n);DWORD ignored;
 if(!VirtualProtect((void*)p,n,old,&ignored))Log("PATCH protection restore failed address=%p",(void*)p);
 return true;
}
using PreviewFn=int(__fastcall*)(void*,uint32_t);
static int __fastcall Preview(void* self,uint32_t id) {
 uint32_t leaf=PreviewTemplate(id);
 if(gChoice.owner==self&&gChoice.choices&&gChoice.choices->outer==id&&gChoice.index<gChoice.choices->count)leaf=kChoiceLeaves[gChoice.choices->offset+gChoice.index];
 int result=((PreviewFn)(gBase+0x75b100))(self,leaf);
 if(leaf!=id)Log("PREVIEW outer=%u leaf=%u result=%d",id,leaf,result);
 return result;
}
static bool InstallPreview() {
 uintptr_t callsite=gBase+0x771273;
 const unsigned char expected[]={0xe8,0x88,0x9e,0xfe,0xff};
 if(memcmp((void*)callsite,expected,5)){Log("PREVIEW rejected client call fingerprint");return false;}
 unsigned char* relay=nullptr;
 for(uintptr_t distance=0x10000;distance<0x70000000;distance+=0x10000){relay=(unsigned char*)VirtualAlloc((void*)((callsite&~(uintptr_t)0xffff)+distance),0x1000,MEM_RESERVE|MEM_COMMIT,PAGE_READWRITE);if(relay)break;}
 if(!relay){Log("PREVIEW rejected relay allocation");return false;}
 relay[0]=0xff;relay[1]=0x25;memset(relay+2,0,4);*(uintptr_t*)(relay+6)=(uintptr_t)&Preview;DWORD protect;
 if(!VirtualProtect(relay,0x1000,PAGE_EXECUTE_READ,&protect)){VirtualFree(relay,0,MEM_RELEASE);return false;}
 FlushInstructionCache(GetCurrentProcess(),relay,14);
 unsigned char jump[5]={0xe8};*(int32_t*)(jump+1)=(int32_t)((intptr_t)relay-(intptr_t)(callsite+5));
 if(!Patch(callsite,jump,5)){VirtualFree(relay,0,MEM_RELEASE);return false;}
 Log("PREVIEW ready first-choice aura/pet packages=%zu",sizeof(kPreviewLeaves)/sizeof(kPreviewLeaves[0]));return true;
}
static bool Install() {
 gBase=(uintptr_t)GetModuleHandleW(nullptr);
 wchar_t host[MAX_PATH];GetModuleFileNameW(nullptr,host,MAX_PATH);const wchar_t* file=wcsrchr(host,L'\\');file=file?file+1:host;
 if(_wcsicmp(file,L"DFO.exe")){Log("INSTALL skipped non-DFO host");return false;}
 auto dos=(IMAGE_DOS_HEADER*)gBase;auto nt=(IMAGE_NT_HEADERS64*)(gBase+dos->e_lfanew);
 if(dos->e_magic!=IMAGE_DOS_SIGNATURE||nt->Signature!=IMAGE_NT_SIGNATURE||nt->OptionalHeader.Magic!=IMAGE_NT_OPTIONAL_HDR64_MAGIC||nt->OptionalHeader.SizeOfImage<0x937dd20){Log("INSTALL rejected image layout");return false;}
 const unsigned char refreshHead[]={0x40,0x55,0x53,0x56,0x57,0x41,0x56,0x48,0x8d,0x6c,0x24,0xc9,0x48,0x81,0xec,0xd0};
 if(memcmp((void*)(gBase+0x8a33d0),refreshHead,sizeof(refreshHead))){Log("INSTALL rejected client function fingerprint");return false;}
 uintptr_t table=gBase+0x937dc08;const size_t offsets[]={8,0xb0,0xb8,0xc8,0x100,0x110};
 const uintptr_t oldRvas[]={0x8a4280,0x8a09a0,0x8a0a50,0x8a15b0,0x8a3950,0x8a33d0};
 uintptr_t old[6],replacement[6]={(uintptr_t)&Init,(uintptr_t)&Lowest,(uintptr_t)&ListingCategory,(uintptr_t)&ValidCategory,(uintptr_t)&Build,(uintptr_t)&Refresh};
 for(int i=0;i<6;i++)if(!Read(table+offsets[i],old[i])||old[i]!=gBase+oldRvas[i]){Log("INSTALL rejected vtable slot=%d",i);return false;}
 uintptr_t callsite=gBase+0x8a344c;unsigned char original[5];SIZE_T got=0;
 if(!ReadProcessMemory(GetCurrentProcess(),(void*)callsite,original,5,&got)||got!=5||original[0]!=0xe8||callsite+5+*(int32_t*)(original+1)!=gBase+0x855af0){Log("INSTALL rejected assignment call");return false;}
 gTls=TlsAlloc();if(gTls==TLS_OUT_OF_INDEXES){Log("INSTALL rejected TLS allocation");return false;}
 unsigned char* relay=nullptr;
 for(uintptr_t distance=0x10000;distance<0x70000000;distance+=0x10000){relay=(unsigned char*)VirtualAlloc((void*)((callsite&~(uintptr_t)0xffff)+distance),0x1000,MEM_RESERVE|MEM_COMMIT,PAGE_READWRITE);if(relay)break;}
 if(!relay){TlsFree(gTls);gTls=TLS_OUT_OF_INDEXES;Log("INSTALL rejected relay allocation");return false;}
 relay[0]=0xff;relay[1]=0x25;memset(relay+2,0,4);*(uintptr_t*)(relay+6)=(uintptr_t)&Assign;DWORD protect;
 if(!VirtualProtect(relay,0x1000,PAGE_EXECUTE_READ,&protect)){VirtualFree(relay,0,MEM_RELEASE);TlsFree(gTls);gTls=TLS_OUT_OF_INDEXES;return false;}
 unsigned char jump[5]={0xe8};*(int32_t*)(jump+1)=(int32_t)((intptr_t)relay-(intptr_t)(callsite+5));
 int installed=0;bool callPatched=false;
 for(int i=0;i<6;i++){if(!Patch(table+offsets[i],&replacement[i],8))goto fail;installed++;}
 if(!Patch(callsite,jump,5))goto fail;callPatched=true;
 Log("INSTALL ready categories=all,outfit,pet,aura,title,weapon,damage,portrait,cutscene,emote,misc masks=%zu; UI verification pending",kCategoryCount);return true;
fail:
 if(callPatched)Patch(callsite,original,5);for(int i=installed-1;i>=0;i--)Patch(table+offsets[i],&old[i],8);
 // Keep the relay and TLS alive if a protection restoration failed after a write.
 Log("INSTALL stopped and attempted rollback");return false;
}
#include "choice_dropdown_hook.h"
static int __fastcall ShopMaximum(void* manager){uint32_t max=0;if(Read((uintptr_t)manager+0x68,max)&&max<=8192)return (int)max;return ((int(__fastcall*)(void*))(gBase+0x84fc20))(manager);}
static bool InstallScrollRange(){
 const uintptr_t sites[]={0x8a34df,0x8a34f9,0x8a350c};
 const unsigned char expected[3][5]={{0xe8,0x3c,0xc7,0xfa,0xff},{0xe8,0x22,0xc7,0xfa,0xff},{0xe8,0x0f,0xc7,0xfa,0xff}};
 for(int i=0;i<3;i++)if(memcmp((void*)(gBase+sites[i]),expected[i],5)){Log("SCROLL rejected maximum callsite=%d",i);return false;}
 unsigned char* relays[3]={};for(int i=0;i<3;i++){relays[i]=ChoiceRelay(gBase+sites[i],(uintptr_t)&ShopMaximum);if(!relays[i]){for(auto p:relays)if(p)VirtualFree(p,0,MEM_RELEASE);return false;}}
 for(int i=0;i<3;i++){unsigned char call[5]={0xe8};int32_t delta=(int32_t)((intptr_t)relays[i]-(intptr_t)(gBase+sites[i]+5));memcpy(call+1,&delta,4);if(!Patch(gBase+sites[i],call,5)){for(int j=i-1;j>=0;j--)Patch(gBase+sites[j],expected[j],5);Log("SCROLL install stopped; rollback attempted");return false;}}
 Log("SCROLL ready filtered extent, viewport maximum and native bottom alignment");return true;
}
#include "beast_controller.h"
#include "beast_hud.h"
#include "beast_attribute_job.h"
#include "selection_text_style.h"
#include "hud_cn_digits.h"
#include "menu_cn_text.h"
#include "head_cn_text.h"
#include "head_mono_text.h"
#include "head_name_trace_v2.h"
#include "head_submit_trace.h"
#include "avatar_equipped_marker.h"
static BOOL CALLBACK Bootstrap(PINIT_ONCE,void*,void**) {
 wchar_t path[MAX_PATH];GetModuleFileNameW(gSelf,gLogPath,MAX_PATH);wchar_t* slash=wcsrchr(gLogPath,L'\\');if(slash)wcscpy_s(slash+1,MAX_PATH-(slash+1-gLogPath),L"115-shop-tabs.log");
 UINT n=GetSystemDirectoryW(path,MAX_PATH);if(n+13>=MAX_PATH)return FALSE;wcscat_s(path,L"\\dinput8.dll");gReal=LoadLibraryW(path);
 if(!gReal||gReal==gSelf){gReal=nullptr;Log("PROXY failed system dinput8 loading");return FALSE;}
 if(Install()){InstallPreview();InstallChoicePreview();InstallScrollRange();beastcontrol::InstallController();InstallBeastAttributeJob();InstallSelectionTextStyle();if(InstallHudCNDigits()){InstallMenuCNText();/* Head font override withdrawn: preserve native bitmap metrics and outline. */if(InstallHeadNameTrace()&&InstallHeadSubmitTrace())InstallHeadMonoText();}avatarmarker::Install();}return TRUE;
}
static FARPROC Real(const char* name){InitOnceExecuteOnce(&gOnce,Bootstrap,nullptr,nullptr);return gReal?GetProcAddress(gReal,name):nullptr;}
extern "C" HRESULT WINAPI DirectInput8Create(HINSTANCE h,DWORD version,REFIID iid,LPVOID* out,LPUNKNOWN outer) {
 using Fn=HRESULT(WINAPI*)(HINSTANCE,DWORD,REFIID,LPVOID*,LPUNKNOWN);auto f=(Fn)Real("DirectInput8Create");return f?f(h,version,iid,out,outer):E_FAIL;
}
extern "C" HRESULT WINAPI DllCanUnloadNow(){using Fn=HRESULT(WINAPI*)();auto f=(Fn)Real("DllCanUnloadNow");return f?f():S_FALSE;}
extern "C" HRESULT WINAPI DllGetClassObject(REFCLSID cls,REFIID iid,LPVOID* out){using Fn=HRESULT(WINAPI*)(REFCLSID,REFIID,LPVOID*);auto f=(Fn)Real("DllGetClassObject");return f?f(cls,iid,out):E_FAIL;}
extern "C" HRESULT WINAPI DllRegisterServer(){using Fn=HRESULT(WINAPI*)();auto f=(Fn)Real("DllRegisterServer");return f?f():E_FAIL;}
extern "C" HRESULT WINAPI DllUnregisterServer(){using Fn=HRESULT(WINAPI*)();auto f=(Fn)Real("DllUnregisterServer");return f?f():E_FAIL;}
extern "C" LPCDIDATAFORMAT WINAPI GetdfDIJoystick(){using Fn=LPCDIDATAFORMAT(WINAPI*)();auto f=(Fn)Real("GetdfDIJoystick");return f?f():nullptr;}
BOOL WINAPI DllMain(HINSTANCE self,DWORD reason,LPVOID){if(reason==DLL_PROCESS_ATTACH){gSelf=self;HudReserveNativeSlotAtAttach();MenuReserveNativeSlotsAtAttach();HeadMonoReserveAtAttach();DisableThreadLibraryCalls(self);}return TRUE;}
