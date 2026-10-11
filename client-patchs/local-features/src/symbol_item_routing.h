#pragma once
// Generated from the paired PVF index and tags. Routing only; Go owns all rules.
namespace symbol_item_routing {
struct Entry {uint32_t id;unsigned char kind;};
static constexpr Entry entries[]={
{490718840u,2},
{490718841u,2},
{490718842u,2},
{490718843u,2},
{490718844u,2},
{490718845u,2},
{490718846u,2},
{490718847u,2},
{490718848u,2},
{490718849u,2},
{490718850u,2},
{490718851u,2},
{490718852u,2},
{490718853u,2},
{490718854u,2},
{490718855u,2},
{490718856u,2},
{490718857u,2},
{490718858u,2},
{490718859u,2},
{490719356u,1},
{490719357u,1},
{490719358u,1},
{490719359u,1},
{490719360u,1},
{490719361u,1},
{490719362u,1},
{490719363u,1},
{490719432u,1},
{490719433u,1},
{490719434u,1},
{490719435u,1},
};
inline unsigned char find(uint32_t id){for(auto& e:entries)if(e.id==id)return e.kind;return 0;}
}
