package inventory

import (
	"encoding/json"
	"errors"
	"fmt"
)

// Typed capacity failures keep invalid source data out of the mail fallback.
// Messages retain their historical spelling for existing callers.
// Keep the upstream pickup-to-mail and downstream reward overflow paths on
// the same sentinel; equal error messages alone do not satisfy errors.Is.
var ErrStackBagFull = ErrBagFull

// GrantOrMail adopts yuanma's partial landing algorithm through the existing
// Awarder. It prepares full mail instances; callers must commit them together
// with the bag and reward claim. Only ordinary bag rewards are supported.
func (a *Awarder) GrantOrMail(raw json.RawMessage, id, amount uint32) (json.RawMessage, AwardReceipt, []MailItem, error) {
	out, receipt, err := a.Grant(raw, id, amount)
	if err == nil {
		return out, receipt, nil, nil
	}
	if !errors.Is(err, ErrStackBagFull) && !errors.Is(err, ErrEquipmentBagFull) {
		return nil, receipt, nil, err
	}
	capacityErr := err
	bag, err := ReadBag(raw)
	if err != nil {
		return nil, receipt, nil, err
	}
	if errors.Is(capacityErr, ErrStackBagFull) {
		definition, ok := a.Catalog.Items[id]
		if !ok || definition.Kind != "stackable" || IsPetConsumable(definition.StackableType) {
			return nil, receipt, nil, fmt.Errorf("overflow mail requires an ordinary stackable")
		}
		// Every probe starts from the same immutable state. Invalid source or
		// amount errors never count as a successful capacity probe.
		lo, hi := uint32(0), amount
		out, receipt.Slots = raw, nil
		for lo < hi {
			mid := lo + (hi-lo)/2 + 1
			candidate, landed, probeErr := a.Grant(raw, id, mid)
			if probeErr == nil {
				lo, out, receipt.Slots = mid, candidate, landed.Slots
			} else if errors.Is(probeErr, ErrStackBagFull) {
				hi = mid - 1
			} else {
				return nil, receipt, nil, probeErr
			}
		}
		item := MailItem{Stack: &BagItem{Template: id, Amount: amount - lo, ExpireTime: GrantExpireTime}}
		if _, err := item.Row(); err != nil {
			return nil, receipt, nil, err
		}
		return out, receipt, []MailItem{item}, nil
	}
	// Preserve the character's root state while emptying the scratch bag:
	// ReadBag then derives the existing training seal exactly as a normal award.
	scratch, err := SaveBag(raw, Bag{Version: bag.Version, Expansion: bag.Expansion})
	if err != nil {
		return nil, receipt, nil, err
	}
	out, receipt.Slots = raw, nil
	var mail []MailItem
	for n := uint32(0); n < amount; n++ {
		candidate, landed, grantErr := a.Grant(out, id, 1)
		if grantErr == nil {
			out = candidate
			receipt.Slots = append(receipt.Slots, landed.Slots...)
			continue
		}
		if !errors.Is(grantErr, ErrEquipmentBagFull) {
			return nil, receipt, nil, grantErr
		}
		generated, _, grantErr := a.Grant(scratch, id, 1)
		if grantErr != nil {
			return nil, receipt, nil, grantErr
		}
		one, readErr := ReadBag(generated)
		if readErr != nil || len(one.Equipment) != 1 {
			return nil, receipt, nil, fmt.Errorf("overflow mail requires one ordinary equipment instance")
		}
		item := one.Equipment[0]
		attachment := MailItem{Equipment: &item}
		if _, err := attachment.Row(); err != nil {
			return nil, receipt, nil, err
		}
		mail = append(mail, attachment)
	}
	return out, receipt, mail, nil
}
