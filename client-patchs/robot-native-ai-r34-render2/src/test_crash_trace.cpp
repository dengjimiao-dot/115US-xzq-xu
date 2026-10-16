#include "crash_trace.h"
#include <assert.h>
static volatile LONG id=0;
static void RaiseAndCatch(DWORD code=EXCEPTION_ACCESS_VIOLATION) {
    bool caught=false;
    __try {RaiseException(code,0,0,nullptr);}
    __except(EXCEPTION_EXECUTE_HANDLER){caught=true;}
    assert(caught); // Observer must not swallow, repair or resume this exception.
}
int main() {
    wchar_t dir[MAX_PATH]={},path[MAX_PATH]={};
    assert(GetTempPathW(MAX_PATH,dir));assert(GetTempFileNameW(dir,L"ect",0,path));
    assert(CrashTrace::Start(path,&id));
    RaiseAndCatch(); assert(CrashTrace::count==0);
    id=17;RaiseAndCatch();assert(CrashTrace::count==1);
    RaiseAndCatch(0xe06d7363); assert(CrashTrace::count==2);
    RaiseAndCatch(0xe0000001); assert(CrashTrace::count==2);
    CrashTrace::Stop();
    FILE* f=nullptr;assert(_wfopen_s(&f,path,L"rb")==0);char data[16384]={};
    size_t n=fread(data,1,sizeof(data)-1,f);fclose(f);assert(n>0);
    assert(strstr(data,"dungeon=17")&&strstr(data,"frame=0")&&strstr(data,"CONTINUE_SEARCH")&&strstr(data,"code=0xe06d7363"));
    assert(DeleteFileW(path));
    puts("PASS passive AV observer: scope, context unwind, native exception propagation");
}
