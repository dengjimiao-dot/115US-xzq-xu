# 巴卡尔困难通关、奖励明细与竞拍取证候选

## 身份与范围

官服完整困难通关：`D:/115us/analysis-tools/capture_work/official_20261013-010216_live`，主游戏流s37。从建团至通关、自动奖励与三件流拍均有记录；没有C1902实际出价，不能将流拍记录称为成功购买验证。9571个解密帧，7627个已有明文前缀逐项吻合；tcp重组与原始bin的序列偏移吻合。

当前客户端为`D:/115us/DFO/DFO.exe` 2.38.2.34，SHA256 `9642044cc2799b4c652a2ea9d970f3244be090d250d69a4989b20c9cf11deb06`。根目录去壳文件版本不同，不搬用其地址。PVF SHA256 `b814d76e6287b9ddd4a50de648aef61e5983d58d4d5ecf2e9c142a8b5b0a5faa`。

继续使用reverse-engineer-anything技能。REA6.4的PE资源Evidence为`ev_210afca97b19eeb410c9523c50b63a03cce34cd8a9cbeb6cb7d08543ca44985a`。使用用户已有Ghidra12.1.4和Java21，未修改全局配置。REA全PE深度分析先330秒、后1200秒超时，已关闭任务自有会话，不重复相同请求。后续限定函数切片Ghidra及原PE汇编为静态辅助证据，切片保存原SHA、VA、偏移和自身SHA；不冒称REA反编译Evidence或动态执行。

## 已修正的源模式和存档

C656困难向量中的Options[1]/[4]均为1。网关原先无条件PrepareBakalOpening(false)，会把困难开团变成普通脚本；现检查两个标志一致并传入HardMode，失败重开保留所属模式。

`contents/system/raidsystem/raidreward.etc` raid8代表地下城100003149含独立`[phase]`和`[raid phase of hardmode]`，当前正常10行、困难15行。catalog分池，FreezeDifficulty按所属模式选择。旧Freeze保留普通API；旧事件键和模型不变，新hard字段缺省false。Cards新增可选字段，将一次冻结时展开的party/squad产品分组保存；展示使用冻结结果，不另掷奖励。Claim兼容旧Products/Items存档，无SQLite schema变动。不消耗每周次数的服主策略保持。

竞拍要区分奖励池容器与真实拍品：源regular pool普通10337823、困难10342710只是抽取表，困难抓包实际拍品10354120在该表中。只展开源池一层，得到实际拍品，再冻结拍品自身的箱内奖励；不能显示池容器，也不能把一个拍品的箱内物品拆成多轮独立售卖。`loot.OpenRewardBoxesOnce`与原递归OpenRewardBoxes共享同一draw/count/no-drop执行器，不复制权重表。Award保存实际拍品，Products保存其一次冻结的最终内容物，成交时一次事务发放。旧无Products的lot仍按Award发放。购买入口检查显示模板，失败事务不重复扣款/发奖。

## 奖励协议与当前接线

当前N587注册144CD59B2→144CE6990。逻辑头u8分支/u32参数/u8成员数；每人u16演员/u8未使用字节/u32姓名长度+姓名/i8卡片索引/u8物品数，每物品37字节。材料模板@1:u32、数量@5:u16，抓包材料辅助字段为0。卡片索引0/ff的signed消费经144B0D5D0→144B0EAE0→144B119A0→1451B3D70验证，ff为无卡片对象。仅已核对的材料布局实现；装备预览属性未猜填。

困难主奖励587先zlib解压为324字节，reader消费319字节；party行消费97字节，尾标记仅6字节。源码发送逻辑裸结构，测试向量保留实际消费区间，只将原生未使用成员字节5规范为0。不会复制官服角色姓名/物品到本地，网关用本账号演员、姓名和已持久化Cards构造0/3分支。

困难01:15:01.128 N588=02003502000005000101后N574=3；01:15:41.133进入N574=4并发布明细，然后6/7。普通完整记录同一成功分支到4约39.16秒。源`[MOVIE END TIME]`按客户端14774AA00/14774B9B4顺序读16/21/40/24。候选只将已见证成功的40秒对应源第三项用于服务器奖励发布期限，未声称客户端电影播放ACK或通用resultKind索引已闭合；失败type3约8秒的旧记录不能推导成24秒门禁。

