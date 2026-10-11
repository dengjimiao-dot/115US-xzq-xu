package gamedata

import (
	"os"
	"testing"
)

// Validate the current source preparation without treating a historical
// archive's dungeon/item cardinality as the current archive's content table.
// This does not open storage, listen on a port or start a client.
func TestCurrentPVFStartupOwnership(t *testing.T) {
	archive := os.Getenv("DFO_PVF_CORE_TEST_ARCHIVE")
	if archive == "" {
		t.Skip("current native archive required")
	}
	inputs := CatalogInputs{
		VerifyBaselines:        false,
		DerivedCacheDir:        "-",
		CharacterPolicyPath:    "../../configs/pvf-character-policy.json",
		BoxPolicyPath:          "../../configs/pvf-box-policy.json",
		ItemShopPolicyPath:     "../../configs/pvf-item-shop-policy.json",
		EnhancementPolicyPath:  "../../configs/pvf-enhancement-policy.json",
		VaultPolicyPath:        "../../configs/pvf-vault-policy.json",
		DropPolicyPath:         "../../configs/pvf-drop-policy.json",
		ScenePolicyPath:        "../../configs/pvf-scene-policy.json",
		ContentPolicyPath:      "../../configs/pvf-mine-policy.json",
		ScriptWarpPolicyPath:   "../../configs/pvf-script-warp-policy.json",
		LayerRevisitPolicyPath: "../../configs/pvf-layer-revisit-policy.json",
		WearRulesPath:          "../../configs/equipment-wear.current35.json",
		CashshopRelease:        true,
	}
	c, err := prepareCatalogsForTest(t, SupportedDomains, archive, os.Getenv("DFO_PVF_CORE_TEST_SHA256"), "missing-characters.json", "missing-quests.json", "missing-progression.json", "missing-world.json", inputs)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Equipment.Close()
	defer c.Learning.Close()
	defer c.Quests.Close()
	defer c.Loot.CloseDetails()
	defer c.Dungeons.CloseMapSource()
	if c.SourceChecksum == "" || len(c.Dungeons.Dungeons) == 0 || len(c.World.Towns) == 0 {
		t.Fatal("current source did not prepare dungeon/world ownership")
	}
	t.Logf("source=%s dungeon_count=%d client_started=false storage_opened=false", c.SourceChecksum, len(c.Dungeons.Dungeons))
}
