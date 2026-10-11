#pragma once
using SelectPreviewFn=void(__fastcall*)(void*,void*);
using DropdownEventFn=void*(__fastcall*)(void*,void*,uintptr_t,uintptr_t);
using ClearPreviewFn=void(__fastcall*)(void*);
using SetVisibleFn=void(__fastcall*)(void*,bool);
using AddChoiceFn=uintptr_t(__fastcall*)(void*,const wchar_t*,int,const wchar_t*,uint32_t,uint32_t,uint32_t,uint32_t,uintptr_t*,bool,bool,uintptr_t*);
using SelectChoiceFn=void(__fastcall*)(void*,int,uintptr_t*,uintptr_t);
using SelectedChoiceFn=int(__fastcall*)(void*);
static bool gPopulating;
static void ChoiceVisible(void* node,bool show) {
 uintptr_t table=0,method=0;
 if(!Read((uintptr_t)node,table)||!Read(table+0x10,method)||method<gBase||method>=gBase+0x8b00000)return;
 ((SetVisibleFn)method)(node,show);
}
static bool PopulateChoices(void* owner) {
 if(!gChoice.choices)return false;
 uintptr_t combo=0;unsigned char ready=0;
 if(!Read((uintptr_t)owner+0x1b40,combo)||!Read(combo+8,ready)||!ready)return false;
 uint32_t c1=0,c2=0,c3=0;
 if(!Read(gBase+0xf1c0918,c1)||!Read(gBase+0xf1c091c,c2)||!Read(gBase+0xf1c0920,c3))return false;
 gPopulating=true;
 ((ClearPreviewFn)(gBase+0x6f1e010))((void*)combo);
 ChoiceVisible((void*)combo,true);
 for(uint32_t i=0;i<gChoice.choices->count;i++) {
  wchar_t text[32];swprintf_s(text,L"款式 %u",i+1);
  uintptr_t emptyCustom=0,emptyShared[2]={0,0};
  uintptr_t result=((AddChoiceFn)(gBase+0x6f1ad60))((void*)combo,text,(int)i,nullptr,c1,c2,c2,c3,&emptyCustom,true,true,emptyShared);
  if(result!=1){((ClearPreviewFn)(gBase+0x6f1e010))((void*)combo);ChoiceVisible((void*)combo,false);gPopulating=false;Log("CHOICES dropdown rejected row=%u result=%zx",i,result);return false;}
 }
 uintptr_t emptySelection[2]={0,0};
 ((SelectChoiceFn)(gBase+0x6f1f2f0))((void*)combo,0,emptySelection,0);
 gPopulating=false;
 Log("CHOICES dropdown populated outer=%u count=%u combo=%p",gChoice.choices->outer,gChoice.choices->count,(void*)combo);
 return true;
}
static void __fastcall SelectChoices(void* owner,void* product) {
 uint32_t outer=0;const PreviewChoices* row=nullptr;
 if(product&&Read((uintptr_t)product+0x1c,outer))row=ChoicesFor(outer);
 gChoice={owner,row,0,GetCurrentThreadId()};
 gPopulating=true;
 ((SelectPreviewFn)(gBase+0x771070))(owner,product);
 if(row&&!PopulateChoices(owner))gChoice.choices=nullptr;
 gPopulating=false;
}
static void* __fastcall DropdownEvent(void* owner,void* output,uintptr_t controlId,uintptr_t eventKind) {
 void* result=((DropdownEventFn)(gBase+0x76a780))(owner,output,controlId,eventKind);
 if(gPopulating||gChoice.owner!=owner||!gChoice.choices||gChoice.thread!=GetCurrentThreadId()||(uint32_t)eventKind!=0x18)return result;
 uintptr_t combo=0;int id=-1;
 if(!Read((uintptr_t)owner+0x1b40,combo)||!Read(combo+0x150,id)||(uint32_t)id!=(uint32_t)controlId)return result;
 int index=((SelectedChoiceFn)(gBase+0x206bb60))((void*)combo);
 if(index<0||(uint32_t)index>=gChoice.choices->count||(uint32_t)index==gChoice.index)return result;
 gChoice.index=(uint32_t)index;
 const auto* row=gChoice.choices;uint32_t leaf=kChoiceLeaves[row->offset+gChoice.index];
 // Clear the preview model, while preserving populated dropdown widgets.
 ((ClearPreviewFn)(gBase+0x76b550))(owner);
 int category=((PreviewFn)(gBase+0x75b100))(owner,leaf);
 ChoiceVisible((void*)combo,true);
 Log("CHOICES dropdown switched outer=%u option=%u/%u leaf=%u category=%d",row->outer,gChoice.index+1,row->count,leaf,category);
 return result;
}
static unsigned char* ChoiceRelay(uintptr_t callsite,uintptr_t callback) {
 unsigned char* relay=nullptr;
 for(uintptr_t distance=0x10000;distance<0x70000000;distance+=0x10000){relay=(unsigned char*)VirtualAlloc((void*)((callsite&~uintptr_t(0xffff))+distance),0x1000,MEM_RESERVE|MEM_COMMIT,PAGE_READWRITE);if(relay)break;}
 if(!relay)return nullptr;
 relay[0]=0xff;relay[1]=0x25;memset(relay+2,0,4);memcpy(relay+6,&callback,8);DWORD old;
 if(!VirtualProtect(relay,0x1000,PAGE_EXECUTE_READ,&old)){VirtualFree(relay,0,MEM_RELEASE);return nullptr;}
 FlushInstructionCache(GetCurrentProcess(),relay,14);return relay;
}
static bool InstallChoicePreview() {
 constexpr uintptr_t sites[]={0x8a20af,0x8a213b};
 const unsigned char expected[2][5]={{0xe8,0xbc,0xef,0xec,0xff},{0xe8,0x40,0x86,0xec,0xff}};
 for(int i=0;i<2;i++)if(memcmp((void*)(gBase+sites[i]),expected[i],5)){Log("CHOICES rejected dropdown callsite=%d",i);return false;}
 // API signature audited from the native fashion dropdown population path.
 unsigned char selectedHead[]={0x8b,0x81,0xd0,3,0,0,0xc3};
 if(memcmp((void*)(gBase+0x206bb60),selectedHead,sizeof(selectedHead))){Log("CHOICES rejected selected-item layout");return false;}
 unsigned char* relays[2]={ChoiceRelay(gBase+sites[0],(uintptr_t)&SelectChoices),ChoiceRelay(gBase+sites[1],(uintptr_t)&DropdownEvent)};
 if(!relays[0]||!relays[1]){for(auto p:relays)if(p)VirtualFree(p,0,MEM_RELEASE);return false;}
 int installed=0;
 for(int i=0;i<2;i++){
  unsigned char call[5]={0xe8};int32_t displacement=(int32_t)((intptr_t)relays[i]-(intptr_t)(gBase+sites[i]+5));memcpy(call+1,&displacement,4);
  if(!Patch(gBase+sites[i],call,5)){for(int j=installed-1;j>=0;j--)Patch(gBase+sites[j],expected[j],5);Log("CHOICES dropdown install failed; rollback attempted");return false;}
  installed++;
 }
 Log("CHOICES dropdown ready packages=258 choices=826 multi=183; UI verification pending");return true;
}
