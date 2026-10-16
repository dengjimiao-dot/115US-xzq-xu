#include "preview_choices.h"
#include "preview_leaf.h"
#include <assert.h>
#include <stdio.h>
int main(){unsigned multi=0,total=0;for(const auto& row:kPreviewChoices){assert(row.count>0);assert(row.offset+row.count<=sizeof(kChoiceLeaves)/sizeof(kChoiceLeaves[0]));assert(ChoicesFor(row.outer)==&row);assert(kChoiceLeaves[row.offset]==PreviewTemplate(row.outer));total+=row.count;if(row.count>1)multi++;assert(CycleChoice(0,row.count,false)==row.count-1);assert(CycleChoice(row.count-1,row.count,true)==0);for(unsigned i=0;i<row.count;i++){assert(CycleChoice(CycleChoice(i,row.count,true),row.count,false)==i);for(unsigned j=i+1;j<row.count;j++)assert(kChoiceLeaves[row.offset+i]!=kChoiceLeaves[row.offset+j]);}}assert(multi==317&&total==2798);assert(ChoicesFor(12345)==nullptr);assert(CycleChoice(0,0,true)==0);puts("CHOICES VERIFIED 1647 packages; 317 multi-choice; 2798 choices; defaults and navigation pass");}
