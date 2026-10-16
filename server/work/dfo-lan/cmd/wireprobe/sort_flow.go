package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"dfolan/internal/workflow"
	"fmt"
	"time"
)

type sortSession struct {
	nonce       [16]byte
	initialized bool
}

// handleSortItem adopts the bag arrangement the client already applied locally
// (CMD20 SORT_ITEM) and answers with the authoritative bag restore.
//
// The command used to be dropped entirely, which left the client's own
// "inventory in use" latch set until the next reconnect: every later inventory
// action, equipment disassembly included, was then refused locally with DSTR
// 1659 ("The Inventory is currently in use."). Answering it is what clears the
// latch, so a replayed frame is answered too.
func (s *sortSession) handle(service *workflow.WearService, w *worldSession, p, raw []byte) ([]outboundPacket, error) {
	if service == nil || w == nil || w.role.ID == 0 {
		return nil, fmt.Errorf("item sort requires the owned character")
	}
	r, e := protocol.DecodeSortItem(p)
	if e != nil {
		return nil, e
	}
	if !s.initialized {
		if _, e = rand.Read(s.nonce[:]); e != nil {
			return nil, e
		}
		s.initialized = true
	}
	hash := sha256.Sum256(raw)
	key := fmt.Sprintf("itemsort:%x:%x", s.nonce, hash)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	saved, applied, e := workflow.SortBag(ctx, w.store, w.role, service.BagRules, key, r)
	if e != nil {
		return nil, e
	}
	w.role = saved
	b, e := inventory.ReadBag(saved.State)
	if e != nil {
		return nil, e
	}
	// A replay still gets the answer: the client's latch clears on the answer,
	// not on the state change.
	name := "item_sort_committed"
	if !applied {
		name = "item_sort_replayed"
	}
	plan := []outboundPacket{{name, 1, 20, protocol.SortItemSuccess()}}
	// [FIX-20261007 特殊容器整理] 整理后的权威排列必须走与登录同通道的
	// NOTI 13 整体恢复（登录 avatar_inventory_restored / creature_inventory_restored
	// 即是如此）：客户端识别 NOTI 13 + 对应 space 为"容器整体替换"，清空旧显示
	// 后重填。实测两种错误做法均异常：NOTI 14 全量（槽位更新通道）被客户端当
	// 增量叠加（旧行不消失，"出现两个"）；restore=true 但走 NOTI 14 则闪退。
	if r.List == 0 {
		if body, e := protocol.InventoryRestore(b.Rows(), b.Expansion); e == nil {
			plan = append(plan, outboundPacket{"item_sort_inventory_restored", 0, 13, body})
		}
	} else if r.List == 1 {
		// [FIX-20261014 寄存行剔除] 客户端侧的装扮栏**不含**寄存在衣柜里的行：
		// 登录 noti1077 mode 1 已经把那一行从栏槽搬进柜槽（客户端搬运、栏槽腾空），
		// 服务端 Special[1] 却仍带着它。原样重推 Special[1] 会把这批件重新塞回
		// 背包——业主 2026-10-14 报"点整理之后会显示在包里"，并连带把客户端的
		// 衣柜链接/实例记账搅乱（"时装栏不显示穿在身上的时装"）。
		// 用 avatarBarRestorePayload 下发剔除后的行集：头部形状与
		// SpecialEquipmentRestorePayload 逐字节一致，空账本时行为不变。
		if body, e := avatarBarRestorePayload(saved.State); e == nil && len(body) > 0 {
			plan = append(plan, outboundPacket{"avatar_inventory_restored", 0, 13, body})
		}
	} else if r.List == 7 {
		if body, e := inventory.PetContainerRestorePayload(saved.State); e == nil && len(body) > 0 {
			plan = append(plan, outboundPacket{"creature_inventory_restored", 0, 13, body})
		}
	}
	return plan, nil
}