当前候选在持久化Freeze/Claim/N13成功后保留N588=2/N574=3，再于上述源期限发布4→冻结N587 squad/party→6→7；每次状态都紧跟所属N578 mode2同一状态更新，与hard6070/6078/6080对应，后续lifecycle不会将7回退为3。重复tick不重发。无法绑定的装备预览只记录`bakal_reward_presentation_unbound`，不阻断已应得的持久发奖。退出、换角色、新建团会清理所属UI队列。

实际Bakal奖励窗口是UI factory0x326（142559730，BakalRaidRewardWindow.xui），共享0x322是另一窗口。Bakal stage6要求结果缓存为2并刷新已存在0x326，stage7清理，不创建窗口。N574还调用resource-context虚拟回调；当前已读初始化不能证明Bakal key8注册路径。N578后续状态更新与明细串是本轮完整抓包所证的补齐，但0x326冷创建路径尚未全部闭合，因此该候选必须用户手动验证，不宣称仅6/7已保证弹窗。当前分析没有发现metric0在0x326构造函数直接阻断，间接路径仍待查。

N578独立回调进一步闭合：144CD5547注册144CE30B0；mode2在144CE36DC找所属raid，144CE370F→144CDBF80写state到owned+2c，144CE3719→144CEC0F0重建成员/cache并通知观察者virtual+10，共同尾144CE3417→141C2DF60向raid resource-context virtual+300发mode2通知。这证明N578不是冗余复制，不能只更新N574；virtual+300的Bakal冷创建实现仍未解析完，不声称state4必开窗。

N587零成员尾标记：困难branch13、普通branch11，header参数未作为总额使用；清临时向量并刷新UI，尚未证明mode如何选择，不以奖励数量猜造。N588 u16度量仍保留兼容0，不将官服5/2猜成副本击杀数。

## 竞拍协议与当前接线

1425C2600注册：N2061→1425C3970、N2060→1425C3190、N2059→1425C3B60、ACK1902→1425C2DC0。

N2061=u32数量+(u16演员/u8资格)。N2060=u32数量+每项13字节基础字段，可选181字节详情。N2059为template/quantity/serverStartPrice三个u32。ACK1902状态1后演员/姓名/lot索引/模板/数量/金额/outcome/剩余毫秒；0更新、1成交、2流拍。新抓包三轮均为服务器outcome2。C1902原生1425C6830/UI1425CF8EC证明正常出价为01+u32金额，未补猜测取消动作。

源`contents/2021/ozma_raid/etc/biddingsystem.cos`由native1425C4B50读取：max5、开始延迟10、间歇3、轮次8秒、报价下限100、goldDefault10000、倍率1.05。它与bakal.etc重复定义计时，catalog显式检查一致，不维护Go计时表。浮点文本保留原始lexeme，避免1.05经float32变成1.04999995。

原生首次显示最低报价为max(服务器起价,10000)，startMin100是输入clamp，不是物品起价。后续最小加价为max(ceil(current*1.05),current+10000)。抓包服务器起价100万的源规则尚未找到。用户在本轮明确选择“1万金币（便于本地测试）”；`workflow.BakalAuctionStartPrice=10000`是仅巴卡尔的服主价格策略，不冒称PVF原生价格，不将startMin100当起拍价。其余抽取内容、权重和计时仍按PVF。

N574所有stage均交给竞拍manager；4/6/7在该manager无操作。8会打开竞拍窗口，要求已收到非空N2060表；2061决定本地资格。当前候选已接入所属单人成员流程：成功结算冻结真实拍品→在4之前发布本账号2061与非空2060→奖励7后等源22秒→8开竞拍→再等源10秒→9及2059开始首轮→源8秒倒计时→ACK1902成交/流拍→10间歇源3秒后下一轮9/2059→最后一件ACK后11。574各状态都同步N578同一所属状态。不会开没有拍品表的窗口，也不将不同拍品混成同一索引。

有效C1902成功先以`bakal-bidding-bid:<run>:<lot>`事务冻结金币，返回ACK1902 outcome0并同步N13；倒计时结束后以既有购买事件发放一次，已冻结金额不再扣第二遍。单人成员原生界面不允许自己再次加价，不伪装成完整12人竞拍。原帧摘要绑定所属轮次，重复帧不重复扣款、不延长倒计时，旧轮帧不能买下一件。原生拒绝10/18/19分别显示金币不足/已出价/无进行中的竞拍，ACK失败会复位pending latch；未支持或非法输入用原生通用22拒绝，不猜取消分支。

