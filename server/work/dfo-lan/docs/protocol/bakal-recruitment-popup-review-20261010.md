# 寻找攻坚队读取弹窗与最新程序复查（2026-10-10）

## 新版已经运行

最新会话 roles_persist_select_actor_town_world_live_detail_dungeon_manual_20261010_205145_443543_next37；当前进程wireprobe-handoff-source.exe PID32464。当前候选文件SHA256 a3b3018faa368347b0e228acbfef52c681a428d0f137df6a2c8e569cb7a436bd，与不扣次数候选一致。进程路径因系统访问限制未返回，不能把磁盘文件哈希冒充运行内存重新采样；同时结合下列新行为确认：

- events第1146行bakal_commander_used slot0，第1149行ACK2072成功，原先32B长度拒绝未再出现。
- 第4746行本体requested[2,1]→resolved[1,1]，中央入场已命中。
- 第5113/5114行奖励库存/结算结果已发，第5117行周次数同步0/0，符合不消费策略。
- 新N583机制计时器和bakal_party_effects_restored也已发。

不是上轮旧wireprobe-pvf.exe的BF22指纹。

## 弹窗仍未完成修复

用户确认开战后点击才出现；官服录制时也点击过，时间记不清。新日志仍没有按钮点击后的新队伍查询，不能因此断言官服“没有包”或提示“正常”。

重新检查官方完整记录s9：有3组C1353→A1353，分别C118（23:10:04.894，建团信息）、C1041（23:18:01.632，开战期间/回营后信息）、C1139（23:19:53.550，失败后重新建团信息）。C1041是最接近按钮问题的活跃期对照，但没有UI点击时间标记，不能把它无条件命名为按钮请求。该记录没有N577招募列表，也没有独立mode0列表查询。说明应继续核对现有攻坚队信息/读取门禁，而不是猜补无请求N577。

本地C1353 mode1已有ACK，信息中基本演员、名字、编队位置、AREA与恢复时间等已编码；成员记录尾部仍38B占位零。官服同mode1尾部有非零数据，活跃期还携带地下城/时间等不同内容；本地另发Kind1/1220，而官服这三组没有这个ACK。1220在原生registry是POST_CHARAC_INFO，不能凭包号就说它能解除找队窗口等待。

以上是确定的数据差异，尚未证明哪一个字段或等待状态正是截图提示的门禁条件。大片零占位不等于每个零都非法；空冒险团名也可能合法。不得把官服其它玩家的名字、数值或状态整包回放，更不能猜改尾部某个byte=1来强行解除等待。

当前IDA查询端点127.0.0.1:8745仍WinError10061。需要native菜单入口的提示分支、1353成员reader144CDC190后续字段和队伍栏数据就绪判定，才能作下一次协议修改。用户的完整抓包已具备该成员信息样本，无需再说“完全没有攻坚队数据”。

本轮只读日志/完整抓包并保存复查记录，更正上份C656 Options字节位置：困难[0,1,0,7,1,0]，普通[0,0,0,7,0,0]，变化位1/4，源码decoder未改。不改游戏/存档/在用exe；未猜加通知、未提交用户其它修改。新版的NPC/中央入场/次数已命中不代表寻找攻坚队弹窗已修好。


## 原生 PE 只读核对：无需先安装 IDA

用户提供 D:/115us/DFO.exe 和 D:/115us/DFO_KEYFIX.exe。两个文件均为 246484992 字节、版本 2.38.3.25，ImageBase 0x140000000；SHA256 分别为 88eae735c27bd9e4b1c879fc720b238e4fe9855e20142f54740a1f56c5633506、8b3cb8034c1754ddc9aa4ba1136b751ddfadc7cab98ed11a54d67af9a9cd3151。二者 .text 哈希不同，不将 KEYFIX 文件当作与原文件代码完全相同，也不执行或替换客户端。

当前实际客户端 D:/115us/DFO/DFO.exe 版本为 2.38.2.34、258972712 字节。用 pefile 读取 PE，并用 capstone 静态反汇编，确认当前客户端 0x144CDC190–0x144CDC688 为有效的成员 reader；直接调用来自 0x144CDBF80 和 0x144CDC690 内。新版本同地址对应另一种成员删除操作，不能跨版本直接套地址。之前因 registry 位于 .rdata 而不能作为代码反汇编，并不表示当前客户端所有原生代码都不可读取。

当前成员 reader 在 AREA(+0x62)、恢复截止时间(+0x64)之后读取 u32 数量，循环每项读取三个 u32；其后依次读取 byte、byte、u16、三个 byte、两个 u32、byte、byte、两个 u32、byte、两个 u32。对应对象字段包括 +0x88/+0x8C/+0x98 三个布尔值以及多个数值；这证明现有尾部需要按结构核对，但尚未证明这些布尔值哪个控制寻找攻坚队等待提示，不能直接赋 1 试包。

