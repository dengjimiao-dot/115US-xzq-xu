#pragma once
#include "beast_native_guards.h"
#include "beast_button_pixels.h"
namespace beastcontrol {
static HWND gameWindow,hudWindow;
static int hudHover=-1,hudPress=-1;
static BOOL CALLBACK FindGame(HWND w,LPARAM) {
 DWORD pid=0;GetWindowThreadProcessId(w,&pid);RECT r{};
 if(pid==GetCurrentProcessId()&&w!=hudWindow&&IsWindowVisible(w)&&!GetWindow(w,GW_OWNER)&&GetClientRect(w,&r)&&r.right>=400&&r.bottom>=300){gameWindow=w;return FALSE;}return TRUE;
}
static int HudHit(LPARAM p){int x=(short)LOWORD(p),y=(short)HIWORD(p);if(x<0||x>=28)return -1;return y>=0&&y<28?0:y>=31&&y<59?1:-1;}
static void DrawHud() {
 if(!gameWindow)return;RECT r{};GetClientRect(gameWindow,&r);POINT dest{(r.right-800)/2+144,r.bottom-145};ClientToScreen(gameWindow,&dest);
 HDC screen=GetDC(nullptr),dc=CreateCompatibleDC(screen);BITMAPINFO bi{};bi.bmiHeader.biSize=sizeof(BITMAPINFOHEADER);bi.bmiHeader.biWidth=28;bi.bmiHeader.biHeight=-59;bi.bmiHeader.biPlanes=1;bi.bmiHeader.biBitCount=32;bi.bmiHeader.biCompression=BI_RGB;void* pixels=nullptr;
 HBITMAP bitmap=CreateDIBSection(dc,&bi,DIB_RGB_COLORS,&pixels,nullptr,0);if(!bitmap||!pixels){if(bitmap)DeleteObject(bitmap);DeleteDC(dc);ReleaseDC(nullptr,screen);return;}
 memset(pixels,0,28*59*4);unsigned frames[2]={InterlockedCompareExchange(&hudSummoned,0,0)?4u:0u,InterlockedCompareExchange(&hudMounted,0,0)?12u:8u};
 for(int n=0;n<2;n++){unsigned state=hudPress==n?2:hudHover==n?1:0;const auto* source=BeastButtonPixels[frames[n]+state];for(int y=0;y<28;y++)memcpy((unsigned char*)pixels+((y+n*31)*28)*4,source+y*28*4,28*4);}
 auto previous=SelectObject(dc,bitmap);SIZE size{28,59};POINT origin{};BLENDFUNCTION blend{AC_SRC_OVER,0,255,AC_SRC_ALPHA};UpdateLayeredWindow(hudWindow,screen,&dest,&size,dc,&origin,0,&blend,ULW_ALPHA);SelectObject(dc,previous);DeleteObject(bitmap);DeleteDC(dc);ReleaseDC(nullptr,screen);
}
static LRESULT CALLBACK HudProc(HWND w,UINT message,WPARAM a,LPARAM b) {
 switch(message){
 case WM_MOUSEACTIVATE:return MA_NOACTIVATE;
 case WM_MOUSEMOVE:{hudHover=HudHit(b);TRACKMOUSEEVENT track{sizeof(track),TME_LEAVE,w,0};TrackMouseEvent(&track);DrawHud();return 0;}
 case WM_MOUSELEAVE:hudHover=-1;DrawHud();return 0;
 case WM_LBUTTONDOWN:hudPress=HudHit(b);if(hudPress>=0)SetCapture(w);DrawHud();return 0;
 case WM_LBUTTONUP:{int hit=HudHit(b),pressed=hudPress;hudPress=-1;ReleaseCapture();if(hit>=0&&hit==pressed)InterlockedExchange(&pending,hit+1);DrawHud();return 0;}
 case WM_CAPTURECHANGED:hudPress=-1;DrawHud();return 0;
 case WM_TIMER:{
  if(!IsWindow(gameWindow)){gameWindow=nullptr;EnumWindows(FindGame,0);}
  uintptr_t town=Town();
  DWORD lastDraw=(DWORD)InterlockedCompareExchange(&townDrawTime,0,0);
  if(!gameWindow||ShopDisplayed()||!InterlockedCompareExchange(&hudVisible,0,0)||!lastDraw||GetTickCount()-lastDraw>350||!town||town!=owner||!Scene(town)||GetForegroundWindow()!=gameWindow||IsIconic(gameWindow)){ShowWindow(w,SW_HIDE);return 0;}
  DrawHud();SetWindowPos(w,HWND_TOPMOST,0,0,0,0,SWP_NOMOVE|SWP_NOSIZE|SWP_NOACTIVATE|SWP_SHOWWINDOW);return 0;}
 case WM_DESTROY:PostQuitMessage(0);return 0;
 }return DefWindowProcW(w,message,a,b);
}
static DWORD WINAPI HudThread(void*) {
 WNDCLASSW c{};c.hInstance=gSelf;c.lpfnWndProc=HudProc;c.lpszClassName=L"DFO115BeastControls";c.hCursor=LoadCursorW(nullptr,MAKEINTRESOURCEW(32512));if(!RegisterClassW(&c))return 0;
 EnumWindows(FindGame,0);hudWindow=CreateWindowExW(WS_EX_LAYERED|WS_EX_NOACTIVATE|WS_EX_TOOLWINDOW,L"DFO115BeastControls",L"F9/F10",WS_POPUP,0,0,28,59,gameWindow,nullptr,gSelf,nullptr);if(!hudWindow)return 0;
 SetTimer(hudWindow,1,100,nullptr);MSG m{};while(GetMessageW(&m,nullptr,0,0)>0){TranslateMessage(&m);DispatchMessageW(&m);}return 0;
}
static bool InstallController() {
 struct Slot{uintptr_t table,offset,original,replacement;};
 Slot slots[4+sizeof(BattleTables)/sizeof(BattleTables[0])]={{TownTable,0x70,TownUpdate,(uintptr_t)&UpdateTown},{TownTable,0x380,NativeMotion,(uintptr_t)&Motion},{TownTable,0x340,0x5bdaaa0,(uintptr_t)&TownHeight},{TownTable,0x60,0x5bc36f0,(uintptr_t)&DrawTown}};
 for(unsigned n=0;n<sizeof(BattleTables)/sizeof(BattleTables[0]);n++)slots[n+4]={BattleTables[n],0x70,BattleUpdate,(uintptr_t)&UpdateBattle};
 if(!gBase)return false;
 for(const auto& f:NativeFingerprints)if(memcmp((void*)(gBase+f.rva),f.bytes,16)){Log("BEAST rejected native function fingerprint rva=%zx",f.rva);return false;}
 for(auto table:BattleTables){uintptr_t job=0,manager=0;if(!Read(gBase+table+0x12e0,job)||job!=gBase+0x5ce08f0||!Read(gBase+table+0xbe0,manager)||manager!=gBase+0x5c14ec0)return false;}
 for(const auto& s:slots){uintptr_t old=0;if(!Read(gBase+s.table+s.offset,old)||old!=gBase+s.original){Log("BEAST rejected actor vtable table=%zx slot=%zx",s.table,s.offset);return false;}}
 if(memcmp((void*)(gBase+0x5bdaa90),"\x8b\x81\xa0\x1f\x00\x00\xc3",7)){Log("BEAST rejected native action getter");return false;}
 const uintptr_t sites[]={0x5be4aff,0x5be4cbe,0x5bc590f,0x6fac479,0x6ff4545};
 constexpr unsigned siteCount=sizeof(sites)/sizeof(sites[0]);
 unsigned char expected[][5]={{0xe8,0x1c,0x3c,0xff,0xff},{0xe8,0x0d,0x38,0xff,0xff},{0xe8,0,0,0,0},{0xe8,0xd2,0xf6,0xff,0xff},{0xe8,0xe6,0xd0,0xff,0xff}};
 int32_t nameDelta=(int32_t)(0x5bcd540-0x5bc590f-5);memcpy(expected[2]+1,&nameDelta,4);
 const uintptr_t callbacks[]={(uintptr_t)&DisguiseUpdate,(uintptr_t)&DisguiseCreate,(uintptr_t)&ObserveName,(uintptr_t)&ObserveVectorCoordinates,(uintptr_t)&ObserveBitmapCoordinates};
 unsigned char* relays[siteCount]{};
 for(unsigned n=0;n<siteCount;n++)if(memcmp((void*)(gBase+sites[n]),expected[n],5)){Log("BEAST rejected disguise call fingerprint");return false;}
 for(unsigned n=0;n<siteCount;n++){
  uintptr_t call=gBase+sites[n];
  for(uintptr_t distance=0x10000;distance<0x70000000;distance+=0x10000){relays[n]=(unsigned char*)VirtualAlloc((void*)((call&~(uintptr_t)0xffff)+distance),0x1000,MEM_RESERVE|MEM_COMMIT,PAGE_READWRITE);if(relays[n])break;}
  if(!relays[n]){for(unsigned j=0;j<n;j++)VirtualFree(relays[j],0,MEM_RELEASE);return false;}
  relays[n][0]=0xff;relays[n][1]=0x25;memset(relays[n]+2,0,4);memcpy(relays[n]+6,&callbacks[n],8);DWORD protection;
  if(!VirtualProtect(relays[n],0x1000,PAGE_EXECUTE_READ,&protection)){for(unsigned j=0;j<=n;j++)VirtualFree(relays[j],0,MEM_RELEASE);return false;}
  FlushInstructionCache(GetCurrentProcess(),relays[n],14);
 }
 unsigned installed=0;
 unsigned calls=0;bool success=true;
 for(const auto& s:slots){if(!Patch(gBase+s.table+s.offset,&s.replacement,8)){success=false;break;}installed++;}
 if(success)for(unsigned n=0;n<siteCount;n++){unsigned char call[5]={0xe8};int32_t delta=(int32_t)((intptr_t)relays[n]-(intptr_t)(gBase+sites[n]+5));memcpy(call+1,&delta,4);if(!Patch(gBase+sites[n],call,5)){success=false;break;}calls++;}
 if(!success){for(unsigned n=0;n<calls;n++)Patch(gBase+sites[n],expected[n],5);for(unsigned n=0;n<installed;n++){uintptr_t original=gBase+slots[n].original;Patch(gBase+slots[n].table+slots[n].offset,&original,8);}for(auto r:relays)VirtualFree(r,0,MEM_RELEASE);return false;}
 HANDLE thread=CreateThread(nullptr,0,HudThread,nullptr,0,nullptr);if(thread)CloseHandle(thread);
 Log("BEAST controller candidate loaded: native town summon/ride, 34 job/set appearance mappings; fourth set source footprints and transitions, F9/F10 controls; dungeon current-equipment 8-piece gate; visual validation required; bounded name/height/final-coordinates observation attempt 2");return true;
}
}
