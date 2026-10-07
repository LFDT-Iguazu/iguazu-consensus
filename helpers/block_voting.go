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

package helpers

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"sort"

	bitfield "github.com/prysmaticlabs/go-bitfield"

	"github.com/LFDT-Iguazu/iguazu-types/common"
	"github.com/LFDT-Iguazu/iguazu-types/coordinator/types"
)

// CountCommitteeVotes returns the total number of set bits across all
// CommitteeVote aggregation bitlists (i.e. total attestor votes).
// AggregationBits is stored as an SSZ bitlist where the last byte carries a
// sentinel bit; Count() subtracts that sentinel.
func CountCommitteeVotes(committeeVotes []*types.CommitteeVote) uint64 {
	count := uint64(0)
	for _, cv := range committeeVotes {
		count += bitfield.Bitlist(cv.AggregationBits).Count()
	}
	return count
}

// AggregateCommitteeVote merges votes that cover the same (Slot, Index)
// committee by OR-ing their aggregation bitlists.
//
// Deprecated: use AddAggregateCommitteeVote for incremental aggregation.
func AggregateCommitteeVote(committeeVotes []*types.CommitteeVote) []*types.CommitteeVote {
	// The grouping key mirrors the coordinator's CommitteeVoteKey, which
	// hashes (aggregation-bit length, slot, index). Including the bit length
	// is consensus-relevant: votes for the same (slot, index) but with
	// differently sized bitlists are kept apart rather than merged.
	type key struct {
		bitLen uint64
		slot   types.Slot
		index  types.CommitteeIndex
	}
	groupMap := map[key][]*types.CommitteeVote{}
	order := make([]key, 0, len(committeeVotes))
	for _, cv := range committeeVotes {
		k := key{bitfield.Bitlist(cv.AggregationBits).Len(), cv.Slot, cv.Index}
		if _, ok := groupMap[k]; !ok {
			order = append(order, k)
			groupMap[k] = nil
		}
		groupMap[k] = append(groupMap[k], cv)
	}

	res := make([]*types.CommitteeVote, 0, len(groupMap))
	for _, k := range order {
		group := groupMap[k]
		agg := bitfield.Bitlist(append([]byte(nil), group[0].AggregationBits...))
		for _, cv := range group[1:] {
			merged, err := agg.Or(bitfield.Bitlist(cv.AggregationBits))
			if err != nil {
				// length mismatch should not happen within the same committee
				continue
			}
			agg = merged
		}
		res = append(res, &types.CommitteeVote{
			AggregationBits: []byte(agg),
			Slot:            k.slot,
			Index:           k.index,
		})
	}
	return res
}

// AddAggregateCommitteeVote appends addVote to committeeVotes, merging it into
// the existing entry for the same (Slot, Index) if one exists.
func AddAggregateCommitteeVote(committeeVotes []*types.CommitteeVote, addVote *types.CommitteeVote) []*types.CommitteeVote {
	for _, cv := range committeeVotes {
		if cv.Slot == addVote.Slot && cv.Index == addVote.Index {
			merged, err := bitfield.Bitlist(cv.AggregationBits).Or(bitfield.Bitlist(addVote.AggregationBits))
			if err != nil {
				// length mismatch: log-worthy in production but should never happen
				continue
			}
			cv.AggregationBits = []byte(merged)
			return committeeVotes
		}
	}
	return append(committeeVotes, addVote.Copy())
}

// BlockVotingCopy deep-copies a BlockVoting entry.
//
// Consensus-critical: every copied CommitteeVote takes the slot of the
// enclosing BlockVoting rather than its own. This looks like a mistake, and
// arguably is one, but it is the behaviour of the live coordinator
// (beacon-chain/core/helpers.BlockVotingCopy) and therefore part of the
// canonical mainnet state transition: copies happen on every attestation
// append, so the stored votes are always normalised this way, and the
// (Slot, Index) aggregation matching in AddAggregateCommitteeVote /
// AggregateCommitteeVote is performed against the normalised slots.
// Copying the real vote slot instead changes which votes get merged and
// produces a different state root.
func BlockVotingCopy(bv *types.BlockVoting) *types.BlockVoting {
	if bv == nil {
		return nil
	}
	cpyVotes := make([]*types.CommitteeVote, len(bv.Votes))
	for i, v := range bv.Votes {
		cpyVotes[i] = &types.CommitteeVote{
			AggregationBits: append([]byte(nil), v.AggregationBits...),
			Slot:            bv.Slot,
			Index:           v.Index,
		}
	}
	return &types.BlockVoting{
		Root:       append([]byte(nil), bv.Root...),
		Slot:       bv.Slot,
		Candidates: append([]byte(nil), bv.Candidates...),
		Votes:      cpyVotes,
	}
}

