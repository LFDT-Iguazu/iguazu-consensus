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

package withdrawal_test

import (
	"fmt"
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LFDT-Iguazu/iguazu-consensus/helpers"
	"github.com/LFDT-Iguazu/iguazu-consensus/withdrawal"
	"github.com/LFDT-Iguazu/iguazu-types/coordinator/types"
)

// ---- minimal BeaconState mock -----------------------------------------------

type testState struct {
	slot       types.Slot
	validators []*types.Validator
	balances   []uint64
}

func (s *testState) Slot() types.Slot                               { return s.slot }
func (s *testState) SpineData() *types.SpineData                    { return &types.SpineData{} }
func (s *testState) BlockVoting() []*types.BlockVoting              { return nil }
func (s *testState) FinalizedCheckpoint() *types.Checkpoint         { return &types.Checkpoint{} }
func (s *testState) CurrentJustifiedCheckpoint() *types.Checkpoint  { return &types.Checkpoint{} }
func (s *testState) PreviousJustifiedCheckpoint() *types.Checkpoint { return &types.Checkpoint{} }
func (s *testState) NumValidators() int                             { return len(s.validators) }
func (s *testState) ValidatorAtIndex(idx types.ValidatorIndex) (*types.Validator, error) {
	if int(idx) >= len(s.validators) {
		return nil, fmt.Errorf("index out of range")
	}
	return s.validators[idx], nil
}
func (s *testState) ValidatorAtIndexReadOnly(idx types.ValidatorIndex) (*types.Validator, error) {
	return s.ValidatorAtIndex(idx)
}
func (s *testState) Balances() []uint64 { return s.balances }
func (s *testState) BalanceAtIndex(idx types.ValidatorIndex) (uint64, error) {
	if int(idx) >= len(s.balances) {
		return 0, fmt.Errorf("index out of range")
	}
	return s.balances[idx], nil
}
func (s *testState) SetSpineData(*types.SpineData) error                   { return nil }
func (s *testState) SetBlockVoting([]*types.BlockVoting) error             { return nil }
func (s *testState) SetFinalizedCheckpoint(*types.Checkpoint) error        { return nil }
func (s *testState) SetCurrentJustifiedCheckpoint(*types.Checkpoint) error { return nil }
func (s *testState) UpdateValidatorAtIndex(idx types.ValidatorIndex, val *types.Validator) error {
	if int(idx) >= len(s.validators) {
		return fmt.Errorf("index out of range")
	}
	s.validators[idx] = val
	return nil
}
func (s *testState) UpdateBalancesAtIndex(idx types.ValidatorIndex, bal uint64) error {
	if int(idx) >= len(s.balances) {
		return fmt.Errorf("index out of range")
	}
	s.balances[idx] = bal
	return nil
}

// ---- helpers ----------------------------------------------------------------

const (
	maxEffective   = uint64(32_000_000_000) // 32 ETH in Gwei
	farFutureEpoch = types.Epoch(math.MaxUint64)
)

func cfg() *helpers.Config {
	return &helpers.Config{
		SlotsPerEpoch:       32,
		FarFutureEpoch:      farFutureEpoch,
		MaxEffectiveBalance: maxEffective,
		WithdrawalOpsLimit:  16,
	}
}

func pubKey(b byte) []byte {
	pk := make([]byte, 48)
	pk[0] = b
	return pk
}

// ---- tests ------------------------------------------------------------------

func TestProcessWithdrawal_Success(t *testing.T) {
	pk := pubKey(0xAA)
	st := &testState{
		slot: 100,
		validators: []*types.Validator{{
			PublicKey:                  pk,
			ActivationEligibilityEpoch: 0,
			ActivationEpoch:            0,
			ExitEpoch:                  farFutureEpoch,
			WithdrawableEpoch:          farFutureEpoch,
		}},
		balances: []uint64{maxEffective + 1_000_000_000}, // 1 ETH above max
	}

	w := &types.Withdrawal{
		PublicKey:      pk,
		ValidatorIndex: 0,
		Amount:         500_000_000,
		InitTxHash:     []byte{0x01},
		Epoch:          0,
	}

	post, err := withdrawal.ProcessWithdrawal(st, []*types.Withdrawal{w}, cfg())
	require.NoError(t, err)

	bal, _ := post.BalanceAtIndex(0)
	assert.Equal(t, maxEffective+1_000_000_000-500_000_000, bal)

	val, _ := post.ValidatorAtIndex(0)
	require.Len(t, val.WithdrawalOps, 1)
	assert.Equal(t, uint64(500_000_000), val.WithdrawalOps[0].Amount)
}

