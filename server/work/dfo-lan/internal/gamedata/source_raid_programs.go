package gamedata

import "dfolan/internal/catalog"

// LegacyRaidRules reads a path discovered from the raid entrance catalog.
// Reading rules does not enable a channel or certify its protocol adapter.
func (s *Source) LegacyRaidRules(path string) (*catalog.LegacyRaidRules, error) {
	return catalog.ImportLegacyRaidRules(s.archive, path)
}

func (s *Source) AsrahanRaidRules(path string) (*catalog.AsrahanRaidRules, error) {
	return catalog.ImportAsrahanRaidRules(s.archive, path)
}
