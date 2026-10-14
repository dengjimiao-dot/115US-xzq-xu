package main

import (
	"encoding/json"
	"sort"

	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
)

// 衣柜内容 → noti1077 下发（2026-10-14 缺口②：业主最早报的"重登衣柜空了"）。
//
// 账本（avatar_closet_ledger.go）只记流水；登录时要下发的是**终态**：每个柜槽里
// 现在摆的是哪一件、它在装扮栏（容器 1）的哪个槽。本文件把流水折叠成终态，再按
// 装扮栏的当前布局解析出帧里的 LinkedSlot。
//
// ★ 帧条目语义（反汇编 DFO.exe 0x1449e8100 + 0x145af7970 钉死）：
//   mode 1 条目 {A=33, B=柜槽, C=1, D=装扮栏槽} 是一记**搬运/交换**——客户端先把
//   装扮栏 D 槽的记录装进柜槽 B（登录恢复走这条：客户端衣橱刚初始化必为空），再把
//   柜槽 B 的原记录装回 D 槽。所以：
//     - 登录推：柜空、栏有货 ⇒ 货从栏"存"进柜，栏槽腾空。正合"寄存衣柜"语义。
//     - ★ 因此**绝不可**在 CMD1102（开窗/刷新）时补推：那时客户端柜槽已被登录帧
//       填上，再推同一批条目会把货原路搬回装扮栏——衣柜重新变空，正是业主报的 bug。
//     - 同一会话内的拖进/拖出客户端本地已生效，服务端只记账、不补推。
//
// ★ LinkedSlot（D）必须按模板**现查**装扮栏当前槽位，不能沿用账本旧 BagSlot：
//   玩家整理装扮栏（sort/drag 经 MoveOrdinary）会持久化按槽匹配的行，货会搬家。
//
// ★ 失败一律降级为空列表（4 个零字节），与 2026-10-14 之前行为逐字节一致：
//   客户端 reader 对 n<=0 直接返回，最坏只是不显示柜内容，绝不卡进城。

// ★ 「洞位」覆盖隐患与兜底行（2026-10-14 下午根治）：货存进柜后客户端腾空
//   装扮栏槽，但服务端 Special[1] 的 backing row 仍在该槽；玩家往这个"看起来
//   空了"的槽拖新货，MoveOrdinary 会**覆盖**被存物品的行 ⇒ 下次登录该柜槽在
//   装扮栏里解析不出模板 ⇒ 件丢。对策：Put 时按柜槽存整行备份
//   （avatar_closet_backing.go），登录编译现查不到就用备份行**合成一个空闲
//   装扮栏槽**，行随 NOTI13 装扮栏快照下发（plan.AvatarReady），客户端再由
//   noti1077 把它搬进柜槽 —— 全程只走已验证安全的「NOTI13 space 1 + noti1077」
//   路径，不给客户端发任何容器 33 的帧。

// foldAvatarClosetLedger 把流水账折叠成"柜槽 → 条目"的终态。
//
// 折叠规则（保序重放，玩家的操作顺序本身是信息）：
//   - Put（1→33）：该柜槽被这一件占据（后来的 Put 覆盖先前的）；
//   - Take（33→1）：该柜槽取空；
//   - Swap33（33→33）：两个柜槽的内容互换（单边为空即单向搬运）。
func foldAvatarClosetLedger(ledger avatarClosetLedger) map[uint16]avatarClosetLedgerEntry {
	folded := make(map[uint16]avatarClosetLedgerEntry, len(ledger.Items))
	for _, item := range ledger.Items {
		switch item.Op {
		case avatarClosetLedgerPut:
			folded[item.ClosetSlot] = item
		case avatarClosetLedgerTake:
			delete(folded, item.ClosetSlot)
		case avatarClosetLedgerSwap33:
			// avatarClosetLedgerSlots 对 swap33 返回 (源槽, 目的槽)，
			// 即 ClosetSlot=源、BagSlot=目的。先取两侧快照再写，才是真交换。
			from, hasFrom := folded[item.ClosetSlot]
			to, hasTo := folded[item.BagSlot]
			if hasFrom {
				from.ClosetSlot = item.BagSlot
				folded[item.BagSlot] = from
			} else {
				delete(folded, item.BagSlot)
			}
			if hasTo {
				to.ClosetSlot = item.ClosetSlot
				folded[item.ClosetSlot] = to
			} else {
				delete(folded, item.ClosetSlot)
			}
		}
	}
	return folded
}