// BlockVotingArrCopy deep-copies a slice of BlockVoting entries.
func BlockVotingArrCopy(votes []*types.BlockVoting) []*types.BlockVoting {
	cpy := make([]*types.BlockVoting, len(votes))
	for i, v := range votes {
		cpy[i] = BlockVotingCopy(v)
	}
	return cpy
}

// BlockVotingArrStateOrder returns a deterministically sorted copy of votes,
// suitable for beacon-state hashing. Each entry's Votes list is itself sorted
// before the outer sort is applied.
func BlockVotingArrStateOrder(votes []*types.BlockVoting) ([]*types.BlockVoting, error) {
	cpy := make([]*types.BlockVoting, len(votes))
	for i, v := range votes {
		c := BlockVotingCopy(v)
		c.Votes = committeeVoteArrSort(c.Votes)
		cpy[i] = c
	}
	return blockVotingArrSort(cpy), nil
}

// IndexOfRoot returns the index of root in arrRoots, or -1 if not present.
func IndexOfRoot(arrRoots [][]byte, root []byte) int {
	for i, r := range arrRoots {
		if bytes.Equal(r, root) {
			return i
		}
	}
	return -1
}

// PrintBlockVoting formats a BlockVoting entry for logging.
func PrintBlockVoting(bv *types.BlockVoting) string {
	cands := common.HashArrayFromBytes(bv.Candidates)
	candStr := "["
	for i, c := range cands {
		candStr += fmt.Sprintf("%q", c.Hex())
		if i < len(cands)-1 {
			candStr += ","
		}
	}
	candStr += "]"

	str := fmt.Sprintf("{root:%q candidates:%s slot:%d votes:[", fmt.Sprintf("%#x", bv.Root), candStr, bv.Slot)
	for i, v := range bv.Votes {
		bl := bitfield.Bitlist(v.AggregationBits)
		str += fmt.Sprintf("{bits:%b len:%d count:%d slot:%d idx:%d}",
			v.AggregationBits, bl.Len(), bl.Count(), v.Slot, v.Index)
		if i < len(bv.Votes)-1 {
			str += ","
		}
	}
	str += "]}"
	return str
}

// ---- sort helpers -----------------------------------------------------------

func committeeVoteSortKey(cv *types.CommitteeVote) [32]byte {
	h := sha256.New()
	h.Write(cv.AggregationBits)
	var buf [16]byte
	binary.LittleEndian.PutUint64(buf[:8], uint64(cv.Slot))
	binary.LittleEndian.PutUint64(buf[8:], uint64(cv.Index))
	h.Write(buf[:])
	var key [32]byte
	copy(key[:], h.Sum(nil))
	return key
}

func committeeVoteArrSort(votes []*types.CommitteeVote) []*types.CommitteeVote {
	if len(votes) == 0 {
		return votes
	}
	type kv struct {
		key [32]byte
		val *types.CommitteeVote
	}
	pairs := make([]kv, len(votes))
	for i, v := range votes {
		pairs[i] = kv{committeeVoteSortKey(v), v}
	}
	sort.Slice(pairs, func(i, j int) bool {
		return bytes.Compare(pairs[i].key[:], pairs[j].key[:]) < 0
	})
	sorted := make([]*types.CommitteeVote, len(votes))
	for i, p := range pairs {
		sorted[i] = p.val
	}
	return sorted
}

func blockVotingSortKey(bv *types.BlockVoting) [32]byte {
	h := sha256.New()
	h.Write(bv.Root)
	var buf [8]byte
	binary.LittleEndian.PutUint64(buf[:], uint64(bv.Slot))
	h.Write(buf[:])
	h.Write(bv.Candidates)
	for _, v := range bv.Votes {
		k := committeeVoteSortKey(v)
		h.Write(k[:])
	}
	var key [32]byte
	copy(key[:], h.Sum(nil))
	return key
}

func blockVotingArrSort(votes []*types.BlockVoting) []*types.BlockVoting {
	if len(votes) == 0 {
		return votes
	}
	type kv struct {
		key [32]byte
		val *types.BlockVoting
	}
	pairs := make([]kv, len(votes))
	for i, v := range votes {
		pairs[i] = kv{blockVotingSortKey(v), v}
	}
	sort.Slice(pairs, func(i, j int) bool {
		return bytes.Compare(pairs[i].key[:], pairs[j].key[:]) < 0
	})
	sorted := make([]*types.BlockVoting, len(votes))
	for i, p := range pairs {
		sorted[i] = p.val
	}
	return sorted
}
