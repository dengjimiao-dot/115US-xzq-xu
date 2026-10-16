#include <windows.h>
#include <assert.h>
int main(int argc,char**) {
    SetEnvironmentVariableW(L"DFO_LOCAL_ROBOT_AI",nullptr);
    if(argc>1) {
        SetEnvironmentVariableW(L"DFO_LOCAL_ROBOT_AI",L"r23-native-input-v1");
        SetEnvironmentVariableW(L"DFO_ROBOT_AI_OFFER",L"C:\\invalid-fixture\\ai-offer.bin");
        SetEnvironmentVariableW(L"DFO_ROBOT_AI_ACK",L"C:\\invalid-fixture\\ai-ack.bin");
    }
    HMODULE m=LoadLibraryW(L"RobotNativeAI.dll");assert(m);
    auto start=(DWORD(WINAPI*)())GetProcAddress(m,"ModStart");assert(start);
    assert(start()==(argc>1?3u:0u));assert(start()==0);
    FreeLibrary(m);
}