新增可选存档字段Reserved/Closed/Released，无schema迁移，旧字段缺省false。未出价轮次持久关闭为流拍；未完成的冻结金额在C13离开时退款并同步背包，断线/换角色后在下次角色进入、初始化库存前Recover退款。已成交的物品不再退款。对冻结成功的确认直接读取提交返回的当前角色快照，不依赖扣款后第二次receipt查询，避免该查询超时却将已扣款出价报成失败。发奖失败保留可退款的冻结金额，不部分发奖；错误路径保留提交后的角色快照。

基础材料/奖励箱预览采用已见证的零辅助字段；周拍品特殊标志及181字节装备预览未闭合时不猜填。未绑定预览或源解析失败记入事件，不能以重掷拍品或假物品替代。完整UI仍需用户实机验收，0x326冷创建和N588度量缺口保持上节结论。

## 验证

PVF原生Bakal专项五包通过，涵盖困难建团/重开、独立奖励池、冻结/领奖幂等、材料原生向量、报价原生向量和奖励箱单lot。奖励页测试检查等待期限、真实冻结数量、顺序、重复tick，以及未绑定装备预览拒绝。构建、vet和最终全量测试的实际结果见本轮交付说明及模块`.tmp/bakal-hard-20261013/`。仅更新隔离候选，不自动启动游戏，不替换已确认默认程序，未提交用户其它改动。

最终`go build ./...`、`go vet ./...`、`go test ./... -count=1`均exit0；真实PVF五包Bakal专项均通过。候选`bin/wireprobe-handoff-source.exe`31388672字节，SHA256 `16137095a7146b306658b16c07f57bad7eeb7aa499b616537a850edd0f11b311`。上版候选3c9638d7保存在模块`.tmp/bakal-hard-20261013/wireprobe-before-3c9638d7.exe`。实机入口是仓库`scripts/bakal-source-test.cmd`，仅由用户手动运行。

上述16137095为竞拍接线前候选；本轮接线后重新执行build/vet/full test，均exit0。拍品池/盒子两层、金币冻结、重复帧、轮次边界、成交、流拍、关闭和退出/重登退款测试通过；真实PVF专项日志为`auction-native-tests.txt`，全量为`auction-full-tests.txt`。最新候选指纹见本轮交付说明。

竞拍接线后候选`bin/wireprobe-handoff-source.exe`31510528字节，SHA256 `db4655146e9e8a78372aca49194fcad06e40f31b9f01135f691105fdd6e6cd57`；前版16137095保存为模块`.tmp/bakal-hard-20261013/wireprobe-before-auction-16137095.exe`。默认`wireprobe-pvf.exe`仍为BF22D24D，不升级默认、不启动游戏。新可选存档字段兼容旧记录，测试使用内存存储，没有写入玩家SQLite。

## 启动器构建不匹配修复

用户20:11:56启动会话在ready.json前退出，gateway.err最后错误为`open configs/apocalypse-flip-gear.generated.json: The system cannot find the file specified.`。实际配置存在于模块configs，网关工作目录不是模块。末世录移植提交1de70c91已在launcher/gateway.go传递该池的绝对路径，现有gateway测试也覆盖；但活动bin/dfolauncher.exe仍是VCS382f4452（10月7日）的旧构建，没有下发该参数。故不是巴卡尔竞拍报错，也不创建第二份配置或修改奖励规则。

备份旧启动器到模块`.tmp/bakal-startup-20261013/dfolauncher-before.exe`，由当前源码重新编译bin/dfolauncher.exe：16970240字节，SHA256 `37761f8967c64be8921fdc2a1ce385c5699d7281ea2d2e7f12d77325d78c6547`。internal/launcher和cmd/dfolauncher完整测试、vet通过；只读launch --dry-run通过，未启动网关/客户端或修改存档。原巴卡尔候选与默认网关均不变，用户重新手动运行同一测试脚本验证实际启动。

## 本次实机出价未知错误与退出攻坚队修复

