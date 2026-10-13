# Current-build entry USERINFO reconstruction

Client 2.38.2.34, SHA256 fda6c33f2a155124a4262f3e7cd6f7a18d4e2191e5f147aec5754716afe5aa7c.
NOTI type 0 / ID 2, distinct from C2S command 8 and roster mode 2.

Mode 0 header: u8 mode=0, u16 count, then each row has two u8 context values.
Reader 145637a20 sets these context values, then calls 14563ec60. Context values
must agree with the client's channel context for a local actor. Zero is a probe
assumption for the current loopback channel, not a universal channel ID.

Minimum row reader sequence, with absent optional collections:

| Native location | Wire fields |
| --- | --- |
| 14563ecd2 | raw160; **advances** the packet cursor |
| 14563ed31 / 14563ee88 | u16 actor ID, u32 UTF-8 length + name |
| 14563eeb1..14563f0d1 | u8 profession, packed advancement, level, PvP grade, state |
| 145639840 / 14563f0f1 | u8 equipment count=0, u32 |
| 14563f10f..14563f161 / 145639b70 | 4*u8, u8 cosmetic count=0 |
| 14563f174..14563f276 | 2*u32, u8, u32, u8 |
| 1456394b0 | u32, u32 string length=0, u8 |
| 14563be50 / 14563f2a3..f2bc | u8 premium, 2*u32 |
| 14563a0b0 / 14563f2de | u32 string length=0, u32, u16, u32 |
| 14563f338..f36c / 145639dc0 | 3*u8, u8, u32 |
| 14563f3db..f430 / 14563a240 | u8, u32, u32, u16, u32, raw8 |
| 14563fc24..1456401d4 | u8, u32, u8, u16, u8 |
| 14563bdc0..145640351 | u16, u8, u16, u8, 3*u8 |
| 1456407ca / 14563c140 | u32, u32 collection count=0 |
| 1456407f1..145640965 | 6*u8, 3*u32 |

The one-row notification with these empty collections is 307 + UTF-8 name bytes.

The fixed160 structure contains inline NUL-terminated narrow strings at +0x1b
and +0x5f. 146e90b00 converts these via the Windows narrow-to-wide routine; they
are not std::string pointers. Other members were mapped to local object offsets
in `../dfo_probe_tools/userinfo_prefix_refs.asm`; most semantics remain unknown.

The serializer uses empty unknown data and native initial flag values only as a
bounded parser experiment. It does not establish official defaults, inventory,
HP/MP, a loaded map, or playable town entry. Equipment is explicitly rejected
until its initial data mapping is complete. Preserve the existing character DB.

Mode 1 requires the mode 0 actor to exist. It starts with raw250 and u16 actor
ID, then u64 experience (helper 145639c20 -> 146d77f90), a u32-sized packed
structure (constructor 147554f30 initializes 91 bytes), u8, switching inventory,
and skills/other collections. Do not use the older x86 mode 1 payload.

## Historical mode 1 experiment (live10 failed; superseded below)

entry_addition.go is an unintegrated experimental codec: minimum one-actor payload 495 bytes plus three bytes per skill, with TWO skill banks. Native 14563d9aa increments the bank counter, compares against 2 and loops to 14563d794. Both self/other branches consume both banks. Each bank has a byte count, u16 ID/u8 level skill rows, three fixed u16/u32 pairs and five fixed u16/u32/u8 triples.

Actor ID begins at byte255, experience257, packed length265 and the 91-byte record269. Native147555f80 expands the packed record into40 protected fields. packed_stats_oracle.py executes the original conversion and protection/getter routines in private emulated memory with initialized TLS epoch. Three vectors cover all40 fields including signed and floating-point values; fixture internal/game/protocol/testdata/native_packed91.json. Codec tests reconstruct original bytes and check both skill banks.

Packed wire widths: HP/MP u32; four u16 core values; four i16 elemental values; nineteen i16 status values; i32; two i16; u32; two u16; two i16; i32; u8 base percentage; float32. Native finalization145cce150 scales HP/MP through divisor10 and core fields by base percentage divided by1000. Other semantics and source PVF skill tuple mappings still need verification. Unknown prefixes and collection values remain experimental, not official defaults.

This codec is NOT called by the active gateway. Full mode1 was not required for the accepted town visual gate; complete stats, skills and inventory remain missing. go test ./... and go vet ./... passed after adding the codec and native fixtures.

## Combined correction, live_detail_14 (2026-09-11 UTC)

The historical 495-byte minimum above was incomplete. The call at14563d6cb
enters1452c1540, which ALWAYS reads an equipment block: u8 row count,
u32 scalar, u8 collection count and raw8 flags, even when both counts are zero.
Those14 bytes were missing between the byte atoffset360 and switching inventory.
The corrected minimum is509 bytes. A fixture with three skills is518 bytes.

`../dfo_probe_tools/addition_cursor_oracle.py` runs the original14563d400
packet-reading branches and nested parsers under Unicorn, with unrelated
business/UI calls stubbed and private dummy actors. It consumes the entire518
byte fixture. Removing the14 bytes reproduces a native read overrun at14563dbf8
(503+4 >504). The Go serializer must match this independent fixture exactly.
This proves wire cursor consumption, not live game/UI acceptance.