// avatarClosetLinkedSlot 现查某一柜槽条目对应的装扮栏槽。
//
// 账本 Item 存的是 **Template ID**（cmd19 SourceItem 的语义，见 next42 文档与
// list 3 的 w.Template == r.SourceItem 实证），按模板在 b.Special[1]（装扮栏）
// 里找当前行：
//   1. 账本记录的 BagSlot 上正好还是这件 ⇒ 用这个槽（玩家没动过它）；
//   2. 否则取同模板的**最小槽**（整理过装扮栏，货搬家了；取最小只是确定性的
//      兜底，同模板两件并存时以账本 BagSlot 为准）；
//   3. 装扮栏里找不到这件（脱下/分解/转移走）⇒ 该柜槽跳过（客户端显示空格，
//      安全降级；条目推下去反而会把别的货吸进柜子）。
func avatarClosetLinkedSlot(rows []inventory.BagEquipment, entry avatarClosetLedgerEntry) (uint16, bool) {
	// BagSlot 哨兵（换装换来的"身上件"）：它**根本不在装扮栏里**，现查只会按模板
	// 撞到一件无关的背包件（同模板时把玩家的真货误判成寄存件藏起来）⇒ 直接交给
	// 兜底行合成路径。
	if entry.BagSlot == avatarClosetNoBarSlot {
		return 0, false
	}
	fallback := int64(-1)
	for _, row := range rows {
		if row.Template != entry.Item {
			continue
		}
		if row.Slot == entry.BagSlot {
			return row.Slot, true
		}
		if fallback < 0 || int64(row.Slot) < fallback {
			fallback = int64(row.Slot)
		}
	}
	if fallback >= 0 {
		return uint16(fallback), true
	}
	return 0, false
}

// avatarClosetInjectedEntry 是一组"兜底行合成"的柜内容条目：帧条目本身，
// 加上要随 NOTI13 装扮栏快照一起下发的那一行（登录时合成进空闲槽）。
type avatarClosetInjectedEntry struct {
	Entry protocol.AvatarClosetSetEntry
	Row   inventory.BagEquipment
}

