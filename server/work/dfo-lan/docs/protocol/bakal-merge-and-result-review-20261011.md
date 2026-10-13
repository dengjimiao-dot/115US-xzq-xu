# 合并保全与结算开窗候选（2026-10-11）

用户确认上一轮实机“基本没有问题”，最终问题明确为“没有出现领奖/结算窗口”。确认范围为基本战斗修复，上一轮程序52b3a1a9；不将尚未出现的结算窗口或完整竞拍标为已验收。

## 合并审计

当前HEAD a3952da8，已包含远程更新。备份D:/115us/115-server-backup-20261011是files-modified/untracked形式的改动备份，不能当作完整checkout比较。按正确布局检查80个相关文件：75个相同，5个有变化。Bakal专用修复文件、源第一/二阶段、NPC出场、地图清理、buff恢复、不消费次数均保留。Session差异只有对齐格式；catalog/dungeons.go和dispatch新增远程内容，保留不回退。

两个确定丢失的hook：client_entry.go角色选择尾部不再调用bakalPostCharacterInfo；dungeon_flow.go不再限制C16直进源Bakal房间。精确恢复备份中的两段，保留远程衣柜、签到、次元回廊、蔚蓝号奖励等增量。不使用整文件覆盖。审计diff和对照清单留在模块.tmp/bakal-merge-review-20261011/。

当前专用启动脚本仍指定--source-build；默认wireprobe-pvf.exe BF22与候选独立。源PVF b814d76e6287b9ddd4a50de648aef61e5983d58d4d5ecf2e9c142a8b5b0a5faa，无改动。游戏/存档未启动或修改。

全量测试发现远程TestDimCloisterLoginFallsThroughForIspinsChannel构造了nil output，却调用30秒保活调度，测试超过30秒后异步panic。只为该测试补net.Pipe及cleanup/output，未改生产保活或军团玩法；原test的pending账本断言保留。第一次full test确实失败，修复后重跑full test通过，不隐藏基线失败。

## 当前结算问题及原生证据

完整官服s9成功S5784 N588逻辑02006402000002000101，然后S5785 N574=0300，S5786 N578 mode2的owned state3/phase0及原开始时间。失败N588=03000000000001000001不能代替成功结果。原服务端最终仍发旧00分支/flag0，再发N574=0000并将owned cache重置state0，未进入原生结算状态。

当前2.38.2.34：144CE4E00读N588的u8结果分支、i8 phase、u32耗时、u16结果度量、u8结果标志、u8切换标志，并在144CE6265调用144B0EBE0把本地演员结果存入缓存。144CE7B60读N574；stage3在144CE7F66/144CE8AAD进入结果窗口分支，用已有N588数据启动原生窗口。Bakal自身状态回调14254CF00与通用结果窗口链均需考虑，不能仅将某个Bakal分支的cleanup误当整个处理器不弹窗。只读反汇编见根.tmp/bakal-native-pe/bakal-result-reader.txt、bakal-result-ui-setter.txt、bakal-state-consumer.txt、bakal-end-state-ui.txt。

本候选对源runtime结算使用N588最终分支2，保留phase0、真实耗时与最后切换标志1；byte8只在持久化eligible计划Claim成功并生成N13库存后置1。u16结果度量的游戏语义仍未闭合，保留兼容零值，不复制官服另一个角色的2，也不猜用本地击杀数。该字段和后续卡片内容仍需继续按消费者验证。

随后N574进入stage3而非idle0；owned N578保持state3及真实开始时间。没有冻结/入库完成的库存不能发布结果，重复tick不重发588。历史无源黄金向量保留原格式，不将其当运行期最终结果。冻结奖励、事务事件键和重复领奖防护不变。

本轮仅补结算开窗第一步；N587真实奖励卡片/明细、N574后续4/6/7/8/9/10/11状态与竞拍2059/2060/2061、C1902出价和奖励结算仍需按完整数据继续接线。没有整包回放别人的物品或自动扣竞拍金币。新窗口是否显示尚待实机，不能宣称最终领奖/竞拍全流程完成。

## 验证及交付

真实PVF六包Bakal专项通过；新增durable readiness、final-result/claimed flag、N13→N588→N574=3顺序、重复tick不发布测试通过。go build ./...、go vet ./...、go test ./... -count=1最终均exit0。日志：模块.tmp/bakal-merge-review-20261011/final-native-tests.txt与final-all-tests.txt。

候选bin/wireprobe-handoff-source.exe：31415808字节，SHA256 3c9638d7045aaf161ed278b724c4c3488bc7e467dc8fe8da342ec773b76a0d2a；包括远程增量和已恢复的本地修复，默认程序未覆盖。用户手动运行scripts/bakal-source-test.cmd通关后验证结算窗口；保留基于库状态的实际发奖及重复防护。
