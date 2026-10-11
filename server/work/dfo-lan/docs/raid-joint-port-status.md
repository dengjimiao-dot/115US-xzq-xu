# R7 / CN 非巴卡尔团本联合移植记录

## 第三轮：准备区与单人攻坚队生命周期候选

本轮恢复并补齐 R7 的非巴卡尔单人攻坚队接入，保留当前 US 编解码器。`commandDispatch` 先分派 `dispatchNonBakalRaid`；按原生频道对应的 raid kind 排除巴卡尔普通/困难。没有移入 R7 的巴卡尔状态、PG 存储或 CN 网络包。

- `RaidEntrance` 增加从当前 PVF `[RECRUITING ROOM]` 解析的准备区。实际归档核验：人造神类型 98/107 是 `205/1 → 205/2`；其它没有该字段的已绑定团本保留原频道落点。仍只支持当前上游入口目录已绑定的内容，未扩展成所有历史团本。
- `nonbakal_rooms.go` 在重登时重新选取准备落点，保持存档版本/修订不变；旧等待区存档不算队伍归属证明。`C35/C36/C1418` 下位置保持会话内，等待区权限检查先于普通区域转换；创建后的场景通过 `privateArea` 隔离。
- `CMD650` 完成费用信息查询等待态；暂时返回空信息向量，不声明费用规则已迁移或代替入场扣费。
- `CMD656` 校验频道种类、源规则、角色状态、准备区、等级和人数上限；从源填写当前 codec 必需的 `MemberMax`。通知顺序沿用 R7：N578、C656、N23、N24。任何发送失败释放精确分配的队伍并恢复原位置。
- `CMD1353` 查询只允许本连接持有的队伍，末尾发送既有 N1220 初始化完成通知；`CMD661` 只分配本人；`CMD657` 离队恢复原位置并清空列表。网关内互斥注册表防止重复角色分配及旧连接清理新队伍。
- 角色重新进入与连接退出均调用精确清理。未接通战斗前拒绝 C658，并禁止等待区走普通 C15/C16 绕过阶段启动。
- 复审又发现旧 `logUnhandledRaidRequest` 没有调用者。本轮把它接到非巴卡尔分发末端，并补 C12/C13 名称。**C656 是攻坚队创建，C12 是四人小队创建，不能混为同一功能。**本轮未宣称 C12 已修复，也未启用多人加入、开战或奖励。

模拟客户端从完整 `dispatch` 覆盖创建、重复请求、查询、离队和通知顺序；另覆盖发送各阶段失败、旧角色清理、旧连接清理新队伍、Bakal/未验证请求不接管。R7 存档回归适配后以空 SQL 写存储验证移动不落库，并核验源版本不兼容时拒绝。证据目录 `G:\电脑问题\.tmp\raid-joint-port-20261012\round3`。

最终验证：`go build ./...`、`go vet ./...`、`go test ./... -count=1 -json` 均退出 0；59 个包、5315 项通过、353 项跳过、0 失败。与第二轮基线逐名比较，新增 21 项通过，原 5294 项通过全部保留。完整 dispatcher 生命周期另行复测通过；指定 SHA256 的实际 PVF 入口读取通过。详情见 `round3/verification-summary.json` 和 `named-test-delta.json`。

本轮属于同一创建入口候选的 `attempt 2/3`，日志已标识；没有启动客户端、实机试包或替换默认程序。测试通过仍需用户用当前源码构建后手动点一次创建，读取 `raid_created_waiting` / `raid_entry_refused` / `raid_unhandled_request`，确认现场发的是 C656 还是 C12。若 C12，不把此候选描述为现场问题已修好。

## 第二轮复审：创建无响应与遗漏链路

业主补充：本地 R7 的非巴卡尔团本应可进入准备室，点击创建的确定按钮无反应。此现象尚未配对到实际运行 exe、环境变量和单次请求日志，不能直接断言某个开关就是现场根因。以下为源码复审结果。

