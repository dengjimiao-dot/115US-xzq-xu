package protocol

import (
	"encoding/binary"
	"fmt"
)

// Avatar Closet（寄存衣柜）：基础 1 套，买 Tier1/2/3 各 +1 套，上限 4 套；
// **每套 12 个柜槽**，总容量 = 套数 × 12（最多 48 槽）。
// 已购档位数（0..3）与套数（1..4）的关系：sets = expansion + 1。
//
// ★ 2026-10-14 容量模型定案（反汇编 + 业主实机双证据，推翻"首字段=槽数"的旧注释）：
//   - noti1076 首字段（= AvatarClosetSlots 的返回值）是**套数**而非总槽数。
//     客户端刷新函数 sub_1449E8B20（noti1076 handler 尾部调用）里
//     `for (i = 1; i < 13; ++i)` + `v19 <= 11` 钉死每套 12 槽；
//     `v12 = selected/2、v13 = selected%2` 说明选中值按"每页 2 套"分解 ⇒ 选中=套号。
//   - 参照实现 110US 的 schema：`capacity BETWEEN 1 AND 4`、
//     `selected_set >= 0 AND < capacity` —— capacity 计量单位同为"套"。
//   - 业主实机：扩容买满（expansion=3 ⇒ 4 套）后往柜里放了 14 件，
//     cmd19 DestinationSlot 出现 0..11（整套）、23（第 2 套末槽）、45（第 4 套）——
//     全部 < 4×12=48，与"套×12"模型吻合；旧代码按 4 槽钳制 ⇒ 10 件被丢
//     （业主报"只保存了几件"的直接根因）。
const (
	MaxAvatarClosetExpansion byte = 3                            // Tier1/2/3
	MaxAvatarClosetSlots     byte = MaxAvatarClosetExpansion + 1 // 4 套
	AvatarClosetSlotsPerSet  byte = 12                           // 每套柜槽数
)

// AvatarClosetSlots 把已购档位数换算成套数（1..4）。
func AvatarClosetSlots(expansion byte) byte {
	if expansion > MaxAvatarClosetExpansion {
		expansion = MaxAvatarClosetExpansion
	}
	return expansion + 1
}

// AvatarClosetCapacity 返回寄存衣柜的**总槽数** = 套数 × 每套 12 槽（12..48）。
// 登录编译 noti1077 条目时的柜槽上界；客户端自己往 23/45 号槽放过货，
// 说明它的容器按这个上限受理，超出部分客户端 apply 助手自己会记日志跳过。
func AvatarClosetCapacity(expansion byte) uint16 {
	return uint16(AvatarClosetSlots(expansion)) * uint16(AvatarClosetSlotsPerSet)
}

// AvatarClosetInfo 构造 ENUM_NOTIPACKET_AVATAR_CLOSET_INFO (noti 1076) 的包体。
//
// 官方 2026-10-02 抓包（每会话登录一帧，10 帧全同）：
//
//	01 00 00 00 00 00 00 00 ae 6b cc 64 3c 00 00 00
//
// 反汇编客户端 handler（DFO.exe 0x1449E8060）只读前两个 u32：
// 第 1 个 u32 = 衣柜**套数**（1..4；官方那份是 1 套基础态，实测下发 01 客户端
// 解锁第一套衣架；总槽数 = 套数 × 12，见 AvatarClosetCapacity），
// 第 2 个 u32 = 当前选中的**套号**（写进衣柜对象 +0x10；官方那帧是 0）。
// 其余 8 字节原样保留官方定值（handler 不消费，官方恒定）。
//
// 2026-10-14：selected 落库后从这里回给客户端（CMD1103 提交的选中套号），
// 治"重登选中栏跳回第一套"。selected 必须 < sets，否则拒绝构造。
func AvatarClosetInfo(sets byte, selected byte) ([]byte, error) {
	if sets < 1 || sets > MaxAvatarClosetSlots {
		return nil, fmt.Errorf("invalid avatar closet sets")
	}
	if selected >= sets {
		return nil, fmt.Errorf("invalid avatar closet selection %d for %d sets", selected, sets)
	}
	body := []byte{sets, 0, 0, 0, 0, 0, 0, 0, 0xae, 0x6b, 0xcc, 0x64, 0x3c, 0, 0, 0}
	body[4] = selected
	return body, nil
}