// avatarClosetSetPlan 编译 noti1077 的柜内容，区分"装扮栏现查"与"兜底行合成"
// 两类条目。
//
// 第一遍（主路径，与旧实现逐字节一致）：账本终态 → 现查装扮栏当前槽位。
// 第二遍（兜底路径）：现查不到的柜槽（行被"洞位"里的新货覆盖了）⇒ 用 Put 时
// 存的备份行合成一个空闲装扮栏槽，随 injected 返回；调用方把行合并进
// plan.AvatarReady 后，客户端再由 noti1077 把它搬进柜槽。
//
// 取舍原则：
//   - 现况永远优先于备份（备份只在现况缺失时兜底）；
//   - LinkedSlot 撞车去重覆盖两类条目的并集（现查小者先占，柜槽小者优先）；
//   - 装扮栏没有空闲槽（120+8×tier 全满）⇒ 合不了，该柜槽只能跳过；
//   - 读账本/背包/备份失败只返错，由调用方记事件并降级，绝不阻断登录。
func avatarClosetSetPlan(state json.RawMessage) (live []protocol.AvatarClosetSetEntry, injected []avatarClosetInjectedEntry, err error) {
	ledger, err := readAvatarClosetLedger(state)
	if err != nil {
		return nil, nil, err
	}
	folded := foldAvatarClosetLedger(ledger)
	if len(folded) == 0 {
		return nil, nil, nil
	}
	bag, err := inventory.ReadBag(state)
	if err != nil {
		return nil, nil, err
	}
	backing, err := readAvatarClosetBacking(state)
	if err != nil {
		return nil, nil, err
	}
	// 柜槽总容量 = 套数 × 每套 12 槽（见 protocol.AvatarClosetCapacity 的定案注释）。
	// 超出容量的柜槽（扩容前存的大号槽、坏账）直接丢：客户端 apply 助手对
	// 越界槽只会记日志丢条目（0x145af7970 的 [container+0x58] 边界检查返
	// false），服务端先把关只是让事件更好读。
	// ★ 2026-10-14 教训：曾按 expansion+1（=套数）当总槽数钳制，业主 14 件只回 4 件
	//   （"只保存了几件"）——首字段是套数，一套 12 槽，客户端真的接受 23/45 号槽。
	closetSlots := protocol.AvatarClosetCapacity(bag.ClosetExpansion)
	rows := bag.Special[protocol.AvatarContainerID] // 装扮栏（list 1）
	order := make([]uint16, 0, len(folded))
	for slot := range folded {
		order = append(order, slot)
	}
	sort.Slice(order, func(i, j int) bool { return order[i] < order[j] })

	// 第一遍：装扮栏现查。
	occupied := make(map[uint16]bool, len(rows))
	for _, row := range rows {
		occupied[row.Slot] = true
	}
	usedLinks := make(map[uint16]uint16, len(order)) // LinkedSlot → 首个占用的柜槽
	liveSlots := make(map[uint16]bool, len(order))   // 第一遍已发条目的柜槽
	live = make([]protocol.AvatarClosetSetEntry, 0, len(order))
	for _, closetSlot := range order {
		if closetSlot >= closetSlots {
			continue
		}
		item := folded[closetSlot]
		linkedSlot, ok := avatarClosetLinkedSlot(rows, item)
		if !ok {
			continue
		}
		// ★ LinkedSlot 撞车保护：两条条目指向同一个装扮栏槽时，客户端会按序两记
		// 搬运——第二记的 1.D 已被第一记换成柜槽旧内容（通常为空），会把错误记录
		// 吸进第二个柜槽。同模板多件且装扮栏只剩一行时才会走到这（兜底取最小槽），
		// 确定性处置：柜槽小的先发，撞车的后续条目交给第二遍（有备份就合成，
		// 没备份只能跳过——客户端显示空格，不串货）。
		if _, taken := usedLinks[linkedSlot]; taken {
			continue
		}
		usedLinks[linkedSlot] = closetSlot
		liveSlots[closetSlot] = true
		live = append(live, protocol.AvatarClosetSetEntry{
			Container:  protocol.AvatarClosetContainerID,
			Slot:       closetSlot,
			Mode:       protocol.AvatarClosetSyncMode,
			LinkedSlot: linkedSlot,
		})
	}
	// 第二遍：现查缺失的柜槽用备份行合成空闲槽（低槽优先，与 clone_avatar.go
	// 的迁移口径一致）。备份行正是当初 Put 时的那一整行（BagEquipment JSON
	// 无损往返，强化/染色/克隆来源/记录全在），客户端拿到的就是原件。
	backup := backingIndex(backing)
	limit := protocol.AvatarInventorySlots(bag.AvatarExpansion)
	for _, closetSlot := range order {
		if closetSlot >= closetSlots || liveSlots[closetSlot] {
			continue
		}
		stored, ok := backup[closetSlot]
		if !ok {
			continue // 没备份（上线前摆的件且行已丢）⇒ 只能跳过
		}
		// 备份行与账本终态对不上（坏数据/历史遗留）⇒ 不注入：宁可空格，也不把
		// 别的货吸进柜槽。
		if stored.Row.Template != folded[closetSlot].Item {
			continue
		}
		free, ok := avatarClosetFreeBarSlot(occupied, limit)
		if !ok {
			break // 装扮栏合满了：后面的柜槽也一样合不了
		}
		occupied[free] = true
		row := stored.Row
		row.Slot = free
		injected = append(injected, avatarClosetInjectedEntry{
			Entry: protocol.AvatarClosetSetEntry{
				Container:  protocol.AvatarClosetContainerID,
				Slot:       closetSlot,
				Mode:       protocol.AvatarClosetSyncMode,
				LinkedSlot: free,
			},
			Row: row,
		})
	}
	return live, injected, nil
}

