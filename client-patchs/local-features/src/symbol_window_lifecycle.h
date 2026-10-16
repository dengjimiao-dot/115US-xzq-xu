#pragma once
namespace symbol_window_lifecycle {
enum class Layout {Summary,Resonance,Upgrade,Disjoint,Shop,Confirm,Change};
// Original 115US popup enum registrations; these factory slots are empty.
constexpr int id(Layout p){return p==Layout::Summary?4113:p==Layout::Resonance?4127:p==Layout::Upgrade?4118:p==Layout::Disjoint?4105:4159;}
constexpr int index(int n){return n==4113?0:n==4127?1:n==4118?2:n==4105?3:n==4159?4:-1;}
constexpr Layout layout(int n){return n==4113?Layout::Summary:n==4127?Layout::Resonance:n==4118?Layout::Upgrade:n==4105?Layout::Disjoint:Layout::Shop;}
struct Entry {void* object=nullptr;void* manager=nullptr;Layout layout=Layout::Summary;};
struct Cache {
 Entry entries[5]{};
 void* active=nullptr;
 Entry* find(void* p){if(!p)return nullptr;for(auto& e:entries)if(e.object==p)return &e;return nullptr;}
 Entry* find(int n){const int i=index(n);return i<0?nullptr:&entries[i];}
 void created(int n,void* p,void* m,Layout l){auto* e=find(n);if(e)*e={p,m,l};}
 bool closed(void* p){if(active!=p)return false;active=nullptr;return true;}
 void destroyed(void* p){closed(p);if(auto* e=find(p))*e={};}
};
}
