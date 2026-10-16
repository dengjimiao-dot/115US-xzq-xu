package main

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"dfolan/internal/database"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
)

// 衣柜「change」按钮 → CMD1102：把**选中套**的寄存件穿到角色身上，同时把身上那套换进柜子。
//
// ── 客户端链路（DFO.exe 反汇编钉死，2026-10-14 晚，无 IDA、capstone 直读） ─────────
//
// 1) 衣柜窗口事件处理 `0x145054280`（args: rcx=窗口对象, edx=控件号, r8d=事件类型）：
//    事件 13 分支里，控件 **0x514(1300) / 0x8FC(2300)** 走这段：
//      `call 0x146D74000`(取发包上下文) → `mov edx, 0x44E` → `call 0x146D746E0`(begin 1102)
//      → `movzx edx, bl`（bl = 选中套号 = 2*下拉索引-2 或 -1）→ `call 0x146D75CC0`(写 1 字节)
//      → `call 0x146D75AF0`(send) → `0x1406BD410(obj)` 置 obj[8]=1（pending）。
//    同一分支里控件 **0x578(1400) / 0x960(2400)** 发的才是 CMD1103（只切选中套号，
//    先拿 `obj[0x10]`(当前选中) 比对，相同就不发）。
//    ⇒ **CMD1102 的包体 = 1 字节：选中的衣柜套号（0..3）。**
//
// 2) 1102 的**回包 handler `0x1449E7F70` 不读任何字段**：
//      `getter(); byte[obj+8] = 0; ret`（只清 pending）。
//    ⇒ 客户端本地**不做任何搬运**，换装后的新状态完全由服务端下发。
//
// 3) 客户端的前置检查 `0x145053540`：遍历**容器 33（衣柜）**，至少一件满足
//    `vtable[0x390]` 才允许发包（空柜点按钮只弹提示 dstr 0xc73c / UI 0xb3b，不发包）。
//
// 4) noti1077 mode 1 条目的客户端实现（`0x1449E8534` / `0x1449E87D6`）是
//    **容器 A 的 B 槽 ↔ 容器 1 的 D 槽 互换**（两记 `0x145AF7970`）：
//      0x145AF7970(containerA, B, item@container1.D)   // A.B ← 栏 D
//      0x145AF7970(container1, D, item@containerA.B)   // 1.D ← 柜 B
//
// 5) 容器 getter `0x145A0E8C0` 支持的 A：
//      0,1,2,4,7,11,12,13,18..22,28,30,**33**,35,42,43,45 —— **不含 3（穿戴）**。
//    ⇒ 穿戴栏只能靠 NOTI13 space 3 + id14 space 3 下发；衣柜（33）只能靠 noti1077。
//
// ── 因此服务端换装 = 三步 ─────────────────────────────────────────────────────
//   a. 下发"预备装扮栏快照"：把**身上那几件**放到装扮栏的空闲临时槽（客户端先有货，
//      noti1077 才有东西可搬）；
//   b. 下发 noti1077 条目 {A=33, B=柜槽, C=1, D=临时槽} —— 客户端把 33.B ↔ 1.D 互换：
//      身上件进柜槽、柜里的件落到装扮栏临时槽；
//   c. 下发最终装扮栏快照（把临时槽清掉）+ NOTI13 space 3 穿戴快照 + id14 槽位 + 外观。
//
// 配对按**部位**：柜槽 s 的件查 `[equipment type]` → 穿戴槽（0..11）；查不到时退回
// "套内第 k 槽 ↔ 穿戴槽 k"（客户端每套 12 槽 = 12 个时装部位，与穿戴槽同序）。
//
// 安全边界：只做**行在容器之间的置换**，绝不新建/销毁物品；任何一步不满足（柜槽没有
// 兜底行、部位解析不出、装扮栏没有临时槽…）⇒ 整个换装放弃，只回 ACK，存档一个字节不动。

// avatarClosetChangeSwitch 是运行时开关（无需重启）：文件缺失 = 开（业主 2026-10-14
// 明确要求该功能）；内容为 0/off/false/no/disable 时关。
const avatarClosetChangeSwitch = "runtime/avatar-closet-change/mode"

