#pragma once
#include <cstdint>
#include <cstring>
namespace menufont {
template<class Parent> bool Within(uintptr_t widget,uintptr_t root,Parent parent){
 if(!widget||!root)return false;
 for(unsigned depth=0;depth<32;depth++){
  if(widget==root)return true;
  uintptr_t next=0;if(!parent(widget,next)||!next||next==widget)return false;
  widget=next;
 }return false;
}
inline void Adapt(const void* original,void* effective,uintptr_t font){
 std::memcpy(effective,original,0x78);
 std::memcpy((unsigned char*)effective+0x10,&font,sizeof(font));
}
}
