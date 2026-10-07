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

package helpers_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LFDT-Iguazu/iguazu-consensus/helpers"
	"github.com/LFDT-Iguazu/iguazu-types/common"
	"github.com/LFDT-Iguazu/iguazu-types/coordinator/types"
)

// ---- mock BeaconState -------------------------------------------------------

type testState struct {
	slot        types.Slot
	spineData   *types.SpineData
	finalized   *types.Checkpoint
	justified   *types.Checkpoint
	prevJust    *types.Checkpoint
	validators  []*types.Validator
	balances    []uint64
	blockVoting []*types.BlockVoting
}

func newTestState() *testState {
	return &testState{
		spineData: &types.SpineData{},
		finalized: &types.Checkpoint{},
		justified: &types.Checkpoint{},
		prevJust:  &types.Checkpoint{},
	}
}

func (s *testState) Slot() types.Slot                       { return s.slot }
func (s *testState) SpineData() *types.SpineData            { return s.spineData }
func (s *testState) BlockVoting() []*types.BlockVoting      { return s.blockVoting }
func (s *testState) FinalizedCheckpoint() *types.Checkpoint { return s.finalized }
func (s *testState) CurrentJustifiedCheckpoint() *types.Checkpoint {
	return s.justified
}
func (s *testState) PreviousJustifiedCheckpoint() *types.Checkpoint {
	return s.prevJust
}
func (s *testState) NumValidators() int { return len(s.validators) }
func (s *testState) ValidatorAtIndex(idx types.ValidatorIndex) (*types.Validator, error) {
	if int(idx) >= len(s.validators) {
		return nil, fmt.Errorf("index %d out of range", idx)
	}
	return s.validators[idx], nil
}
func (s *testState) ValidatorAtIndexReadOnly(idx types.ValidatorIndex) (*types.Validator, error) {
	return s.ValidatorAtIndex(idx)
}
func (s *testState) Balances() []uint64 { return s.balances }
func (s *testState) BalanceAtIndex(idx types.ValidatorIndex) (uint64, error) {
	if int(idx) >= len(s.balances) {
		return 0, fmt.Errorf("index %d out of range", idx)
	}
	return s.balances[idx], nil
}
func (s *testState) SetSpineData(sd *types.SpineData) error {
	s.spineData = sd
	return nil
}
func (s *testState) SetBlockVoting(bv []*types.BlockVoting) error {
	s.blockVoting = bv
	return nil
}
func (s *testState) SetFinalizedCheckpoint(cp *types.Checkpoint) error {
	s.finalized = cp
	return nil
}
func (s *testState) SetCurrentJustifiedCheckpoint(cp *types.Checkpoint) error {
	s.justified = cp
	return nil
}
func (s *testState) UpdateValidatorAtIndex(idx types.ValidatorIndex, val *types.Validator) error {
	if int(idx) >= len(s.validators) {
		return fmt.Errorf("index %d out of range", idx)
	}
	s.validators[idx] = val
	return nil
}
func (s *testState) UpdateBalancesAtIndex(idx types.ValidatorIndex, balance uint64) error {
	if int(idx) >= len(s.balances) {
		return fmt.Errorf("index %d out of range", idx)
	}
	s.balances[idx] = balance
	return nil
}

// ---- helpers ----------------------------------------------------------------

func h(b ...byte) common.Hash { return common.BytesToHash(b) }

// ---- tests ------------------------------------------------------------------

func TestValidateUnpublishedChains(t *testing.T) {
	chain1 := common.HashArray{h('a', '1')}
	chain2 := common.HashArray{h('a', '1'), h('b', '2')}
	chain3 := common.HashArray{h('a', '1'), h('a', '2')}

	require.NoError(t, helpers.ConsensusValidateUnpublishedChains([]common.HashArray{chain3, chain2, chain1}))

	err := helpers.ConsensusValidateUnpublishedChains([]common.HashArray{chain3, {}, chain1})
	require.Error(t, err)
	assert.ErrorIs(t, err, helpers.ErrBadUnpublishedChains)
	assert.Contains(t, err.Error(), "contains empty chain")

	chain4 := common.HashArray{h('c', '1'), h('a', '2')}
	err = helpers.ConsensusValidateUnpublishedChains([]common.HashArray{chain4, chain3, chain2, chain1})
	require.Error(t, err)
	assert.ErrorIs(t, err, helpers.ErrBadUnpublishedChains)
	assert.Contains(t, err.Error(), "the first values of chain are not equal")

	chain5 := common.HashArray{h('a', '1'), h('d', '2'), h('d', '2')}
	err = helpers.ConsensusValidateUnpublishedChains([]common.HashArray{chain5, chain3, chain2, chain1})
	require.Error(t, err)
	assert.ErrorIs(t, err, helpers.ErrBadUnpublishedChains)
	assert.Contains(t, err.Error(), "chain is not uniq")
}