The opt-in detail build restores source base attributes from saved character
state. Slayer source includes HP520, MP480, movement850; native units are
documented above. Optional equipment, skills, rewards and unknown fixed-prefix
fields remain incomplete. No original EXE or PVF change was made for this fix.
Live14 includes the corrected mode1, vault initialization, USER_AREA23,
AREA_USERS24 and final FATIGUE36. Client verification is pending.

## Ordinary worn objects and the equipment guide (2026-10-12)

Mode 0 binds appearance indices; mode 1 restores actual worn item objects.
The equipment guide `playguide_5.co` checks `equipment slot / empty / weapon`:
native `145364F50` resolves weapon to slot12 and calls the player virtual
getter at +0x1F08. It does not inspect the mode-0 appearance index.

`14563D6CB` invokes `1452C1540`, whose final 48-slot pass deletes omitted
worn objects (except slot31) through virtual +0x1F30. Filtering ordinary
slots12..25 from the mode-1 snapshot therefore removes real owned objects
before the Seria-room guide checks them. Later inventory/window notifications
do not substitute for this complete initial snapshot.

Ordinary slots12..25 now use the native plain row: 40-byte header and 87-byte
tail, with no avatar blobs or type26 creature extension. The service projects
the persisted worn slot/template/durability and leaves rich instance records
to the existing NOTI13/14 channel. A weapon solely in the bag remains unworn.
Avatar/creature layouts and the disabled optional-detail profile are unchanged.

The finished GF115 capture20261006-165615, stream63/wire380, carries a real
weapon row in its mode-1 snapshot (slot12, template101010912, durability44).
Its fixed prefix and packed-stat length differ from US; only the ownership
semantics are used here, not cross-region fixed offsets. Tests pin ordinary
US row boundaries, actual worn restoration and bag-only/empty-slot cases.
Compilation and packet tests do not establish client UI acceptance.

## Ordinary worn rows reverted: cold-start auto-unequip (2026-10-13)

Listing ordinary slots12..25 in the mode-1 snapshot made the client send a
native unequip (CMD19 MOVE_ITEMSPACE) for the worn weapon on every cold start:
source=(list0 bag, first empty slot, item0) destination=(list3 worn, slot12,
item101040831), roughly 150-180ms after the login burst, and the server
committed it (fame 82671 -> 69956, 30 worn items -> 29). Re-logging in the same
client session did not repeat it; only a fresh process did.

Cause isolation used one A/B pair with the same SQLite archive, the same
character and the same launcher arguments, changing only the server binary:

| binary | mode-1 rows | login CMD19 | fame at login |
| --- | --- | --- | --- |
| built 17:46 (slots12..25 included) | 30 | yes, 11:56:46.134Z | 82671 -> 69956 |
| built 20:47 (slots12..25 omitted) | 30 | none | 82671, then only rises |

Session logs: `runtime/roles_persist_..._20261013_195539_391972_next37` and
`..._20261013_204936_054992_next37`.

What this does NOT explain, and what was falsified on the way:

- The quest auto-accept hypothesis is dead. In the passing session CMD31
  (accept quest 3145) and CMD143 still fire exactly as before, `synopsis_read`
  is still absent (NOTI2310 body stays 3 bytes), and yet no unequip follows.
  The ~150ms gap after CMD143 was coincidence, not a chain.
- Which cell of the ordinary row makes the client reject the worn object is
  still unproven. The row currently carries slot/template/durability and zero
  fixed-prefix cells, and the instance Record is ignored on non-avatar rows.

Cost of this revert: the 2026-10-12 guide fix above is undone in code, so
`playguide_5.co` was expected to see no item object on slot12 after the
`1452C1540` final 48-slot pass deletes omitted worn objects.

That expected cost did NOT appear on live 2026-10-13 21:46-21:48 (same session
`..._204936_054992_next37`): with the weapon worn on slot12 the owner unequipped
and re-equipped five items (slots 18, 12, 23, 14, 14 — five `change item`
recomputations, every pair committed, fame 84199 -> 71484 -> 84199 on the weapon
pair), and the alarm `[Equipment Missing] / No weapon is equipped.`
(`string/clientonly.uv.str` key `playguide_05`, listed in
`list/characterplayguidealarm.lst`) never showed. Independent corroboration from
the same frames: the client itself filled `dst list3 slot12 item101040831` in
its unequip packet, i.e. its worn model does hold a slot12 object with the right
template. So MR !229's premise ("mode-1 omits ordinary rows => the client sees a
worn weapon as an empty slot") is not observed on this build; what remains
possible is only that the guide reads a different store through `+0x1F08`.
Caveats before treating this as settled: one character, one observation, and the
alarm rotation carries an 86400 interval, so a same-day alarm that had already
been consumed would not re-show.

Closure path (not needed until a guide symptom is actually observed): keep the
ordinary rows and fill their instance cells so the client no longer reconciles
them away. The 2026-10-13 live check above removed the only symptom that was
ever claimed for the widened rows. The known-answer sample for the row shape is
the GF115 capture20261006-165615 stream63/wire380 quoted above, which is not
present on this machine as of 2026-10-13. The cap43 official capture is present,
but its S2C USERINFO mode-1 body (conn1071 seq115, 6848 payload bytes) yields no
readable slot/template row pairs, so it cannot serve as the oracle for this
field set. Do not guess the cells in.

Tests: `internal/character/detail_weapon_regression_test.go` and
`detail_avatar_test.go` now pin the omission (count byte at payload offset 361,
avatar135 + creature132 = 267 delta). `protocol.DetailedEquipment` still
encodes and validates ordinary rows, which is where the closure work resumes.
