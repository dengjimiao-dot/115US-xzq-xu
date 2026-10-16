#pragma once
#include <stdint.h>
#include <math.h>
struct ShopScrollGeometry { uint32_t rows, maximum; float extent; };
inline ShopScrollGeometry ShopGeometry(uint32_t count,uint32_t columns,uint32_t visibleRows,float gap,float height) {
 if(!columns||!visibleRows||!isfinite(gap)||!isfinite(height)||gap<0||height<=0)return {};
 uint32_t rows=count/columns+(count%columns!=0);
 return {rows,rows>visibleRows?rows-visibleRows:0,rows?rows*(gap+height)-gap:0};
}
inline float ShopBottomOffset(float extent,float padding,int viewport) {
 float result=extent+2*padding-viewport;return result>0?result:0;
}
inline uint32_t ShopMaximumSteps(float extent,float padding,int viewport,float step){
 if(viewport<1||!isfinite(step)||step<=0)return 0;
 return (uint32_t)ceilf(ShopBottomOffset(extent,padding,viewport)/step);
}
