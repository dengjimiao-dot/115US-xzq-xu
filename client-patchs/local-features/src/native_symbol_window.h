#pragma once
#include <array>
#include <string>
#include <vector>
#include "../../elite-companion-pilot/src/inline_hook.h"
#include "native_symbol_widgets.h"
#include "native_symbol_window_binding.h"
#include "symbol_native_ui_abi.h"
#include "symbol_window_lifecycle.h"
#include "symbol_window_repair_binding.h"
#include "symbol_growth_presentation_binding.h"

// The native manager owns allocation, focus, parent attachment, close and
// destruction. No foreign HWND and no writes to manager container internals.
// Install only after the caller verifies the PE and the paired resource hash.
namespace native_symbol_window {
using Factory = void*(__fastcall*)(int,void*);
using Destroy = void*(__fastcall*)(void*,unsigned);
using Tick = void(__fastcall*)(void*);
using InfoLayout=symbol_window_lifecycle::Layout;
struct Callbacks {
    void (*infoOpened)(InfoLayout,bool)=nullptr;
    void (*opened)(const std::string&)=nullptr;
    void (*selected)(const std::string&,uint16_t)=nullptr;
    void (*closed)()=nullptr;
    void (*updated)()=nullptr;
    void (*removed)(const std::string&,uint16_t)=nullptr;
};
static uintptr_t base;
static InlineHook factoryHook;
static native_symbol_widgets::API widgets;
static void* popup;
static void* infoPopup;
static uint64_t infoGeneration=0;
static void* infoManager;
static bool infoRequested=false;
static bool infoRequestInternal=true,pendingInfoInternal=true;
static symbol_window_lifecycle::Cache infoCache;
static InfoLayout infoLayout=InfoLayout::Summary,pendingInfoLayout=InfoLayout::Summary;
static bool reopenInfo=false;
static bool openInfo(InfoLayout layout=InfoLayout::Summary,bool internal=true);
static void* manager;
static FILE* log;
static DWORD uiThread;
static bool closing;
static bool rightWasDown;
static uint16_t selection=0;
static int hoverSocket=-1;
static ULONGLONG flashUntil=0;
static uint16_t flashSocket=0;
static std::string group;
static std::string requestedGroup;
static Callbacks callbacks;
// Include the RTTI pointer preceding the table for native dynamic_cast.
static std::array<uintptr_t,121> table;
static uintptr_t address(uintptr_t va){
#ifdef SYMBOL_WINDOW_TEST_ADDRESS
    return SYMBOL_WINDOW_TEST_ADDRESS(va);
#else
    return base+va-0x140000000ull;
#endif
}
template<class T> static T read(void* p,size_t off=0){
    T v{};SIZE_T n=0;
    if(p)ReadProcessMemory(GetCurrentProcess(),static_cast<char*>(p)+off,&v,sizeof v,&n);
    return n==sizeof v?v:T{};
}
static void release(void* owner){
    if(!owner)return;
    if(InterlockedDecrement(reinterpret_cast<volatile LONG*>(static_cast<char*>(owner)+8))==0){
        auto vt=read<uintptr_t*>(owner);
        reinterpret_cast<void(__fastcall*)(void*)>(vt[0])(owner);
        if(InterlockedDecrement(reinterpret_cast<volatile LONG*>(static_cast<char*>(owner)+12))==0)
            reinterpret_cast<void(__fastcall*)(void*)>(vt[1])(owner);
    }
}
struct Control {
    void* value=nullptr;void* owner=nullptr;
    Control(void* root,const wchar_t* name,bool child=false){
        // Window and widget lookup have different native object layouts.
        // Never send a popup object into IControl's child-tree traversal.
        if(child&&(root==popup||root==infoPopup))return;
        if(root)reinterpret_cast<void*(__fastcall*)(void*,void*,const wchar_t*)>
            (address(child?0x146ec8e30:0x145f6e4e0))(root,this,name);
    }
    ~Control(){release(owner);}
    Control(const Control&)=delete;
};
static void visible(void* control,bool show){
    if(control)reinterpret_cast<void(__fastcall*)(void*,bool)>(read<uintptr_t*>(control)[2])(control,show);
}
static bool clicked(void* control){
    // Same click predicate used by the native popup's close button.
    return control&&reinterpret_cast<bool(__fastcall*)(void*)>(address(0x146ecfd90))(control);
}
static bool hovered(void* control){
    // Continuous hover including descendants. 146ecfff0 is a different
    // transient input identity and does not stay set while the pointer rests.
    return control&&reinterpret_cast<bool(__fastcall*)(void*)>(address(0x146ed0030))(control);
}
static void text(void* control,const std::wstring& value){
    // CNUIControlText::SetText(wchar_t*), virtual +2b0. Native code copies
    // the string into its own allocator; no plugin-owned string is retained.
    if(control&&read<uintptr_t>(control)==address(0x14b1c9490))
        reinterpret_cast<void(__fastcall*)(void*,const wchar_t*)>(address(0x146ef7860))(control,value.c_str());
}
static void styledText(void* control,const std::wstring& value){
    // Replace wrapper clears the owned styled paragraphs before virtual +2a0 append.
    // Factory 146f0d840 -> 146f11010 -> 146f467c0 -> 146f46b50;
    // 146f19840 -> 146f6eab0 -> 146f6ef30 reads length/capacity (16/24).
    static_assert(sizeof(std::wstring)==32,"native const wstring ABI");
    if(control&&read<uintptr_t>(control)==address(0x14b1d61f0))
        reinterpret_cast<bool(__fastcall*)(void*,const std::wstring*,bool)>(address(0x146f19350))(control,&value,false);
}
static void gauge(void* control,uint32_t points){
    // Original XUI reader uses vtable +2a0(current,max), 146f272a0.
    if(control&&points<=100&&read<uintptr_t>(control)==address(0x14b1d0068))
        reinterpret_cast<void(__fastcall*)(void*,uint32_t,uint32_t)>(address(0x146f272a0))(control,points,100);
}
static void animationPath(void* control,const wchar_t* path){
    // Original AnimationForRDAR XUI reader appends owned game strings (146af0220),
    // then selects a path (146af0950), which clears the native ANI player.
    if(control&&path&&read<uintptr_t>(control)==address(0x14b13a9a8))
        reinterpret_cast<void(__fastcall*)(void*,const wchar_t*)>(address(0x146af0220))(control,path);
}
static void animationIndex(void* control,uint32_t index){
    if(control&&read<uintptr_t>(control)==address(0x14b13a9a8))
        reinterpret_cast<void(__fastcall*)(void*,uint32_t)>(address(0x146af0950))(control,index);
}
static void animationRestart(void* control){
    if(control&&read<uintptr_t>(control)==address(0x14b13a9a8)){
        // Native owned-player reset and lazy resource loader. The next draw
        // is rewound by the native restart entry, then advanced by native rendering.
        reinterpret_cast<void(__fastcall*)(void*)>(address(0x146af06e0))(control);
        reinterpret_cast<void(__fastcall*)(void*,bool)>(address(0x146af04c0))(control,false);
        reinterpret_cast<void(__fastcall*)(void*)>(address(0x146af0000))(control);
    }
}
static void position(void* control,float x,float y){
    if(!control)return;
    const float point[2]={x,y};
    reinterpret_cast<void(__fastcall*)(void*,const float*)>(address(0x146ecc720))(control,point);
}
// Original ItemControl setters used by the XUI property reader. Preserve native
// item-name lookup and update the authoritative amount, not the XUI placeholder.
static void itemResult(void* control,uint32_t amount){
    if(!control||read<uintptr_t>(control)!=address(0x14a6cf678))return;
    reinterpret_cast<void(__fastcall*)(void*,int,int)>(address(0x14501c260))(control,36,8);
    reinterpret_cast<void(__fastcall*)(void*,bool)>(address(0x14501c2d0))(control,true);
    reinterpret_cast<void(__fastcall*)(void*,bool)>(address(0x14501c4e0))(control,true);
    reinterpret_cast<void(__fastcall*)(void*,uint32_t)>(address(0x14501c490))(control,amount);
}
static HWND gameWindow(void* control){
    // Generic popup child controls can have no attached HWND at +190.
    auto hwnd=read<HWND>(control,0x190);DWORD pid=0;
    if(hwnd&&IsWindow(hwnd))GetWindowThreadProcessId(hwnd,&pid);
    if(pid==GetCurrentProcessId())return hwnd;
#ifdef SYMBOL_WINDOW_TEST_FOREGROUND
    hwnd=SYMBOL_WINDOW_TEST_FOREGROUND();
#else
    hwnd=GetForegroundWindow();
#endif
    pid=0;if(hwnd)GetWindowThreadProcessId(hwnd,&pid);
    return pid==GetCurrentProcessId()?hwnd:nullptr;
}
static std::wstring itemName(void* control){
    if(!control||read<uintptr_t>(control)!=address(0x14a6cf678))return L"";
    // Native ItemControl render reads the owned MSVC name at +380.
    auto data=static_cast<char*>(control)+0x380;const size_t n=read<size_t>(data,16),cap=read<size_t>(data,24);
    if(!n||n>4096||cap<n)return L"";
    auto chars=cap>7?read<const wchar_t*>(data):reinterpret_cast<const wchar_t*>(data);
    std::wstring out(n,L'\0');SIZE_T copied=0;
    if(!chars||!ReadProcessMemory(GetCurrentProcess(),chars,&out[0],n*sizeof(wchar_t),&copied)||copied!=n*sizeof(wchar_t))return L"";
    return out;
}
static void hideItemName(void* control){
    if(control&&read<uintptr_t>(control)==address(0x14a6cf678))
        reinterpret_cast<void(__fastcall*)(void*,bool)>(address(0x14501c2d0))(control,false);
}
static bool cursorInUI(void* control,float& x,float& y){
    if(!control)return false;
    auto hwnd=gameWindow(control);
    auto layout=read<void*>(reinterpret_cast<void*>(address(0x14e6343e0)));
    if(!hwnd||!layout)return false;
    const uint64_t packed=reinterpret_cast<uint64_t(__fastcall*)(void*)>(address(0x146ec45d0))(hwnd);
    const int cx=int(uint32_t(packed)),cy=int(uint32_t(packed>>32));
    if(cx<0||cy<0)return false;
    const int anchor=reinterpret_cast<int(__fastcall*)(void*)>(address(0x140738710))(layout);
    x=reinterpret_cast<float(__fastcall*)(void*,int,float)>(address(0x146ec6b00))(layout,anchor,float(cx));
    y=reinterpret_cast<float(__fastcall*)(void*,int,float)>(address(0x146ec6c90))(layout,anchor,float(cy));
    return true;
}
static void followCursor(void* control,void* parent){
    float x=0,y=0;if(!control||!parent||!cursorInUI(control,x,y))return;
    x+=16.f;y+=16.f;
    const float width=read<float>(control,0x98),height=read<float>(control,0x9c);
    // Convert viewport edges with the same native anchor as the cursor.
    auto layout=read<void*>(reinterpret_cast<void*>(address(0x14e6343e0)));
    RECT rect{};auto hwnd=gameWindow(control);
    if(layout&&GetClientRect(hwnd,&rect)){
        const int anchor=reinterpret_cast<int(__fastcall*)(void*)>(address(0x140738710))(layout);
        const float right=reinterpret_cast<float(__fastcall*)(void*,int,float)>(address(0x146ec6b00))(layout,anchor,float(rect.right));
        const float bottom=reinterpret_cast<float(__fastcall*)(void*,int,float)>(address(0x146ec6c90))(layout,anchor,float(rect.bottom));
        x=(std::max)(0.f,(std::min)(x,right-width));y=(std::max)(0.f,(std::min)(y,bottom-height));
    }
    position(control,x-read<float>(parent,0xcc),y-read<float>(parent,0xd0));
}
static int scrollPosition(void*control){
    // CNVScroll 146ee58c0 reads child +378, then leaf 14328bd20 (+3a8).
    if(!control||read<uintptr_t>(control)!=address(0x14b1c6270))return 0;
    auto slider=read<void*>(control,0x378);
    return slider?reinterpret_cast<int(__fastcall*)(void*)>(address(0x14328bd20))(slider):0;
}
static void scrollRange(void*control,int maximum){
    if(control&&maximum>=0&&read<uintptr_t>(control)==address(0x14b1c6270))
        reinterpret_cast<void(__fastcall*)(void*,int,bool)>(address(0x146ee5390))(control,maximum,true);
}
static bool pointed(void* control){return control&&reinterpret_cast<bool(__fastcall*)(void*)>(address(0x146eca5a0))(control);}
static void imageIndex(void* control,int index){
    if(control)reinterpret_cast<void(__fastcall*)(void*,int)>(address(0x146eecbb0))(control,index);
}
static void highlight(uint16_t index){
    selection=index;
    if(!popup||group.empty())return;
    Control root(popup,std::wstring(group.begin(),group.end()).c_str());
    for(uint16_t i=0;i<256;++i){
        Control slot(root.value,(L"slot_"+std::to_wstring(i)).c_str(),true);if(!slot.value)break;
        Control selected(slot.value,L"dodge",true);visible(selected.value,i==selection);
    }
}
static const wchar_t* groups[]={L"red_symbol",L"yellow_symbol",L"green_symbol",L"blue_symbol",L"multi_symbol",L"platinum",L"buff_platinum"};
static std::wstring wideGroup(){return std::wstring(group.begin(),group.end());}
static bool knownGroup(const std::string& s){
    std::wstring w(s.begin(),s.end());
    for(auto n:groups)if(w==n)return true;
    return false;
}
static void selectGroup(){
    if(!popup)return;
    auto selected=wideGroup();
    for(auto name:groups){Control c(popup,name);visible(c.value,selected==name);}
}
static void notifyClosed(){
    group.clear();hoverSocket=-1;
    if(callbacks.closed)callbacks.closed();
}
static void* __fastcall destroy(void* self,unsigned flags){
    if(popup==self){popup=nullptr;manager=nullptr;notifyClosed();}
    if(infoPopup==self){infoPopup=nullptr;infoManager=nullptr;}
    infoCache.destroyed(self);
    // Native destructor installs its own table before freeing any children.
    return reinterpret_cast<Destroy>(address(0x145f6dca0))(self,flags);
}
static void __fastcall onClose(void* self){
    // Original generic popup onClose is the empty native method.
    if(self==popup)notifyClosed();
    if(infoCache.closed(self)){infoPopup=nullptr;infoManager=nullptr;}
    reinterpret_cast<Tick>(address(native_symbol_window_binding::methods[0x30/8]))(self);
}
static bool __fastcall onOpen(void* self,void* arg,intptr_t data){
    const bool ok=reinterpret_cast<bool(__fastcall*)(void*,void*,intptr_t)>(address(0x145f6f4b0))(self,arg,data);
    if(ok&&self==popup&&!requestedGroup.empty()){
        group=requestedGroup;requestedGroup.clear();selectGroup();highlight(0);rightWasDown=(GetAsyncKeyState(VK_RBUTTON)&0x8000)!=0;
        if(callbacks.opened)callbacks.opened(group);
    }
    if(ok)if(auto* entry=infoCache.find(self)){
        const bool matches=infoRequested&&symbol_window_lifecycle::id(infoLayout)==symbol_window_lifecycle::id(entry->layout);
        const bool requested=matches&&infoRequestInternal;
        infoCache.active=self;infoPopup=self;infoManager=entry->manager;infoLayout=entry->layout;
        if(matches)infoRequested=false;
        ++infoGeneration;
        if(callbacks.infoOpened)callbacks.infoOpened(infoLayout,requested);
        if(log){fprintf(log,"symbol native info opened id=%d layout=%d generation=%llu requested=%d\n",symbol_window_lifecycle::id(infoLayout),int(infoLayout),infoGeneration,int(requested));fflush(log);}
    }
    return ok;
}
static void pumpInfo();
static void __fastcall tick(void* self){
    if(GetCurrentThreadId()==uiThread)pumpInfo();
    reinterpret_cast<Tick>(address(0x145f70b70))(self);
    if(self==infoPopup&&GetCurrentThreadId()==uiThread){
        if(callbacks.updated)callbacks.updated();
        Control button(self,L"close_button");
        if(clicked(button.value)&&infoManager)
            reinterpret_cast<void(__fastcall*)(void*,void*,bool)>(address(0x146671a70))(infoManager,self,false);
        return;
    }
    if(self!=popup||group.empty()||GetCurrentThreadId()!=uiThread)return;
    hoverSocket=-1;
    DWORD foregroundPID=0;GetWindowThreadProcessId(GetForegroundWindow(),&foregroundPID);
    const bool rightDown=foregroundPID==GetCurrentProcessId()&&(GetAsyncKeyState(VK_RBUTTON)&0x8000)!=0;
    const bool rightPressed=rightDown&&!rightWasDown;rightWasDown=rightDown;
    // Resolve source slots by presence; no duplicated gameplay slot counts.
    Control root(self,wideGroup().c_str());
    for(uint16_t i=0;i<256;++i){
        auto name=L"slot_"+std::to_wstring(i);
        Control slot(root.value,name.c_str(),true);
        if(!slot.value)break;
        Control icon(slot.value,L"emblemcontrol",true);
        Control border(slot.value,L"slot_image",true);
        Control glow(slot.value,L"dodge",true);
        Control animation(slot.value,L"top_ani",true);
        visible(animation.value,i==flashSocket&&GetTickCount64()<flashUntil);
        const bool over=hovered(slot.value)||hovered(icon.value)||hovered(border.value)||hovered(glow.value);
        if(over)hoverSocket=i;
        if(rightPressed&&over&&callbacks.removed){
            highlight(i);callbacks.removed(group,i);break;
        }
        if((clicked(slot.value)||clicked(icon.value)||clicked(border.value)||clicked(glow.value))&&callbacks.selected){
            highlight(i);callbacks.selected(group,i);break;
        }
    }
    if(callbacks.updated)callbacks.updated();
}
static void flash(uint16_t index){flashSocket=index;flashUntil=GetTickCount64()+900;}
static void* popupRoot(){
    // Growth panels render after the inventory; draw the drag icon above them.
    if(infoPopup&&(infoLayout==InfoLayout::Upgrade||infoLayout==InfoLayout::Disjoint||infoLayout==InfoLayout::Resonance))return read<void*>(infoPopup,0xd0);
    return popup&&!group.empty()?read<void*>(popup,0xd0):nullptr;
}
static int dropTarget(){
    if(!popup||group.empty())return -1;
    Control root(popup,wideGroup().c_str());
    for(uint16_t i=0;i<256;++i){
        Control slot(root.value,(L"slot_"+std::to_wstring(i)).c_str(),true);if(!slot.value)break;
        // Native CNUIControl::CheckMousePoint (146ed27d0 -> 146eca5a0).
        // During a drag the hover identity remains owned by the bag cell.
        // This predicate reads the actual cursor and the control's scaled
        // native rectangle, independent of that capture identity.
        // Source slot containers all start at (0,0); their children carry
        // the distinct socket positions. Hit the image, not the container.
        for(auto name:{L"slot_image",L"emblemcontrol"}){
            Control target(slot.value,name,true);
            if(target.value&&reinterpret_cast<bool(__fastcall*)(void*)>(address(0x146eca5a0))(target.value))return i;
        }
    }
    return -1;
}
static const wchar_t* infoPath(InfoLayout layout){
    switch(layout){
    case InfoLayout::Summary:return L"local/20261011/emblem-growth-r4/ui/symbolemblemdetailinfowindow.xui";
    case InfoLayout::Resonance:return L"local/20261011/emblem-growth-r4/ui/symbolresonance.xui";
    case InfoLayout::Upgrade:return L"local/20261011/emblem-growth-r4/ui/upgradeemblem.xui";
    case InfoLayout::Disjoint:return L"local/20261011/emblem-growth-r4/ui/disjointemblem.xui";
    case InfoLayout::Shop:return L"local/20261011/emblem-growth-r4/ab5347b62ddf401549e9.xui";
    case InfoLayout::Confirm:return L"local/20261011/emblem-growth-r4/7a2243028ca1ff1858c2.xui";
    case InfoLayout::Change:return L"local/20261011/emblem-growth-r4/4b30927ba98e5e057c86.xui";
    }return nullptr;
}
static bool reloadInfo(symbol_window_lifecycle::Entry& entry,InfoLayout layout){
    if(entry.layout==layout)return true;
    // 145f6da20 constructor calls this same owned XUI setter at 145f6db9c.
    // Reload only while the object is closed, before manager reattachment.
    if(!entry.object||infoCache.active==entry.object)return false;
    struct NativeString {wchar_t data[8];size_t length=0,capacity=7;};
    NativeString path{};const auto* name=infoPath(layout);
    reinterpret_cast<void*(__fastcall*)(void*,const wchar_t*,size_t)>(address(0x14014c8d0))(&path,name,wcslen(name));
    const bool ok=reinterpret_cast<bool(__fastcall*)(void*,void*)>(address(0x145f6f2e0))(entry.object,&path);
    // The native setter consumes and destroys its by-value string argument.
    if(ok)entry.layout=layout;return ok;
}
static void* __fastcall factory(int id,void* context){
    auto original=reinterpret_cast<Factory>(factoryHook.original());
    if(id!=4111&&symbol_window_lifecycle::index(id)<0)return original(id,context);
    if(!context||(id==4111&&requestedGroup.empty())||(id==4159&&!infoRequested))return original(id,context);
    if(uiThread&&GetCurrentThreadId()!=uiThread)return nullptr;
    uiThread=GetCurrentThreadId();
    const InfoLayout layout=infoRequested&&symbol_window_lifecycle::id(infoLayout)==id?infoLayout:symbol_window_lifecycle::layout(id);
    auto memory=reinterpret_cast<void*(__fastcall*)(size_t)>(address(0x146e9f2a0))(0x620);
    if(!memory)return nullptr;
    // Native std::wstring is consumed by the constructor. Its allocation must
    // use the game's CRT, so build it with the game's string constructor.
    struct NativeString {wchar_t data[8];size_t length=0,capacity=7;};
    static_assert(sizeof(NativeString)==32,"native wstring ABI");
    NativeString path{};
    const wchar_t* name=id==4111?L"local/20261008/symbol_r30/ui/equipemblemwindow.xui":infoPath(layout);
    reinterpret_cast<void*(__fastcall*)(void*,const wchar_t*,size_t)>(address(0x14014c8d0))(&path,name,wcslen(name));
    reinterpret_cast<void*(__fastcall*)(void*,void*,int,void*,int)>(address(0x145f6da20))(memory,context,id,&path,1);
    if(read<uintptr_t>(memory)!=address(native_symbol_window_binding::tableVA)){
        if(log){fputs("symbol popup constructor table mismatch\n",log);fflush(log);}
        reinterpret_cast<Destroy>(address(0x145f6dca0))(memory,1);return nullptr;
    }
    // Only this object receives the private table. Other generic popups keep
    // their original methods and native behavior.
    *static_cast<uintptr_t**>(memory)=table.data()+1;
    if(id==4111){popup=memory;manager=context;}else{infoCache.created(id,memory,context,layout);}
    return memory;
}
static bool render(const std::string& name,const std::vector<uint32_t>& items){
    if(!popup||name!=group||GetCurrentThreadId()!=uiThread)return false;
    Control root(popup,wideGroup().c_str());
    if(!root.value)return false;
    // Require exact source/control agreement before updating any item.
    for(size_t i=0;i<=items.size();++i){
        Control slot(root.value,(L"slot_"+std::to_wstring(i)).c_str(),true);
        if((i<items.size())!=bool(slot.value))return false;
    }
    for(size_t i=0;i<items.size();++i){
        Control slot(root.value,(L"slot_"+std::to_wstring(i)).c_str(),true);
        Control icon(slot.value,L"emblemcontrol",true);
        if(!widgets.setTemplate(icon.value,items[i]))return false;
        Control idle(slot.value,L"idle_ani",true);visible(idle.value,items[i]!=0);
    }
    highlight(selection);
    return true;
}
static bool open(const std::string& name){
    if(!factoryHook.installed||!knownGroup(name))return false;
    if(uiThread&&uiThread!=GetCurrentThreadId())return false;
    uiThread=GetCurrentThreadId();requestedGroup=name;
    auto context=read<void*>(reinterpret_cast<void*>(address(0x14e683c78)));
    if(!context){requestedGroup.clear();return false;}
    // Native manager may defer opening until its safe UI phase. The factory
    // uses the pending group when that request reaches the main UI thread.
    reinterpret_cast<void*(__fastcall*)(void*,int,uintptr_t,uintptr_t)>(address(0x14668c520))(context,4111,0,0);
    return true;
}
static bool openInfo(InfoLayout layout,bool internal){
    if(!factoryHook.installed||(uiThread&&uiThread!=GetCurrentThreadId()))return false;
    uiThread=GetCurrentThreadId();
    auto context=read<void*>(reinterpret_cast<void*>(address(0x14e683c78)));
    if(!context)return false;
    if(infoPopup){
        if(infoLayout==layout)return true;
        pendingInfoLayout=layout;pendingInfoInternal=internal;reopenInfo=true;
        return true;
    }
    if(infoRequested){pendingInfoLayout=layout;pendingInfoInternal=internal;reopenInfo=true;return true;}
    auto* entry=infoCache.find(symbol_window_lifecycle::id(layout));
    if(entry&&entry->object&&!reloadInfo(*entry,layout))return false;
    infoLayout=layout;infoRequestInternal=internal;infoRequested=true;
    reinterpret_cast<void*(__fastcall*)(void*,int,uintptr_t,uintptr_t)>(address(0x14668c520))(context,symbol_window_lifecycle::id(layout),0,0);
    return true;
}
static void pumpInfo(){
    if(!reopenInfo||infoRequested||GetCurrentThreadId()!=uiThread)return;
    if(infoPopup){if(infoManager)reinterpret_cast<void(__fastcall*)(void*,void*,bool)>(address(0x146671a70))(infoManager,infoPopup,false);}
    // onClose clears the active pointer without destroying the cached object.
    // Finish the native close call before opening; a closed popup stops ticking.
    if(!infoPopup){const auto layout=pendingInfoLayout;const bool internal=pendingInfoInternal;reopenInfo=false;openInfo(layout,internal);}
}
static void close(bool includeInfo=true){
    if(closing||GetCurrentThreadId()!=uiThread)return;
    closing=true;
    requestedGroup.clear();
    if(includeInfo){infoRequested=false;reopenInfo=false;
    if(infoPopup&&infoManager)reinterpret_cast<void(__fastcall*)(void*,void*,bool)>(address(0x146671a70))(infoManager,infoPopup,false);}
    if(popup&&manager)reinterpret_cast<void(__fastcall*)(void*,void*,bool)>(address(0x146671a70))(manager,popup,false);
    if(!group.empty())notifyClosed();
    closing=false;
}
static bool install(uintptr_t imageBase,FILE* sink,Callbacks handlers){
    if(factoryHook.installed)return false;
    base=imageBase;log=sink;
    if(!widgets.bind(base))return false;
    for(auto& g:native_symbol_window_binding::guards){
        auto p=reinterpret_cast<void*>(address(g.va));
        if(!Readable(p,g.size)||memcmp(p,g.bytes,g.size))return false;
    }
    for(auto& g:symbol_window_repair_binding::guards){
        auto p=reinterpret_cast<void*>(address(g.va));
        if(!Readable(p,g.size)||memcmp(p,g.bytes,g.size))return false;
    }
    for(auto& g:symbol_native_ui_abi::guards){
        auto p=reinterpret_cast<void*>(address(g.va));
        if(!Readable(p,g.size))return false;
        if(g.size==8){uintptr_t expected=0;memcpy(&expected,g.bytes,8);if(read<uintptr_t>(p)!=address(expected))return false;}
        else if(memcmp(p,g.bytes,g.size))return false;
    }
    for(const auto& g:symbol_growth_presentation_binding::guards){
        auto p=reinterpret_cast<void*>(address(g.va));
        if(!Readable(p,g.size)||memcmp(p,g.bytes,g.size))return false;
    }
    auto vt=reinterpret_cast<uintptr_t*>(address(native_symbol_window_binding::tableVA));
    if(!Readable(vt-1,sizeof table))return false;
    for(size_t i=0;i<120;++i)if(vt[i]!=address(native_symbol_window_binding::methods[i]))return false;
    memcpy(table.data(),vt-1,sizeof table);
    table[1]=reinterpret_cast<uintptr_t>(destroy);
    table[1+0x18/8]=reinterpret_cast<uintptr_t>(tick);
    table[1+0x28/8]=reinterpret_cast<uintptr_t>(onOpen);
    table[1+0x30/8]=reinterpret_cast<uintptr_t>(onClose);
    auto& prologue=native_symbol_window_binding::guards[0];
    if(!PrepareHook(factoryHook,reinterpret_cast<void*>(address(prologue.va)),prologue.bytes,prologue.size,reinterpret_cast<void*>(factory)))return false;
    callbacks=handlers;
    return PublishHook(factoryHook);
}
}
