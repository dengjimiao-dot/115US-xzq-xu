#pragma once
// Diagnostics only. Preserve native font, style, coordinates and all settings.
// 146EBDF3A calls the common string draw 146FACB50 after active-style selection.
struct HeadTraceRecord {
 uintptr_t style=0,font=0;
 float size=0,scale=0;
 int width=0,height=0;
 wchar_t text[128]{};
};
static HeadTraceRecord gHeadTraceSeen[64];
static unsigned gHeadTraceCount;
static SRWLOCK gHeadTraceLock=SRWLOCK_INIT;
static bool HeadTraceName(const wchar_t* text){
 return text&&(wcsstr(text,L"Deemo")||!wcscmp(text,L"Creature")||
  wcsstr(text,L"\u6ce2\u8bfa\u5179")||wcsstr(text,L"\u7f57\u7c73\u5965")||
  wcsstr(text,L"\u88c5\u5907\u5e93\u6dfb\u52a0")||wcsstr(text,L"\u5de1\u793c\u4e4b\u5370"));
}
static void __fastcall HeadTraceDraw(void* context,float x,float y,uint32_t color,const wchar_t* text,unsigned char flag){
 if(HeadTraceName(text)){
  uintptr_t root=0,style=0,font=0;
  selectionfont::Style snapshot{};
  if(Read(gBase+0xf1c0ca0,root)&&Read(root+0x148,style)&&Read(style,snapshot)){
   memcpy(&font,snapshot.bytes+0x10,8);
   if(HudNativeFont(font)){
    HeadTraceRecord record{};record.style=style;record.font=font;
    memcpy(&record.size,snapshot.bytes+0x3c,4);memcpy(&record.scale,snapshot.bytes+0x74,4);
    wcsncpy_s(record.text,text,_TRUNCATE);
    HWND window=GetActiveWindow();DWORD owner=0;GetWindowThreadProcessId(window,&owner);
    RECT rect{};if(owner==GetCurrentProcessId()&&GetClientRect(window,&rect)){record.width=rect.right-rect.left;record.height=rect.bottom-rect.top;}
    bool seen=false,emit=false;
    AcquireSRWLockExclusive(&gHeadTraceLock);
    for(unsigned i=0;i<gHeadTraceCount;i++){
     const auto& r=gHeadTraceSeen[i];
     if(r.style==record.style&&r.font==record.font&&r.size==record.size&&r.scale==record.scale&&
        r.width==record.width&&r.height==record.height&&!wcscmp(r.text,record.text)){seen=true;break;}
    }
    if(!seen&&gHeadTraceCount<64){gHeadTraceSeen[gHeadTraceCount++]=record;emit=true;}
    ReleaseSRWLockExclusive(&gHeadTraceLock);
    if(emit){
     wchar_t file[260]{},family[128]{};
     HudDiagWString(font+0x58,file,260);HudDiagWString(font+0x78,family,128);
     uint32_t enumValue=0;float defaultSize=0;int key[7]{};
     Read(font+0x9c,enumValue);Read(font+0x4c,defaultSize);Read(font+0x20,key);
     char value[512]{};WideCharToMultiByte(CP_UTF8,0,record.text,-1,value,sizeof(value),nullptr,nullptr);
     char bytes[0x78*2+1]{};for(unsigned i=0;i<0x78;i++)sprintf_s(bytes+i*2,3,"%02x",snapshot.bytes[i]);
     Log("HEAD_TRACE text=%s style=%llx font=%llx file=%ls family=%ls enum=%u default=%.3f override=%.3f scale=%.3f pos=(%.3f,%.3f) client=%dx%d color=%08x flag=%u key=[%d,%d,%d,%d,%d,%d,%d] bytes=%s",
      value,(unsigned long long)style,(unsigned long long)font,file,family,enumValue,defaultSize,record.size,record.scale,x,y,record.width,record.height,color,(unsigned)flag,key[0],key[1],key[2],key[3],key[4],key[5],key[6],bytes);
     void* frames[8]{};USHORT count=CaptureStackBackTrace(0,8,frames,nullptr);
     for(USHORT i=0;i<count;i++)Log("HEAD_TRACE caller text=%s depth=%u address=%llx",value,(unsigned)i,(unsigned long long)(uintptr_t)frames[i]);
    }
   }
  }
 }
 ((HudDrawFn)(gBase+0x6facb50))(context,x,y,color,text,flag);
}
static bool InstallHeadNameTrace(){
 constexpr uintptr_t site=0x6ebdf3a,target=0x6facb50;
 unsigned char expected[5]={0xe8};int32_t displacement=(int32_t)(target-site-5);memcpy(expected+1,&displacement,4);
 if(memcmp((void*)(gBase+site),expected,5)){Log("HEAD_TRACE native call guard rejected; no changes");return false;}
 unsigned char* relay=ChoiceRelay(gBase+site,(uintptr_t)&HeadTraceDraw);if(!relay)return false;
 unsigned char call[5]={0xe8};displacement=(int32_t)((intptr_t)relay-(intptr_t)(gBase+site+5));memcpy(call+1,&displacement,4);
 if(!Patch(gBase+site,call,5)){VirtualFree(relay,0,MEM_RELEASE);return false;}
 Log("HEAD_TRACE installed diagnostics only; native draw unchanged; bounded selected-name records=64");return true;
}
