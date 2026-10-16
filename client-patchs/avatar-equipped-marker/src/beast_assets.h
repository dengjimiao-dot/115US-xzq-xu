#pragma once
namespace beastcontrol {
struct Assets {int job; int battleRow; const wchar_t* idle; unsigned count; std::array<int,9> appearance;};
inline constexpr Assets assets[] = {
{0,9001,L"uniquebeast/swordman/animation/unique_1st/stay_body.ani",9,{{-1,-1,-1,787990101,-1,-1,787990102,-1,-1}}},
{11,9002,L"uniquebeast/at_swordman/animation/unique_1st/sinsuatswordman1ststay_03.ani",9,{{-1,-1,-1,787990201,-1,-1,787990202,-1,-1}}},
{16,9003,L"uniquebeast/archer/animation/unique_1st/02_stay/archerstay_04.ani",9,{{-1,-1,-1,787990301,-1,-1,787990302,-1,-1}}},
{3,9004,L"uniquebeast/mage/animation/unique_1st/stay_body.ani",9,{{-1,-1,-1,787990401,-1,-1,787990402,-1,-1}}},
{7,9005,L"uniquebeast/at_fighter/animation/unique_1st/sinsuatfighterstay_04.ani",9,{{-1,-1,-1,787990501,-1,-1,787990502,-1,-1}}},
{1,9006,L"uniquebeast/fighter/animation/unique_1st/sinsufighterstay_00.ani",9,{{-1,-1,-1,787990601,-1,-1,787990602,-1,-1}}},
{2,9007,L"uniquebeast/gunner/animation/unique_1st/sinsugunnerstay_02.ani",9,{{-1,-1,-1,787990701,-1,-1,787990702,-1,-1}}},
{5,9008,L"uniquebeast/at_gunner/animation/unique_1st/sinsuatgunnerstay_00.ani",9,{{-1,-1,-1,787990801,-1,-1,787990802,-1,-1}}},
{8,9009,L"uniquebeast/at_mage/animation/unique_1st/sinsuatmagestand_06.ani",9,{{-1,-1,-1,787990901,-1,787990915,787990902,-1,-1}}},
{4,9010,L"uniquebeast/priest/animation/unique_1st/sinsupriest1ststay_00.ani",9,{{-1,-1,-1,787991001,-1,-1,787991002,-1,-1}}},
{14,9011,L"uniquebeast/at_priest/animation/unique_1st/sinsuatprieststay_03.ani",9,{{-1,-1,-1,787991101,-1,-1,787991102,-1,-1}}},
{6,9012,L"uniquebeast/thief/animation/unique_1st/sinsuthiefstay_03.ani",9,{{-1,-1,-1,787991201,-1,-1,787991202,-1,-1}}},
{12,9013,L"uniquebeast/knight/animation/unique_1st/sinsuknightstandby_sinsu_knight_1st.ani",9,{{-1,-1,-1,787991301,-1,787991315,787991302,-1,-1}}},
{13,9014,L"uniquebeast/demonic_lancer/animation/unique_1st/sinsudemoniclancerstandby_standby.ani",9,{{-1,-1,-1,787991401,-1,787991415,787991402,-1,-1}}},
{15,9015,L"uniquebeast/gun_blader/animation/unique_1st/sinsugunbladerstay_body_00.ani",9,{{-1,-1,-1,787991501,-1,787991515,787991502,-1,-1}}},
// Generated from functional/migration-plan.json; do not edit content mappings.
{0,19000,L"uniquebeast/swordman/animation/unique_3rd/us4_summon_idle.ani",9,{{941400006,941400007,941400008,941400001,941400002,941400005,941400003,941400004,-1}}},
{11,19011,L"uniquebeast/at_swordman/animation/unique_3rd/us4_summon_idle.ani",9,{{941400078,941400079,941400080,941400073,941400074,941400077,941400075,941400076,-1}}},
{16,19016,L"uniquebeast/archer/animation/unique_3rd/us4_summon_idle.ani",9,{{941400118,941400119,941400120,941400113,941400114,941400117,941400115,941400116,-1}}},
{3,19003,L"uniquebeast/mage/animation/unique_3rd/us4_summon_idle.ani",9,{{941400046,941400047,941400048,941400041,941400042,941400045,941400043,941400044,-1}}},
{7,19007,L"uniquebeast/at_fighter/animation/unique_3rd/us4_summon_idle.ani",9,{{941400022,941400023,941400024,941400017,941400018,941400021,941400019,941400020,-1}}},
{1,19001,L"uniquebeast/fighter/animation/unique_3rd/us4_summon_idle.ani",9,{{941400014,941400015,941400016,941400009,941400010,941400013,941400011,941400012,-1}}},
{2,19002,L"uniquebeast/gunner/animation/unique_3rd/us4_summon_idle.ani",9,{{941400030,941400031,941400032,941400025,941400026,941400029,941400027,941400028,-1}}},
{5,19005,L"uniquebeast/at_gunner/animation/unique_3rd/us4_summon_idle.ani",9,{{941400038,941400039,941400040,941400033,941400034,941400037,941400035,941400036,-1}}},
{8,19008,L"uniquebeast/at_mage/animation/unique_3rd/us4_summon_idle.ani",9,{{941400054,941400055,941400056,941400049,941400050,941400053,941400051,941400052,-1}}},
{4,19004,L"uniquebeast/priest/animation/unique_3rd/us4_summon_idle.ani",9,{{941400062,941400063,941400064,941400057,941400058,941400061,941400059,941400060,-1}}},
{14,19014,L"uniquebeast/at_priest/animation/unique_3rd/us4_summon_idle.ani",9,{{941400102,941400103,941400104,941400097,941400098,941400101,941400099,941400100,-1}}},
{6,19006,L"uniquebeast/thief/animation/unique_3rd/us4_summon_idle.ani",9,{{941400070,941400071,941400072,941400065,941400066,941400069,941400067,941400068,-1}}},
{12,19012,L"uniquebeast/knight/animation/unique_3rd/us4_summon_idle.ani",9,{{941400086,941400087,941400088,941400081,941400082,941400085,941400083,941400084,-1}}},
{13,19013,L"uniquebeast/demonic_lancer/animation/unique_3rd/us4_summon_idle.ani",9,{{941400094,941400095,941400096,941400089,941400090,941400093,941400091,941400092,-1}}},
{15,19015,L"uniquebeast/gun_blader/animation/unique_3rd/us4_summon_idle.ani",9,{{941400110,941400111,941400112,941400105,941400106,941400109,941400107,941400108,-1}}},
};
inline constexpr Assets darkWarriorAssets={9,9016,L"uniquebeast/swordman/animation/unique_1st/stay_body.ani",9,{{-1,-1,-1,787991601,-1,-1,787991602,-1,-1}}};
inline constexpr Assets creatorAssets={10,9017,L"uniquebeast/mage/animation/unique_1st/stay_body.ani",9,{{-1,-1,-1,787991701,-1,-1,787991702,-1,-1}}};
// Generated from functional/migration-plan.json; do not edit content mappings.
inline constexpr Assets fourthDarkWarriorAssets={9,19009,L"uniquebeast/swordman/animation/unique_3rd/us4_summon_idle.ani",9,{{941400006,941400007,941400008,941400001,941400002,941400005,941400003,941400004,-1}}};
inline constexpr Assets fourthCreatorAssets={10,19010,L"uniquebeast/mage/animation/unique_3rd/us4_summon_idle.ani",9,{{941400046,941400047,941400048,941400041,941400042,941400045,941400043,941400044,-1}}};
inline const Assets* AssetsFor(int job,unsigned profile) {
 if(profile>=beast115::profiles.size())return nullptr;
 if(assets[profile].job==job)return &assets[profile];
 if(profile==0&&job==9)return &darkWarriorAssets;
 if(profile==3&&job==10)return &creatorAssets;
 if(profile==15&&job==9)return &fourthDarkWarriorAssets;
 if(profile==18&&job==10)return &fourthCreatorAssets;
 return nullptr;
}
}
