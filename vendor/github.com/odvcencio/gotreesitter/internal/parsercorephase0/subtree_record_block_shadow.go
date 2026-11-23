//go:build gts_eof_recovery_shadow

package parsercorephase0

import (
	"runtime"
	"unsafe"
)

func equalDiagnosticSubtreeBlocks(left, right *subtreeRecordBlocks) bool {
	return left.Len() == right.Len() && equalDiagnosticSubtreePrefix(left, right)
}

func equalDiagnosticSubtreePrefix(left, right *subtreeRecordBlocks) bool {
	if left.Len() > right.Len() {
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

type subtreeBlockSpan struct{ start, bytes uintptr }

func subtreeBlockSpans(s *subtreeRecordBlocks) []subtreeBlockSpan {
	spans := make([]subtreeBlockSpan, 0, 2+len(s.blocks))
	if cap(s.first) > 0 {
		spans = append(spans, subtreeBlockSpan{uintptr(unsafe.Pointer(unsafe.SliceData(s.first))), uintptr(cap(s.first)) * unsafe.Sizeof(subtreeRecord{})})
	}
	if cap(s.blocks) > 0 {
		spans = append(spans, subtreeBlockSpan{uintptr(unsafe.Pointer(unsafe.SliceData(s.blocks))), uintptr(cap(s.blocks)) * unsafe.Sizeof(subtreeRecordBlock{})})
	}

	for _, block := range s.blocks {
		if cap(block.records) > 0 {
			spans = append(spans, subtreeBlockSpan{uintptr(unsafe.Pointer(unsafe.SliceData(block.records))), uintptr(cap(block.records)) * unsafe.Sizeof(subtreeRecord{})})
		}
	}

	return spans
}

func disjointDiagnosticSubtreeBlocks(left, right *subtreeRecordBlocks) bool {
	defer runtime.KeepAlive(left)
	defer runtime.KeepAlive(right)
	for _, a := range subtreeBlockSpans(left) {
		for _, b := range subtreeBlockSpans(right) {
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
