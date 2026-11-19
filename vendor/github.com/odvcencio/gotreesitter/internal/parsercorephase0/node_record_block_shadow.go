//go:build gts_eof_recovery_shadow

package parsercorephase0

import (
	"runtime"
	"unsafe"
)

func equalDiagnosticNodeBlocks(left, right *nodeRecordBlocks) bool {
	if left.Len() != right.Len() || left.retiredThrough != right.retiredThrough {
		return false
	}
	for index := 0; index < left.Len(); index++ {
		a, b := left.At(index), right.At(index)
		if (a == nil) != (b == nil) || a != nil && *a != *b {
			return false
		}
	}
	return true
}

type nodeBlockSpan struct{ start, bytes uintptr }

func nodeBlockSpans(s *nodeRecordBlocks) []nodeBlockSpan {
	spans := make([]nodeBlockSpan, 0, 3+len(s.blocks)+len(s.spare))
	if cap(s.first) > 0 {
		spans = append(spans, nodeBlockSpan{uintptr(unsafe.Pointer(unsafe.SliceData(s.first))), uintptr(cap(s.first)) * unsafe.Sizeof(nodeRecord{})})
	}
	if cap(s.blocks) > 0 {
		spans = append(spans, nodeBlockSpan{uintptr(unsafe.Pointer(unsafe.SliceData(s.blocks))), uintptr(cap(s.blocks)) * unsafe.Sizeof(nodeRecordBlock{})})
	}
	if cap(s.spare) > 0 {
		spans = append(spans, nodeBlockSpan{uintptr(unsafe.Pointer(unsafe.SliceData(s.spare))), uintptr(cap(s.spare)) * unsafe.Sizeof([]nodeRecord{})})
	}
	for _, block := range s.blocks {
		if cap(block.records) > 0 {
			spans = append(spans, nodeBlockSpan{uintptr(unsafe.Pointer(unsafe.SliceData(block.records))), uintptr(cap(block.records)) * unsafe.Sizeof(nodeRecord{})})
		}
	}
	for _, records := range s.spare {
		if cap(records) > 0 {
			spans = append(spans, nodeBlockSpan{uintptr(unsafe.Pointer(unsafe.SliceData(records))), uintptr(cap(records)) * unsafe.Sizeof(nodeRecord{})})
		}
	}
	return spans
}

func disjointDiagnosticNodeBlocks(left, right *nodeRecordBlocks) bool {
	defer runtime.KeepAlive(left)
	defer runtime.KeepAlive(right)
	for _, a := range nodeBlockSpans(left) {
		for _, b := range nodeBlockSpans(right) {
			if a.start <= b.start {
				if b.start-a.start < a.bytes {
					return false
				}
			} else if a.start-b.start < b.bytes {
				return false
			}
		}
	}
	return true
}
