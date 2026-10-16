#pragma once
#include <stdint.h>
#include <limits.h>
// Spacing is an adapter behavior choice. Combat offsets stay small so the
// native melee lane/range still works; native pathfinding handles obstructions.
static int OffsetGoal(int value,unsigned seat,bool y,bool combat) {
 if(seat>=3)return value;
 static const int followX[3]={-72,0,72},followY[3]={-24,24,0};
 static const int combatX[3]={-18,0,18},combatY[3]={-8,8,0};
 int offset=combat?(y?combatY[seat]:combatX[seat]):(y?followY[seat]:followX[seat]);
 int64_t n=int64_t(value)+offset;return int(n<INT_MIN?INT_MIN:n>INT_MAX?INT_MAX:n);
}
static unsigned InitialBrainDelay(unsigned seat,unsigned identity) {return 120+seat*330+(identity*37)%97;}
static unsigned InitialAwakeningDelay(unsigned seat,unsigned identity) {return 400+seat*1100+(identity*53)%251;}
