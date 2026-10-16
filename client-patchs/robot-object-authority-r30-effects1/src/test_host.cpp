#include <windows.h>
#include <assert.h>
#include <stdio.h>
int wmain(int argc,wchar_t** argv) {
 assert(argc==3);bool opt=wcscmp(argv[2],L"opt-in")==0;
 SetEnvironmentVariableW(L"DFO_LOCAL_ROBOT_AUTHORITY",opt?L"r16-object-authority-v1":nullptr);
 HMODULE dll=LoadLibraryW(argv[1]);assert(dll);
 auto start=(DWORD(WINAPI*)())GetProcAddress(dll,"ModStart");assert(start&&GetProcAddress(dll,"ModName"));
 assert(start()==(opt?2:0));puts(opt?"PASS wrong image rejected":"PASS inactive without opt-in");
}
