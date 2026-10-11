#include "shop115.cpp"
#include <cassert>
int main() {
 auto archer=beast115::profiles[2].equipment;
 assert(beastprobe::eligible(16,archer));
 assert(!beastprobe::eligible(0,archer));
 assert(!beastprobe::eligible(14,archer));
 archer[3]=0;assert(!beastprobe::eligible(16,archer));
 archer=beast115::profiles[2].equipment;
 archer[3]=beast115::profiles[0].equipment[3];assert(!beastprobe::eligible(16,archer));
 assert(!beastprobe::CurrentArcher(0));
 assert(!beastprobe::Install());
 puts("BEAST GUARDS VERIFIED archer-only; missing/mixed sets rejected; absent runtime rejected");
}