// avatarClosetNoBarSlot 是账本 BagSlot 的哨兵：这件**不在装扮栏里**（换装换来的身上件）。
// 登录编译看到它就走兜底行合成，绝不按模板去装扮栏里乱认 —— 同模板撞车会把玩家真正的
// 背包件误判成"寄存在柜里"而藏起来。
const avatarClosetNoBarSlot uint16 = 0xFFFF

// avatarClosetChangeModel 是落库模型名（幂等键的一部分）。
const avatarClosetChangeModel = "avatar-closet-change-v1"

// avatarClosetChangeDisabled 读运行时开关。
func avatarClosetChangeDisabled() bool {
	b, err := os.ReadFile(avatarClosetChangeSwitch)
	if err != nil {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(string(b))) {
	case "0", "off", "false", "no", "disable", "disabled":
		return true
	}
	return false
}

// avatarClosetChangePair 是换装里"某个时装部位"的一对：柜里那件 ↔ 身上那件。
//
// 两侧都可能缺席（业主 2026-10-14 明确：身上没穿也要能把柜里的穿上；柜里没货也要能
// 把身上的收进柜）——所以 ClosetRow / WornRow 各自用 Has* 标记，缺哪边就只做另一边。
type avatarClosetChangePair struct {
	ClosetSlot uint16
	WornSlot   uint16
	Scratch    uint16 // 装扮栏里的临时槽（noti1077 的 D）
	ClosetRow  inventory.BagEquipment
	HasCloset  bool
	WornRow    inventory.BagEquipment
	HasWorn    bool
	// Deposit：身上那件**可以**寄存进柜子（普通时装）。false 时该临时槽留空，
	// noti1077 只把柜槽腾空、柜里那件落到装扮栏，身上那件改由最终装扮栏快照放回栏里。
	Deposit bool
}

// avatarClosetChangeCatalog 是换装需要的目录能力（注入式，便于单测）。
type avatarClosetChangeCatalog struct {
	// WornSlotOf：模板 → 穿戴槽（0..11），来自 PVF `[equipment type]` 与装备穿戴规则表。
	WornSlotOf func(template uint32) (uint16, bool)
	// IsClone：是不是克隆时装。克隆件**不许进柜**（业主 2026-10-14：clone 时装不能放进
	// 时装柜 —— 它只是替换 avatar 外观的叠加件，不是可寄存的实体）。
	IsClone func(template uint32) bool
	// GroupOf：模板 → 穿戴行分组（普通时装 1 / 克隆 0），与 MoveOrdinary 的 itemGroup 同口径。
	GroupOf func(template uint32) byte
}

// avatarClosetChangePlan 是一次换装的完整结果（纯数据，便于单测）。
type avatarClosetChangePlan struct {
	Set      byte
	Pairs    []avatarClosetChangePair
	BarRows  []inventory.BagEquipment // 换装后的装扮栏 = 客户端侧装扮栏
	Worn     []inventory.BagEquipment // 换装后的穿戴栏（bag.Worn 全量）
	WornBase []inventory.BagEquipment // 上面那份的逐槽展示行（space 3 的 id14 用）
	PrepRows []inventory.BagEquipment // 预备快照（装扮栏 + 可寄存的身上件放临时槽）
	Entries  []protocol.AvatarClosetSetEntry
	// ClonesToBar 是"身上是克隆件、不能被寄存"的部位：件要放回装扮栏（槽 = Scratch）。
	ClonesToBar []inventory.BagEquipment
	// Skipped 是本套里被跳过的柜槽（行找不回来 / 克隆件 / 同部位重复）——只记日志用。
	Skipped []uint16
	Ledger  avatarClosetLedger
	Backing avatarClosetBacking
}