// 寄存器衣柜在客户端容器注册表（0x145a0e8c0）里的容器号。
const (
	// AvatarContainerID 装扮栏（cmd19 List=1，status 1 = 时装/外观行）。
	AvatarContainerID byte = 1
	// AvatarClosetContainerID 寄存衣柜（List=0x21=33）。NOTI13 空间帧**绝不能**
	// 对它下发（客户端容器不认识会崩，2026-10-10 事故）——柜内容只走 noti1077。
	AvatarClosetContainerID byte = 33
	// AvatarClosetSyncMode 是 noti1077 条目里唯一承载内容的 mode：与装扮栏槽位
	// 双向同步。客户端 reader 里，柜槽有数据 ⇒ 写进 D 指定的装扮栏槽（穿上方向）；
	// 柜槽为空 ⇒ 从 D 槽回填（登录恢复走这条：客户端衣橱刚初始化必为空）。
	// 其余 mode 客户端只记日志；mode 3 走内联块；D==0xB 触发槽位特殊 UI 动作。
	AvatarClosetSyncMode byte = 1
)

// AvatarClosetSetEntry 是 noti1077 的一条"柜槽 ↔ 装扮栏槽"同步条目。
//
// wire 布局（反汇编 DFO.exe 0x1449e8100 钉死，每条固定 6B、无载荷）：
//
//	[u8 Container][u16le Slot][u8 Mode][u16le LinkedSlot]
//
// - Container 恒为 33（衣柜容器）；
// - Slot 是衣柜槽号（0 基，跨套连续编号：第 s 套占 [s*12, s*12+12)，上限 AvatarClosetCapacity）；
// - Mode 恒为 1（双向同步）；
// - LinkedSlot 是装扮栏（容器 1）里的槽号，柜内容显示以它为准。
type AvatarClosetSetEntry struct {
	Container  byte
	Slot       uint16
	Mode       byte
	LinkedSlot uint16
}

// AvatarClosetSet 构造 ENUM_NOTIPACKET_CHANGE_AVATAR_CLOSET_SET (noti 1077) 的包体。
//
// 反汇编客户端 reader（DFO.exe 0x1449e8100）：先 ReadU32(n) 读条目数，n<=0 直接返回；
// 否则逐条 ReadU8 + ReadU16 + ReadU8(mode) + ReadU16 + mode 专属载荷。mode 1 没有
// 额外载荷 ⇒ 每条 6B 定长。空账本/无有效条目时发 4 个零字节（空列表），与 2026-10-14
// 之前的行为逐字节一致——客户端 reader 对 n<=0 直接返回，最坏只是不显示柜内容。
func AvatarClosetSet() []byte {
	return []byte{0, 0, 0, 0}
}

// AvatarClosetSetWith 用真实条目构造 noti1077 包体：u32le(n) + n×6B 条目。
// entries 为空时与 AvatarClosetSet() 逐字节一致（4 个零字节）。
func AvatarClosetSetWith(entries []AvatarClosetSetEntry) []byte {
	if len(entries) == 0 {
		return AvatarClosetSet()
	}
	body := make([]byte, 4+6*len(entries))
	binary.LittleEndian.PutUint32(body[0:4], uint32(len(entries)))
	off := 4
	for _, e := range entries {
		body[off] = e.Container
		binary.LittleEndian.PutUint16(body[off+1:off+3], e.Slot)
		body[off+3] = e.Mode
		binary.LittleEndian.PutUint16(body[off+4:off+6], e.LinkedSlot)
		off += 6
	}
	return body
}