func TestProcessWithdrawal_AlreadyApplied(t *testing.T) {
	pk := pubKey(0xBB)
	txHash := []byte{0x42}
	st := &testState{
		slot: 100,
		validators: []*types.Validator{{
			PublicKey:     pk,
			ExitEpoch:     farFutureEpoch,
			WithdrawalOps: []*types.WithdrawalOp{{Hash: txHash}},
		}},
		balances: []uint64{maxEffective + 2_000_000_000},
	}

	w := &types.Withdrawal{
		PublicKey:      pk,
		ValidatorIndex: 0,
		Amount:         1_000_000_000,
		InitTxHash:     txHash,
		Epoch:          0,
	}

	post, err := withdrawal.ProcessWithdrawal(st, []*types.Withdrawal{w}, cfg())
	require.NoError(t, err)
	// already applied → state is returned unchanged
	bal, _ := post.BalanceAtIndex(0)
	assert.Equal(t, maxEffective+2_000_000_000, bal)
}

func TestProcessWithdrawal_BadPublicKey(t *testing.T) {
	st := &testState{
		slot: 100,
		validators: []*types.Validator{{
			PublicKey: pubKey(0xCC),
			ExitEpoch: farFutureEpoch,
		}},
		balances: []uint64{maxEffective + 1_000_000_000},
	}

	w := &types.Withdrawal{
		PublicKey:      pubKey(0xDD), // wrong
		ValidatorIndex: 0,
		Amount:         100,
		InitTxHash:     []byte{0x01},
	}

	_, err := withdrawal.ProcessWithdrawal(st, []*types.Withdrawal{w}, cfg())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "PublicKey")
}

func TestProcessWithdrawal_LowBalance(t *testing.T) {
	pk := pubKey(0xEE)
	st := &testState{
		slot: 100,
		validators: []*types.Validator{{
			PublicKey: pk,
			ExitEpoch: farFutureEpoch,
		}},
		balances: []uint64{maxEffective}, // no excess
	}

	w := &types.Withdrawal{
		PublicKey:      pk,
		ValidatorIndex: 0,
		Amount:         1,
		InitTxHash:     []byte{0x01},
	}

	_, err := withdrawal.ProcessWithdrawal(st, []*types.Withdrawal{w}, cfg())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "low balance")
}

func TestProcessWithdrawal_WithdrawalOpsLimitEviction(t *testing.T) {
	pk := pubKey(0xFF)
	// Fill ops to limit
	ops := make([]*types.WithdrawalOp, 16)
	for i := range ops {
		ops[i] = &types.WithdrawalOp{Hash: []byte{byte(i)}}
	}
	st := &testState{
		slot: 100,
		validators: []*types.Validator{{
			PublicKey:     pk,
			ExitEpoch:     types.Epoch(0), // deactivated → can withdraw all
			WithdrawalOps: ops,
		}},
		balances: []uint64{5_000_000_000},
	}

	w := &types.Withdrawal{
		PublicKey:      pk,
		ValidatorIndex: 0,
		Amount:         1_000_000_000,
		InitTxHash:     []byte{0xFF},
	}

	post, err := withdrawal.ProcessWithdrawal(st, []*types.Withdrawal{w}, cfg())
	require.NoError(t, err)
	val, _ := post.ValidatorAtIndex(0)
	// oldest entry should have been evicted; length stays at limit
	assert.Len(t, val.WithdrawalOps, 16)
	// last entry is the new one
	assert.Equal(t, []byte{0xFF}, val.WithdrawalOps[len(val.WithdrawalOps)-1].Hash)
}