// avatarClosetStoredRow 取某个柜槽里那件的**整行**：优先兜底行表（模板必须与账本终态
// 一致，坏数据不认），否则按登录编译同款规则去装扮栏现查。
func avatarClosetStoredRow(entry avatarClosetLedgerEntry, index map[uint16]avatarClosetBackingEntry,
	barRows []inventory.BagEquipment) (inventory.BagEquipment, bool) {
	if stored, ok := index[entry.ClosetSlot]; ok && stored.Row.Template == entry.Item {
		return stored.Row, true
	}
	slot, ok := avatarClosetLinkedSlot(barRows, entry)
	if !ok {
		return inventory.BagEquipment{}, false
	}
	for _, row := range barRows {
		if row.Slot == slot && row.Template == entry.Item {
			return row, true
		}
	}
	return inventory.BagEquipment{}, false
}

// planAvatarClosetChange 计算一次换装（纯函数；目录能力通过 catalog 注入，便于单测）。
//
// ★ 语义（业主 2026-10-14 两次明确）：
//   - "点 change 会把衣柜上的衣服穿到角色的身上"；
//   - "即使我没穿时装，衣柜里的衣服点 change 也应该换到我的身上"；
//   - "即使衣柜里没衣服，点 change，也要把我身上的衣服换到衣柜里"。
//   ⇒ 这是**整套时装的全量互换**：逐部位（0..11）把柜里的穿上、把身上的收进柜，
//     **两侧各自缺席也要照做**（身上没穿 ⇒ 只穿上；柜里没货 ⇒ 只收进）。
//
// 克隆件的规矩：**clone 时装不能放进时装柜**——它只是替换 avatar 外观的叠加件。所以：
//   - 柜槽里若是克隆件（异常数据）⇒ 该格跳过，不穿上、也不动它；
//   - 身上那个部位是克隆件 ⇒ 它**不寄存进柜**，而是放回装扮栏（Scratch 槽），
//     柜槽照样腾空、柜里那件照样穿上（不丢件、不把克隆塞进柜）；
//   - 身上是克隆件且柜里该部位没货 ⇒ 这个部位整个不动（克隆继续穿着）。
func planAvatarClosetChange(state json.RawMessage, set byte, catalog avatarClosetChangeCatalog) (*avatarClosetChangePlan, error) {
	bag, err := inventory.ReadBag(state)
	if err != nil {
		return nil, err
	}
	perSet := uint16(protocol.AvatarClosetSlotsPerSet)
	capacity := protocol.AvatarClosetCapacity(bag.ClosetExpansion)
	base := uint16(set) * perSet
	if uint32(base)+uint32(perSet) > uint32(capacity) {
		return nil, fmt.Errorf("closet set %d outside purchased capacity (%d slots)", set, capacity)
	}
	ledger, err := readAvatarClosetLedger(state)
	if err != nil {
		return nil, err
	}
	folded := foldAvatarClosetLedger(ledger)
	backing, err := readAvatarClosetBacking(state)
	if err != nil {
		return nil, err
	}
	index := backingIndex(backing)

	barRows := bag.Special[protocol.AvatarContainerID]
	// 换装后服务端 Special[1] = 客户端侧装扮栏（寄存行全部转为穿戴/换出）。
	bar := avatarBarRowsWithoutCloset(barRows, avatarClosetHiddenBarSlots(state))

	plan := &avatarClosetChangePlan{Set: set}

	// 1) 本套柜槽 → 部位。柜里那件自己带部位（`[equipment type]`），查不到才退回套内序号。
	type closetItem struct {
		slot uint16
		row  inventory.BagEquipment
	}
	closetByPart := map[uint16]closetItem{}
	occupied := map[uint16]bool{}
	for k := 0; k < int(perSet); k++ {
		slot := base + uint16(k)
		entry, ok := folded[slot]
		if !ok {
			continue
		}
		occupied[slot] = true
		row, ok := avatarClosetStoredRow(entry, index, barRows)
		if !ok {
			// 行已经找不回来（洞位被覆盖且没有备份）：这一格跳过，别拖累整次换装。
			plan.Skipped = append(plan.Skipped, slot)
			continue
		}
		if catalog.IsClone != nil && catalog.IsClone(row.Template) {
			plan.Skipped = append(plan.Skipped, slot) // 克隆件本就不该在柜里
			continue
		}
		part := uint16(k)
		if catalog.WornSlotOf != nil {
			if mapped, ok := catalog.WornSlotOf(row.Template); ok {
				part = mapped
			}
		}
		if part >= perSet {
			plan.Skipped = append(plan.Skipped, slot)
			continue
		}
		if _, dup := closetByPart[part]; dup {
			// 同一部位在柜里有两件（同套两顶帽子之类）：留靠前那件，靠后的留在柜里不动。
			plan.Skipped = append(plan.Skipped, slot)
			continue
		}
		closetByPart[part] = closetItem{slot: slot, row: row}
	}

	// 2) 穿戴栏 → 部位（穿戴槽本身就是部位号）。
	wornBySlot := map[uint16]inventory.BagEquipment{}
	for _, row := range bag.WornBaseItems() {
		if row.Slot < perSet && row.Template != 0 {
			wornBySlot[row.Slot] = row
		}
	}

	// 3) 逐部位配对（并集驱动：两侧任一边有货就要处理）。
	freeSlots := make([]uint16, 0, perSet)
	for k := 0; k < int(perSet); k++ {
		if slot := base + uint16(k); !occupied[slot] {
			freeSlots = append(freeSlots, slot)
		}
	}
	takeFree := func(prefer uint16) (uint16, bool) {
		for i, s := range freeSlots {
			if s == prefer {
				freeSlots = append(freeSlots[:i], freeSlots[i+1:]...)
				return s, true
			}
		}
		if len(freeSlots) == 0 {
			return 0, false
		}
		s := freeSlots[0]
		freeSlots = freeSlots[1:]
		return s, true
	}
	for part := uint16(0); part < perSet; part++ {
		ci, hasCloset := closetByPart[part]
		worn, hasWorn := wornBySlot[part]
		deposit := hasWorn && (catalog.IsClone == nil || !catalog.IsClone(worn.Template))
		if !hasCloset && !deposit {
			// 柜里该部位没货、身上的又是不能寄存的克隆件 ⇒ 这个部位不动。
			continue
		}
		pair := avatarClosetChangePair{WornSlot: part}
		if hasCloset {
			pair.ClosetSlot = ci.slot
			pair.ClosetRow = ci.row
			pair.HasCloset = true
		} else {
			slot, ok := takeFree(base + part)
			if !ok {
				// 本套已经没有空格可收：这件留在身上（绝不丢件），只记日志。
				continue
			}
			pair.ClosetSlot = slot
		}
		if hasWorn {
			pair.WornRow = worn
			pair.HasWorn = true
			pair.Deposit = deposit
		}
		plan.Pairs = append(plan.Pairs, pair)
	}
	if len(plan.Pairs) == 0 {
		return nil, fmt.Errorf("closet set %d and the worn set are both empty", set)
	}

	// 临时槽：客户端装扮栏与服务端 Special[1] 都空着的槽（两边同一份 bar，天然一致）。
	barOccupied := make(map[uint16]bool, len(bar)+len(plan.Pairs))
	for _, row := range bar {
		barOccupied[row.Slot] = true
	}
	limit := protocol.AvatarInventorySlots(bag.AvatarExpansion)
	for i := range plan.Pairs {
		pair := &plan.Pairs[i]
		free := uint16(0xFFFF)
		for s := uint16(0); s < limit; s++ {
			if !barOccupied[s] {
				free = s
				break
			}
		}
		if free == 0xFFFF {
			return nil, fmt.Errorf("avatar bar has no free slot for the swap")
		}
		barOccupied[free] = true
		pair.Scratch = free
		plan.Entries = append(plan.Entries, protocol.AvatarClosetSetEntry{
			Container:  protocol.AvatarClosetContainerID,
			Slot:       pair.ClosetSlot,
			Mode:       protocol.AvatarClosetSyncMode,
			LinkedSlot: free,
		})
	}

	// 预备快照：客户端侧装扮栏 + **可寄存的**身上件放到临时槽。
	// 克隆件不放临时槽（否则 noti1077 会把它搬进柜子）；它改由最终装扮栏快照放回栏里。
	plan.PrepRows = append(plan.PrepRows, bar...)
	for _, pair := range plan.Pairs {
		if !pair.HasWorn || !pair.Deposit {
			continue
		}
		row := pair.WornRow
		row.Slot = pair.Scratch
		plan.PrepRows = append(plan.PrepRows, row)
	}

	// 新装扮栏：先放回"不能被寄存"的克隆件（槽 = 它自己的临时槽，那里正好空着）。
	plan.BarRows = append(plan.BarRows, bar...)
	for _, pair := range plan.Pairs {
		if !pair.HasWorn || pair.Deposit {
			continue
		}
		row := pair.WornRow
		row.Slot = pair.Scratch
		plan.BarRows = append(plan.BarRows, row)
		plan.ClonesToBar = append(plan.ClonesToBar, row)
	}

	// 新穿戴栏：参与换装的部位整槽（所有分组）清掉，柜里有货的再穿上。
	// 只收不穿的部位（柜里没货）就保持空着 —— 这正是"身上的衣服被收进柜子"。
	plan.Worn = make([]inventory.BagEquipment, 0, len(bag.Worn)+len(plan.Pairs))
	for _, row := range bag.Worn {
		drop := false
		for _, pair := range plan.Pairs {
			if row.Slot == pair.WornSlot {
				drop = true
				break
			}
		}
		if !drop {
			plan.Worn = append(plan.Worn, row)
		}
	}
	groupOf := catalog.GroupOf
	if groupOf == nil {
		groupOf = func(uint32) byte { return 1 }
	}
	for _, pair := range plan.Pairs {
		if !pair.HasCloset {
			continue
		}
		row := pair.ClosetRow
		row.Slot = pair.WornSlot
		row.Group = groupOf(row.Template)
		plan.Worn = append(plan.Worn, row)
	}
	plan.WornBase = inventory.Bag{Worn: plan.Worn}.WornBaseItems()

	// 账本 + 兜底行：柜槽那件被换走（Take）；**只有可寄存的**身上件才进柜（Put，BagSlot = 哨兵）。
	nextLedger := ledger
	nextIndex := index
	at := time.Now().UTC().Format(time.RFC3339Nano)
	for _, pair := range plan.Pairs {
		if pair.HasCloset {
			nextLedger, _, err = appendAvatarClosetLedger(nextLedger, avatarClosetLedgerEntry{
				Op: avatarClosetLedgerTake, ClosetSlot: pair.ClosetSlot, Item: pair.ClosetRow.Template, At: at,
			})
			if err != nil {
				return nil, err
			}
		}
		if !pair.Deposit {
			delete(nextIndex, pair.ClosetSlot)
			continue
		}
		nextLedger, _, err = appendAvatarClosetLedger(nextLedger, avatarClosetLedgerEntry{
			Op: avatarClosetLedgerPut, ClosetSlot: pair.ClosetSlot,
			BagSlot: avatarClosetNoBarSlot, Item: pair.WornRow.Template, At: at,
		})
		if err != nil {
			return nil, err
		}
		row := pair.WornRow
		row.Slot = 0
		nextIndex[pair.ClosetSlot] = avatarClosetBackingEntry{
			ClosetSlot: pair.ClosetSlot, BagSlot: avatarClosetNoBarSlot, Row: row,
		}
	}
	plan.Ledger = nextLedger
	plan.Backing = rebuildAvatarClosetBacking(nextIndex)
	return plan, nil
}

