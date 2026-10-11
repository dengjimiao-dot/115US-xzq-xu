#pragma once
using SelectPreviewFn=void(__fastcall*)(void*,void*);
using PreviewArrowFn=void(__fastcall*)(void*,int);
using ClearPreviewFn=void(__fastcall*)(void*);
using ShowArrowFn=void(__fastcall*)(void*,bool);
static void ShowChoiceArrows(void* owner) {
 if(!gChoice.choices||gChoice.choices->count<2)return;
 for(uintptr_t offset:{uintptr_t(0x1af8),uintptr_t(0x1b08)}){
  uintptr_t node=0,table=0,method=0;
  if(!Read((uintptr_t)owner+offset,node)||!Read(node,table)||!Read(table+0x10,method)||method<gBase||method>=gBase+0x8b00000)continue;
  ((ShowArrowFn)method)((void*)node,true);
 }
}
static void __fastcall SelectChoices(void* owner,void* product) {
 uint32_t outer=0;const PreviewChoices* row=nullptr;
 if(product&&Read((uintptr_t)product+0x1c,outer))row=ChoicesFor(outer);
 gChoice={owner,row,0,GetCurrentThreadId()};
 ((SelectPreviewFn)(gBase+0x771070))(owner,product);
 ShowChoiceArrows(owner);
 if(row)Log("CHOICES selected outer=%u count=%u first=%u",outer,row->count,kChoiceLeaves[row->offset]);
}
static void __fastcall ChoiceArrow(void* owner,int controlId) {
 bool handled=false,next=false;
 if(gChoice.owner==owner&&gChoice.choices&&gChoice.choices->count>1&&gChoice.thread==GetCurrentThreadId()){
  uintptr_t left=0,right=0;int leftId=-1,rightId=-1;
  Read((uintptr_t)owner+0x1af8,left);Read((uintptr_t)owner+0x1b08,right);
  if(left)Read(left+0x150,leftId);if(right)Read(right+0x150,rightId);
  handled=(left&&controlId==leftId)||(right&&controlId==rightId);next=right&&controlId==rightId;
 }
 if(!handled){((PreviewArrowFn)(gBase+0x769870))(owner,controlId);return;}
 const auto* row=gChoice.choices;
 gChoice.index=CycleChoice(gChoice.index,row->count,next);
 uint32_t leaf=kChoiceLeaves[row->offset+gChoice.index];
 // Same reset and application sequence as the native product selection handler.
 ((ClearPreviewFn)(gBase+0x76b140))(owner);
 ((ClearPreviewFn)(gBase+0x76b550))(owner);
 int result=((PreviewFn)(gBase+0x75b100))(owner,leaf);
 ShowChoiceArrows(owner);
 Log("CHOICES switched outer=%u option=%u/%u leaf=%u result=%d",row->outer,gChoice.index+1,row->count,leaf,result);
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
 constexpr uintptr_t sites[]={0x8a20af,0x76aaa3};
 const unsigned char expected[2][5]={{0xe8,0xbc,0xef,0xec,0xff},{0xe8,0xc8,0xed,0xff,0xff}};
 const unsigned char reset1[]={0x40,0x53,0x48,0x83,0xec,0x40,0x48,0xc7};
 const unsigned char reset2[]={0x40,0x53,0x48,0x83,0xec,0x20,0x48,0x8b};
 for(int i=0;i<2;i++)if(memcmp((void*)(gBase+sites[i]),expected[i],5)){Log("CHOICES rejected callsite=%d",i);return false;}
 if(memcmp((void*)(gBase+0x76b140),reset1,8)||memcmp((void*)(gBase+0x76b550),reset2,8)){Log("CHOICES rejected reset function fingerprints");return false;}
 unsigned char* relays[2]={ChoiceRelay(gBase+sites[0],(uintptr_t)&SelectChoices),ChoiceRelay(gBase+sites[1],(uintptr_t)&ChoiceArrow)};
 if(!relays[0]||!relays[1]){for(auto p:relays)if(p)VirtualFree(p,0,MEM_RELEASE);return false;}
 int installed=0;
 for(int i=0;i<2;i++){
  unsigned char call[5]={0xe8};int32_t displacement=(int32_t)((intptr_t)relays[i]-(intptr_t)(gBase+sites[i]+5));memcpy(call+1,&displacement,4);
  if(!Patch(gBase+sites[i],call,5)){for(int j=installed-1;j>=0;j--)Patch(gBase+sites[j],expected[j],5);Log("CHOICES install failed; rollback attempted");return false;}
  installed++;
 }
 Log("CHOICES ready packages=256 choices=822 multi=181; UI verification pending");return true;
}
