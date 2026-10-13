# 巴卡尔指挥官使用时的 NPC 出场效果（2026-10-10）

用户报告战斗中点击右上角NPC后buff生效，但缺少左下角NPC出场特效。本次只补当前原生使用通知，不改客户端资源或常态NPC语音触发规则。

## 官服及 native 证据

完整20261009 s9成功激活顺序为N2288 → N2289 mode2 → N2285 → A2072。例如S1351/1352/1353/1354、S1437/1438/1439/1440、S2269/2270/2271/2272。五槽位mode2逻辑体：S1352=020100000000000000000000000000000000，S1438=020101000000000000000000000000000000，S2270=020102000000000000000000000000000000，S2276=020103000000000000000000000000000000，S2537=020104000000000000000000000000000000。保留逻辑18B，不复制采集体后续加密填充。

当前2.38.2.34 registry144CD5C0F将144CDE830注册为NOTI2289/RAID_ADDITIONAL_BRIEFING。144CDE830用146EA0BE0整块读取18B，只对cached raid kind8调用142543E70。consumer body0为分支；mode2在142544013读取body1队伍颜色和body2起u32指挥官槽位，先142549390发布对应源简报。后续按本角色member+0x2D的1/2/3编队转换颜色，14256D430将颜色索引0/1/2转回1/2/3并与body1比较；匹配后1421F29E0解析NPC，1421F4FD0设置出场资源，1421F4210启动窗口动画。

原生1421F4FD0仍遵从客户端现有编号0xD显示选项（值1时取源资源，否则不激活），本次不修改用户设置或强行绕过显示选项。NPC资源、技能映射和表现时长继续由客户端同源资源定义。

N2288本身只负责库存/使用目标及角色效果，不等同于这条mode2出场通知。native反汇编记录在根.tmp/bakal-native-pe/commander-briefing.txt、commander-briefing-consumer.txt、commander-cutin-window.txt，库存及角色效果链另存commander-effects.txt和commander-cutscene-start.txt。

## 服务端实现

新增protocol.BakalCommanderPresentation(slot,team)：kind0 N2289的mode2固定18B结构，byte0=2、byte1=已拥有分队颜色（当前单分队1）、u32@2=已接受槽位0..4，其余mode2未用字段置0。拒绝非法槽位及颜色，提前构造后才执行既有源库存/冷却事务。

C2072成功时发送N2288激活 → N2289出场 → 本次源party effect/update → A2072。原先源效果帧早于激活通知，现按官服使用顺序编排；不改变源buff效果、消耗、冷却、金币或回营逻辑。拒绝使用、重复冷却、进房库存恢复、常态库存增减不发送mode2，不循环播NPC出场。已有倒计时、菜单修复保持。

修改cmd/wireprobe/bakal_flow.go；新增internal/game/protocol/bakal_commander_presentation.go及原生向量测试，增强cmd/wireprobe/bakal_commander_live_test.go。测试覆盖五槽位官服18B向量、非法参数、所有5个源技能成功各1次、N2288/N2289/N2285/ACK顺序与重复拒绝不播放。

## 验证与实机边界

显式DFO_PVF_CORE_TEST_ARCHIVE=server/work/client-build/Script.inner.pvf的cmd/wireprobe及protocol全部TestBakal通过。go build ./...、go vet ./...、go test ./... -count=1均exit0，候选指纹见下文。用户手动运行scripts/bakal-source-test.cmd，在加载完成的战斗房点击NPC一次，检查buff和左下角出场效果；尚未实机确认，不标为confirmed。常态位置语音/其它N2289分支仍未接入，不扩大本次修复范围。

候选编译：go build ./...及go build -o bin/wireprobe-handoff-source.exe ./cmd/wireprobe通过；go vet ./...通过。候选30567936字节，SHA256 fd08c3cbf729eb71876ba1aab5b54a1d3ccec242be0da4a18659a557c96b4df4。全量测试日志留在模块.tmp/bakal-commander-presentation/all-tests.txt。更新前只读进程查询无DFO/wireprobe进程；没有启动游戏或覆盖确认默认wireprobe-pvf.exe。
