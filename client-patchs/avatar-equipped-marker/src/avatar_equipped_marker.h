#pragma once
#include "avatar_marker_identity.h"
namespace avatarmarker {
using DrawFn=void(__fastcall*)(void*,float,float,const void*);
static DrawFn original;
static LONG observed,disabled;
// Native intrusive image references, as returned by the resource manager and
// ImagePack::frame (+78). Both own one reference at object+8.
struct ImageRef { uintptr_t value; };
using PackFn=ImageRef*(__fastcall*)(void*,ImageRef*,const wchar_t*);
using FrameFn=ImageRef*(__fastcall*)(void*,ImageRef*,int);
using ImageDrawFn=void(__fastcall*)(void*,float,float,const ImageRef*,const void*,unsigned,float,float);
static bool NativeCode(uintptr_t p) {
 auto dos=(IMAGE_DOS_HEADER*)gBase;
 auto nt=(IMAGE_NT_HEADERS64*)(gBase+dos->e_lfanew);
 if(p<gBase||p-gBase>=nt->OptionalHeader.SizeOfImage)return false;
 MEMORY_BASIC_INFORMATION m{};
 if(!VirtualQuery((void*)p,&m,sizeof(m))||m.State!=MEM_COMMIT||m.Protect&(PAGE_GUARD|PAGE_NOACCESS))return false;
 DWORD protection=m.Protect&0xff;
 return protection==PAGE_EXECUTE||protection==PAGE_EXECUTE_READ||
        protection==PAGE_EXECUTE_READWRITE||protection==PAGE_EXECUTE_WRITECOPY;
}
static void Release(ImageRef& ref) {
 uintptr_t p=ref.value;ref.value=0;if(!p)return;
 uintptr_t vt=0,destructor=0;LONG count=0;
 if(!Read(p,vt)||!Read(p+8,count)||count<=0||
    !Read(vt+8,destructor)||!NativeCode(destructor))return;
 if(InterlockedDecrement((volatile LONG*)(p+8))==0)
  ((void(__fastcall*)(void*))destructor)((void*)p);
}
static bool Bytes(uintptr_t rva,const unsigned char* want,size_t count) {
 unsigned char got[64]{};SIZE_T n=0;
 return count<=sizeof(got)&&ReadProcessMemory(GetCurrentProcess(),(void*)(gBase+rva),got,count,&n)&&n==count&&!memcmp(got,want,count);
}
static bool Source(uintptr_t item,int& slot,int& part) {
 uintptr_t vt=0,manager=0,root=0,table=0,entry=0,actual=0;int capacity=0;
 if(!Read(item,vt)||(vt!=gBase+0xa924448&&vt!=gBase+0xa9b29f8&&vt!=gBase+0xa9b38f0)||
    !Read(gBase+0xe683cb0,manager)||!manager||!Read(manager+0x58,capacity)||capacity<=0||capacity>8192||
    !Read(manager+0x70,root)||!root||!Read(gBase+0xe682288,root)||!root)return false;
 // These are the exact helpers used by native 1469E3090. Its shared_ptr
 // wrapper is deliberately not called with a raw object (ownership ABI).
 part=((int(__fastcall*)(void*))(gBase+0x5a71020))((void*)item);
 if(part<0||part>=12)return false;
 slot=((int(__fastcall*)(void*,void*))(gBase+0x5ad5c20))((void*)manager,(void*)item);
 if(slot<0||slot>=capacity)return false;
 int source=((int(__fastcall*)(unsigned char))(gBase+0x58c2580))((unsigned char)part);
 if(source!=slot)return false;
 if(!Read(manager+0xa8,table)||!Read(table+slot*8,entry)||entry<0x40||!Read(entry-0x40,actual))return false;
 return IsCurrentSource(slot,source,part,capacity,item,actual);
}
static void Paint(uintptr_t item,float x,float y) {
 int slot=-1,part=-1;if(!Source(item,slot,part))return;
 uintptr_t manager=0,vt=0,getPack=0,getFrame=0;unsigned char contextFlag=0;
 if(!Read(item+0x18+0x30,contextFlag)||contextFlag>1||
    !Read(gBase+0xf1c39c8,manager)||!manager||!Read(manager,vt)||
    !Read(vt+0x10,getPack)||!NativeCode(getPack))return;
 ImageRef pack{},frame{};
 __try {
  // Lazy native lookup on the existing render thread; no private texture,
  // font, atlas initialization, or cached raw pointer survives this draw.
  ((PackFn)getPack)((void*)manager,&pack,L"Item/IconMark.img");
  if(pack.value&&Read(pack.value,vt)&&Read(vt+0x78,getFrame)&&NativeCode(getFrame))
   ((FrameFn)getFrame)((void*)pack.value,&frame,403);
  if(frame.value) {
   // Frame 403 has an 11x13 crop at (17,15) on a 28x28 canvas. Move its
   // canvas up 15 px to place the official person at the slot's upper right.
   // 145006577 proves this exact eight-argument image draw ABI/context.
   ((ImageDrawFn)(gBase+0x6ebbd30))((void*)(item+0x18),x,y-15.0f,&frame,nullptr,0xffffffff,1.0f,1.0f);
   if(InterlockedCompareExchange(&observed,1,0)==0)
    Log("AVATAR_MARKER native Item/IconMark.img frame=403 actual_source_slot=%d part=%d; manual UI acceptance pending",slot,part);
  }
 } __finally { Release(frame);Release(pack); }
}
static void __fastcall Draw(void* item,float x,float y,const void* options) {
 original(item,x,y,options);
 if(disabled||!options||!std::isfinite(x)||!std::isfinite(y))return;
 // Treat corrupt/mismatched native state as an unavailable badge, not a
 // reason to abort the native item draw. Native renderer itself stays intact.
 __try { Paint((uintptr_t)item,x,y); }
 __except(EXCEPTION_EXECUTE_HANDLER) {
  InterlockedExchange(&disabled,1);
  Log("AVATAR_MARKER disabled after native state exception=%08x",GetExceptionCode());
 }
}
static bool Install() {
 wchar_t host[MAX_PATH]{};if(!GetModuleFileNameW(nullptr,host,MAX_PATH))return false;
 const wchar_t* leaf=wcsrchr(host,L'\\');leaf=leaf?leaf+1:host;
 if(_wcsicmp(leaf,L"DFO.exe")){Log("AVATAR_MARKER skipped non-DFO host");return false;}
 static const unsigned char draw[]={0x48,0x8b,0xc4,0x55,0x56,0x57,0x41,0x54,0x41,0x55,0x41,0x56,0x41,0x57,0x48,0x8d};
 static const unsigned char find[]={0x48,0x83,0xec,0x08,0x4c,0x8b,0xc2,0x48,0x85,0xd2,0x0f,0x84,0x99,0,0,0};
 static const unsigned char part[]={0x40,0x53,0x48,0x83,0xec,0x20,0x48,0x8b,0x01,0x48,0x8b,0xd9,0xff,0x90,0x10,0x04};
 static const unsigned char source[]={0x88,0x4c,0x24,0x08,0x48,0x83,0xec,0x28,0x4c,0x8b,0x05,0xf9,0xfc,0xdb,0x08};
 static const unsigned char image[]={0x48,0x8b,0xc4,0x57,0x48,0x83,0xec,0x70,0x48,0xc7,0x40,0xc8,0xfe,0xff,0xff,0xff};
 static const unsigned char lookup[]={0x48,0x8b,0x05,0xc1,0x43,0xa9,0x09,0x48,0x8b,0x08,0x48,0x8b,0x59,0x10};
 static const unsigned char frame[]={0x48,0x8b,0x08,0x48,0x8b,0x01,0x41,0xb8,0x17,0x00,0x00,0x00,0x48,0x8d,0x55,0x67,0xff,0x50,0x78};
 if(!Bytes(0x5a662d0,draw,sizeof(draw))||!Bytes(0x5ad5c20,find,sizeof(find))||!Bytes(0x5a71020,part,sizeof(part))||
    !Bytes(0x58c2580,source,sizeof(source))||!Bytes(0x6ebbd30,image,sizeof(image))||
    !Bytes(0x572f600,lookup,sizeof(lookup))||!Bytes(0x572f62b,frame,sizeof(frame))) {
  Log("AVATAR_MARKER skipped native byte fingerprint mismatch");return false;
 }
 const uintptr_t slots[]={0xa9246e8,0xa9b2c98,0xa9b3b90};uintptr_t old=0,replacement=(uintptr_t)&Draw;
 for(auto rva:slots)if(!Read(gBase+rva,old)||old!=gBase+0x5a662d0){Log("AVATAR_MARKER skipped vtable mismatch rva=%llx",(unsigned long long)rva);return false;}
 original=(DrawFn)(gBase+0x5a662d0);int installed=0;
 for(auto rva:slots){if(!Patch(gBase+rva,&replacement,8)){for(int i=installed-1;i>=0;i--){uintptr_t native=(uintptr_t)original;Patch(gBase+slots[i],&native,8);}Log("AVATAR_MARKER stopped; rollback attempted");return false;}installed++;}
 Log("AVATAR_MARKER ready; instance identity, native source relation, Item/IconMark.img frame=403 upper-right; UI pending");return true;
}
}
