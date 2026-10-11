#include "avatar_marker_identity.h"
#include <assert.h>
int main() {
 using avatarmarker::IsCurrentSource;
 assert(IsCurrentSource(48,48,0,120,0x1000,0x1000));
 assert(!IsCurrentSource(49,48,0,120,0x2000,0x2000)); // identical template, other instance
 assert(!IsCurrentSource(48,48,0,120,0x1000,0x2000)); // stale pointer / slot recycled
 assert(!IsCurrentSource(48,-1,0,120,0x1000,0x1000)); // relation removed
 assert(!IsCurrentSource(48,49,0,120,0x1000,0x1000)); // appearance changed
 assert(IsCurrentSource(7,7,0,120,0x1000,0x1000)); // source moved
 assert(!IsCurrentSource(-1,-1,0,120,0x1000,0x1000));
 assert(!IsCurrentSource(120,120,0,120,0x1000,0x1000));
 assert(!IsCurrentSource(1,1,12,120,0x1000,0x1000));
 assert(!IsCurrentSource(1,1,-1,120,0x1000,0x1000));
 assert(!IsCurrentSource(1,1,0,8193,0x1000,0x1000));
 assert(!IsCurrentSource(1,1,0,120,0,0));
 return 0;
}
