#pragma once
// Diagnostics only. All native ABIs, arguments and results stay unchanged.
struct HeadTraceRecord {
 uintptr_t style=0,font=0,context=0,caller=0;
 float size=0,scale=0,sx=0,sy=0;
 int width=0,height=0,key[7]{};
 unsigned stage=0;
 wchar_t text[128]{};
};
static HeadTraceRecord gHeadTraceSeen[96];
static unsigned gHeadTraceCount;
static SRWLOCK gHeadTraceLock=SRWLOCK_INIT;
static HWND gHeadTraceWindow;
static __declspec(thread) const wchar_t* gHeadSubmitName;
static BOOL CALLBACK HeadTraceFindWindow(HWND window,LPARAM){
 DWORD owner=0;GetWindowThreadProcessId(window,&owner);RECT rect{};
 if(owner==GetCurrentProcessId()&&GetClientRect(window,&rect)&&rect.right>=800&&rect.bottom>=500){gHeadTraceWindow=window;return FALSE;}return TRUE;
}
static bool HeadTraceName(const wchar_t* text){
 return text&&(wcsstr(text,L"Deemo")||wcsstr(text,L"Creature")||
  wcsstr(text,L"\u6ce2\u8bfa\u5179")||wcsstr(text,L"\u7f57\u7c73\u5965")||
  wcsstr(text,L"\u88c5\u5907\u5e93\u6dfb\u52a0")||wcsstr(text,L"\u5de1\u793c\u4e4b\u5370"));
}
struct HeadTraceDrawScope {
 const wchar_t* previous;
 explicit HeadTraceDrawScope(const wchar_t* text):previous(gHeadSubmitName){
  gHeadSubmitName=nullptr;uintptr_t root=0,style=0;
  if(HeadTraceName(text)&&Read(gBase+0xf1c0ca0,root)&&Read(root+0x148,style)&&
    (style==gBase+0xe6ba420||style==gBase+0xe6ba520||style==gBase+0xe6ba890||style==(uintptr_t)&gHeadMonoStyle||!wcscmp(text,L"Creature")))gHeadSubmitName=text;
 }
 ~HeadTraceDrawScope(){gHeadSubmitName=previous;}
};
static void HeadTraceObserve(unsigned stage,void* context,float x,float y,uint32_t color,const wchar_t* text,unsigned char flag,float inputX=0,float inputY=0){
 if(!HeadTraceName(text))return;
 uintptr_t root=0,style=0,font=0;selectionfont::Style snapshot{};
 if(!Read(gBase+0xf1c0ca0,root)||!Read(root+0x148,style)||!Read(style,snapshot))return;
 memcpy(&font,snapshot.bytes+0x10,8);if(!HudNativeFont(font))return;
 HeadTraceRecord record{};record.stage=stage;record.style=style;record.font=font;record.context=(uintptr_t)context;
 memcpy(&record.size,snapshot.bytes+0x3c,4);memcpy(&record.scale,snapshot.bytes+0x74,4);
 Read(font+0x20,record.key);wcsncpy_s(record.text,text,_TRUNCATE);
 RECT rect{};if(!IsWindow(gHeadTraceWindow))EnumWindows(HeadTraceFindWindow,0);
 if(GetClientRect(gHeadTraceWindow,&rect)){record.width=rect.right;record.height=rect.bottom;}
 uintptr_t manager=0,view=0,count=0,start=0,capacity=0,array=0;unsigned char enabled=0;
 // Read the same ring entry selected by native 146EF5DE0, without invoking it.
 if(Read(gBase+0xe6343f8,manager)&&Read(manager+0x30,enabled)&&enabled&&
    Read(manager+0x28,count)&&count&&Read(manager+0x20,start)&&Read(manager+0x18,capacity)&&capacity&&
    Read(manager+0x10,array)&&Read(array+((count+start-1)&(capacity-1))*8,view)&&view){
  Read(view+0x38,record.sx);Read(view+0x3c,record.sy);
 }
 void* frames[10]{};USHORT depth=CaptureStackBackTrace(0,10,frames,nullptr);
 record.caller=depth>3?(uintptr_t)frames[3]:0;
 bool emit=false,seen=false;AcquireSRWLockExclusive(&gHeadTraceLock);
 for(unsigned i=0;i<gHeadTraceCount;i++){
  const auto& r=gHeadTraceSeen[i];
  if(r.stage==record.stage&&r.style==record.style&&r.font==record.font&&r.context==record.context&&
     r.caller==record.caller&&r.size==record.size&&r.scale==record.scale&&r.sx==record.sx&&r.sy==record.sy&&
     r.width==record.width&&r.height==record.height&&!memcmp(r.key,record.key,sizeof(r.key))&&!wcscmp(r.text,record.text)){seen=true;break;}
 }
 if(!seen&&gHeadTraceCount<96){gHeadTraceSeen[gHeadTraceCount++]=record;emit=true;}
 ReleaseSRWLockExclusive(&gHeadTraceLock);if(!emit)return;
 wchar_t file[260]{},family[128]{};HudDiagWString(font+0x58,file,260);HudDiagWString(font+0x78,family,128);
 uint32_t enumValue=0;float defaultSize=0,activeSize=0;
 Read(font+0x9c,enumValue);Read(font+0x4c,defaultSize);Read(font+0xa8,activeSize);
 char value[512]{};WideCharToMultiByte(CP_UTF8,0,record.text,-1,value,sizeof(value),nullptr,nullptr);
 char bytes[0x78*2+1]{};for(unsigned i=0;i<0x78;i++)sprintf_s(bytes+i*2,3,"%02x",snapshot.bytes[i]);
 Log("HEAD_TRACE_V2 stage=%u text=%s style=%llx font=%llx file=%ls family=%ls enum=%u default=%.3f override=%.3f active=%.3f scale=%.3f view=%llx view_scale=(%.5f,%.5f) pos=(%.3f,%.3f) input=(%.3f,%.3f) client=%dx%d color=%08x flag=%u key=[%d,%d,%d,%d,%d,%d,%d]",
  stage,value,(unsigned long long)style,(unsigned long long)font,file,family,enumValue,defaultSize,record.size,activeSize,record.scale,(unsigned long long)view,record.sx,record.sy,x,y,inputX,inputY,record.width,record.height,color,(unsigned)flag,record.key[0],record.key[1],record.key[2],record.key[3],record.key[4],record.key[5],record.key[6]);
 Log("HEAD_TRACE_V2 style stage=%u text=%s context=%llx bytes=%s",stage,value,(unsigned long long)record.context,bytes);
 for(USHORT i=0;i<depth;i++)Log("HEAD_TRACE_V2 caller stage=%u text=%s depth=%u address=%llx",stage,value,(unsigned)i,(unsigned long long)(uintptr_t)frames[i]);
}
static void __fastcall HeadTraceFit(void* context,float x,float y,uint32_t color,const wchar_t* text,unsigned char flag){
 HeadTraceDrawScope scope(text);
 HeadTraceObserve(0,context,x,y,color,text,flag);((HudDrawFn)(gBase+0x6facb50))(context,x,y,color,text,flag);
}
static void __fastcall HeadTracePlain(void* context,float x,float y,uint32_t color,const wchar_t* text,unsigned char flag){
 HeadTraceDrawScope scope(text);
 HeadTraceObserve(1,context,x,y,color,text,flag);((HudDrawFn)(gBase+0x6fac310))(context,x,y,color,text,flag);
}
using HeadTraceIntFn=void(__fastcall*)(void*,int,int,uint32_t,const wchar_t*,unsigned char);
static void __fastcall HeadTraceInteger(void* context,int x,int y,uint32_t color,const wchar_t* text,unsigned char flag){
 HeadTraceDrawScope scope(text);
 HeadTraceObserve(2,context,(float)x,(float)y,color,text,flag);((HeadTraceIntFn)(gBase+0x6fac840))(context,x,y,color,text,flag);
}
using HeadTraceTransformFn=void(__fastcall*)(void*,void*,void*,const wchar_t*,float*,float*);
static HeadTraceTransformFn gHeadTraceTransformOriginal;
using HeadTraceBitmapFn=void(__fastcall*)(void*,int*,int*,const wchar_t*,void*);
static HeadTraceBitmapFn gHeadTraceBitmapOriginal;
static void __fastcall HeadTraceTransform(void* context,void* style,void* font,const wchar_t* text,float* x,float* y){
 const float inputX=*x,inputY=*y;
 gHeadTraceTransformOriginal(context,style,font,text,x,y);
 HeadTraceObserve(3,context,*x,*y,0,text,0,inputX,inputY);
}
static void __fastcall HeadTraceBitmap(void* context,int* x,int* y,const wchar_t* text,void* style){
 const int inputX=*x,inputY=*y;
 gHeadTraceBitmapOriginal(context,x,y,text,style);
 HeadTraceObserve(4,context,(float)*x,(float)*y,0,text,0,(float)inputX,(float)inputY);
}
static bool InstallHeadNameTrace(){
 constexpr uintptr_t sites[]={0x6ebdf3a,0x6ebdd2a,0x6ebde32,0x6fac479,0x6ff4545};
 constexpr uintptr_t targets[]={0x6facb50,0x6fac310,0x6fac840,0x6fabb50,0x6ff1630};
 const uintptr_t wrappers[]={(uintptr_t)&HeadTraceFit,(uintptr_t)&HeadTracePlain,(uintptr_t)&HeadTraceInteger,(uintptr_t)&HeadTraceTransform,(uintptr_t)&HeadTraceBitmap};
 unsigned char expected[5][5]{};unsigned char* relays[5]{};
 gHeadTraceTransformOriginal=(HeadTraceTransformFn)(gBase+0x6fabb50);
 gHeadTraceBitmapOriginal=(HeadTraceBitmapFn)(gBase+0x6ff1630);
 for(unsigned i=0;i<5;i++){
  expected[i][0]=0xe8;int32_t displacement=(int32_t)(targets[i]-sites[i]-5);memcpy(expected[i]+1,&displacement,4);
  if(memcmp((void*)(gBase+sites[i]),expected[i],5)){
   // The current base already owns the vector-coordinate call for dragonrobe
   // diagnostics. Chain only that exact in-module callback, never an unknown hook.
   unsigned char actual[5]{},prefix[6]{};uintptr_t destination=0;
   if(i<3||!Read(gBase+sites[i],actual)||actual[0]!=0xe8){Log("HEAD_TRACE_V2 call guard rejected index=%u",i);return false;}
   int32_t oldDisplacement=0;memcpy(&oldDisplacement,actual+1,4);
   const uintptr_t previousRelay=gBase+sites[i]+5+oldDisplacement;
   if(!Read(previousRelay,prefix)||memcmp(prefix,"\xff\x25\x00\x00\x00\x00",6)||
      !Read(previousRelay+6,destination)||destination!=(i==3?(uintptr_t)&beastcontrol::ObserveVectorCoordinates:(uintptr_t)&beastcontrol::ObserveBitmapCoordinates)){
    Log("HEAD_TRACE_V2 unrelated coordinate hook rejected");return false;
   }
   memcpy(expected[i],actual,5);
   if(i==3)gHeadTraceTransformOriginal=(HeadTraceTransformFn)destination;else gHeadTraceBitmapOriginal=(HeadTraceBitmapFn)destination;
  }
 }
 for(unsigned i=0;i<5;i++){
  relays[i]=ChoiceRelay(gBase+sites[i],wrappers[i]);
  if(!relays[i]){for(auto p:relays)if(p)VirtualFree(p,0,MEM_RELEASE);return false;}
 }
 for(unsigned i=0;i<5;i++){
  unsigned char call[5]={0xe8};int32_t displacement=(int32_t)((intptr_t)relays[i]-(intptr_t)(gBase+sites[i]+5));memcpy(call+1,&displacement,4);
  if(!Patch(gBase+sites[i],call,5)){
   for(unsigned j=0;j<=i;j++)Patch(gBase+sites[j],expected[j],5);
   Log("HEAD_TRACE_V2 install failed; native calls restored");return false;
  }
 }
 Log("HEAD_TRACE_V2 installed diagnostics only; 5 validated native/owned callsites; bitmap/vector observers chained; integer/float arguments preserved; maximum records=96");return true;
}