| 优先级 | 确认的问题/遗漏 | 证据与处置 |
| --- | --- | --- |
| P1 | 创建入口存在默认跳过条件，但未核验现场是否触发 | R7 `client_raid_entrance.go` 在非 82 频道且 `DFO_RAID_OPEN_EVENTS_PROBE != 1` 时直接返回；当前上游没有此 handler。应先追踪一次 CMD656 到处理器/拒绝日志，不能仅凭界面现象定因 |
| P1 | 准备区、等待区与权限未成套移植 | R7 `RaidEntrance.Recruiting`、`loadWorldEntry`、`authorizeRaidArea`、`worldSession.raidWaiting` 和 `privateArea` 是一组依赖。缺少私有等待区可能将不同队伍发布到同一公共场景；不能只迁创建 ACK 和区域通知 |
| P1 | 角色切换与断线清理缺口 | R7 `client_entry.go:356` 在重入流程调用 `releaseOwnedRaid`。仅在断线清理或下一次团本请求时检查旧角色不足以替代此调用 |
| P1 | “所有非巴卡尔”没有源覆盖 | 当前上游入口只列 5 条路径，含巴卡尔；R7 还含 Inae/Delezie 等路径和 CTP 依赖。普雷/超时空及其它入口不能因有 opcode 映射就宣布支持 |
| P1 | 规则层尚未进入业务装配 | `Source.LegacyRaidRules` / `Source.AsrahanRaidRules` 没有生产调用者；当前 571 条只完成语法编译覆盖。理智、共享伤害、首领特殊动作、恢复与奖励事务仍缺实现 |
| P1 | 雾神延迟 Tick 丢事件 | 新回归复现同一每秒重设计时器：逐秒推进 5 次得 5，延迟 5 秒一次推进仅得 1。已改为按实际到期时间逐个消费并重查重设的计时器；循环超限整次回滚。CN `ModernRaidPhase.Advance` 同样按 NextDue 推进 |
| P1 | 旧团本初始计时与 binding 校验不完整 | 已新增 `NewLegacyPhaseAt` 明确传入毫秒时钟及源 SINGLE RAID TYPE 值，检查 binding 注入过去/零延迟计时、未知房间、超容量/重复队伍、非法房间状态。旧构造器保留零时钟语义，仅适合显式使用相对时间的调用方 |
| P2 | R7 与当前协议对象约束不同 | R7 创建路径原代码未填 `RaidRecruitment.MemberMax`，当前 `validateRaidRecruitment115` 要求非零。移入时必须从当前 PVF 填写并验证，不能整段原样复制 |
| P2 | 模式和源身份仍有缺口 | `SingleType` 参数可传入不等于已实现路线选择；普通/困难/引导/小队的入场投影尚未装配。领域身份当前主要绑定脚本 SHA，符号表身份及跨阶段恢复仍需一并设计 |

临时新增的非巴卡尔创建 handler 和 R7 队伍注册表已移到本机 `round2/unreviewed-entry/`，撤销了其默认分发和运行时字段接入；三个原有网关文件与本轮修改前备份逐字节一致。此候选没有启动、没有实机试包，也不作为本轮交付功能。恢复接入前须成套覆盖创建前置、准备/等待区权限、私有场景、角色切换、离队、断线、失败回滚，并验证请求与通知顺序。

本轮保留的代码是已补回归的领域内核修正。复核材料在 `G:\电脑问题\.tmp\raid-joint-port-20261012\round2`；失败复现 `review-timer-before.log`，最终 build/vet/test 为 `final-*`。没有更新运行程序、玩家存档或巴卡尔。

本轮只完成服务端规则读取与领域内核候选，尚未完成可玩团本。目标为业主指定的 `G:\电脑问题\上游仓库版本服务端\服务端` 当前磁盘源码，不等同于旧 `D:\115us` 运行目录。没有启动游戏、替换运行程序、修改存档、客户端补丁或巴卡尔代码。

## 来源与边界

- 目标 Git HEAD：`2bec94ba6ed7895e0e11fb929a2afd9144f679fe`。开始时已有 785 条工作区状态记录；以实际文件为基线，本轮只增加独立文件。
- R7：`G:\电脑问题\45G累计更新制作-20261010\additional-r7-20261011\source\server\work\dfo-lan`。
- CN：`D:\115us\台改客户端\原版客户端\DNF115CN-Test-1.0.3-20261012-012124-363468\Server\DNF115CN.Game.dll`；SHA256 `66e6109c55db3b5bd4d64f34b2a2e798298b784a1d5fbba96bcaa50a6e7da3e5`。以 ILSpy 反编译的 `PvfRaidProgram`、`PvfRaidCatalog`、`RaidPhase` 为领域逻辑参考，不把 CN 协议号、数据库或奖励常量移入 US。
- 当前 US PVF：`server/work/client-build/Script.inner.pvf`，SHA256 `d175ea7def414ec665f5052e90b82208786b7caae0d8d916470e1301bca50aca`。它是运行内容来源；审计导出的 JSON 不是运行输入。