// applyAvatarClosetChangeState 把换装结果落进角色存档（调用方在事务里执行）。
func applyAvatarClosetChangeState(state json.RawMessage, plan *avatarClosetChangePlan) (json.RawMessage, error) {
	bag, err := inventory.ReadBag(state)
	if err != nil {
		return nil, err
	}
	bag.Worn = plan.Worn
	if bag.Special == nil {
		bag.Special = map[byte][]inventory.BagEquipment{}
	}
	bag.Special[protocol.AvatarContainerID] = plan.BarRows
	state, err = inventory.SaveBag(state, bag)
	if err != nil {
		return nil, err
	}
	if state, err = writeAvatarClosetLedger(state, plan.Ledger); err != nil {
		return nil, err
	}
	return writeAvatarClosetBacking(state, plan.Backing)
}

// avatarWornSlotOf 用装备目录把一件时装映射到穿戴槽（0..11）。
func (client *gameConnection) avatarWornSlotOf(template uint32) (uint16, bool) {
	if client.wearService == nil || client.wearService.Catalog == nil {
		return 0, false
	}
	d, err := client.wearService.Catalog.Definition(template)
	if err != nil {
		return 0, false
	}
	kind := d.Fields["[equipment type]"]
	if len(kind) == 0 || kind[0].Type != 6 {
		return 0, false
	}
	slot, ok := client.wearService.Rules.Slots[kind[0].Text]
	if !ok || slot >= uint16(protocol.AvatarClosetSlotsPerSet) {
		return 0, false
	}
	return slot, true
}

