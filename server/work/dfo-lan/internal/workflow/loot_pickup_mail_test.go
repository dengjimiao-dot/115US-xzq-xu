package workflow

import (
	"context"
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/database"
	"dfolan/internal/dungeon"
	"dfolan/internal/inventory"
	"dfolan/internal/loot"
	"dfolan/internal/savecontract"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// pickupMailFixture 造一个「[etc] 类别只剩一格、并且已经被别的模板占掉」的角色：
// 这样拾取这一类物品必然走容量分支，不必真的塞满 56 格真实背包。
func pickupMailFixture(t *testing.T) (context.Context, *database.Store, database.Character, *LootService, *loot.Session, *dungeon.Session, uint32) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	t.Cleanup(cancel)
	fixture, err := database.OpenTestFixture(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if e := fixture.Close(); e != nil {
			t.Error(e)
		}
	})
	store := fixture.Storage()
	for _, migrate := range []func(context.Context) error{store.Migrate, store.MigrateCharacterEvents, store.MigrateMailbox} {
		if err = migrate(ctx); err != nil {
			t.Fatal(err)
		}
	}
	account, err := store.DevelopmentAccount(ctx, "pickup-mail-fixture")
	if err != nil {
		t.Fatal(err)
	}
	version := savecontract.Identity()
	items := map[uint32]catalog.LootItem{
		pickupMailDrop:  {ID: pickupMailDrop, Kind: "stackable", StackableType: "[etc]", StackLimit: 1000},
		pickupMailBlock: {ID: pickupMailBlock, Kind: "stackable", StackableType: "[etc]", StackLimit: 1000},
	}
	c := catalog.LootCatalog{Source: pvf.ArchiveSnapshot{Checksum: version}, Items: items}
	rules, err := loot.LoadRules("../../configs/drop.current36.json")
	if err != nil {
		t.Fatal(err)
	}
	// 拾取不掷骰，掉落表留零值就够（Parse 只服务 Death 侧）。
	tables := loot.Tables{}
	bagRules, err := inventory.LoadBagRules("../../configs/inventory.current37.json")
	if err != nil {
		t.Fatal(err)
	}
	bagRules.Slots = map[string][2]uint16{"[etc]": {5, 5}}

	// 背包里那一格占着另一个模板：同模板合并这条路走不通，只能撞到满包。
	bag := inventory.Bag{Items: []inventory.BagItem{{Slot: 5, Template: pickupMailBlock, Amount: 1}}}
	bag.Version = "ordinary-bag-v1"
	if bag.Gold == 0 {
		bag.Gold = 1
	}
	state, err := inventory.SaveBag(json.RawMessage(`{"level":115}`), bag)
	if err != nil {
		t.Fatal(err)
	}
	role, err := store.CreateCharacter(ctx, database.Character{
		AccountID: account, Name: "PickupMailFixture", Request: []byte{0}, ConfigVersion: version, State: state,
	}, 24)
	if err != nil {
		t.Fatal(err)
	}
	role.WireID = 2

	d := &dungeon.Session{RunID: "0123456789abcdef0123456789abcdef", Loaded: true, Room: catalog.DungeonRoom{Map: 1}}
	session := loot.NewSession(c, tables, rules, nil, d.RunID, role.AccountID, role.ID, role.WireID)
	session.Objects[pickupMailObject] = loot.Drop{Run: d.RunID, Map: 1, Owner: role.WireID, Object: pickupMailObject, Award: loot.Award{Template: pickupMailDrop, Amount: 3}}
	domain := &loot.Service{Catalog: c, Rules: rules, Tables: tables, BagRules: bagRules}
	return ctx, store, role, &LootService{Store: store, Loot: domain}, session, d, pickupMailObject
}

const (
	pickupMailDrop   = uint32(590722601)
	pickupMailBlock  = uint32(590722598)
	pickupMailObject = uint32(0x77)
)

