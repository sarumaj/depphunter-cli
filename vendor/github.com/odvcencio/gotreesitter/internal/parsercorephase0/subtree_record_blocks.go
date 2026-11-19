package parsercorephase0

import "unsafe"

// Keep small subtree arenas contiguous and grow large arenas in bounded blocks.
// Logical SubtreeIDs and records survive growth; no record reclamation occurs.
const subtreeRecordBlockShift = 16
const subtreeRecordBlockSize = 1 << subtreeRecordBlockShift

type subtreeRecordBlock struct {
	records []subtreeRecord
}

type subtreeRecordBlocks struct {
	first        []subtreeRecord
	blocks       []subtreeRecordBlock
	length       int
	slotCapacity int
	bufferBytes  uint64
}

func (s *subtreeRecordBlocks) Len() int { return s.length }

func (s *subtreeRecordBlocks) At(index int) *subtreeRecord {
	if index < 0 || index >= s.length {
		return nil
	}
	if index < cap(s.first) {
		if index >= len(s.first) {
			return nil
		}
		return &s.first[index]
	}
	index -= cap(s.first)
	block, offset := index>>subtreeRecordBlockShift, index&(subtreeRecordBlockSize-1)
	if block >= len(s.blocks) || offset >= len(s.blocks[block].records) {
		return nil
	}
	return &s.blocks[block].records[offset]
}

func (s *subtreeRecordBlocks) Append(record subtreeRecord, limit uint32) (SubtreeID, bool) {
	if s.length == int(^uint(0)>>1) || uint64(s.length) >= uint64(limit) {
		return 0, false
	}
	if len(s.first) < cap(s.first) || len(s.blocks) == 0 && cap(s.first) < subtreeRecordBlockSize {
		oldCapacity := cap(s.first)
		s.first = append(s.first, record)
		s.length++
		if cap(s.first) != oldCapacity {
			s.refreshGeometry()
		}
		return SubtreeID(s.length), true
	}
	suffix := s.length - cap(s.first)
	block, offset := suffix>>subtreeRecordBlockShift, suffix&(subtreeRecordBlockSize-1)
	oldMetadataCapacity := cap(s.blocks)
	if block == len(s.blocks) {
		s.blocks = append(s.blocks, subtreeRecordBlock{})
	}
	records := s.blocks[block].records
	changedCapacity := offset >= cap(records)
	if changedCapacity {
		// A partial tail can grow when a caller raises its configured limit.
		if offset > 0 && records == nil {
			return 0, false
		}
		remaining := min(uint64(limit), uint64(^uint(0)>>1)) - uint64(cap(s.first)) - uint64(block)*subtreeRecordBlockSize
		capacity := int(min(uint64(subtreeRecordBlockSize), remaining))
		next := make([]subtreeRecord, 0, capacity)
		next = next[:offset]
		copy(next, records)
		records = next
	}
	records = records[:offset+1]
	records[offset] = record
	s.blocks[block].records = records
	if changedCapacity || cap(s.blocks) != oldMetadataCapacity {
		s.refreshGeometry()
	}
	s.length++
	return SubtreeID(s.length), true
}

// Truncate restores the active record prefix without discarding retained capacity.
func (s *subtreeRecordBlocks) Truncate(length int) bool {
	if length < 0 || length > s.length {
		return false
	}
	s.first = s.first[:min(length, len(s.first))]
	for index := range s.blocks {
		records := s.blocks[index].records
		keep := max(length-cap(s.first)-index*subtreeRecordBlockSize, 0)
		s.blocks[index].records = records[:min(keep, len(records))]
	}
	s.length = length
	return true
}

// CloneActive preserves records and logical identities without borrowing backing
// buffers or copying unused append capacity.
func (s *subtreeRecordBlocks) CloneActive() subtreeRecordBlocks { return s.cloneActive(0) }

// A detached recovery clone reserves its one appended root exactly. It must
// not allocate a whole suffix block outside the diagnostic clone's budget.
func (s *subtreeRecordBlocks) cloneActive(extra int) subtreeRecordBlocks {
	if extra < 0 || extra > int(^uint(0)>>1)-s.length {
		panic("invalid subtree clone reserve")
	}
	want := s.length + extra
	firstCapacity := max(len(s.first), min(want, subtreeRecordBlockSize))
	if s.length > cap(s.first) {
		firstCapacity = cap(s.first)
	}
	clone := subtreeRecordBlocks{length: s.length, first: make([]subtreeRecord, len(s.first), firstCapacity)}
	copy(clone.first, s.first)
	if want > firstCapacity {
		count := (want - firstCapacity + subtreeRecordBlockSize - 1) >> subtreeRecordBlockShift
		clone.blocks = make([]subtreeRecordBlock, count)
		for i := range clone.blocks {
			goal := min(subtreeRecordBlockSize, want-firstCapacity-i*subtreeRecordBlockSize)
			active := min(goal, max(s.length-firstCapacity-i*subtreeRecordBlockSize, 0))
			clone.blocks[i].records = make([]subtreeRecord, active, goal)
			if active > 0 {
				copy(clone.blocks[i].records, s.blocks[i].records[:active])
			}
		}
	}
	clone.refreshGeometry()
	return clone
}

func (s *subtreeRecordBlocks) CloneActiveFootprintBytes() uint64 { return s.cloneFootprintBytes(0) }
func (s *subtreeRecordBlocks) cloneFootprintBytes(extra int) uint64 {
	if extra < 0 || extra > int(^uint(0)>>1)-s.length {
		panic("invalid subtree clone reserve")
	}
	want := s.length + extra
	firstCapacity := max(len(s.first), min(want, subtreeRecordBlockSize))
	if s.length > cap(s.first) {
		firstCapacity = cap(s.first)
	}
	total := uint64(want) * uint64(unsafe.Sizeof(subtreeRecord{}))
	if want > firstCapacity {
		count := (want - firstCapacity + subtreeRecordBlockSize - 1) >> subtreeRecordBlockShift
		total += uint64(count) * uint64(unsafe.Sizeof(subtreeRecordBlock{}))
	}
	return total
}

func (s *subtreeRecordBlocks) Reset() {
	s.first = s.first[:0]
	for i := range s.blocks {
		s.blocks[i].records = s.blocks[i].records[:0]
	}
	s.length = 0
}

// Geometry is fixed between backing-buffer mutations. Budget polling reads
// these cached scalars rather than scanning the arena's block table.
func (s *subtreeRecordBlocks) FootprintBytes() uint64 { return s.bufferBytes }

func (s *subtreeRecordBlocks) refreshGeometry() {
	slots := cap(s.first)
	total := uint64(slots)*uint64(unsafe.Sizeof(subtreeRecord{})) +
		uint64(cap(s.blocks))*uint64(unsafe.Sizeof(subtreeRecordBlock{}))
	for _, block := range s.blocks {
		slots += cap(block.records)
		total += uint64(cap(block.records)) * uint64(unsafe.Sizeof(subtreeRecord{}))
	}

	s.slotCapacity, s.bufferBytes = slots, total
}