// avatarWornGroupOf 用装备目录给出穿戴行分组（普通时装 1 / 克隆 0），与 MoveOrdinary 同口径。
func (client *gameConnection) avatarWornGroupOf(template uint32) byte {
	if client.avatarIsClone(template) {
		return 0
	}
	return 1
}

// avatarIsClone 判断一件时装是不是克隆件（克隆件不许进柜：业主 2026-10-14 明确）。
func (client *gameConnection) avatarIsClone(template uint32) bool {
	if client.wearService == nil || client.wearService.Catalog == nil {
		return false
	}
	d, err := client.wearService.Catalog.Definition(template)
	if err != nil {
		return false
	}
	return d.IsAvatar() && d.IsCloneAvatar()
}

// avatarClosetChangeCatalog 组装换装用的目录能力。
func (client *gameConnection) avatarClosetChangeCatalog() avatarClosetChangeCatalog {
	return avatarClosetChangeCatalog{
		WornSlotOf: client.avatarWornSlotOf,
		IsClone:    client.avatarIsClone,
		GroupOf:    client.avatarWornGroupOf,
	}
}

// avatarClosetChangePackets 生成换装的下发帧（顺序即客户端必须看到的顺序）。
func (client *gameConnection) avatarClosetChangePackets(role database.Character, plan *avatarClosetChangePlan) ([]outboundPacket, error) {
	bag, err := inventory.ReadBag(role.State)
	if err != nil {
		return nil, err
	}
	out := make([]outboundPacket, 0, 9)
	// a. 预备装扮栏快照：客户端先拿到"身上件在临时槽"的栏，noti1077 才有东西可搬。
	prep, err := inventory.EquipmentPayload(1, plan.PrepRows, true, bag.AvatarExpansion)
	if err != nil {
		return nil, err
	}
	out = append(out, outboundPacket{"avatar_closet_change_prep", 0, 13, prep})
	// b. 衣柜搬运：{33, 柜槽, 1, 临时槽} —— 客户端做容器 33.B ↔ 容器 1.D 互换。
	out = append(out, outboundPacket{"avatar_closet_change_swap", 0, 1077, protocol.AvatarClosetSetWith(plan.Entries)})
	// c. 最终装扮栏快照（临时槽清空）+ 穿戴栏 + 槽位 + 外观。
	bar, err := inventory.EquipmentPayload(1, plan.BarRows, true, bag.AvatarExpansion)
	if err != nil {
		return nil, err
	}
	out = append(out, outboundPacket{"avatar_inventory_restored", 0, 13, bar})
	if worn, e := inventory.WornPayload(role.State); e == nil && len(worn) > 0 {
		out = append(out, outboundPacket{"worn_equipment_restored", 0, 13, worn})
	}
	if slots, e := inventory.EquipmentPayload(3, plan.WornBase, false); e == nil && len(slots) > 0 {
		out = append(out, outboundPacket{"equipment_slots_updated", 0, 14, slots})
	}
	if upd, e := inventory.WornSpaceUpdate(role.State); e == nil && len(upd) > 0 {
		out = append(out, outboundPacket{"worn_equipment_window_refreshed", 0, 14, upd})
	}
	if src, e := cloneAvatarSourcePackets(role.State); e == nil {
		out = append(out, src...)
	}
	if client.characters != nil {
		if probe, e := client.characters.AppearanceProbe(role, [2]byte{}); e == nil {
			out = append(out, outboundPacket{"equipment_appearance_refreshed", 0, 2, probe})
		}
	}
	return out, nil
}

