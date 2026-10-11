#pragma once
// Visual fields only, projected from original CN CTP [ui info]. No gameplay rules.
namespace symbol_growth_ui_binding {
struct Color {int symbol,normal,dodge;const wchar_t* name;const wchar_t* description;const wchar_t* progress;const wchar_t* fail;const wchar_t* upgrade;};
static const Color colors[]={
{2,4,5,L"\u7ea2\u8272\u5171\u9e23\u4e4b\u5149",L"\u5c1d\u8bd5\u5171\u9e23\u7ea2\u8272\u661f\u5fbd\u3002",L"local/20261011/emblem-growth-r4/symbolresonance/nodbefore_r_00.ani",L"local/20261011/emblem-growth-r4/symbolresonance/nodfail_r_01.ani",L"local/20261011/emblem-growth-r4/symbolresonance/symbolupeffect_r_00.ani"},
{3,6,7,L"\u9ec4\u8272\u5171\u9e23\u4e4b\u5149",L"\u5c1d\u8bd5\u5171\u9e23\u9ec4\u8272\u661f\u5fbd\u3002",L"local/20261011/emblem-growth-r4/symbolresonance/nodbefore_y_00.ani",L"local/20261011/emblem-growth-r4/symbolresonance/nodfail_y_01.ani",L"local/20261011/emblem-growth-r4/symbolresonance/symbolupeffect_y_00.ani"},
{4,8,9,L"\u7eff\u8272\u5171\u9e23\u4e4b\u5149",L"\u5c1d\u8bd5\u5171\u9e23\u7eff\u8272\u661f\u5fbd\u3002",L"local/20261011/emblem-growth-r4/symbolresonance/nodbefore_g_00.ani",L"local/20261011/emblem-growth-r4/symbolresonance/nodfail_g_01.ani",L"local/20261011/emblem-growth-r4/symbolresonance/symbolupeffect_g_00.ani"},
{5,10,11,L"\u84dd\u8272\u5171\u9e23\u4e4b\u5149",L"\u5c1d\u8bd5\u5171\u9e23\u84dd\u8272\u661f\u5fbd\u3002",L"local/20261011/emblem-growth-r4/symbolresonance/nodbefore_b_00.ani",L"local/20261011/emblem-growth-r4/symbolresonance/nodfail_b_01.ani",L"local/20261011/emblem-growth-r4/symbolresonance/symbolupeffect_b_00.ani"},
{1,2,3,L"\u5f69\u8272\u5171\u9e23\u4e4b\u5149",L"\u5c1d\u8bd5\u5171\u9e23\u6240\u6709\u661f\u5fbd\u3002",L"local/20261011/emblem-growth-r4/symbolresonance/nodbefore_m_00.ani",L"local/20261011/emblem-growth-r4/symbolresonance/nodfail_m_01.ani",L""},
};
}
