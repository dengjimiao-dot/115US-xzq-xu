#include <windows.h>
#include <assert.h>
#include <stdio.h>
int wmain(int argc,wchar_t** argv){
    assert(argc==3);
    bool opt=wcscmp(argv[2],L"opt-in")==0;
    SetEnvironmentVariableW(L"DFO_LOCAL_DUNGEON_EXIT",opt?L"r13-local-exit-v1":nullptr);
    HMODULE dll=LoadLibraryW(argv[1]);assert(dll);
    auto start=(DWORD(WINAPI*)())GetProcAddress(dll,"ModStart");assert(start);
    assert(GetProcAddress(dll,"ModName"));
    // A process that is not the pinned game must remain unmodified.
    assert(start()==(opt?2:0));
    puts(opt?"PASS incompatible image fails closed":"PASS no opt-in is inactive");
}
