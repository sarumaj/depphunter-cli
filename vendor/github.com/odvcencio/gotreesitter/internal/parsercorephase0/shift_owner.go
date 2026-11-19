package parsercorephase0

import "errors"

// ShiftHeadLineage snapshots the same owner and scalar/alternative lineage
// inputs as the scheduler's publication calls. Drop-cohort references are
// deliberately absent: callers with any references keep their existing path.
type ShiftHeadLineage struct {
	Owner          uint32
	PreviousHead   Head
	Set            AlternativeSet
	Lineage        uint16
	Rank           CleanPathRankSelection
	Blended        bool
	PublishLineage bool
	SetDirty       bool
}

// ShiftDirectAndPublishLineageOwned publishes the same corridor-proven shift as
// ShiftDirectWithLiveCondenseCandidatesOwned. For a new graph head only, it
// binds the scheduler lineage inside that existing owned operation. A caller
// must keep its normal owner-publication path when ownerPublished is false.
// The caller still owns action-shape and post-shift canonicalization proofs.
func (c *Core) ShiftDirectAndPublishLineageOwned(
	owner SchedulerTransactionToken,
	candidates []CondenseCandidate,
	head Head,
	lookahead Symbol,
	targetState StateID,
	extra bool,
	shifted Token,
	fork ForkOrder,
	publication ShiftHeadLineage,
) (out Head, ownerPublished bool, err error) {
	if err = c.beginSchedulerOwned(owner); err != nil {
		return out, false, err
	}
	defer c.recoverSchedulerOwnedPanic(owner)
	if publication.Owner == 0 {
		return out, false, c.finishSchedulerOwned(owner, errors.New("parser-core phase zero: zero scheduler lineage"))
	}
	if err = c.enterLiveCondenseCandidates(candidates); err != nil {
		return out, false, c.finishSchedulerOwned(owner, err)
	}
	freshSession := c.schedulerFrame.fresh
	if !freshSession {
		defer c.clearLiveCondenseCandidates()
	}
	out, err = c.shiftDirectUncheckpointed(head, lookahead, targetState, extra, shifted, fork)
	c.clearLiveCondenseCandidates()
	if err == nil && c.lastShiftFresh {
		// Keep the existing identity, conflict and journal checks. No remembered
		// owner fact survives beyond this call or bypasses recordHeadOwner.
		err = c.recordHeadOwner(out, publication.Owner)
		if err == nil && publication.PublishLineage {
			err = c.publishShiftLineage(out, publication)
		}
		ownerPublished = err == nil
	}
	err = c.finishSchedulerOwned(owner, err)
	if err != nil {
		return out, false, err
	}
	return out, ownerPublished, nil
}

// publishShiftLineage keeps RecordHeadLineageOwned's exact normalization,
// scalar merge and conditional set-union rules. The root adapter proves the
// omitted reference argument is empty, non-overflowed and non-blended; that
// is precisely recordNodeLineageRefs' immediate no-op case.
func (c *Core) publishShiftLineage(head Head, publication ShiftHeadLineage) error {
	rank, lineage := publication.Rank, publication.Lineage
	if rank != CleanPathRankSelected && rank != CleanPathRankUnselected || lineage == 0 {
		rank = CleanPathRankUnknown
		lineage = 0
	}
	if err := c.recordNodeLineage(head, rank, lineage); err != nil {
		return err
	}
	if publication.SetDirty || head != publication.PreviousHead {
		return c.recordNodeLineageSet(head, publication.Set, publication.Blended)
	}
	return nil
}
