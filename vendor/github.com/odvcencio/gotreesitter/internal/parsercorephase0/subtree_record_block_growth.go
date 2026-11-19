package parsercorephase0

import "unsafe"

// Capacity counts all reserved subtree-record slots.
func (s *subtreeRecordBlocks) Capacity() int { return s.slotCapacity }

func subtreeRecordBlockMetadataBytes(count int) uint64 {
	if count <= subtreeRecordBlockSize {
		return 0
	}
	blocks := (count - subtreeRecordBlockSize + subtreeRecordBlockSize - 1) >> subtreeRecordBlockShift
	return uint64(blocks) * uint64(unsafe.Sizeof(subtreeRecordBlock{}))
}

// GrowthFootprintBytes is an upper bound on Grow's capacity increase. It
// includes pointer-table storage.
func (s *subtreeRecordBlocks) GrowthFootprintBytes(want int) uint64 {
	if want <= s.Capacity() {
		return 0
	}
	first := cap(s.first)
	if len(s.blocks) == 0 && first < subtreeRecordBlockSize {
		first = max(first, min(want, subtreeRecordBlockSize))
	}
	total := uint64(first-cap(s.first)) * uint64(unsafe.Sizeof(subtreeRecord{}))
	count := 0
	if want > first {
		count = (want - first + subtreeRecordBlockSize - 1) >> subtreeRecordBlockShift
	}
	if count > cap(s.blocks) {
		total += uint64(count-cap(s.blocks)) * uint64(unsafe.Sizeof(subtreeRecordBlock{}))
	}
	for index := 0; index < count; index++ {
		goal := min(subtreeRecordBlockSize, want-first-index*subtreeRecordBlockSize)
		current := 0
		if index < len(s.blocks) {
			current = cap(s.blocks[index].records)
		}
		if goal > current {
			total += uint64(goal-current) * uint64(unsafe.Sizeof(subtreeRecord{}))
		}
	}
	return total
}

// Grow reserves exact slot capacity without copying complete suffix blocks.
func (s *subtreeRecordBlocks) Grow(want int) {
	if want <= s.Capacity() {
		return
	}
	if len(s.blocks) == 0 && cap(s.first) < subtreeRecordBlockSize {
		s.first = growArena(s.first, min(want, subtreeRecordBlockSize))
	}
	if want <= cap(s.first) {
		s.refreshGeometry()
		return
	}
	count := (want - cap(s.first) + subtreeRecordBlockSize - 1) >> subtreeRecordBlockShift
	if count > cap(s.blocks) {
		blocks := make([]subtreeRecordBlock, count)
		copy(blocks, s.blocks)
		s.blocks = blocks
	} else if count > len(s.blocks) {
		s.blocks = s.blocks[:count]
	}
	for index := 0; index < count; index++ {
		goal := min(subtreeRecordBlockSize, want-cap(s.first)-index*subtreeRecordBlockSize)
		records := s.blocks[index].records
		if cap(records) >= goal {
			continue
		}
		next := make([]subtreeRecord, len(records), goal)
		copy(next, records)
		s.blocks[index].records = next
	}
	s.refreshGeometry()
}