实机会话20261013_202658_006762_next37：四次C1902正文均为0010c07a120000000000000000000000，金额310034448。服务端以“unsupported native bidding action”拒绝并发001600，客户端显示Unknown Error(22)。用户明确确认是在出价/确认出价时出现。当前磁盘2.38.2.34静态普通出价和大额确认仍预期动作1（1425CF8D0阈值100000000、1425C51DD清r8后1425C6830异或发送），不能伪称静态已证明确认发0。服务端根据这次真实输入及用户操作确认，兼容0/1的正金额出价；0金额、未知动作和非零padding仍拒绝，不推测弃拍动作。余额、所属轮次、旧帧、冻结金额、重复扣款及发奖保护保持。真实3.1亿向量和全余额出价回归已覆盖。

同会话先C1353查看我的攻坚队，再C657(actor2)、C13，原代码只记录657，却在C13里nil掉攻坚队。服务端记录已空，而客户端后续仍以旧团ID查询1353，退出窗口没有同步。官服hard s37在01:17:19先C13，只离开小队：mode2/3/4及N9更新，团ID保持；01:17:28 C657(actor41626)+C13后ACK657成功、N578mode1删除团缓存；重复657为error19、重复13为error18。当前N578mode1 reader144CE3723→144CDB900按团ID删除缓存，不读取成员或随机尾部。

现在C657按当前所属演员校验，等待或结束阶段先退还未完成冻结金额，再成功ACK657及N578mode1删除所属缓存；如仍有普通小队，发已确认N9清理。队伍、竞拍和奖励定时队列随后释放。C13只清普通小队并刷新owned/member记录，不再删除攻坚队；重复退出按官服已见错误应答，避免pending等待。保留活动战斗阶段的退出门禁，未猜补战斗中整团退出。旧存档事件和每周不消耗次数策略不变。

改动：bakal_flow.go、bakal_auction_test.go、协议bid解码与真实向量、RaidGone115及原生删除向量；C657常量改为准确的LeaveRaid，旧别名保留。没有改动客户端、PVF、启动器或玩家SQLite。真实PVF巴卡尔五包专项通过；本次完整build/vet/test及候选指纹见交付记录，实机仍由用户操作。

本次交付：go build ./...、go vet ./...、常规go test ./... -count=1均exit0，日志bid-exit-standard-full-tests.txt；真实PVF巴卡尔五包专项通过。额外将PVF专项环境带入整个仓库的检查发现TestNativeLearningCurrentArchiveCompleteDetails拒绝当前b814归档，该测试固定另一份源指纹，internal/character未改；记录失败而不修改无关角色规则。候选bin/wireprobe-handoff-source.exe为31519744字节，SHA256 265448b3ff2f91be17dfd9c885cd3bb334d630c8d5adf8c4cde49bc7bff45bb5。前版db465514备份在模块.tmp/bakal-hard-20261013/wireprobe-before-bid-exit-db465514.exe。默认程序未替换，不启动游戏；请用户手动验证实际出价及“我的攻坚队→退出”。

补充扩大全库原生检查的完整失败集合（13项，非巴卡尔专项）：TestNativeLearningCurrentArchiveCompleteDetails、TestPVFMigrationSourceOnlyLocalArchive、TestNativeDungeonsCurrentArchiveFingerprint、TestNativeEnhancementsCurrentArchiveFingerprint、TestPVFSourceOnlyLocalArchiveDoesNotReadSelectedJSON、TestNativeWorldQuestsCurrentArchiveFingerprint、TestNativeItemEquipmentCurrentArchiveFingerprint、TestNativeCommerceCurrentArchiveFingerprint、TestPVFClosingScenesLocalArchive、TestPVFScenesLocalArchive、TestCurrentPVFItemContentHistoricalFieldParity、TestJointComplexItemsLocalArchiveParity、TestItemIndexLocalArchive。分别涉及8b2a/7ef2历史源指纹与当前b814不一致、历史scene/item投影差异及缺少历史items.index.json；这些测试/相关生产域未因本轮出价与退出修改。不将这次额外检查描述为全通过，不更改源指纹或补造JSON以压过失败；完整原始输出为bid-exit-final-full-tests.txt。常规无专项环境的全量结果另存bid-exit-standard-full-tests.txt(exit0)，巴卡尔真实PVF专项独立验证通过。本轮未提交，相关非巴卡尔基线差异留待单独处理。
