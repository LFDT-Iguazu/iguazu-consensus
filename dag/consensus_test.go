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

package dag_test

import (
	"context"
	"fmt"
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LFDT-Iguazu/iguazu-consensus/dag"
	"github.com/LFDT-Iguazu/iguazu-consensus/helpers"
	"github.com/LFDT-Iguazu/iguazu-types/common"
	"github.com/LFDT-Iguazu/iguazu-types/coordinator/iface"
	"github.com/LFDT-Iguazu/iguazu-types/coordinator/types"
)

// ---- mock DagBeaconState ----------------------------------------------------

type testDagState struct {
	slot       types.Slot
	spineData  *types.SpineData
	finalized  *types.Checkpoint
	blockVote  []*types.BlockVoting
	candidates []byte
	validators []*types.Validator
	balances   []uint64
}

func newDagState() *testDagState {
	return &testDagState{
		spineData: &types.SpineData{},
		finalized: &types.Checkpoint{},
	}
}

func (s *testDagState) Slot() types.Slot                       { return s.slot }
func (s *testDagState) SpineData() *types.SpineData            { return s.spineData }
func (s *testDagState) BlockVoting() []*types.BlockVoting      { return s.blockVote }
func (s *testDagState) FinalizedCheckpoint() *types.Checkpoint { return s.finalized }
func (s *testDagState) CurrentJustifiedCheckpoint() *types.Checkpoint {
	return &types.Checkpoint{}
}
func (s *testDagState) PreviousJustifiedCheckpoint() *types.Checkpoint {
	return &types.Checkpoint{}
}
func (s *testDagState) NumValidators() int { return len(s.validators) }
func (s *testDagState) ValidatorAtIndex(idx types.ValidatorIndex) (*types.Validator, error) {
	if int(idx) >= len(s.validators) {
		return nil, fmt.Errorf("index out of range")
	}
	return s.validators[idx], nil
}
func (s *testDagState) ValidatorAtIndexReadOnly(idx types.ValidatorIndex) (*types.Validator, error) {
	return s.ValidatorAtIndex(idx)
}
func (s *testDagState) Balances() []uint64 { return s.balances }
func (s *testDagState) BalanceAtIndex(idx types.ValidatorIndex) (uint64, error) {
	if int(idx) >= len(s.balances) {
		return 0, fmt.Errorf("index out of range")
	}
	return s.balances[idx], nil
}
func (s *testDagState) SetSpineData(sd *types.SpineData) error       { s.spineData = sd; return nil }
func (s *testDagState) SetBlockVoting(bv []*types.BlockVoting) error { s.blockVote = bv; return nil }
func (s *testDagState) SetFinalizedCheckpoint(cp *types.Checkpoint) error {
	s.finalized = cp
	return nil
}
func (s *testDagState) SetCurrentJustifiedCheckpoint(*types.Checkpoint) error { return nil }
func (s *testDagState) UpdateValidatorAtIndex(idx types.ValidatorIndex, val *types.Validator) error {
	if int(idx) >= len(s.validators) {
		return fmt.Errorf("index out of range")
	}
	s.validators[idx] = val
	return nil
}
func (s *testDagState) UpdateBalancesAtIndex(idx types.ValidatorIndex, bal uint64) error {
	if int(idx) >= len(s.balances) {
		return fmt.Errorf("index out of range")
	}
	s.balances[idx] = bal
	return nil
}
func (s *testDagState) Eth1Candidates() []byte           { return s.candidates }
func (s *testDagState) SetEth1Candidates(b []byte) error { s.candidates = b; return nil }

// ---- mock iface.SignedBeaconBlock hierarchy ----------------------------------

type testAtt struct {
	root    []byte
	aggBits []byte
	slot    types.Slot
	idx     types.CommitteeIndex
}

func (a *testAtt) BeaconBlockRoot() []byte              { return a.root }
func (a *testAtt) AggregationBits() []byte              { return a.aggBits }
func (a *testAtt) AttestationSlot() types.Slot          { return a.slot }
func (a *testAtt) CommitteeIndex() types.CommitteeIndex { return a.idx }

type testBlockBody struct {
	atts       []iface.Attestation
	candidates []byte
}

func (b *testBlockBody) Attestations() []iface.Attestation { return b.atts }
func (b *testBlockBody) Eth1Candidates() []byte            { return b.candidates }

type testBlock struct {
	slot       types.Slot
	parentRoot []byte
	body       iface.BeaconBlockBody
}

