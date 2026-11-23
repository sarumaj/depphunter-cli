package parsercorephase0

import (
	"errors"
	"math"
)

// Common maxima share the existing lineage record without increasing its
// width. One value selects full-width storage; arithmetic stays int64.
const wideNodePrecedence int16 = math.MinInt16

var errMissingWideNodePrecedence = errors.New("parser-core phase zero: missing wide node precedence")

func inlineNodePrecedence(value int64) int16 {
	inline := int16(value)
	if int64(inline) != value || inline == wideNodePrecedence {
		return wideNodePrecedence
	}
	return inline
}

func (c *Core) storedNodePrecedence(id NodeID) (int64, error) {
	if uint64(id)-1 >= uint64(len(c.nodeLineages)) {
		return 0, errMissingWideNodePrecedence
	}
	inline := c.nodeLineages[id-1].precedenceInline
	if inline != wideNodePrecedence {
		return int64(inline), nil
	}
	if c.nodePrecedenceWideBase == 0 || id < c.nodePrecedenceWideBase {
		return 0, errMissingWideNodePrecedence
	}
	index := uint64(id) - uint64(c.nodePrecedenceWideBase)
	if index >= uint64(len(c.nodePrecedenceWide)) {
		return 0, errMissingWideNodePrecedence
	}
	return c.nodePrecedenceWide[index], nil
}

// Wide storage starts at the first NodeID that needs it. Earlier nodes keep
// their exact inline values, so the unused prefix needs no allocation. Reads
// remain constant-time and all stored values remain int64. Published entries
// never change; transaction marks restore both the origin and logical end.
func (c *Core) storeWideNodePrecedence(id NodeID, value int64) {
	if c.nodePrecedenceWideBase == 0 {
		c.nodePrecedenceWideBase = id
	}
	needed := int(id-c.nodePrecedenceWideBase) + 1
	if cap(c.nodePrecedenceWide) < needed {
		values := make([]int64, needed, c.nodes.Capacity()-int(c.nodePrecedenceWideBase)+1)
		copy(values, c.nodePrecedenceWide)
		c.nodePrecedenceWide = values
	} else if len(c.nodePrecedenceWide) < needed {
		c.nodePrecedenceWide = c.nodePrecedenceWide[:needed]
	}
	c.nodePrecedenceWide[needed-1] = value
}
