package parsercorephase0

import "unsafe"

// Keep small arenas contiguous. The blocked suffix has stable logical IDs;
// only backing storage is recycled. Collection remains disabled in Core.
const nodeRecordBlockShift = 16
const nodeRecordBlockSize = 1 << nodeRecordBlockShift

type nodeRecordBlock struct {
	records []nodeRecord
}

type nodeRecordBlocks struct {
	first          []nodeRecord
	blocks         []nodeRecordBlock
	spare          [][]nodeRecord
	length         int
	retiredThrough int
	slotCapacity   int
	bufferBytes    uint64
}

func (s *nodeRecordBlocks) Len() int { return s.length }

func (s *nodeRecordBlocks) At(index int) *nodeRecord {
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
	block, offset := index>>nodeRecordBlockShift, index&(nodeRecordBlockSize-1)
	if block >= len(s.blocks) || offset >= len(s.blocks[block].records) {
		return nil
	}
	return &s.blocks[block].records[offset]
}

func (s *nodeRecordBlocks) Append(record nodeRecord, limit uint32) (NodeID, bool) {
	if s.length == int(^uint(0)>>1) || uint64(s.length) >= uint64(limit) {
		return 0, false
	}
	if len(s.first) < cap(s.first) || len(s.blocks) == 0 && cap(s.first) < nodeRecordBlockSize {
		oldCapacity := cap(s.first)
		s.first = append(s.first, record)
		s.length++
		if cap(s.first) != oldCapacity {
			s.refreshGeometry()
		}
		return NodeID(s.length), true
	}
	suffix := s.length - cap(s.first)
	block, offset := suffix>>nodeRecordBlockShift, suffix&(nodeRecordBlockSize-1)
	oldMetadataCapacity := cap(s.blocks)
	if block == len(s.blocks) {
		s.blocks = append(s.blocks, nodeRecordBlock{})
	}
	records := s.blocks[block].records
	changedCapacity := offset >= cap(records)
	if changedCapacity {
		// A retired page behind the append frontier must never be resurrected.
		// A partial tail can grow when a caller raises its configured limit.
		if offset > 0 && records == nil {
			return 0, false
		}
		remaining := min(uint64(limit), uint64(^uint(0)>>1)) - uint64(cap(s.first)) - uint64(block)*nodeRecordBlockSize
		capacity := int(min(uint64(nodeRecordBlockSize), remaining))
		var next []nodeRecord
		spare := -1
		if offset == 0 {
			for i, records := range s.spare {
				if cap(records) >= capacity && (spare < 0 || cap(records) < cap(s.spare[spare])) {
					spare = i
				}
			}
		}
		if spare >= 0 {
			next = s.spare[spare][:0]
			last := len(s.spare) - 1
			s.spare[spare] = s.spare[last]
			s.spare[last] = nil
			s.spare = s.spare[:last]
		} else {
			next = make([]nodeRecord, 0, capacity)
		}
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
	return NodeID(s.length), true
}

// RetireBlock is a storage primitive, not an ownership proof. The caller must
// establish that no future reader can require any ID in this entire block.
// The append tail cannot be retired, so Append never fills an earlier hole.
func (s *nodeRecordBlocks) RetireBlock(index int) bool {
	if index < 0 || index >= len(s.blocks) || len(s.blocks[index].records) == 0 {
		return false
	}
	start := cap(s.first) + index*nodeRecordBlockSize
	if s.length-start <= nodeRecordBlockSize {
		return false
	}
	s.spare = append(s.spare, s.blocks[index].records[:0])
	s.blocks[index].records = nil
	s.retiredThrough = max(s.retiredThrough, start+nodeRecordBlockSize)
	s.refreshGeometry()
	return true
}

// Truncate supports a checkpoint only while it cannot resurrect a retired
// logical ID. The owner must finish every older checkpoint before retirement.
func (s *nodeRecordBlocks) Truncate(length int) bool {
	if length < s.retiredThrough || length < 0 || length > s.length {
		return false
	}
	s.first = s.first[:min(length, len(s.first))]
	for index := range s.blocks {
		records := s.blocks[index].records
		keep := max(length-cap(s.first)-index*nodeRecordBlockSize, 0)
		s.blocks[index].records = records[:min(keep, len(records))]
	}
	s.length = length
	return true
}

// CloneActive preserves the logical-ID map, including retired holes, without
// borrowing any active or spare backing buffer. Retained spare blocks and
// unused append capacity are not part of a diagnostic snapshot.
func (s *nodeRecordBlocks) CloneActive() nodeRecordBlocks {
	clone := nodeRecordBlocks{length: s.length, retiredThrough: s.retiredThrough}
	clone.first = make([]nodeRecord, len(s.first))
	copy(clone.first, s.first)
	if s.length > cap(s.first) {
		// Preserve the frozen prefix exactly, rather than append's size-class
		// rounding, because suffix identities are relative to this capacity.
		count := (s.length - cap(s.first) + nodeRecordBlockSize - 1) >> nodeRecordBlockShift
		clone.blocks = make([]nodeRecordBlock, count)
		for i := range clone.blocks {
			records := s.blocks[i].records
			if records != nil {
				clone.blocks[i].records = make([]nodeRecord, len(records))
				copy(clone.blocks[i].records, records)
			}
		}
	}
	clone.refreshGeometry()
	return clone
}

func (s *nodeRecordBlocks) CloneActiveFootprintBytes() uint64 {
	total := uint64(len(s.first)) * uint64(unsafe.Sizeof(nodeRecord{}))
	if s.length > cap(s.first) {
		count := (s.length - cap(s.first) + nodeRecordBlockSize - 1) >> nodeRecordBlockShift
		total += uint64(count) * uint64(unsafe.Sizeof(nodeRecordBlock{}))
		for _, block := range s.blocks[:count] {
			total += uint64(len(block.records)) * uint64(unsafe.Sizeof(nodeRecord{}))
		}
	}
	return total
}

func (s *nodeRecordBlocks) Reset() {
	if s.retiredThrough == 0 {
		s.first = s.first[:0]
		for i := range s.blocks {
			s.blocks[i].records = s.blocks[i].records[:0]
		}
		s.length = 0
		return
	}
	s.first = s.first[:0]
	for i := range s.blocks {
		if records := s.blocks[i].records; records != nil {
			s.spare = append(s.spare, records[:0])
		}
	}
	clear(s.blocks)
	s.blocks = s.blocks[:0]
	s.length = 0
	s.retiredThrough = 0
	s.refreshGeometry()
}

// Geometry is fixed between backing-buffer mutations. Budget polling reads
// these cached scalars rather than scanning the arena's block table.
func (s *nodeRecordBlocks) FootprintBytes() uint64 { return s.bufferBytes }

func (s *nodeRecordBlocks) refreshGeometry() {
	slots := cap(s.first)
	total := uint64(slots)*uint64(unsafe.Sizeof(nodeRecord{})) +
		uint64(cap(s.blocks))*uint64(unsafe.Sizeof(nodeRecordBlock{})) +
		uint64(cap(s.spare))*uint64(unsafe.Sizeof([]nodeRecord{}))
	for _, block := range s.blocks {
		slots += cap(block.records)
		total += uint64(cap(block.records)) * uint64(unsafe.Sizeof(nodeRecord{}))
	}
	for _, records := range s.spare {
		total += uint64(cap(records)) * uint64(unsafe.Sizeof(nodeRecord{}))
	}
	s.slotCapacity, s.bufferBytes = slots, total
}
