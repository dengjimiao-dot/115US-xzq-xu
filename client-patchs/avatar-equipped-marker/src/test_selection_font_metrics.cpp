#include "selection_font_metrics.h"
#include <cstdio>
#include <initializer_list>
#include <cstdlib>
#include <limits>
static void require(bool ok){if(!ok)std::abort();}
int main(){
 using namespace selectionfont;Style original{};for(int i=0;i<sizeof(original);i++)original.bytes[i]=(unsigned char)(i*37+19);float lower=11;memcpy(original.bytes+0x3c,&lower,4);const Style before=original;
 for(float actual:{9.f,12.f,14.5f}){
  Upper upper{0x1100,0x2200,actual,66,true};Style result{};
  require(ForLower(upper,0x1100,0x2200,329,original,result));float got=0;memcpy(&got,result.bytes+0x3c,4);require(got==actual);
  for(int i=0;i<sizeof(original);i++)if(i<0x3c||i>=0x40)require(result.bytes[i]==original.bytes[i]);
  require(memcmp(&original,&before,sizeof(before))==0);
  require(!ForLower(upper,0x1101,0x2200,329,original,result));require(!ForLower(upper,0x1100,0x2201,329,original,result));require(!ForLower(upper,0x1100,0x2200,66,original,result));require(!ForLower(upper,0x1100,0x2200,30,original,result));
  upper.ready=false;require(!ForLower(upper,0x1100,0x2200,329,original,result));
 }
 for(float invalid:{0.f,-1.f,std::numeric_limits<float>::quiet_NaN(),std::numeric_limits<float>::infinity()})require(!ValidSize(invalid));
 puts("SELECTION FONT VERIFIED source-dependent 9/12/14.5 sizes; original immutable; exactly one native field changed; other owner/effect/upper/invalid rejected");
}
