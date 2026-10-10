package main

import "dfolan/internal/legion"

// Current 2.38.2.34 registry146D58EFC registers NOTI1220 (kind0) at
// handler146D5A3C0, which reads one byte and calls144CDB100. That call
// initializes controller+200 with GetTickCount. Finder gate144CF40E0
// waits while it is -1 or for the next3000ms in a raid channel.
// Official s9 S282 sends this at login, not after each C1353 query.
func (client *gameConnection) bakalPostCharacterInfo() []outboundPacket {
	if client.worldState == nil || client.selectedCharacterID == 0 ||
		client.worldState.role.ID != client.selectedCharacterID ||
		client.worldState.channelType != legion.BakalChannelType {
		return nil
	}
	// The native reader consumes only this byte (official value0); the
	// captured trailing bytes are not copied into a new logical payload.
	return []outboundPacket{{"bakal_post_character_info_ready", 0, 1220, []byte{0}}}
}