func (b *testBlock) Slot() types.Slot            { return b.slot }
func (b *testBlock) ParentRoot() []byte          { return b.parentRoot }
func (b *testBlock) Body() iface.BeaconBlockBody { return b.body }

type testSignedBlock struct{ blk iface.BeaconBlock }

func (s *testSignedBlock) Block() iface.BeaconBlock { return s.blk }

// ---- mock iface.CommitteeCounter --------------------------------------------

type fixedCC struct{ count int }

func (c *fixedCC) CountSlotAttestors(_ context.Context, _ types.Slot) (int, error) {
	return c.count, nil
}

// ---- helpers ----------------------------------------------------------------

func h(b ...byte) common.Hash { return common.BytesToHash(b) }

func cfg(support int) *helpers.Config {
	return &helpers.Config{
		SlotsPerEpoch:                  32,
		FarFutureEpoch:                 types.Epoch(math.MaxUint64),
		SpinePublicationsPrefixSupport: support,
		VotingRequiredSlots:            2,
		BlockVotingMinSupportPrc:       50,
	}
}

func newBlock(slot types.Slot, parentRoot []byte, candidates []byte, atts []iface.Attestation) iface.SignedBeaconBlock {
	return &testSignedBlock{blk: &testBlock{
		slot:       slot,
		parentRoot: parentRoot,
		body: &testBlockBody{
			candidates: candidates,
			atts:       atts,
		},
	}}
}

// ---- tests ------------------------------------------------------------------

func TestCalcPrefixAndParentSpines_Empty(t *testing.T) {
	sd := &types.SpineData{}
	prefix, chains, err := dag.CalcPrefixAndParentSpines(sd, nil, cfg(2))
	require.NoError(t, err)
	assert.Empty(t, prefix)
	assert.Empty(t, chains)
}

func TestCalcPrefixAndParentSpines_SingleChainNoSupport(t *testing.T) {
	// SpinePublicationsPrefixSupport=2 with only 1 chain → no prefix extension.
	sd := &types.SpineData{}
	cands := common.HashArray{h('a'), h('b'), h('c')}.ToBytes()
	prefix, chains, err := dag.CalcPrefixAndParentSpines(sd, cands, cfg(2))
	require.NoError(t, err)
	assert.Empty(t, prefix)
	assert.Len(t, chains, 1)
}

func TestCalcPrefixAndParentSpines_SupportOne(t *testing.T) {
	// SpinePublicationsPrefixSupport=1 → single chain becomes full prefix.
	sd := &types.SpineData{}
	cands := common.HashArray{h('a'), h('b'), h('c')}.ToBytes()
	prefix, _, err := dag.CalcPrefixAndParentSpines(sd, cands, cfg(1))
	require.NoError(t, err)
	assert.Equal(t, common.HashArray{h('a'), h('b'), h('c')}, prefix)
}

func TestCalcPrefixAndParentSpines_WithParentChain(t *testing.T) {
	a, b, c, d := h('a'), h('b'), h('c'), h('d')
	// One parent chain [a, b, c]; new candidates [a, b, c, d].
	// After diff with parentPrefix=[], the parent chain starts with 'a' == spines[0].
	// resUnpubChains = [[a,b,c,d], [a,b,c]] → support=2 → common prefix = [a,b,c].
	sd := &types.SpineData{
		ParentSpines: []*types.SpinesSeq{
			{Spines: common.HashArray{a, b, c}.ToBytes()},
		},
	}
	cands := common.HashArray{a, b, c, d}.ToBytes()
	prefix, _, err := dag.CalcPrefixAndParentSpines(sd, cands, cfg(2))
	require.NoError(t, err)
	assert.Equal(t, common.HashArray{a, b, c}, prefix)
}

func TestCalcPrefixAndParentSpines_SubtractsFinalization(t *testing.T) {
	a, b, c := h('a'), h('b'), h('c')
	// Already finalized [a, b] must not appear in the resulting prefix.
	sd := &types.SpineData{
		Finalization: common.HashArray{a, b}.ToBytes(),
		ParentSpines: []*types.SpinesSeq{
			{Spines: common.HashArray{a, b, c}.ToBytes()},
		},
	}
	cands := common.HashArray{a, b, c}.ToBytes()
	prefix, _, err := dag.CalcPrefixAndParentSpines(sd, cands, cfg(2))
	require.NoError(t, err)
	assert.Equal(t, common.HashArray{c}, prefix)
}

