# 本体首次开场与一、二阶段房间复查（2026-10-10）

用户要求用去壳版DFO及官服完整抓包核对首次进入本体的开场动画、两阶段是否同一房间。本轮只读抓包、当前2.38.2.34 native代码及当前内层PVF，未修改运行协议、地图或存档，未启动或操作游戏。

## 阶段换房是正常流程

完整20261009 s9两轮本体入口N28均dungeon100003149；N29一阶段grid1,1、map100007529，二阶段grid1,5、map100007763。第一轮S2248附近23:16:02.452一阶段，S2524于23:17:19.449转二阶段；第二轮S5084附近23:26:16.780一阶段，S5343于23:27:35.792转二阶段。N29地图字段u32@32，不能从@31错读。

二阶段紧邻N570 symbol230/value1及N2194模板109014483（第一阶段109014482）。20261008普通轮亦同：22:27:31.428进入1,1/100007529，22:29:10.051转1,5/100007763。不是两个不同副本，也不是人物误进入右侧小房间；两阶段同属100003149的3x7源maze，分别位于两套3x3房间的中央。源DGN的[start map]同时声明1,1和1,5。既有服务端二阶段转图和两模板测试保持。

## 首次开场场景确实缺失，资源已存在

当前PVF contents/2022/bakalraid/dungeon/100003149_bakal/100003149_bakal.dgn的map specification把1,1列为100007434、1,5列为100007763。本地直接采用100007434；但同一归档还存在map/100007529.map，不应继续把差异仅归为官服版本不匹配。

100007529.map的Incoming背景和[passive object]含109105654（list/passiveobject.lst解析为Monster/Bakal/PassiveObject/IncomingObject/IncomingObject.obj）；其[phase shift map]的phase0 map index100007529、phase1 map index100007434，[init phase]0，[init immediatly phase]0 1。两phase声明SCENE TRANSITION、2000ms、允许不同地图尺寸。

IncomingObject/Action/Base.act：玩家距离<=150或3000ms后启动序列，进入动作时封红门；Sequence_1/Sequence_2包含镜头、角色表现和屏幕效果，Sequence_2的[PHASE SHIFT MAP]1 1最终切至战斗场景并销毁开场对象、释放按键。100007434.map含CenterPosDummy109105712，没有该IncomingObject。因此本地直接选434会跳过这套开场控制对象和序列，是已定位的实现差异，不应另发猜测的独立电影包。

当前native读取phase-shift标签的xref：1471C2FD0内1471C7427读取[phase shift map]；1420B93E0内1420BB3F0读取[phase shift map info]，进一步解密确认其加载的是2025/Delezie ClientOnly.ctp，故该处不能用作巴卡尔专用选图证据。地址以当前2.38.2.34匹配，不将根去壳2.38.3.25相同地址直接当同一函数。记录见根.tmp/bakal-native-pe/phase-shift-map-tag-xrefs.txt。源导出位于模块.tmp/bakal-first-entry-source、bakal-intro-object、bakal-intro-sequence；均为只读源导出，不参与运行配置。

## 仍需闭合的选择条件

同源bakal.etc首次入口事件检查[IS BAKAL FIRST ENTER]=1并在ON ENTER DUNGEON将其置0，官服首次N29之后也收到相应符号归零。不能把该值每次强设1来播放开场，更不能只将所有1,1地图硬编码替换529。还需要补通用phase-map源索引及首次入场/重进/二阶段选图契约，验证开场后服务器逻辑房间与客户端434场景一致；本轮没有将未实现的动画恢复标记为修好。

原始.symbol文本顺序存在前导未命名单元，抓包symbol216按已解析规则对应first-enter；报告使用源名称避免把旧导出文本编号直接当wire号。二阶段资源差异与首次人物侧房问题分别处理：正确中央坐标已修复，不把阶段地图不同再强行合并。


## 2026-10-10 源驱动开场修复候选

用户明确要求补齐。现有clientbakalslotscript.cos的bakal槽位已包含额外开场地图，loadBakalTables也已将它导入同一只读地图目录，不需要新建文件索引或内容表。新增BakalEntryScene只从这些已声明地图读取[phase shift map]，投影two-phase/init0/immediate0,1/self-map→battle-map的当前原生契约；要求战斗图属于该副本源Start房间且有槽位归属，冲突明确报错。实际源给出529→434，改变测试源里的目标则结果随之改变，不在运行代码写死这两个ID。

入场前读取源[IS BAKAL FIRST ENTER]锁存；首次将N29地图字段替换为已导入的开场Map，人物仍在源中央1,1。domain Session.Room.Map继续保存同一格的战斗图，EntrySceneMap仅保存最初的客户端呈现图；避免ACT内部PHASE SHIFT MAP切到434后，服务端仍把529当不同战斗实例。Room移动清理呈现图；2069仅允许该owned场景或owned战斗图，拒绝相邻/其它房间。Actor、死亡、phase判定、buff与门状态仍属于源战斗房，阶段1→2维持1,5及SecondTemplate。

源ON ENTER仍消耗first-enter并在服务端变0。协议发布前保持原先pre-entry值1至N29创建开场场景，然后发送已排队的源置0通知；不在后续入口重新改1。回营重新进入同run直接发战斗Map；重开run由源INIT自行恢复。开场镜头、动作、模糊、红门和结束后的场景切换由IncomingObject及其ACT/phase-map表执行，没有猜加独立movie通知、固定动画时长或客户端补丁。

修改internal/catalog/bakal_raid.go、bakal_tables.go，新增bakal_entry_scene.go及源变化测试；internal/legion/bakal_opening.go提供只读first-latch；internal/dungeon/session.go记录呈现别名并在移动时清除；cmd/wireprobe/bakal_dungeons.go、bakal_flow.go进行选图/通知顺序与owned报图绑定，新增bakal_entry_scene_test.go。入口事件增加battle_map/entry_scene_map用于用户测试后核对。

显式真实PVF的catalog/wireprobe/legion/dungeon巴卡尔专项全部通过；first scene/reentry/二阶段新增测试再跑通过。覆盖开场N29中央位置、first-symbol归零在N29之后、首次只选一次、回营重进不重复、两类owned场景报图及foreign map拒绝、一二阶段Boss归属。测试自身早期用1小时推进触发源怒气失败，已改为源复活等待结束时刻；没有放宽玩法失败门禁来迁就测试。

go build ./...、go vet ./...、go test ./... -count=1均exit0，全量test结果留在模块.tmp/bakal-entry-scene-fix/final-all-tests.txt。实机画面尚未确认，用户运行scripts/bakal-source-test.cmd测试首次开场→战斗、回营重进、二阶段转场；保留之前菜单/倒计时/NPC出场候选，默认wireprobe-pvf.exe不覆盖。

候选bin/wireprobe-handoff-source.exe：30585344字节，SHA256 52b3a1a9caa4a037d14ea7029e0b5ade69a844be9fecf40f941fe816d9f79d05。
