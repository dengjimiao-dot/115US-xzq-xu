package character

import (
	"bytes"
	"encoding/json"
	"testing"
)

// 实机 2026-10-13：mode-1 快照一旦列出普通装备槽 12..25，客户端每次冷启动进城镇
// 都会补发一条原生「脱下」CMD19，把槽 12 的武器挪进背包第一空格（名望 82671→69956）。
// 同一存档、同一启动参数，只换服务端二进制：带 12..25 的那版复现，去掉的那版不复现，
// 登录后再没有出现任何 CMD19。所以这条投影必须继续排除普通槽。
//
// 代价（上游 MR !229 放宽这两个槽的理由）：原生 sub_1452C1540 末尾的 48 格遍历会删掉
// 未列出的穿戴对象，而装备指南 playguide_5.co 检查 `equipment slot / empty / weapon`
// （native 145364F50 解析到槽 12，再走虚函数 +0x1F08 取物品对象），按该理由排除后应当
// 误报红条「[Equipment Missing] / No weapon is equipped.」（文案 key `playguide_05`，
// 在 list/characterplayguidealarm.lst 的轮播清单里）。实机 2026-10-13 武器穿着中连做
// 五组脱→穿（每次都触发 change item 重算），红条一次都没出现，且客户端仍能在脱下包里
// 自己填出槽 12 的模板 ⇒ !229 的前提未被证实（它自己的验收栏也写着"实机效果仍待验收"）。
// 若将来真出现指南侧症状，收口是保留普通行并补齐实例字段（mode-1 行 +32..+36，
// 见 docs/protocol/entry-userinfo.md 与旧树 G0504），不是回到"整批塞进来"。
func TestEntryAdditionOmitsOrdinaryWornObjects(t *testing.T) {
	service := Service{DetailedWornCandidate: true}
	for _, tc := range []struct {
		name      string
		inventory string
		wantRows  int
	}{
		{"weapon-and-armour", `"worn":[{"slot":12,"template":101000013,"durability":44},{"slot":14,"template":10005,"durability":27},{"slot":25,"template":100610096,"durability":9}]`, 0},
		{"weapon-only-in-bag", `"equipment":[{"slot":12,"template":101000013,"durability":44}],"worn":[]`, 0},
		{"armour-without-weapon", `"worn":[{"slot":14,"template":10005,"durability":27}]`, 0},
		{"empty", `"worn":[]`, 0},
		{"avatar-and-creature-still-project", `"worn":[{"slot":3,"template":40601,"durability":9},{"slot":12,"template":101000013},{"slot":26,"template":500991361}]`, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			state := json.RawMessage(`{"source_sha256":"fixture","attributes":{"[hp max]":100,"[mp max]":100},"inventory":{` + tc.inventory + `}}`)
			before := append([]byte(nil), state...)
			packet, err := service.EntryAddition(Character{WireID: 503, State: state})
			if err != nil {
				t.Fatal(err)
			}
			if got := int(packet[361]); got != tc.wantRows {
				t.Fatalf("mode-1 worn count=%d, want %d", got, tc.wantRows)
			}
			if !bytes.Equal(state, before) {
				t.Fatal("entry projection changed stored equipment")
			}
		})
	}
}
