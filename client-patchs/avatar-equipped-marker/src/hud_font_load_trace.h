#pragma once
// Evidence-only reproduction of the previous failed private import.
// No HUD font substitution, width, placement, colour or outline change.
static __declspec(thread) bool gHudDiagActive;
static void HudDiagWString(uintptr_t p,wchar_t* out,size_t cap){
 size_t n=0,c=0;uintptr_t data=p;SIZE_T got=0;
 out[0]=0;if(!Read(p+0x10,n)||!Read(p+0x18,c)||n>=cap||c<n)return;
 if(c>=8&&!Read(p,data))return;
 if(n&&ReadProcessMemory(GetCurrentProcess(),(void*)data,out,n*2,&got)&&got==n*2)out[n]=0;
}
static int __fastcall HudDiagFace(void* lib,const char* path,int index,void** face){
 int result=((int(__fastcall*)(void*,const char*,int,void**))(gBase+0x8af9270))(lib,path,index,face);
 DWORD err=GetLastError();
 if(gHudDiagActive)Log("HUD_DIAG FT_New_Face path=%s index=%d error=0x%02x face=%p",path?path:"<null>",index,result,face?*face:nullptr);
 SetLastError(err);return result;
}
static int __fastcall HudDiagStream(void* stream,const char* path){
 int result=((int(__fastcall*)(void*,const char*))(gBase+0x8b01530))(stream,path);
 DWORD err=GetLastError();
 if(gHudDiagActive){uint32_t size=0;Read((uintptr_t)stream+8,size);Log("HUD_DIAG FT_Stream_Open path=%s error=0x%02x size=%u last_error=%lu",path?path:"<null>",result,size,err);}
 SetLastError(err);return result;
}
static void __fastcall HudDiagPaths(void* manager,const void* desc,void* key){
 ((void(__fastcall*)(void*,const void*,void*))(gBase+0x700d3b0))(manager,desc,key);
 DWORD err=GetLastError();
 if(gHudDiagActive){uintptr_t first=0,last=0;Read((uintptr_t)key+0x18,first);Read((uintptr_t)key+0x20,last);
  if(first&&last>=first&&last-first<=16*0x28&&(last-first)%0x28==0){
   for(uintptr_t p=first;p<last;p+=0x28){uint32_t type=0,index=0;Read(p,type);Read(p+4,index);wchar_t path[512]{};HudDiagWString(p+8,path,512);
    Log("HUD_DIAG native_file item=%u type=%u index=%u path=%ls attrs=%08lx",(unsigned)((p-first)/0x28),type,index,path,GetFileAttributesW(path));}
  }else Log("HUD_DIAG native_file invalid vector");
 }
 SetLastError(err);
}
static int __fastcall HudDiagImport(void* manager,void* font,const void* desc){
 int result=((int(__fastcall*)(void*,void*,const void*))(gBase+0x700bf40))(manager,font,desc);
 DWORD err=GetLastError();if(gHudDiagActive)Log("HUD_DIAG ImportFont error=%d",result);SetLastError(err);return result;
}
static void* __fastcall HudDiagFile(void* manager,const void* path,int index){
 void* result=((void*(__fastcall*)(void*,const void*,int))(gBase+0x87a7be0))(manager,path,index);
 DWORD err=GetLastError();
 if(gHudDiagActive){wchar_t text[512]{};HudDiagWString((uintptr_t)path,text,512);Log("HUD_DIAG file_cache path=%ls index=%d result=%p",text,index,result);}
 SetLastError(err);return result;
}
static void* __fastcall HudDiagSlot(void* manager,const void* object){
 void* result=((void*(__fastcall*)(void*,const void*))(gBase+0x700b4b0))(manager,object);
 DWORD err=GetLastError();
 if(gHudDiagActive){unsigned char slot=0;Read((uintptr_t)result,slot);uintptr_t cache=0,vt=0,method=0;
  Read(gBase+0xf3bb2c0,cache);Read(cache,vt);Read(vt+8,method);
  Log("HUD_DIAG dispatch slot=%u backend=%p vtable=%p method=%p",slot,(void*)cache,(void*)vt,(void*)method);
  for(unsigned side: {8u,16u}){uintptr_t importer=0,first=0,last=0,mgr=0;Read(cache+side,importer);Read(importer,mgr);Read(importer+0x60,first);Read(importer+0x68,last);
   Log("HUD_DIAG importer side=%u object=%p manager=%p slots=%llu",side,(void*)importer,(void*)mgr,(unsigned long long)(last>=first?(last-first)/8:~0ull));}
 }
 SetLastError(err);return result;
}
