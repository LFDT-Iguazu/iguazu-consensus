// Copyright 2026 Digital Clever Solution LLC
// SPDX-License-Identifier: Apache-2.0
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

package blockvoting_test

import (
	"bytes"
	"testing"

	"github.com/prysmaticlabs/go-bitfield"
	"github.com/stretchr/testify/require"

	"github.com/LFDT-Iguazu/iguazu-consensus/blockvoting"
	"github.com/LFDT-Iguazu/iguazu-types/coordinator/types"
)

// twoHashBytes returns the byte-concatenation of two 32-byte hashes so that
// the resulting slice matches what the coordinator produced via
// gwatCommon.HashArray{a, b}.ToBytes().
func twoHashBytes(a, b [32]byte) []byte {
	out := make([]byte, 64)
	copy(out[:32], a[:])
	copy(out[32:], b[:])
	return out
}

func bitsOf(size int, positions ...uint64) []byte {
	b := bitfield.NewBitlist(uint64(size))
	for _, p := range positions {
		b.SetBitAt(p, true)
	}
	return []byte(b)
}

func TestAddAggregateCommitteeVote(t *testing.T) {
	// Base list: single vote at (slot=5, index=0).
	list0 := bitsOf(10, 0, 1, 2)
	votes := []*types.CommitteeVote{
		{Index: 0, Slot: 5, AggregationBits: list0},
	}

	// Aggregate a second vote at the same (slot, index): should merge bits.
	add := &types.CommitteeVote{
		Index:           0,
		Slot:            5,
		AggregationBits: bitsOf(10, 4, 6, 8),
	}
	// Different committee.
	diffCommittee := &types.CommitteeVote{
		Index:           1,
		Slot:            5,
		AggregationBits: bitsOf(10, 5),
	}
	// Different slot.
	diffSlot := &types.CommitteeVote{
		Index:           0,
		Slot:            6,
		AggregationBits: bitsOf(10, 7),
	}
	votes = blockvoting.AddAggregateCommitteeVote(votes, diffCommittee)
	votes = blockvoting.AddAggregateCommitteeVote(votes, diffSlot)
	votes = blockvoting.AddAggregateCommitteeVote(votes, add)

	require.Len(t, votes, 3)

	var mergedBits []byte
	for _, v := range votes {
		if v.Slot == 5 && v.Index == 0 {
			mergedBits = v.AggregationBits
		}
	}
	wantBits := bitsOf(10, 0, 1, 2, 4, 6, 8)
	require.Equal(t, wantBits, mergedBits)

	// Append new — (slot=10, index=7) not present before.
	newVote := &types.CommitteeVote{
		Index:           7,
		Slot:            10,
		AggregationBits: bitsOf(10, 4),
	}
	votes = blockvoting.AddAggregateCommitteeVote(votes, newVote)
	require.Len(t, votes, 4)
}

func TestCommitteeVoteArrSort_Stable(t *testing.T) {
	list := bitsOf(4, 0, 1, 2)
	a := &types.CommitteeVote{Slot: 5, Index: 0, AggregationBits: list}
	b := &types.CommitteeVote{Slot: 6, Index: 0, AggregationBits: list}
	c := &types.CommitteeVote{Slot: 7, Index: 0, AggregationBits: list}

	// Same content, different instance: should collapse to one entry.
	aDup := &types.CommitteeVote{AggregationBits: list, Slot: 5, Index: 0}

	sorted, err := blockvoting.CommitteeVoteArrSort([]*types.CommitteeVote{a, b, c, aDup})
	require.NoError(t, err)
	require.Len(t, sorted, 3)

	// Double-sorting is stable.
	again, err := blockvoting.CommitteeVoteArrSort(sorted)
	require.NoError(t, err)
	require.Equal(t, len(sorted), len(again))
	for i := range sorted {
		aRoot, err := blockvoting.CommitteeVoteRoot(sorted[i])
		require.NoError(t, err)
		bRoot, err := blockvoting.CommitteeVoteRoot(again[i])
		require.NoError(t, err)
		require.Equal(t, aRoot, bRoot)
	}
}

func TestArrSort_Lexicographic(t *testing.T) {
	root0 := [32]byte{}
	copy(root0[:], []byte("root-0--------------------------"))
	list := bitsOf(4, 0, 1, 2)
	votes0 := []*types.CommitteeVote{{Index: 0, Slot: 5, AggregationBits: list}}

	cand := twoHashBytes([32]byte{0x11, 0x11}, [32]byte{0x11, 0x22})

	bv0 := &types.BlockVoting{Root: root0[:], Slot: 5, Candidates: cand, Votes: votes0}
	bv1 := &types.BlockVoting{Root: root0[:], Slot: 7, Candidates: cand, Votes: votes0}
	bv2 := &types.BlockVoting{Root: root0[:], Slot: 6, Candidates: cand, Votes: votes0}

	sorted, err := blockvoting.ArrSort([]*types.BlockVoting{bv0, bv1, bv2})
	require.NoError(t, err)
	require.Len(t, sorted, 3)

	// The output is sorted by root bytes ascending.
	for i := 0; i+1 < len(sorted); i++ {
		ri, err := blockvoting.DataRoot(sorted[i])
		require.NoError(t, err)
		rj, err := blockvoting.DataRoot(sorted[i+1])
		require.NoError(t, err)
		require.True(t, bytes.Compare(ri[:], rj[:]) < 0, "sort[i] !< sort[i+1]")
	}
}

func TestArrStateOrder_EmptyVotes(t *testing.T) {
	root0 := [32]byte{}
	copy(root0[:], []byte("root-0--------------------------"))
	cand := twoHashBytes([32]byte{0x11, 0x11}, [32]byte{0x11, 0x22})

	bvs := []*types.BlockVoting{
		{Root: root0[:], Slot: 5, Candidates: cand, Votes: []*types.CommitteeVote{}},
		{Root: root0[:], Slot: 6, Candidates: cand, Votes: []*types.CommitteeVote{}},
		{Root: root0[:], Slot: 7, Candidates: cand, Votes: []*types.CommitteeVote{}},
	}
	sorted, err := blockvoting.ArrStateOrder(bvs)
	require.NoError(t, err)
	// Re-order is idempotent.
	again, err := blockvoting.ArrStateOrder(sorted)
	require.NoError(t, err)
	require.Equal(t, len(sorted), len(again))
}

func TestCountCommitteeVotes(t *testing.T) {
	votes := []*types.CommitteeVote{
		{AggregationBits: bitsOf(10, 0, 1, 2)},
		{AggregationBits: bitsOf(10, 4, 6)},
	}
	require.Equal(t, uint64(5), blockvoting.CountCommitteeVotes(votes))
}

func TestListRoot_LimitMattersForRoot(t *testing.T) {
	list := bitsOf(4, 0, 1)
	bv := &types.BlockVoting{
		Root:       bytes.Repeat([]byte{0xab}, 32),
		Slot:       42,
		Candidates: bytes.Repeat([]byte{0x01}, 32),
		Votes:      []*types.CommitteeVote{{Slot: 42, Index: 0, AggregationBits: list}},
	}
	r4, err := blockvoting.ListRoot([]*types.BlockVoting{bv}, 4)
	require.NoError(t, err)
	r8, err := blockvoting.ListRoot([]*types.BlockVoting{bv}, 8)
	require.NoError(t, err)
	require.NotEqual(t, r4, r8, "different list limits must produce different roots")
}