// avatarClosetChangeKey 是 CMD1102 换装的幂等键。
//
// ★ 必须带**账本条数**当前缀：CMD1102 的包体只有 1 字节套号，两次点击的内容完全相同，
// 只用内容摘要会让"第二次点 change"被当成重发而静默丢弃（2026-10-14 实机现场：
// 业主点了一次生效、之后再点 24 次全是 `avatar_closet_change_replayed`，换装像是坏了）。
// 沿用账本自己的口径 —— 每次成功的换装都会往账本追加条目（Take/Put），所以下一次点击的键
// 必然不同；而网络重发（同一次点击被重放）账本没变 ⇒ 键相同 ⇒ 走幂等回执，不会换两次。
// 被拒绝的点击（两侧都空 / 栏满）在事务里返回错误、不留事件行 ⇒ 不占键。
func avatarClosetChangeKey(wireID int64, configVersion string, ledgerEntries int, set byte, payload []byte) string {
	sum := sha256.Sum256(append([]byte{set}, payload...))
	return fmt.Sprintf("avatar-closet-change:%d:%s:%d:%x", wireID, configVersion, ledgerEntries, sum[:8])
}

// avatarClosetChange 处理 CMD1102（衣柜 change 按钮）。返回"是否有帧要发"。
//
// 任何失败都只记事件、**绝不改存档**（事务里返回错误即整体回滚），ACK 照发。
func (client *gameConnection) avatarClosetChange(requestData *clientRequest) []outboundPacket {
	if avatarClosetChangeDisabled() {
		return nil
	}
	if client.gameStore == nil || client.worldState == nil || client.wearService == nil {
		return nil
	}
	role := client.worldState.role
	if role.ID == 0 {
		return nil
	}
	set := byte(0)
	if len(requestData.plaintext) > 0 {
		set = requestData.plaintext[0]
	} else {
		// 客户端总是带 1 字节套号；缺了就用存档里的选中套号兜底。
		selected, _, _ := readAvatarClosetSelected(role.State)
		set = selected
	}
	key := avatarClosetChangeKey(int64(role.WireID), role.ConfigVersion,
		avatarClosetLedgerLen(role.State), set, requestData.plaintext)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var plan *avatarClosetChangePlan
	var planErr error
	saved, applied, err := client.gameStore.CommitCharacterEvent(ctx, role.AccountID, role.ID, role.ConfigVersion,
		key, avatarClosetChangeModel,
		func(current database.Character) (json.RawMessage, json.RawMessage, error) {
			p, e := planAvatarClosetChange(current.State, set, client.avatarClosetChangeCatalog())
			if e != nil {
				planErr = e
				return nil, nil, e
			}
			state, e := applyAvatarClosetChangeState(current.State, p)
			if e != nil {
				return nil, nil, e
			}
			plan = p
			receipt, _ := json.Marshal(map[string]any{"applied": true, "set": set, "pairs": len(p.Pairs)})
			return state, receipt, nil
		})
	if err != nil {
		client.event(map[string]any{"kind": "avatar_closet_change_refused", "set": set,
			"reason": err.Error(), "plan": errorString(planErr)})
		return nil
	}
	if !applied || plan == nil {
		client.event(map[string]any{"kind": "avatar_closet_change_replayed", "set": set})
		return nil
	}
	saved.WireID = role.WireID
	client.worldState.role = saved
	packets, e := client.avatarClosetChangePackets(saved, plan)
	if e != nil {
		client.event(map[string]any{"kind": "avatar_closet_change_packets_error", "set": set, "error": e.Error()})
		return nil
	}
	client.event(map[string]any{"kind": "avatar_closet_change_applied", "set": set,
		"pairs": len(plan.Pairs), "entries": len(plan.Entries),
		"clones_to_bar": len(plan.ClonesToBar), "skipped": len(plan.Skipped)})
	return packets
}

// errorString 把可能为 nil 的 error 变成日志友好的字符串。
func errorString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
