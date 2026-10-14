package protocol

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// 寄存衣柜容量模型（2026-10-14 定案）：套数 = expansion+1（1..4），
// 每套 12 槽，总容量 = 套数 × 12（12..48）。
// 证据：客户端 sub_1449E8B20 刷新循环 for(i=1;i<13) + idx<=11（每套 12 槽）、
// selected/2、selected%2（选中=套号，每页 2 套）；110US schema capacity 1..4；
// 业主实机往 23/45 号柜槽放过货（expansion=3 ⇒ 48 槽内）。
func TestAvatarClosetCapacityIsSetsTimesTwelve(t *testing.T) {
	require.Equal(t, uint16(12), AvatarClosetCapacity(0), "基础 1 套 = 12 槽")
	require.Equal(t, uint16(24), AvatarClosetCapacity(1), "Tier1 = 2 套 = 24 槽")
	require.Equal(t, uint16(36), AvatarClosetCapacity(2), "Tier2 = 3 套 = 36 槽")
	require.Equal(t, uint16(48), AvatarClosetCapacity(3), "Tier3 = 4 套 = 48 槽")
	require.Equal(t, uint16(48), AvatarClosetCapacity(9), "坏档位钳到上限，绝不放大")

	// 套数换算保持 1..4（noti1076 首字段）。
	require.Equal(t, byte(1), AvatarClosetSlots(0))
	require.Equal(t, byte(4), AvatarClosetSlots(3))
	require.Equal(t, byte(4), AvatarClosetSlots(200))
}
