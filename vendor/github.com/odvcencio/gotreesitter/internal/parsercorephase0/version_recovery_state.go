package parsercorephase0

// VersionRecoveryState keeps optional recovery ownership separate from the
// frequently published lexer cursor and visible-node baseline. Every identity
// retains its original width; Region and Group must refer to immutable values.
type VersionRecoveryState[Region, Group comparable] struct {
	Region          Region
	RecoveryGroup   uint64
	MissingGroup    uint64
	AcceptanceGroup Group
}

// ReuseVersionRecoveryState leaves the common empty value implicit. Equal
// values retain their immutable holder; updates never mutate an older holder.
func ReuseVersionRecoveryState[Region, Group comparable](previous *FrozenValue[VersionRecoveryState[Region, Group]], next VersionRecoveryState[Region, Group]) *FrozenValue[VersionRecoveryState[Region, Group]] {
	if next == (VersionRecoveryState[Region, Group]{}) {
		return nil
	}
	return ReuseFrozenValue(previous, next)
}