// 满包拾取：这一件不再整笔拒绝，而是同一事务里落成系统邮件附件，背包一格都没动。
func TestPickupToMailWhenBagFull(t *testing.T) {
	ctx, store, role, lsvc, session, d, object := pickupMailFixture(t)

	saved, receipt, applied, err := lsvc.AutoPickup(ctx, role, session, d, object)
	if err != nil {
		t.Fatalf("满包拾取应当转邮寄而不是失败：%v", err)
	}
	if !applied || !receipt.Mailed || receipt.MailID == 0 {
		t.Fatalf("回执应当是已邮寄并带邮件编号：%+v", receipt)
	}
	if receipt.Destination != 0 || receipt.Object != object || receipt.Award.Amount != 3 {
		t.Fatalf("回执内容与这一件对不上：%+v", receipt)
	}

	// 背包真值：那一格还占着原来的模板，新拾取的没有进袋。
	stored, err := store.Characters(ctx, role.AccountID)
	if err != nil {
		t.Fatal(err)
	}
	var fresh database.Character
	for _, cur := range stored {
		if cur.ID == role.ID {
			fresh = cur
		}
	}
	if len(fresh.State) == 0 {
		t.Fatal("角色没有读回来")
	}
	b, err := inventory.ReadBag(fresh.State)
	if err != nil {
		t.Fatal(err)
	}
	if len(b.Items) != 1 || b.Items[0].Template != pickupMailBlock || b.Items[0].Slot != 5 {
		t.Fatalf("满包拾取不该改背包：%+v", b.Items)
	}

	// 邮件真值：一封未领的署名邮件，附件就是这一件。
	messages, err := store.Mailbox(ctx, role.AccountID, role.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(messages) != 1 {
		t.Fatalf("邮箱里应当只有一封：%d", len(messages))
	}
	mail := messages[0]
	if mail.ID != receipt.MailID || mail.SenderName != pickupMailSender || len(mail.Assets) != 1 || mail.Assets[0].Claimed {
		t.Fatalf("邮件与回执对不上：%+v", mail)
	}
	if !strings.Contains(string(mail.Assets[0].Item), "590722601") {
		t.Fatalf("附件里没有这一件：%s", mail.Assets[0].Item)
	}

	// 重放（客户端连点）：不再发第二封，回执仍是同一条、同一个邮件编号。
	_, again, replayed, err := lsvc.AutoPickup(ctx, saved, session, d, object)
	if err != nil {
		t.Fatal(err)
	}
	if replayed || !again.Mailed || again.MailID != receipt.MailID {
		t.Fatalf("重放不该二次入账：%+v applied=%v", again, replayed)
	}
	if messages, err = store.Mailbox(ctx, role.AccountID, role.ID); err != nil || len(messages) != 1 {
		t.Fatalf("重放后邮箱应当还是 %d 封：%v", len(messages), err)
	}
	t.Log("满包拾取 → 邮件附件 1 封、背包未动、重发不二次")
}

// 容量之外的失败不许转邮寄：目录里认不出的模板仍然整笔拒绝，也不会留下邮件。
func TestPickupKeepsRefusingNonCapacityErrors(t *testing.T) {
	ctx, store, role, lsvc, session, d, _ := pickupMailFixture(t)
	session.Objects[0x78] = loot.Drop{Run: d.RunID, Map: 1, Owner: role.WireID, Object: 0x78, Award: loot.Award{Template: 999999999, Amount: 1}}
	if _, _, _, err := lsvc.AutoPickup(ctx, role, session, d, 0x78); err == nil {
		t.Fatal("未知模板的拾取应当失败")
	}
	if messages, err := store.Mailbox(ctx, role.AccountID, role.ID); err != nil || len(messages) != 0 {
		t.Fatalf("非容量失败不该发信：%d 封 %v", len(messages), err)
	}
}

// 满包时金币仍然正常入袋（它有独立计数，不存在容量问题）。
func TestPickupGoldStillGrantsWhenBagFull(t *testing.T) {
	ctx, store, role, lsvc, session, d, _ := pickupMailFixture(t)
	session.Objects[0x79] = loot.Drop{Run: d.RunID, Map: 1, Owner: role.WireID, Object: 0x79, Award: loot.Award{Template: 0, Amount: 500}}
	saved, receipt, applied, err := lsvc.AutoPickup(ctx, role, session, d, 0x79)
	if err != nil || !applied {
		t.Fatalf("金币拾取应当成功：%v applied=%v", err, applied)
	}
	if receipt.Mailed {
		t.Fatalf("金币不该转邮寄：%+v", receipt)
	}
	b, err := inventory.ReadBag(saved.State)
	if err != nil || b.Gold < 501 {
		t.Fatalf("金币没进袋：%+v %v", b, err)
	}
	if messages, err := store.Mailbox(ctx, role.AccountID, role.ID); err != nil || len(messages) != 0 {
		t.Fatalf("金币拾取不该留下邮件：%d 封 %v", len(messages), err)
	}
}
