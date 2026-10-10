# 巴卡尔开始攻坚倒计时与进房门禁（2026-10-10）

用户报告倒计时数字未出现、数字未结束时可进房。仅修改服务端；当前客户端2.38.2.34为消费契约，根目录去壳DFO/DFO_KEYFIX为2.38.3.25参考。

## 原生链

N581/PREPARE_START_RAID为kind0空体。当前注册144CD5622指向144CE0810，144CD564C以opcode0x245调用14599D650；去壳两文件相应注册144CD5EB2/144CD5EDC指向144CE10A0，调用14599DF10，说明地址需要按版本对应。

当前144CE0810按owned raid对象+0x28选择raid kind；kind8跳转144CE0AAC。144CF3870从owned raid id取其kind，再144CF1F70取配置对象+0x40时长。原生解密该分支资源路径为Contents/2022/BakalRaid/Xui/Message/m_countdown.ani；144CE0E55调用1451A39F0设置数字时长，1451A44D0启动显示。N581不携带新的数值体，不改客户端资源。当前PVF解析contents/2022/bakalraid的[start delay time]=3，服务端以同源StartDelaySecs推进准备阶段；不能用任意新Go常量替换时长。

官服完整s9两次：N574 preparing S384/S2995 → N578 mode2 S385/S2996 → 空N581 S386/S2997 → N578 mode3 S387/S2998。采集尾部填充不复制。原生注册及handler只读记录见根.tmp/bakal-native-pe/countdown-registry.txt、countdown-handler.txt、countdown-unpacked-registry.txt。

## 确定错误及修复

最新runtime roles_persist_select_actor_town_world_live_detail_dungeon_manual_20261010_223635_472467_next37：14:38:39.5249744Z发N581，随后事件bakal_lifecycle_owned_raid却为mode3而非计划生成的mode2；14:38:43.1586826Z才发active。服务端实际存在3秒准备门禁，不能把“发过N581”当作屏幕数字已显示。

bakalStartRaid先追加lifecycle，再按ID==578替换所有包为成员编队资料，因mode2和mode3共用opcode，准备阶段描述也被覆盖。改为按bakal_real_member_assigned事件身份替换单个成员帧，并将合法lifecycle mode2插入N581之前；保留C2089应答在末尾。倒计时发送前当前团的准备状态及kind可被native读取。

另查通用C16选图未限制巴卡尔源房间，可能通过普通副本入口绕过raid状态门禁；现在对巴卡尔频道、源Dungeons中声明的房间拒绝通用入口，要求已有C2062 native portal。不是对全部普通频道禁止进图。新增只读ValidateDungeonEntry复用原EnterDungeon的准备/恢复/阶段/清场规则，网关在构造地图、角色及库存初始化计划前调用；EnterDungeon本身也继续调用同一门禁。未额外延长PVF时长、未改金币/奖励/存档。

## 测试与交付

新增TestBakalCountdownDescriptorAndAdmission：真实PVF准备状态N578 mode2应在空N581前，编队frame仍mode3；同源时长分别改为3与6，倒计时前与截止前1ns都拒绝native入场且不创建room，截止时tick后允许入场；C16拒绝绕过。旧TestBakalLiveWaitingRoomCreateAndDetails原来把所有N578当mode3检查演员位置，现改仅对mode3读取对应偏移。真实PVF的cmd/wireprobe与internal/legion全部TestBakal通过。

修改bakal_flow.go、bakal_dungeons.go、dungeon_flow.go、bakal_opening.go及相关测试。go build ./...与go vet ./...通过；全量go test ./... -count=1结果见模块.tmp/bakal-countdown-fix/all-tests.txt。

用户回复已退出后已更新bin/wireprobe-handoff-source.exe，30564352字节，SHA25679dd0ad8ef69ba3ee57f79b9905ca9182936ca7540cc2b9af78ce3c2d56a50cb。默认wireprobe-pvf.exe保持。由用户运行scripts/bakal-source-test.cmd，检查开始攻坚的数字及数字结束后进房。当前候选尚未实机确认，不标为confirmed，不自动启动游戏。
