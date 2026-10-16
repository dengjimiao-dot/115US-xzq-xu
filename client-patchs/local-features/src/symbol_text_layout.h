#pragma once
#include <string>
#include <vector>
namespace symbol_text_layout {
inline std::vector<std::wstring> wrap(const std::wstring& value,size_t columns){
    std::vector<std::wstring> rows;std::wstring row;size_t width=0;
    if(!columns)return rows;
    for(wchar_t c:value){
        if(c==L'\r')continue;
        const size_t n=c<128?1:2;
        if(c==L'\n'||(!row.empty()&&width+n>columns)){
            rows.push_back(row);row.clear();width=0;if(c==L'\n')continue;
        }
        row+=c;width+=n;
    }
    if(!row.empty())rows.push_back(row);return rows;
}
inline std::vector<std::wstring> resonance(const std::wstring& value,size_t columns){
    // Keep the attribute name and its complete signed numeric value apart.
    auto at=value.find_first_of(L"+-0123456789");
    if(at==std::wstring::npos)return wrap(value,columns);
    auto rows=wrap(value.substr(0,at),columns);auto amounts=wrap(value.substr(at),columns);
    rows.insert(rows.end(),amounts.begin(),amounts.end());return rows;
}
}