// avatarClosetFreeBarSlot 取装扮栏最小的空闲槽；满了返回 false。
func avatarClosetFreeBarSlot(occupied map[uint16]bool, limit uint16) (uint16, bool) {
	for slot := uint16(0); slot < limit; slot++ {
		if !occupied[slot] {
			return slot, true
		}
	}
	return 0, false
}

// mergeClosetEntries 把现查条目与注入条目合并成按柜槽升序的帧条目表
// （客户端按序 apply，升序与旧行为一致，输出确定）。
func mergeClosetEntries(live []protocol.AvatarClosetSetEntry, injected []avatarClosetInjectedEntry) []protocol.AvatarClosetSetEntry {
	if len(injected) == 0 {
		return live
	}
	entries := make([]protocol.AvatarClosetSetEntry, 0, len(live)+len(injected))
	entries = append(entries, live...)
	for _, item := range injected {
		entries = append(entries, item.Entry)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Slot < entries[j].Slot })
	return entries
}

// ─────────────────────────────────────────────────────────────────────────────
// 装扮栏下发剔除（2026-10-14 晚，"点整理后寄存件又回到背包"根治）
//
// 机理（会话 roles_..._20261014_145617_703554_next37 逐帧对账钉死）：
//   - cmd19(1→33) 只记账、不动背包 ⇒ 货"存"进柜后服务端 Special[1] 里那件时装的行
//     **仍留在原装扮栏槽**（登录 noti1077 正是靠这行把它搬进柜槽完成恢复，所以它暂时有用）；
//   - noti1077 mode 1 在客户端侧是一记**搬运**（`0x145af7970(33,B,record@1.D)` 后
//     `0x145af7970(1,D,record@33.B)`）⇒ 搬完之后**客户端侧的装扮栏不含这些行**；
//   - 于是服务端 Special[1]（含寄存行）与客户端装扮栏（不含）**行数不等**，任何"把
//     Special[1] 整体重推给客户端"的路径都会把这批件**重新塞回背包**——业主 2026-10-14
//     报的"点整理之后会显示在错误位置、会显示在包里"，以及连带的
//     "角色的时装栏也出现显示错误、不显示穿在身上的时装"（同一批重复行让客户端的
//     衣柜链接/实例记账错位）。
//
// 证据（同一会话登录帧 vs 整理帧逐槽对齐）：
//   - 登录帧 118 行；客户端整理请求 perm（216 槽表）非恒等项只在 42..86/104/116；
//   - 客户端整理结果 = perm(装扮栏 118 行 **减去** 14 个寄存槽) = 104 行；
//   - 服务端实际推的 = perm(118 行) = 客户端 104 行 + **多推的 14 行**，
//     多推槽位 = perm(寄存槽) = {85,105..117}，正是那 14 件寄存件。
//   ⇒ 修法：非登录路径一律用 avatarBarDeliveryRows / avatarBarRestorePayload 下发。
//
// 为什么登录路径**不能**剔除：登录时客户端柜子刚初始化必为空，noti1077 要靠装扮栏
// 快照里那一行才能把它"搬"进柜槽（见本文件头注的 mode-1 语义）。登录之后客户端
// 自己已经把行搬走，再重推才会出错。
//
// 只读、失败即降级：读账本/背包失败 ⇒ 返回 nil（等价"没有寄存行"），退化成 2026-10-14
// 晚之前的行为，绝不阻断整理/抽奖/商城/扩容这些路径。
// ─────────────────────────────────────────────────────────────────────────────

