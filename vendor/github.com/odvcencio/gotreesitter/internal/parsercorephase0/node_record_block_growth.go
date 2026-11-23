package parsercorephase0

import "unsafe"

// Capacity counts slots available in the logical arena. Spare buffers are
// charged by FootprintBytes, but do not make retired IDs addressable.
func (s *nodeRecordBlocks) Capacity() int { return s.slotCapacity }

func nodeRecordBlockMetadataBytes(count int) uint64 {
	if count <= nodeRecordBlockSize {
		return 0
	}
	blocks := (count - nodeRecordBlockSize + nodeRecordBlockSize - 1) >> nodeRecordBlockShift
	return uint64(blocks) * uint64(unsafe.Sizeof(nodeRecordBlock{}))
}

// GrowthFootprintBytes is an upper bound on Grow's capacity increase. It
// includes the pointer table and does not assume any spare buffer fits.
func (s *nodeRecordBlocks) GrowthFootprintBytes(want int) uint64 {
	if want <= s.Capacity() {
		return 0
	}
	first := cap(s.first)
	if len(s.blocks) == 0 && first < nodeRecordBlockSize {
		first = max(first, min(want, nodeRecordBlockSize))
	}
	total := uint64(first-cap(s.first)) * uint64(unsafe.Sizeof(nodeRecord{}))
	count := 0
	if want > first {
		count = (want - first + nodeRecordBlockSize - 1) >> nodeRecordBlockShift
	}
	if count > cap(s.blocks) {
		total += uint64(count-cap(s.blocks)) * uint64(unsafe.Sizeof(nodeRecordBlock{}))
	}
	for index := 0; index < count; index++ {
		goal := min(nodeRecordBlockSize, want-first-index*nodeRecordBlockSize)
		current := 0
		if index < len(s.blocks) {
			current = cap(s.blocks[index].records)
		}
		if goal > current {
			total += uint64(goal-current) * uint64(unsafe.Sizeof(nodeRecord{}))
		}
	}
	return total
}

// Grow reserves exact slot capacity without copying complete suffix blocks.
// It must not run after retirement: capacity planning cannot resurrect holes.
func (s *nodeRecordBlocks) Grow(want int) {
	if want <= s.Capacity() || s.retiredThrough != 0 {
		return
	}
	if len(s.blocks) == 0 && cap(s.first) < nodeRecordBlockSize {
		s.first = growArena(s.first, min(want, nodeRecordBlockSize))
	}
	if want <= cap(s.first) {
		s.refreshGeometry()
		return
	}
	count := (want - cap(s.first) + nodeRecordBlockSize - 1) >> nodeRecordBlockShift
	if count > cap(s.blocks) {
		blocks := make([]nodeRecordBlock, count)
		copy(blocks, s.blocks)
		s.blocks = blocks
	} else if count > len(s.blocks) {
		s.blocks = s.blocks[:count]
	}
	for index := 0; index < count; index++ {
		goal := min(nodeRecordBlockSize, want-cap(s.first)-index*nodeRecordBlockSize)
		records := s.blocks[index].records
		if cap(records) >= goal {
			continue
		}
		spare := -1
		for i, buffer := range s.spare {
			if cap(buffer) >= goal && (spare < 0 || cap(buffer) < cap(s.spare[spare])) {
				spare = i
			}
		}
		var next []nodeRecord
		if spare >= 0 {
			next = s.spare[spare][:len(records)]
			last := len(s.spare) - 1
			s.spare[spare] = s.spare[last]
			s.spare[last] = nil
			s.spare = s.spare[:last]
		} else {
			next = make([]nodeRecord, len(records), goal)
		}
		copy(next, records)
		s.blocks[index].records = next
	}
	s.refreshGeometry()
}
