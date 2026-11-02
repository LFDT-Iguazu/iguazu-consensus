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

package withdrawal

import (
	"bytes"
	"fmt"
	"log/slog"
	"math"

	"github.com/pkg/errors"

	"github.com/LFDT-Iguazu/iguazu-consensus/helpers"
	"github.com/LFDT-Iguazu/iguazu-types/coordinator/iface"
	"github.com/LFDT-Iguazu/iguazu-types/coordinator/types"
)

var (
	ErrWithdrawalIsNil             = errors.New("nil withdrawal")
	ErrWithdrawalBadValidatorIndex = errors.New("withdrawal bad ValidatorIndex")
	ErrWithdrawalBadPublicKey      = errors.New("withdrawal bad PublicKey")
	ErrWithdrawalBadEpoch          = errors.New("withdrawal bad Epoch")
	ErrWithdrawalLowBalance        = errors.New("withdrawal low balance")
	ErrWithdrawalBadAmount         = errors.New("withdrawal bad amount")
	ErrWithdrawalAlreadyApplied    = errors.New("withdrawal already applied")
)

// ProcessWithdrawal applies each withdrawal operation in the list to beaconState.
// It mirrors coordinator/beacon-chain/core/blocks.ProcessWithdrawal but
// uses iguazu-types and iface instead of Prysm concrete types.
func ProcessWithdrawal(
	beaconState iface.BeaconState,
	withdrawals []*types.Withdrawal,
	cfg *helpers.Config,
) (iface.BeaconState, error) {
	for _, w := range withdrawals {
		if w == nil {
			return nil, ErrWithdrawalIsNil
		}
		roVal, err := beaconState.ValidatorAtIndexReadOnly(w.ValidatorIndex)
		if err != nil {
			return nil, err
		}
		available, err := helpers.AvailableWithdrawalAmount(w.ValidatorIndex, beaconState, cfg)
		if err != nil {
			return nil, err
		}
		if err = verifyWithdrawalData(w, roVal, beaconState.Slot(), available, cfg); err != nil {
			slog.Error("Withdrawal transition: validation failed",
				"err", err,
				"VIndex", fmt.Sprintf("%d", w.ValidatorIndex),
				"PublicKey", fmt.Sprintf("%#x", w.PublicKey),
				"Epoch", fmt.Sprintf("%d", w.Epoch),
				"Amount", fmt.Sprintf("%d", w.Amount),
				"InitTxHash", fmt.Sprintf("%#x", w.InitTxHash),
				"availableBalance", available,
			)
			if errors.Is(err, ErrWithdrawalAlreadyApplied) {
				return beaconState, nil
			}
			return nil, errors.Wrapf(err, "withdrawal verify %d", w.ValidatorIndex)
		}

		amount := w.Amount
		if amount == 0 {
			amount = available
		}

		if err = applyWithdrawal(beaconState, w.ValidatorIndex, amount); err != nil {
			return nil, err
		}

		upVal, err := beaconState.ValidatorAtIndex(w.ValidatorIndex)
		if err != nil {
			return nil, err
		}
		if upVal.WithdrawalOps == nil {
			upVal.WithdrawalOps = make([]*types.WithdrawalOp, 0, 1)
		}
		if uint64(len(upVal.WithdrawalOps)) >= cfg.WithdrawalOpsLimit {
			upVal.WithdrawalOps = upVal.WithdrawalOps[1:]
		}
		upVal.WithdrawalOps = append(upVal.WithdrawalOps, &types.WithdrawalOp{
			Amount: amount,
			Hash:   append([]byte(nil), w.InitTxHash...),
			Slot:   beaconState.Slot(),
		})
		if err = beaconState.UpdateValidatorAtIndex(w.ValidatorIndex, upVal); err != nil {
			return nil, err
		}

		slog.Info("Withdrawal transition: success",
			"stSlot", fmt.Sprintf("%d", beaconState.Slot()),
			"VIndex", fmt.Sprintf("%d", w.ValidatorIndex),
			"PublicKey", fmt.Sprintf("%#x", w.PublicKey),
			"Epoch", fmt.Sprintf("%d", w.Epoch),
			"Amount", fmt.Sprintf("%d", w.Amount),
			"InitTxHash", fmt.Sprintf("%#x", w.InitTxHash),
			"availableBalance", available,
			"WithdrawalOps", len(upVal.WithdrawalOps),
		)
	}
	return beaconState, nil
}

// verifyWithdrawalData checks that a withdrawal is structurally valid and
// consistent with the current validator record and state.
func verifyWithdrawalData(
	w *types.Withdrawal,
	val *types.Validator,
	currentSlot types.Slot,
	availableBalance uint64,
	cfg *helpers.Config,
) error {
	if w == nil {
		return ErrWithdrawalIsNil
	}
	if w.ValidatorIndex == math.MaxUint64 {
		return ErrWithdrawalBadValidatorIndex
	}
	if !bytes.Equal(w.PublicKey, val.PublicKey) {
		return ErrWithdrawalBadPublicKey
	}

	currentEpoch := cfg.SlotToEpoch(currentSlot)
	if w.Epoch > currentEpoch {
		return ErrWithdrawalBadEpoch
	}

	// Refund of insufficient deposit: amount must be explicitly set.
	if val.ActivationEligibilityEpoch == cfg.FarFutureEpoch &&
		availableBalance < cfg.MaxEffectiveBalance {
		if w.Amount == 0 {
			return ErrWithdrawalBadAmount
		}
	}

	if availableBalance < w.Amount {
		return ErrWithdrawalLowBalance
	}

	for _, op := range val.WithdrawalOps {
		if bytes.Equal(op.Hash, w.InitTxHash) {
			return ErrWithdrawalAlreadyApplied
		}
	}
	return nil
}

func applyWithdrawal(state iface.BeaconState, idx types.ValidatorIndex, delta uint64) error {
	bal, err := state.BalanceAtIndex(idx)
	if err != nil {
		return err
	}
	return state.UpdateBalancesAtIndex(idx, helpers.DecreaseBalance(bal, delta))
}