完整反汇编只留在忽略目录 .tmp/bakal-native-pe/current-member-reader.txt 及 current-member-reader-callers.txt。该轮只读二进制并补充证据记录；没有启动游戏、附加进程、改客户端、改变服务端运行包或宣称弹窗已修复。下一步沿当前版本的菜单提示与 reader 消费路径确认等待门禁。


## 菜单门禁闭环与服务端候选修复

当前 2.38.2.34 的 DSTR101033002 为 Loading Party Window information...，对应用户截图。静态立即数交叉引用命中 145077D20 内1450780B9和145078AF0内145078B27。145078090/145078C65调用144CF40E0；该函数先经1459AC8D0检查频道条件，再检查controller+0x200：初值0xffffffff返回等待，GetTickCount小于该值+3000也返回等待。这里并非成员对象的+0x88/+0x8C/+0x98，也不是仅由C1353期望应答树决定。

144CD49D0内144CD4F70将该计时字段初始化为0xffffffff。144CDB100调用GetTickCount并写controller+0x200；唯一找到的直接调用来自146D5A3C0内146D5A3DF。146D5A3C0先读取一个byte（值未参与分支），再设置计时。进一步找到146D58EFC的lea r8指向该函数，随后mov edx,0x4c4并call14599D5D0，明确注册为NOTI1220/kind0。现有kind1/1220不走该通知处理器，因此不能初始化就绪计时。

完整官服捕获s2 S289和s9 S282均为kind0 N1220，明文003e745e7a4000000000000000000000，首byte=0；三次C1353没有另附1220。native处理器只消费首byte，后续采集字节不复制为新的逻辑字段。证据链保存在根.tmp/bakal-native-pe/finder-prompt.txt、finder-gate.txt、finder-gate-writes.txt、finder-waiting.txt、finder-timestamp-registry.txt。

服务端在角色选择成功、身份已设定后，对巴卡尔频道发送一次kind0 N1220，逻辑体00。C1353 mode1/2只保留自身应答，不再发送kind1/1220，也不在查询时重复发送通知重置3000ms计时。其它频道、不匹配角色与未选角色不发送。修改client_entry.go、bakal_flow.go，新增bakal_post_character.go及回归测试，更新已有成员查询测试。未修改成员尾部38B、客户端、存档或NPC/结算规则。

候选已编译到bin/wireprobe-handoff-source.exe，30562304字节，SHA256 c452ee185636827151f222c414c18359ecdbe8dce7b32163ac9b4f353a252051；保留已确认默认wireprobe-pvf.exe。只读进程查询当时未发现DFO/wireprobe运行，因此编译没有覆盖在用程序。用户手动运行scripts/bakal-source-test.cmd测试；入场后客户端原生仍有3000ms短暂门禁，开战后长期等待弹窗应消失。源码/native测试不能替代实机确认；若后续列表内容仍有差异，应继续按新C1353/其它请求和native窗口消费排查。

验证：go build ./...、go vet ./...和全量go test ./... -count=1已通过；显式DFO_PVF_CORE_TEST_ARCHIVE=server/work/client-build/Script.inner.pvf的TestBakal专项通过。新增测试先前依赖native客户端fixture但未创建recruitment，显式PVF运行暴露测试自身nil问题；已改为独立有效协议fixture，再跑真实PVF巴卡尔专项通过。最终测试再次执行并记录于模块.tmp/bakal-native-pe/final-tests.txt。


## 业主实机验收

2026-10-10，业主回复“ok,完美了,现在”，并提供codex-clipboard-3530e188-1e11-4837-9067-8f27f0527ff6.png。截图中寻找攻坚队列表窗及当前攻坚队成员窗均正常打开，旧的“正在读取队伍栏信息…”阻塞弹窗未出现。仅确认本次菜单读取门禁修复；不将空招募列表、成员名望0、其它机制或奖励竞拍标记为已验收。确认程序为上一轮交付候选c452ee18。


收口提交状态：提交卫生脚本exit0；fetch origin/fork成功，当前HEAD382f4452分别落后origin/main287个、fork/main233个提交。按根AGENTS.md提交前合并要求尝试git merge --ff-only origin/main，Git因大量既有本地修改及未跟踪文件会被覆盖而拒绝，merge未执行。保留用户工作树和原有暂存内容，没有stash、覆盖、提交或推送；待整理既有工作区并完成远端合并后再收口提交，不能在过期基线上绕过要求。