func TestBlockVotingMinSupport_Basic(t *testing.T) {
	cc := &fixedCC{count: 100}
	c := &helpers.Config{BlockVotingMinSupportPrc: 50}
	ms, err := dag.BlockVotingMinSupport(context.Background(), newDagState(), 1, cc, c)
	require.NoError(t, err)
	// ceil(100/100*50)+1 = 51
	assert.Equal(t, 51, ms)
}

func TestBlockVotingMinSupport_SmallCommittee(t *testing.T) {
	cc := &fixedCC{count: 3}
	c := &helpers.Config{BlockVotingMinSupportPrc: 50}
	ms, err := dag.BlockVotingMinSupport(context.Background(), newDagState(), 1, cc, c)
	require.NoError(t, err)
	// ceil(3/100*50)+1 = ceil(1.5)+1 = 3; clamped to min(3, 3) = 3
	assert.Equal(t, 3, ms)
}

func TestProcessDagConsensus_NilStateError(t *testing.T) {
	blk := newBlock(1, []byte{0x01}, nil, nil)
	_, err := dag.ProcessDagConsensus(context.Background(), nil, blk, &fixedCC{}, cfg(2))
	require.Error(t, err)
}

func TestProcessDagConsensus_UpdatesSpineDataAndCandidates(t *testing.T) {
	a, b := h('a'), h('b')
	st := newDagState()
	st.slot = 10
	st.spineData = &types.SpineData{
		ParentSpines: []*types.SpinesSeq{
			{Spines: common.HashArray{a, b}.ToBytes()},
		},
	}

	cands := common.HashArray{a, b}.ToBytes()
	blk := newBlock(11, []byte{0x01}, cands, nil)

	c := cfg(2)
	post, err := dag.ProcessDagConsensus(context.Background(), st, blk, &fixedCC{count: 10}, c)
	require.NoError(t, err)

	assert.Equal(t, cands, post.Eth1Candidates())
	sd := post.SpineData()
	require.NotNil(t, sd)
	assert.Equal(t, cands, sd.Spines)
}

func TestProcessDagConsensus_AddsBlockVotingOnPrefix(t *testing.T) {
	// When prefix is non-empty and PrefixFinFork is active, a BlockVoting
	// entry must be added for the block's parent root.
	a, b := h('a'), h('b')
	st := newDagState()
	st.slot = 10
	st.spineData = &types.SpineData{
		ParentSpines: []*types.SpinesSeq{
			{Spines: common.HashArray{a, b}.ToBytes()},
		},
	}

	parentRoot := []byte{0xDE, 0xAD}
	blk := newBlock(11, parentRoot, common.HashArray{a, b}.ToBytes(), nil)

	c := cfg(1)             // support=1 → prefix = (a, b)
	c.PrefixFinForkSlot = 0 // always active

	post, err := dag.ProcessDagConsensus(context.Background(), st, blk, &fixedCC{count: 10}, c)
	require.NoError(t, err)

	bvs := post.BlockVoting()
	require.NotEmpty(t, bvs)

	found := false
	for _, bv := range bvs {
		if string(bv.Root) == string(parentRoot) {
			found = true
			// slot should be blockSlot - 1
			assert.Equal(t, types.Slot(10), bv.Slot)
		}
	}
	assert.True(t, found, "BlockVoting entry for parentRoot not found")
}

func TestProcessDagConsensus_AttestationsAppended(t *testing.T) {
	a := h('a')
	parentRoot := []byte{0x01}

	st := newDagState()
	st.slot = 10
	st.spineData = &types.SpineData{}
	// Seed a BlockVoting for parentRoot so appendBlockVotingAtt can find it.
	st.blockVote = []*types.BlockVoting{{
		Root:  parentRoot,
		Slot:  9,
		Votes: []*types.CommitteeVote{},
	}}

	// SSZ bitlist for 1 bit: byte 0x03 (bit 0 set + sentinel bit 1)
	aggBits := []byte{0x03}
	att := &testAtt{
		root:    parentRoot,
		aggBits: aggBits,
		slot:    9,
		idx:     0,
	}

	blk := newBlock(11, parentRoot, common.HashArray{a}.ToBytes(), []iface.Attestation{att})

	c := cfg(1)
	c.BlockVotingForkSlot = 0 // always active

	post, err := dag.ProcessDagConsensus(context.Background(), st, blk, &fixedCC{count: 10}, c)
	require.NoError(t, err)

	bvs := post.BlockVoting()
	require.NotEmpty(t, bvs)

	for _, bv := range bvs {
		if string(bv.Root) == string(parentRoot) {
			assert.NotEmpty(t, bv.Votes, "attestation should be added to BlockVoting")
		}
	}
}
