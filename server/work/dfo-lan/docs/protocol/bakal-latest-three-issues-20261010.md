# 巴卡尔三个测试问题复查（2026-10-10）

输入最新会话 roles_persist_select_actor_town_world_live_detail_dungeon_manual_20261010_004007_257216_next37。run.json server_pid38880，与此前观察到的wireprobe-pvf.exe进程一致；没有自动启动或操作游戏。

## 程序版本

当前默认wireprobe-pvf.exe SHA256 bf22d24d7ef17141b6a6306b9791ac57b252d5b54bcb6548347071a488592029；新候选wireprobe-handoff-source.exe SHA256 a3b3018faa368347b0e228acbfef52c681a428d0f137df6a2c8e569cb7a436bd。当前文件指纹不是对历史运行映像的重新采样；日志中的旧行为与此前进程名称共同证明本次没有启用最新候选。

## 1. 寻找攻坚队

用户确认仅开战后点击出现“正在读取队伍栏信息…”弹窗。最新日志没有对应的新列表请求：1353在早期建团查询时有owned详情回复，最后菜单2285是退出菜单的CONTENT_BRIEFING，已回复exit_dialog_ready，不能把它当“寻找攻坚队”。末段请求1302+2285与退出统计一致。

因此目前没有证据证明是服务端遗漏了一次列表应答，也不能宣称这是正常提示或已修复。需要进一步核对开战状态与客户端队伍栏/招募缓存门禁。不无请求地猜发N577空列表，也不改已有退出确认链。

## 2. NPC点击无Buff

events.jsonl第2145/2267/2330/2335/2340/3045/4784/4789/5650行记录2072被拒，理由都是invalid native commander request size。完整32B真实点击被旧24B上限拒绝，与20261009官服16次点击的长度证据相同。

候选已修32B解码、已发布分队目标、库存/冷却保护和ACK2072。因此这次旧程序测试不能据此判定候选修复失败。困难规则选择尚未完全接入是另一项未完成工作，不能因此声称困难模式已全部正常。

## 3. 本体首次动画

events.jsonl第5528行，dungeon100003149 requested_grid=[2,1]且resolved_grid=[2,1]；gateway.err为map100007435。此为旧右中房入口。候选已根据源SpecificXY进入中央(1,1)，人物与Boss同房。

官服记录首次进入时，[IS BAKAL FIRST ENTER]216由开战1变进入0，PVF ON ENTER DUNGEON也执行同一转换；不能为了强播动画把它永久改1或反复推Movie标志。新候选是否自然播放首次开场动画尚需复测；不能把Boss生成或中央入场回归当作动画通过。若新候选仍缺动画，继续核对N2194原生构造字段及ACT开场分支。

## 下次测试入口

新增scripts/bakal-source-test.cmd，可双击。整份ASCII、CRLF、无BOM；使用现有storage-route.ps1 game-current --source-build，当前存储仍SQLite。继承正常入口的经营环境参数，不自动改默认profile或覆盖已确认wireprobe-pvf.exe。

已核对脚本引用文件/候选存在，候选launcher launch --source-build --server-only --dry-run通过：打印选择wireprobe-handoff-source.exe、SQLite与PVF直读，没有启动服务/客户端。新入口本身未代用户执行。

本轮只加测试入口及记录；未重复修改已经修好的2072 decoder，未猜加列表、动画或客户端补丁。原有构建/vet/full/真实PVF专项通过的A3B3018候选仍可用于复测；本轮未修改Go行为，不重跑无必要全仓测试。
