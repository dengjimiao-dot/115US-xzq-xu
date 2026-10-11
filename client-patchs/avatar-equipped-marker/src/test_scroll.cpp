#include "scroll_geometry.h"
#include "preview_leaf.h"
#include <assert.h>
#include <stdio.h>
#include <initializer_list>
int main(){
 const uint32_t counts[]={0,1,2,3,8,9,10,85,113,145,172,377,722,2286,3667};
 for(auto n:counts){auto g=ShopGeometry(n,3,3,20,225);assert(g.rows==(n+2)/3);assert(g.maximum==(g.rows>3?g.rows-3:0));assert(g.extent>=0);
  for(int viewport:{480,645,735,960}){float bottom=ShopBottomOffset(g.extent,0,viewport);assert(bottom>=0);if(g.extent>viewport)assert(fabs(g.extent-bottom-viewport)<0.01);else assert(bottom==0);auto maximum=ShopMaximumSteps(g.extent,0,viewport,245);assert(maximum*245>=bottom);assert(!maximum||(maximum-1)*245<bottom);assert(!n||maximum*245<g.extent);}
 }
 assert(ShopGeometry(893,0,3,20,225).extent==0);assert(ShopGeometry(893,3,3,20,0).extent==0);
 auto pet=ShopGeometry(145,3,3,20,225),all=ShopGeometry(893,3,3,20,225);assert(pet.maximum==46&&all.maximum==295);assert(pet.extent<all.extent);
 assert(PreviewTemplate(910100014)==500330644);assert(PreviewTemplate(910100012)==500990771);
 assert(ShopMaximumSteps(ShopGeometry(9,3,3,20,225).extent,0,645,245)==1);
 puts("SCROLL VERIFIED partial rows, empty/short lists, all ten category counts, exact bottom alignment; rabbit title preview mapping verified");
}