// avatarClosetHiddenBarSlots 返回装扮栏（Special[1]）里"其实已经寄存在衣柜"的行槽集合。
//
// 取 avatarClosetSetPlan 第一遍（装扮栏现查）的 LinkedSlot：它正是客户端 noti1077
// 搬走的那一批槽。第二遍（兜底行合成）的槽不在装扮栏里，不需要剔除。
func avatarClosetHiddenBarSlots(state json.RawMessage) map[uint16]bool {
	live, _, err := avatarClosetSetPlan(state)
	if err != nil || len(live) == 0 {
		return nil
	}
	hidden := make(map[uint16]bool, len(live))
	for _, entry := range live {
		hidden[entry.LinkedSlot] = true
	}
	return hidden
}

// avatarBarRowsWithoutCloset 从装扮栏行切片里剔除 hidden 指定的槽。
// hidden 为空（nil）时**原样返回入参**，保证老存档/空账本路径逐字节不变。
func avatarBarRowsWithoutCloset(rows []inventory.BagEquipment, hidden map[uint16]bool) []inventory.BagEquipment {
	if len(hidden) == 0 {
		return rows
	}
	kept := make([]inventory.BagEquipment, 0, len(rows))
	for _, row := range rows {
		if hidden[row.Slot] {
			continue
		}
		kept = append(kept, row)
	}
	return kept
}

// avatarBarDeliveryRows 返回"客户端侧装扮栏真正持有的行"：Special[1] 剔除衣柜寄存行。
func avatarBarDeliveryRows(state json.RawMessage, bag inventory.Bag) []inventory.BagEquipment {
	return avatarBarRowsWithoutCloset(bag.Special[protocol.AvatarContainerID], avatarClosetHiddenBarSlots(state))
}

// avatarBarRestorePayload 构造剔除衣柜寄存行后的 NOTI13 space 1 整体恢复帧。
//
// 与 SpecialEquipmentRestorePayload(state, 1) 的唯一差别就是剔除了寄存行：
// 头部（space / tier*8 / 行数）与逐行编码完全一致，所以空账本时逐字节相同。
func avatarBarRestorePayload(state json.RawMessage) ([]byte, error) {
	bag, err := inventory.ReadBag(state)
	if err != nil {
		return nil, err
	}
	return inventory.EquipmentPayload(1, avatarBarDeliveryRows(state, bag), true, bag.AvatarExpansion)
}

// avatarBarUpdatePayload 构造剔除衣柜寄存行后的装扮栏**增量**更新帧
// （NOTI14 非 restore 形态：无 tier 头）。商城/邮件领取等"整栏重推"路径用它。
func avatarBarUpdatePayload(state json.RawMessage) ([]byte, error) {
	bag, err := inventory.ReadBag(state)
	if err != nil {
		return nil, err
	}
	return inventory.EquipmentPayload(1, avatarBarDeliveryRows(state, bag), false)
}

// avatarClosetSetEntries 从角色存档编译 noti1077 的柜内容条目（两类条目合并）。
//
// 保留这个入口给"只看帧条目、不合并快照"的调用方（既有测试与 wearService 为
// nil 的老路径）。注入条目的行由调用方另外并进 plan.AvatarReady —— 合并失败时
// 用 avatarClosetSetPlan 的 live 单独降级，保证帧条目永远引用的都是已下发行。
//
// 返回空切片 = 空账本或无有效条目（调用方发 4 个零字节，与旧行为一致）。
// 读账本失败/读背包失败只返错，由调用方记事件并降级，绝不阻断登录。
func avatarClosetSetEntries(state json.RawMessage) ([]protocol.AvatarClosetSetEntry, error) {
	live, injected, err := avatarClosetSetPlan(state)
	if err != nil {
		return nil, err
	}
	if len(injected) == 0 {
		return live, nil
	}
	return mergeClosetEntries(live, injected), nil
}
