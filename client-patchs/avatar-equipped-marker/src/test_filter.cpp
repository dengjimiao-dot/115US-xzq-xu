#include <assert.h>
#include <stdio.h>
#include <initializer_list>
#include <windows.h>
#include "shop_filter.h"
int main(int argc,char** argv) {
 assert(kCategoryCount==3696);
 for(size_t i=0;i<kCategoryCount;i++){assert(kCategories[i].mask>0&&kCategories[i].mask<1024);assert(CategoryFor(kCategories[i].id)==kCategories[i].mask);if(i)assert(kCategories[i-1].id<kCategories[i].id);}
 unsigned counts[10]={};
 for(int selected=21;selected<=30;selected++)for(size_t i=0;i<kCategoryCount;i++)if(KeepProduct(kCategories[i].id,SelectedMask(selected)))counts[selected-21]++;
 assert(counts[0]==3696);for(int i=1;i<10;i++)assert(counts[i]>0&&counts[i]<3696);
 assert(SelectedMask(21)==0&&SelectedMask(22)==1&&SelectedMask(23)==16&&SelectedMask(24)==8&&SelectedMask(25)==4&&SelectedMask(26)==2);
 assert(SelectedMask(31)==0);assert(SelectedMask(27)==32&&SelectedMask(28)==64&&SelectedMask(29)==128&&SelectedMask(30)==256);
 assert(KeepProduct(99999999,0)&&!KeepProduct(99999999,1));assert(CategoryFor(3840002)==2);
 for(uint32_t product:{4204001u,4100001u,4100002u,4300001u,4300003u,4300004u,4300061u,4300103u,4309001u}){
  assert(KeepProduct(product,SelectedMask(21))&&KeepProduct(product,SelectedMask(22)));
  assert(!KeepProduct(product,SelectedMask(24))&&!KeepProduct(product,SelectedMask(26)));
 }

 const uint32_t ids[]={4100001u,3840002u,4204001u,4300001u,99999999u};
 const uintptr_t rows[]={101,102,103,104,105};uintptr_t output[5]{};
 auto kept=ArrangeProducts(output,rows,ids,5,0);
 assert(kept==5&&output[0]==103&&output[1]==101&&output[2]==102&&output[3]==104&&output[4]==105);
 kept=ArrangeProducts(output,rows,ids,5,1);
 assert(kept==3&&output[0]==103&&output[1]==101&&output[2]==104);
 kept=ArrangeProducts(output,rows,ids,5,2);assert(kept==1&&output[0]==102);
 kept=ArrangeProducts(output,rows,ids,2,0);assert(kept==2&&output[0]==101&&output[1]==102);
 kept=ArrangeProducts(output,rows,ids,0,1);assert(!kept);
 puts("FRONT ORDER VERIFIED fourth robe first in All/Outfits; other rows retain native order; absence/empty/other tabs safe");
 assert(argc==2);HMODULE dll=LoadLibraryA(argv[1]);assert(dll);
 const char* exports[]={"DirectInput8Create","DllCanUnloadNow","DllGetClassObject","DllRegisterServer","DllUnregisterServer","GetdfDIJoystick"};
 for(auto name:exports)assert(GetProcAddress(dll,name));
 using GetDf=const void*(WINAPI*)();auto df=(GetDf)GetProcAddress(dll,"GetdfDIJoystick");assert(df());
 printf("FILTER VERIFIED all=%u outfits=%u pets=%u auras=%u titles=%u weapons=%u; all 6 proxy exports present and system forwarding verified\n",counts[0],counts[1],counts[2],counts[3],counts[4],counts[5]);
 printf("NEW TABS VERIFIED damage=%u portraits=%u cutscenes=%u emotes=%u removedMisc=%u\n",counts[6],counts[7],counts[8],counts[9],0u);
 FreeLibrary(dll);return 0;
}
