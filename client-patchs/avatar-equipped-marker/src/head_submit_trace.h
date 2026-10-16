#pragma once
// Diagnostic-only: native queued command after 146FABA30 copies its font key
// and render state. No arguments, coordinates, fonts, flags or cache changed.
struct HeadSubmitRecord {
 unsigned char key[0x2c]{},state[0x20]{};
 uintptr_t nodeTable=0,fontTable=0;
 int width=0,height=0;
 bool settled=false;
 wchar_t text[128]{};
};
static HeadSubmitRecord gHeadSubmitSeen[48];
static unsigned gHeadSubmitCount;
static SRWLOCK gHeadSubmitLock=SRWLOCK_INIT;
static DWORD gHeadSubmitStart;
static LONG gHeadGlyphRecords;
static __declspec(thread) uintptr_t gHeadGlyphFace;
static __declspec(thread) unsigned gHeadGlyphIndex;
static __declspec(thread) int gHeadGlyphFlags;
static void HeadHex(const unsigned char* input,size_t size,char* out){
 for(size_t i=0;i<size;i++)sprintf_s(out+i*2,3,"%02x",input[i]);
}
static void HeadUnicode(const wchar_t* text,char* out){
 unsigned n=0;for(;n<127&&text[n];n++)sprintf_s(out+n*4,5,"%04x",(unsigned)text[n]);out[n*4]=0;
}
static void __fastcall HeadTraceSubmit(void* node){
 ((void(__fastcall*)(void*))(gBase+0x6faba30))(node);
 if(!gHeadSubmitName)return;
 unsigned char data[0xb8]{};uintptr_t font=0;
 if(!Read((uintptr_t)node,data))return;memcpy(&font,data+0x58,8);
 HeadSubmitRecord rec{};memcpy(&rec.nodeTable,data,8);memcpy(rec.key,data+0x80,sizeof(rec.key));memcpy(rec.state,data+0x60,sizeof(rec.state));
 Read(font,rec.fontTable);wcsncpy_s(rec.text,gHeadSubmitName,_TRUNCATE);
 RECT rect{};if(GetClientRect(gHeadTraceWindow,&rect)){rec.width=rect.right;rec.height=rect.bottom;}
 DWORD now=GetTickCount();bool emit=false,seen=false;
 AcquireSRWLockExclusive(&gHeadSubmitLock);
 if(!gHeadSubmitStart)gHeadSubmitStart=now;rec.settled=now-gHeadSubmitStart>=1500;
 for(unsigned i=0;i<gHeadSubmitCount;i++){
  const auto& old=gHeadSubmitSeen[i];
  if(old.nodeTable==rec.nodeTable&&old.fontTable==rec.fontTable&&old.width==rec.width&&old.height==rec.height&&old.settled==rec.settled&&
    !memcmp(old.key,rec.key,sizeof(rec.key))&&!wcscmp(old.text,rec.text)){seen=true;break;}
 }
 if(!seen&&gHeadSubmitCount<48){gHeadSubmitSeen[gHeadSubmitCount++]=rec;emit=true;}
 ReleaseSRWLockExclusive(&gHeadSubmitLock);if(!emit)return;
 char text[512]{},command[sizeof(data)*2+1]{},fontBytes[0x80*2+1]{};
 HeadUnicode(rec.text,text);HeadHex(data,sizeof(data),command);
 unsigned char fb[0x80]{};if(Read(font,fb))HeadHex(fb,sizeof(fb),fontBytes);
 float x=0,y=0;memcpy(&x,data+0xb0,4);memcpy(&y,data+0xb4,4);
 Log("HEAD_SUBMIT text_utf16=%s ms=%lu settled=%u client=%dx%d node=%llx node_vtable=%llx draw_font=%llx draw_font_vtable=%llx pos=%.3f/%.3f caller=%p",
  text,now-gHeadSubmitStart,rec.settled?1u:0u,rec.width,rec.height,(unsigned long long)(uintptr_t)node,(unsigned long long)rec.nodeTable,
  (unsigned long long)font,(unsigned long long)rec.fontTable,x,y,_ReturnAddress());
 Log("HEAD_SUBMIT command=%s draw_font_bytes=%s",command,fontBytes);
}
static int __fastcall HeadGlyphLoad(void* face,unsigned index,int flags){
 int result=((int(__fastcall*)(void*,unsigned,int))(gBase+0x8af8470))(face,index,flags);
 gHeadGlyphFace=(uintptr_t)face;gHeadGlyphIndex=index;gHeadGlyphFlags=flags;
 return result;
}
static int __fastcall HeadGlyphRender(void* slot,unsigned mode){
 unsigned actual=HeadMonoRenderMode(gHeadGlyphFace,(uintptr_t)slot,mode);
 int result=((int(__fastcall*)(void*,unsigned))(gBase+0x8afa9d0))(slot,actual);
 HeadMonoObserveGlyph(gHeadGlyphFace,(uintptr_t)slot,mode,actual,result);
 if(gHeadSubmitName&&InterlockedIncrement(&gHeadGlyphRecords)<=32){
  unsigned char data[0xa0]{};char text[512]{},glyph[sizeof(data)*2+1]{};HeadUnicode(gHeadSubmitName,text);
  if(Read((uintptr_t)slot,data))HeadHex(data,sizeof(data),glyph);
  Log("HEAD_GLYPH text_utf16=%s face=%llx index=%u load_flags=%08x render_mode=%u result=%d slot=%p bytes=%s",
   text,(unsigned long long)gHeadGlyphFace,gHeadGlyphIndex,(unsigned)gHeadGlyphFlags,actual,result,slot,glyph);
 }
 return result;
}
static bool InstallHeadSubmitTrace(){
 constexpr uintptr_t sites[]={0x6fac572,0x6fac66a,0x6fac709,0x6fac798,0x700cdd9,0x700cf1c,0x700cf58};
 constexpr uintptr_t targets[]={0x6faba30,0x6faba30,0x6faba30,0x6faba30,0x8af8470,0x8afa9d0,0x8afa9d0};
 const uintptr_t wrappers[]={(uintptr_t)&HeadTraceSubmit,(uintptr_t)&HeadTraceSubmit,(uintptr_t)&HeadTraceSubmit,(uintptr_t)&HeadTraceSubmit,
  (uintptr_t)&HeadGlyphLoad,(uintptr_t)&HeadGlyphRender,(uintptr_t)&HeadGlyphRender};
 unsigned char expected[7][5]{};unsigned char* relays[7]{};
 for(unsigned i=0;i<7;i++){
  expected[i][0]=0xe8;int32_t d=(int32_t)(targets[i]-sites[i]-5);memcpy(expected[i]+1,&d,4);
  if(memcmp((void*)(gBase+sites[i]),expected[i],5)){Log("HEAD_SUBMIT native call guard rejected index=%u",i);return false;}
 }
 for(unsigned i=0;i<7;i++){
  relays[i]=ChoiceRelay(gBase+sites[i],wrappers[i]);
  if(!relays[i]){for(auto relay:relays)if(relay)VirtualFree(relay,0,MEM_RELEASE);return false;}
 }
 for(unsigned i=0;i<7;i++){
  unsigned char call[5]={0xe8};int32_t d=(int32_t)((intptr_t)relays[i]-(intptr_t)(gBase+sites[i]+5));memcpy(call+1,&d,4);
  if(!Patch(gBase+sites[i],call,5)){
   for(unsigned j=0;j<=i;j++)Patch(gBase+sites[j],expected[j],5);
   Log("HEAD_SUBMIT install failed; original calls restored");return false;
  }
 }
 Log("HEAD_SUBMIT installed diagnostics only; 7 validated native calls; queued command key/state, native glyph results; no rendering changes");return true;
}