func TestConsensusCalcPrefix(t *testing.T) {
	cfg := &helpers.Config{SpinePublicationsPrefixSupport: 2}

	chain1 := common.HashArray{h('a', '1')}
	chain2 := common.HashArray{h('a', '1'), h('b', '2')}
	chain3 := common.HashArray{h('a', '1'), h('a', '2')}

	prefix, err := helpers.ConsensusCalcPrefix([]common.HashArray{chain3, chain2, chain1}, cfg)
	require.NoError(t, err)
	want := common.HashArray{h('a', '1')}
	assert.Equal(t, fmt.Sprintf("%v", want), fmt.Sprintf("%v", prefix))
}

func TestConsensusCalcPrefix_AllEqual(t *testing.T) {
	cfg := &helpers.Config{SpinePublicationsPrefixSupport: 2}

	a1, a2, a3, a4 := h('a', '1'), h('a', '2'), h('a', '3'), h('a', '4')
	b3 := h('b', '3')

	chain1 := common.HashArray{a1, a2, a3, a4}
	chain2 := common.HashArray{a1, a2, a3, a4}
	chain3 := common.HashArray{a1, a2, a3, a4}
	chain4 := common.HashArray{a1, a2, b3, a4}

	prefix, err := helpers.ConsensusCalcPrefix([]common.HashArray{chain4, chain3, chain2, chain1}, cfg)
	require.NoError(t, err)
	want := common.HashArray{a1, a2}
	assert.Equal(t, fmt.Sprintf("%v", want), fmt.Sprintf("%v", prefix))
}

func TestProcessWithdrawalOps(t *testing.T) {
	const (
		slotsPerEpoch    = types.Slot(32)
		cleanAfterEpochs = types.Epoch(2)
		delegateForkSlot = types.Slot(0)
		staleAfterSlots  = types.Slot(cleanAfterEpochs) * slotsPerEpoch // 64
	)
	cfg := &helpers.Config{
		DelegateForkSlot:          delegateForkSlot,
		SlotsPerEpoch:             slotsPerEpoch,
		CleanWithdrawalsAftEpochs: cleanAfterEpochs,
	}

	var finRoot [32]byte
	finRoot[0] = 0x22

	preFinRoot := make([]byte, 32)
	preFinRoot[0] = 0x11

	st := newTestState()
	st.slot = 10 + staleAfterSlots
	st.finalized = &types.Checkpoint{Root: finRoot}
	st.validators = []*types.Validator{
		{
			WithdrawalOps: []*types.WithdrawalOp{
				{Slot: 5},
				{Slot: 10},
				{Slot: 11},
				{Slot: 55},
			},
		},
	}

	// minSlot = slot - staleAfterSlots = 10
	// ops with Slot <= 10 are stale: {5}, {10}
	// ops with Slot > 10 remain: {11}, {55}
	postState, err := helpers.ProcessWithdrawalOps(st, cfg, preFinRoot)
	require.NoError(t, err)
	val, err := postState.ValidatorAtIndex(0)
	require.NoError(t, err)
	require.Len(t, val.WithdrawalOps, 2)
	assert.Equal(t, types.Slot(11), val.WithdrawalOps[0].Slot)
	assert.Equal(t, types.Slot(55), val.WithdrawalOps[1].Slot)

	// advance slot so all ops are stale
	st.slot = 100 + staleAfterSlots
	postState, err = helpers.ProcessWithdrawalOps(st, cfg, preFinRoot)
	require.NoError(t, err)
	val, err = postState.ValidatorAtIndex(0)
	require.NoError(t, err)
	assert.Empty(t, val.WithdrawalOps)
}

func TestProcessWithdrawalOps_SkipBeforeFork(t *testing.T) {
	cfg := &helpers.Config{
		DelegateForkSlot:          types.Slot(1000),
		SlotsPerEpoch:             32,
		CleanWithdrawalsAftEpochs: 2,
	}
	st := newTestState()
	st.slot = 5
	st.finalized = &types.Checkpoint{Root: [32]byte{0x22}}
	st.validators = []*types.Validator{{WithdrawalOps: []*types.WithdrawalOp{{Slot: 1}, {Slot: 2}}}}

	postState, err := helpers.ProcessWithdrawalOps(st, cfg, []byte{0x11})
	require.NoError(t, err)
	val, _ := postState.ValidatorAtIndex(0)
	// no changes — slot is before DelegateForkSlot
	assert.Len(t, val.WithdrawalOps, 2)
}

func TestProcessWithdrawalOps_SameFin(t *testing.T) {
	cfg := &helpers.Config{
		DelegateForkSlot:          0,
		SlotsPerEpoch:             32,
		CleanWithdrawalsAftEpochs: 2,
	}
	var root [32]byte
	root[0] = 0x22
	st := newTestState()
	st.slot = 100
	st.finalized = &types.Checkpoint{Root: root}
	st.validators = []*types.Validator{{WithdrawalOps: []*types.WithdrawalOp{{Slot: 1}}}}

	// preFinRoot == finRoot → no processing
	postState, err := helpers.ProcessWithdrawalOps(st, cfg, root[:])
	require.NoError(t, err)
	val, _ := postState.ValidatorAtIndex(0)
	assert.Len(t, val.WithdrawalOps, 1)
}
