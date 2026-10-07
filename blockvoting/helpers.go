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

package blockvoting

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"sort"

	"github.com/prysmaticlabs/go-bitfield"

	"github.com/LFDT-Iguazu/iguazu-types/coordinator/types"
)

// CountCommitteeVotes sums the population count of every CommitteeVote's
// aggregation bits.
func CountCommitteeVotes(votes []*types.CommitteeVote) uint64 {
	var count uint64
	for _, v := range votes {
		count += bitfield.Bitlist(v.AggregationBits).Count()
	}
	return count
}

// CommitteeVoteKey derives a stable key for grouping identical committee
// votes. The key covers aggregation-bit length, slot and committee index —
// enough to identify votes from the same committee at the same slot with
// the same expected size.
func CommitteeVoteKey(v *types.CommitteeVote) [32]byte {
	if v == nil {
		return [32]byte{}
	}
	bits := bitfield.Bitlist(v.AggregationBits)
	var buf [24]byte
	binary.LittleEndian.PutUint64(buf[0:8], uint64(bits.Len()))
	binary.LittleEndian.PutUint64(buf[8:16], uint64(v.Slot))
	binary.LittleEndian.PutUint64(buf[16:24], uint64(v.Index))
	return sha256.Sum256(buf[:])
}

// AddAggregateCommitteeVote folds addVote into an existing list. If a vote
// with the same (slot, index) already exists the two bitlists are OR-ed
// together in place; otherwise addVote is appended.
func AddAggregateCommitteeVote(votes []*types.CommitteeVote, addVote *types.CommitteeVote) []*types.CommitteeVote {
	for _, v := range votes {
		if v.Slot != addVote.Slot || v.Index != addVote.Index {
			continue
		}
		merged, err := bitfield.Bitlist(v.AggregationBits).Or(bitfield.Bitlist(addVote.AggregationBits))
		if err != nil {
			// Length mismatch between the two bitlists — drop the
			// conflicting vote instead of panicking, as the coordinator
			// did. The caller retains the untouched list.
			continue
		}
		v.AggregationBits = []byte(merged)
		return votes
	}
	return append(votes, addVote)
}

// Copy returns a deep copy of a BlockVoting entry, normalising every
// CommitteeVote's slot to the slot of the enclosing BlockVoting. See
// helpers.BlockVotingCopy for why this apparent bug is consensus-critical.
func Copy(bv *types.BlockVoting) *types.BlockVoting {
	if bv == nil {
		return nil
	}
	cpVotes := make([]*types.CommitteeVote, len(bv.Votes))
	for i, v := range bv.Votes {
		cpVotes[i] = &types.CommitteeVote{
			AggregationBits: append([]byte(nil), v.AggregationBits...),
			Slot:            bv.Slot,
			Index:           v.Index,
		}
	}
	return &types.BlockVoting{
		Root:       append([]byte(nil), bv.Root...),
		Slot:       bv.Slot,
		Candidates: append([]byte(nil), bv.Candidates...),
		Votes:      cpVotes,
	}
}

// ArrCopy returns a deep copy of a BlockVoting slice.
func ArrCopy(votes []*types.BlockVoting) []*types.BlockVoting {
	cp := make([]*types.BlockVoting, len(votes))
	for i, v := range votes {
		cp[i] = Copy(v)
	}
	return cp
}

// ArrStateOrder puts a BlockVoting array into a canonical order suitable
// for state-root computation: each inner vote list is first sorted by
// HashTreeRoot, then the outer list is sorted the same way.
func ArrStateOrder(votes []*types.BlockVoting) ([]*types.BlockVoting, error) {
	cp := make([]*types.BlockVoting, len(votes))
	for i, v := range votes {
		item := Copy(v)
		sorted, err := CommitteeVoteArrSort(item.Votes)
		if err != nil {
			return nil, err
		}
		item.Votes = sorted
		cp[i] = item
	}
	return ArrSort(cp)
}

// ArrSort sorts a BlockVoting array lexicographically by HashTreeRoot.
// Duplicate-root entries are collapsed (last-write-wins) to match the
// coordinator's map-based sort.
func ArrSort(votes []*types.BlockVoting) ([]*types.BlockVoting, error) {
	byKey := map[[32]byte]*types.BlockVoting{}
	keys := make([][32]byte, 0, len(votes))
	for _, v := range votes {
		k, err := DataRoot(v)
		if err != nil {
			return nil, err
		}
		if _, seen := byKey[k]; !seen {
			keys = append(keys, k)
		}
		byKey[k] = v
	}
	sort.Slice(keys, func(i, j int) bool {
		return bytes.Compare(keys[i][:], keys[j][:]) < 0
	})
	sorted := make([]*types.BlockVoting, len(keys))
	for i, k := range keys {
		sorted[i] = byKey[k]
	}
	return sorted, nil
}

// CommitteeVoteArrSort sorts a CommitteeVote slice lexicographically by
// HashTreeRoot, collapsing duplicates (last-write-wins).
func CommitteeVoteArrSort(votes []*types.CommitteeVote) ([]*types.CommitteeVote, error) {
	byKey := map[[32]byte]*types.CommitteeVote{}
	keys := make([][32]byte, 0, len(votes))
	for _, v := range votes {
		k, err := CommitteeVoteRoot(v)
		if err != nil {
			return nil, err
		}
		if _, seen := byKey[k]; !seen {
			keys = append(keys, k)
		}
		byKey[k] = v
	}
	sort.Slice(keys, func(i, j int) bool {
		return bytes.Compare(keys[i][:], keys[j][:]) < 0
	})
	sorted := make([]*types.CommitteeVote, len(keys))
	for i, k := range keys {
		sorted[i] = byKey[k]
	}
	return sorted, nil
}

// IndexOfRoot returns the index of root in arrRoots, or -1 if absent.
func IndexOfRoot(arrRoots [][]byte, root []byte) int {
	for i, r := range arrRoots {
		if bytes.Equal(r, root) {
			return i
		}
	}
	return -1
}
