#pragma once
#include <stdint.h>

namespace headtext {
struct RouteInput {
    unsigned depth;
    int category;
    bool enhancement;
    bool overheadStyle;
    bool matchingViews;
};
inline int Category(const RouteInput& in) {
    return in.depth && in.category >= 1 && in.category <= 7 && in.enhancement &&
           in.overheadStyle && in.matchingViews ? 8 : in.category;
}
}
