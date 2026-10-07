// Copyright 2026 Digital Clever Solution LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package prevote

import (
	"context"

	"github.com/patrickmn/go-cache"
	"github.com/pkg/errors"
	"github.com/prysmaticlabs/go-bitfield"

	"github.com/LFDT-Iguazu/iguazu-types/coordinator/types"
)

// HasPrevote reports whether the pool already holds a prevote for the given
// slot/committee whose aggregation bits cover those of pv.
func (c *PrevoteCache) HasPrevote(pv *types.PreVote) (bool, error) {
	if pv == nil || pv.Data == nil {
		return false, errors.New("prevote data cannot be nil")
	}

	c.prevoteCacheLock.RLock()
	defer c.prevoteCacheLock.RUnlock()

	stored, ok := c.prevoteCache[pv.Data.Slot]
	if !ok {
		return false, nil
	}
	incoming := bitfield.Bitlist(pv.AggregationBits)
	for _, v := range stored {
		if v.Data == nil || v.Data.Index != pv.Data.Index {
			continue
		}
		existing := bitfield.Bitlist(v.AggregationBits)
		contains, err := existing.Contains(incoming)
		if err != nil {
			return false, err
		}
		if contains {
			return true, nil
		}
	}
	return false, nil
}

// SavePrevote adds pv to the pool unless its aggregation bits are already
// covered by a previously-seen entry (tracked in seenPrevote).
func (c *PrevoteCache) SavePrevote(pv *types.PreVote) error {
	if pv == nil {
		return nil
	}

	c.prevoteCacheLock.Lock()
	defer c.prevoteCacheLock.Unlock()

	seen, err := c.hasSeenBit(pv)
	if err != nil {
		return err
	}
	if seen {
		return nil
	}

	c.prevoteCache[pv.Data.Slot] = append(c.prevoteCache[pv.Data.Slot], pv.Copy())
	return nil
}

// GetPrevoteBySlot returns the cached prevotes for slot. Result is a live
// slice; callers must not mutate it.
func (c *PrevoteCache) GetPrevoteBySlot(_ context.Context, slot types.Slot) []*types.PreVote {
	c.prevoteCacheLock.RLock()
	defer c.prevoteCacheLock.RUnlock()

	if pv := c.prevoteCache[slot]; len(pv) > 0 {
		return pv
	}
	return []*types.PreVote{}
}

// PurgeOutdatedPrevote drops every cached prevote whose slot is strictly
// less than curSlot, moving their aggregation bits into the seen-bits index
// for the remainder of the TTL.
func (c *PrevoteCache) PurgeOutdatedPrevote(curSlot types.Slot) error {
	c.prevoteCacheLock.RLock()
	defer c.prevoteCacheLock.RUnlock()

	for k, v := range c.prevoteCache {
		if k >= curSlot {
			continue
		}
		for _, p := range v {
			if err := c.insertSeenBit(p); err != nil {
				return err
			}
		}
		delete(c.prevoteCache, k)
	}
	return nil
}

func (c *PrevoteCache) hasSeenBit(pv *types.PreVote) (bool, error) {
	root := dataRoot(pv.Data)
	v, ok := c.seenPrevote.Get(string(root[:]))
	if !ok {
		return false, nil
	}
	seenBits, ok := v.([]bitfield.Bitlist)
	if !ok {
		return false, errors.New("could not convert seen-bits to bitlist")
	}
	incoming := bitfield.Bitlist(pv.AggregationBits)
	for _, bit := range seenBits {
		contains, err := bit.Contains(incoming)
		if err != nil {
			return false, err
		}
		if contains {
			return true, nil
		}
	}
	return false, nil
}

func (c *PrevoteCache) insertSeenBit(pv *types.PreVote) error {
	root := dataRoot(pv.Data)
	incoming := bitfield.Bitlist(pv.AggregationBits)
	key := string(root[:])

	if v, ok := c.seenPrevote.Get(key); ok {
		seenBits, ok := v.([]bitfield.Bitlist)
		if !ok {
			return errors.New("could not convert seen-bits to bitlist")
		}
		for _, bit := range seenBits {
			contains, err := bit.Contains(incoming)
			if err != nil {
				return err
			}
			if contains {
				// Already covered — nothing to add.
				c.seenPrevote.Set(key, seenBits, cache.DefaultExpiration)
				return nil
			}
		}
		seenBits = append(seenBits, incoming)
		c.seenPrevote.Set(key, seenBits, cache.DefaultExpiration)
		return nil
	}

	c.seenPrevote.Set(key, []bitfield.Bitlist{incoming}, cache.DefaultExpiration)
	return nil
}
