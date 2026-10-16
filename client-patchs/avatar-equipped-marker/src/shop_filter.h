#pragma once
#include <stdint.h>
#include <stddef.h>
struct ProductCategory { uint32_t id; uint32_t mask; };
extern const ProductCategory kCategories[];
extern const size_t kCategoryCount;
inline uint32_t CategoryFor(uint32_t id) {
 size_t lo=0,hi=kCategoryCount;
 while(lo<hi){size_t mid=lo+(hi-lo)/2;if(kCategories[mid].id<id)lo=mid+1;else hi=mid;}
 return lo<kCategoryCount&&kCategories[lo].id==id?kCategories[lo].mask:0;
}
inline uint32_t SelectedMask(int selected) {
 const uint32_t masks[]={0,1,16,8,4,2,32,64,128,256};
 return selected>=21&&selected<=30?masks[selected-21]:0;
}
inline bool KeepProduct(uint32_t id,uint32_t selectedMask) {return selectedMask==0||(CategoryFor(id)&selectedMask)!=0;}

// User-requested presentation policy: pin fourth robe in All and Outfits.
// Product eligibility/pricing remains defined by the existing PVF catalogue.
inline size_t ArrangeProducts(uintptr_t* output,const uintptr_t* rows,const uint32_t* ids,size_t count,uint32_t mask) {
 const bool pin=mask==0||mask==1;size_t kept=0;
 if(pin)for(size_t i=0;i<count;i++)if(ids[i]==4204001u&&KeepProduct(ids[i],mask))output[kept++]=rows[i];
 for(size_t i=0;i<count;i++)if(KeepProduct(ids[i],mask)&&(!pin||ids[i]!=4204001u))output[kept++]=rows[i];
 return kept;
}
