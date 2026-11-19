package parsercorephase0

import (
	"errors"
	"math"
)

const nodePathCountShift = 2
const nodePathCountMask uint8 = 0xfc
const nodePathCountSaturated uint8 = 62
const nodePathCountWide uint8 = 63

// Published path counts are immutable. Common counts share unused lineage
// flag bits; exceptional counts retain all 64 bits in a sorted sparse arena.
type nodePathCountRecord struct {
	node  NodeID
	value uint64
}

var errNodePathCountUnavailable = errors.New("parser-core phase zero: node path count is unavailable")

func (c *Core) nodePathCount(id NodeID) (uint64, error) {
	if id == 0 || uint64(id) > uint64(len(c.nodeLineages)) {
		return 0, errNodePathCountUnavailable
	}
	code := c.nodeLineages[id-1].flags >> nodePathCountShift
	if code == nodePathCountWide {
		return c.wideNodePathCount(id)
	}
	if code == nodePathCountSaturated {
		return math.MaxUint64, nil
	}
	return uint64(code), nil
}
func (c *Core) wideNodePathCount(id NodeID) (uint64, error) {
	lo, hi := 0, len(c.nodePathCountsWide)
	for lo < hi {
		mid := lo + (hi-lo)/2
		if c.nodePathCountsWide[mid].node < id {
			lo = mid + 1
		} else {
			hi = mid
		}
	}
	if lo == len(c.nodePathCountsWide) || c.nodePathCountsWide[lo].node != id {
		return 0, errNodePathCountUnavailable
	}
	return c.nodePathCountsWide[lo].value, nil
}
func (c *Core) appendNodePathCount(id NodeID, value uint64) uint8 {
	if value < uint64(nodePathCountSaturated) {
		return uint8(value) << nodePathCountShift
	}
	if value == math.MaxUint64 {
		return nodePathCountSaturated << nodePathCountShift
	}
	c.nodePathCountsWide = append(c.nodePathCountsWide, nodePathCountRecord{node: id, value: value})
	return nodePathCountWide << nodePathCountShift
}
func (c *Core) truncateNodePathCounts(count int) {
	n := len(c.nodePathCountsWide)
	for n > 0 && uint64(c.nodePathCountsWide[n-1].node) > uint64(count) {
		n--
	}
	c.nodePathCountsWide = c.nodePathCountsWide[:n]
}

func (c *Core) nodeWithPathCount(id NodeID) (*nodeRecord, uint64, error) {
	node, err := c.node(id)
	if err != nil {
		return nil, 0, err
	}
	count, err := c.nodePathCount(id)
	return node, count, err
}
