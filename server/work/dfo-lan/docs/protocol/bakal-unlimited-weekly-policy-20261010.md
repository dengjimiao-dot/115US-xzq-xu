# 巴卡尔通关不消耗次数（2026-10-10）

用户明确要求“改成打完不消耗次数”。这是服主策略，区别于PVF原生周次数规则；不改PVF奖励表、数量、概率、通关资格或其它副本。

## 当前行为

- workflow.BakalUnlimitedWeeklyRuns默认true，巴卡尔统一不消耗入场/奖励周次数。
- BakalWeeklyUsage先校验存档，向准入和N1434显示投影0已用；旧周计数不阻止新开团。
- Freeze仍按源MinimalDungeonClearCount决定资格，每个不同run可冻结奖励；不增加clears/rewards、不再按历史reward计数取消本轮资格。
- 同run的bakal-clear/bakal-grant事务事件键、角色归属、存档来源、奖励来源校验及待领奖plans保持。重复同场不会重复发奖励。
- 历史计数和待领奖计划保留，没有迁移或清空玩家数据库；跨周兼容沿用既有路径。
- 通关后强制重新同步N1434零已用，避免客户端结果流程改变本地缓存而服务端因同值跳过发送。客户端仍按自身PVF上限显示可用数（通常1/1），不会硬塞无限符号。

## 验证

go build ./...、go vet ./...、普通go test ./... -count=1通过。实际内层PVF的Bakal/恢复专项通过（workflow、wireprobe、bakalreset）。

新增回归在PVF周上限1/1和旧存档计数1/1条件下，同周连续三场分别冻结、领取，计数不变、UI零已用且资格正常；既有重复领奖、来源/归属门禁继续通过。周次数契约测试按新服主策略更新，未删除现行测试。

日志/修改前备份在模块.tmp/bakal-unlimited-20261010/（不提交）。未操作游戏、未写当前玩家存档、未自动停当前wireprobe-pvf.exe。

新源码候选为bin/wireprobe-handoff-source.exe，用户手动正常退出当前环境后运行scripts/启动游戏-SQLite.cmd --source-build生效。默认wireprobe-pvf.exe未覆盖，普通默认入口仍可能运行旧程序；不可把“源码已修改”说成当前旧进程已经启用新策略。
