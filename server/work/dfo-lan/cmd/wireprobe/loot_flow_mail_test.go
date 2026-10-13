package main

import (
	"context"
	"dfolan/internal/database"
	"dfolan/internal/game/protocol"
	"dfolan/internal/loot"
	"testing"
)

// 满包转邮箱后背包一格都没动，所以应答里**不能有 NOTI14**（那会在客户端造出玩家其实
// 没有的行）；场景物用 NOTI39 的中性体摘掉（八条 flag=0 行、不带普通尾，不写任何增量）。
func TestMailedPickupRemovesSceneWithoutInventoryUpdate(t *testing.T) {
	w := &worldSession{role: database.Character{ID: 9, WireID: 3}}
	receipt := loot.PickupReceipt{Run: "run", Map: 1, Object: 0x55, Award: loot.Award{Template: 590722601, Amount: 3}, Mailed: true, MailID: 42}
	plan, err := w.finishPickup(context.Background(), w.role, receipt, true, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan) != 2 {
		t.Fatalf("满包应答应当只有 ACK 与场景移除：%+v", plan)
	}
	if plan[0].ID != 43 || plan[0].Kind != 1 {
		t.Fatalf("第一条应当是 CMD43 回执：%+v", plan[0])
	}
	wanted, err := protocol.PickupConfirmed(receipt.Object, w.role.WireID, 0, true)
	if err != nil {
		t.Fatal(err)
	}
	if plan[1].ID != 39 || string(plan[1].Payload) != string(wanted) {
		t.Fatalf("场景移除体与中性 NOTI39 不一致：%+v", plan[1])
	}
	for _, p := range plan {
		if p.ID == 14 {
			t.Fatal("满包转邮箱不能发背包更新")
		}
	}

	// 自动拾取没有 ACK，但同样要把地面物摘掉（重发安全，所以不看 applied）。
	if auto, err := w.finishPickup(context.Background(), w.role, receipt, false, false); err != nil || len(auto) != 1 || auto[0].ID != 39 {
		t.Fatalf("自动拾取的满包应答：%+v %v", auto, err)
	}
}
