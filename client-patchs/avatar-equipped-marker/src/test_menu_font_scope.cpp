#include "menu_font_scope.h"
#include <cassert>
#include <cstdio>
int main(){
 auto parents=[](uintptr_t p,uintptr_t& n){switch(p){case 1:n=2;break;case 2:n=3;break;case 3:n=0;break;case 4:n=5;break;case 5:n=4;break;default:return false;}return true;};
 assert(menufont::Within(1,3,parents));assert(!menufont::Within(1,6,parents));assert(!menufont::Within(4,3,parents));assert(!menufont::Within(0,3,parents));assert(!menufont::Within(1,0,parents));
 unsigned char a[0x78],b[0x78];for(unsigned i=0;i<sizeof(a);i++)a[i]=(unsigned char)(i*13);
 menufont::Adapt(a,b,0x12345678);for(unsigned i=0;i<sizeof(a);i++)if(i<0x10||i>=0x18)assert(a[i]==b[i]);
 uintptr_t p=0;std::memcpy(&p,b+0x10,8);assert(p==0x12345678);
 puts("menu scope: outside roots/cycles rejected; size, outline, color and all non-font bytes preserved");
}
