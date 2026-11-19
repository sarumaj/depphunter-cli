package parsercorephase0

// NewScannerSnapshotRecord allocates an unpublished record and freezes all of
// its mutable scanner fields. Large trial masks and small byte payloads share
// the record's allocation; other shapes keep independent, exact-sized copies.
// The caller must initialize the record before publishing it.
//
// Each returned field owns a disjoint range. Spare capacity belongs only to
// the final present byte field and the trial mask, so cap-based memory budgets
// account for every reserved byte without allowing appends into a neighbor.
func NewScannerSnapshotRecord[T any](payload, start, end []byte, tried []bool) (*T, []byte, []byte, []byte, []bool) {
	if len(tried) > 64 && len(tried) <= 128 && len(payload) <= 64 && len(start) <= 64-len(payload) && len(end) <= 64-len(payload)-len(start) {
		total := len(payload) + len(start) + len(end)
		if total > 0 {
			if total <= 32 {
				block := new(struct {
					record T
					bytes  [32]byte
					tried  [128]bool
				})
				p, s, e := freezeScannerFields(block.bytes[:], payload, start, end)
				copy(block.tried[:], tried)
				return &block.record, p, s, e, block.tried[:len(tried)]
			}
			block := new(struct {
				record T
				bytes  [64]byte
				tried  [128]bool
			})
			p, s, e := freezeScannerFields(block.bytes[:], payload, start, end)
			copy(block.tried[:], tried)
			return &block.record, p, s, e, block.tried[:len(tried)]
		}
	}
	return new(T), cloneSnapshotBytes(payload), cloneSnapshotBytes(start), cloneSnapshotBytes(end), cloneSnapshotBools(tried)
}

func freezeScannerFields(storage, payload, start, end []byte) ([]byte, []byte, []byte) {
	var fields [3][]byte
	last := -1
	for i, field := range [3][]byte{payload, start, end} {
		if field != nil {
			last = i
		}
	}
	offset := 0
	for i, source := range [3][]byte{payload, start, end} {
		if source == nil {
			continue
		}
		limit := offset + len(source)
		capacity := limit
		if i == last {
			capacity = len(storage)
		}
		fields[i] = storage[offset:limit:capacity]
		copy(fields[i], source)
		offset = limit
	}
	return fields[0], fields[1], fields[2]
}

func cloneSnapshotBytes(src []byte) []byte {
	if src == nil {
		return nil
	}
	out := make([]byte, len(src))
	copy(out, src)
	return out
}

func cloneSnapshotBools(src []bool) []bool {
	if src == nil {
		return nil
	}
	out := make([]bool, len(src))
	copy(out, src)
	return out
}
