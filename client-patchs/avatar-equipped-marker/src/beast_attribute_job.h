#pragma once

// The package attribute window's old ID ranges miss newer avatars and misclassify
// some existing items. The audited usable-job table overrides only those IDs.
#include "avatar_recommend_jobs.h"
static int BeastAttributeJob(uint32_t item) {
 for(unsigned p=0;p<beast115::profiles.size();++p)
  for(auto id:beast115::profiles[p].equipment)
   if(id==item)return beastcontrol::assets[p].job;
 return -1;
}
static int __fastcall PackageAttributeJob(int item) {
 int job=BeastAttributeJob((uint32_t)item);
 if(job<0){int native=((int(__fastcall*)(int))(gBase+0x46ffd20))(item);job=ResolveAvatarRecommendationJob((uint32_t)item,native);if(job==native)return native;}
 static LONG logged=0;
 if(InterlockedIncrement(&logged)<=32)Log("AVATAR attribute package item=%d recognized job=%d",item,job);
 return job;
}
static bool InstallBeastAttributeJob() {
 const uintptr_t site=0x539b992;
 const unsigned char expected[5]={0xe8,0x89,0x43,0x36,0xff};
 const unsigned char classifier[16]={0x45,0x33,0xd2,0x4c,0x8d,0x1d,0xd6,0x02,0x90,0xfb,0x41,0x8b,0xc2,0x45,0x8b,0xca};
 if(memcmp((void*)(gBase+site),expected,5)||memcmp((void*)(gBase+0x46ffd20),classifier,16)) {
  Log("AVATAR attribute classifier rejected client fingerprint");return false;
 }
 auto relay=ChoiceRelay(gBase+site,(uintptr_t)&PackageAttributeJob);
 if(!relay)return false;
 unsigned char call[5]={0xe8};
 int32_t delta=(int32_t)((intptr_t)relay-(intptr_t)(gBase+site+5));
 memcpy(call+1,&delta,4);
 if(!Patch(gBase+site,call,5)){VirtualFree(relay,0,MEM_RELEASE);return false;}
 Log("AVATAR attribute classifier ready: %zu audited IDs; beast mapping and native fallback preserved",sizeof(kAvatarRecommendationJobs)/sizeof(kAvatarRecommendationJobs[0]));
 return true;
}