## 本轮代码

| 内容 | 实现与证据 | 尚未完成 |
| --- | --- | --- |
| 希洛克 | `catalog/raid_legacy.go` 读取 `etc/raid/siroco/siroco.etc` 及 `.symbol`；普通、困难、引导、小队分别 105/110/52/52 条事件，共 319 条 | 专用首领行为、单人路线初始化、状态通知、结算 |
| 奥兹玛 | 同一 reader 读取 `contents/2021/ozma_raid/etc/ozma.etc` 及 `.symbol`；普通、引导、小队分别 130/66/56 条，共 252 条 | 理智、专用机制、共享伤害、入场与奖励事务 |
| 旧团本通用执行器 | `raid/legacy_program.go`：源符号、条件、赋值、计时、房间状态、撤离效果、阶段完成、整次级联失败回滚；缺失专用 binding 拒绝构造 | 通用核心不等于上述 571 条事件全部可执行；奖励仅保留源字段，没有直接发奖 |
| 雾神 | 迁入 R7 `raid_asrahan*` reader、规则编译、战斗分组和 `raid/asrahan_script.go`；普通/困难 98/90 条源事件 | 专用动作与网关链、跨阶段/断线持久化及奖励 |
| 雾神 COS | 独立 `raid_source_cos.go` 兼容当前源中的显示引用；没有修改巴卡尔 reader | 每模式 9 个战斗组中 8 个可从 COS 匹配动态首领；Lopes 组的占位记录不能当作可生成怪物，需核验地图原生领主链 |
| 同源读取 | `gamedata/source_raid_programs.go` 提供 `Source.LegacyRaidRules` / `Source.AsrahanRaidRules` | 尚未接入 `PrepareCatalogs` 与请求 handler，读取方法不会开放频道 |

希洛克 authored SET 同值仍派发事件的行为，已用 CN 原始程序集执行纯内存输入核对：初始 HP=100，计时到 2000ms 再 SET HP=100，COUNT 从 0 变为 1；Go 同输入测试一致。该证据只支持这条领域语义，不支持 CN 与 US 的封包兼容结论。

## 下一阶段必须闭合的链路

1. 当前 US `cmd/wireprobe/raid_channels.go` 对 CREATE_RAID、START_RAID、REQUEST_RAID_INFO 等仍只记录 `raid_unhandled_request`。现有 opcode 名称证据不能证明 reader 字段、长度、顺序、等待态。需要当前 US `DFO.exe` / 权威 IDB 的具体调用链及用户手动会话日志，不能直接移入 CN 封包或 R7 旧入口。
2. 希洛克先完成模式/单人路线初始化、特殊首领动作、实际战斗归属；奥兹玛再加理智与共享伤害。领域 Apply 的事件必须由网关验证真实战斗与角色归属后产生，不能接受客户端直接自报清图。
3. 雾神补齐专用动作和 Lopes 地图原生领主；不能把 COS 动态生成 8/9 说成全部首领修复。
4. 发奖前接入目标 SQLite 事务、事件键与重复领奖防护，保持已有存档兼容；本轮没有数据库迁移。
5. 普雷、超时空、人造神等仍是后续迁移对象。本轮没有新增它们的实现。CN 普雷存在手工最终路线（`PvfRaidCatalog.ReadPreyPhase` 中 `320,318,320,319,320,317`），须先追溯当前 PVF 定义，不能作为内容常量照搬。CN 存在类或 handler 也不代表其玩法已修复。

以上接入限制遵循根 `AGENTS.md` §0 第 2/3/6 条和 §0.2：当前 US 客户端证据优先、禁止猜包、实机由用户操作。本轮不计任何 C2S 试包尝试，也未标记 confirmed baseline。

## 复核材料

本机隔离目录：`G:\电脑问题\.tmp\raid-joint-port-20261012`。`evidence/` 保存原基线、候选 build/vet/full test、原生 PVF 测试、逐名测试差集、CN 单语义 oracle 和新增文件 SHA256 清单。完整验证数字以 `verification-summary.json` 为准。

中间隔离测试曾因缺少包根 mods 失败；补齐包结构后消除。随后发现部分原基线资源测试因缺少 PVF 被跳过，补齐同一 PVF 后重新执行完整测试。没有修改这些测试来放行。
