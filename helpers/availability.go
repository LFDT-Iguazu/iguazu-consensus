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
	"github.com/LFDT-Iguazu/iguazu-types/coordinator/iface"
	"github.com/LFDT-Iguazu/iguazu-types/coordinator/types"
)

// AvailableWithdrawalAmount returns how many Gwei the validator at idx can
// withdraw right now.  The rules mirror coordinator/helpers.AvailableWithdrawalAmount:
//
//   - Not-yet-active validators (ActivationEligibilityEpoch == FarFutureEpoch)
//     with balance < MaxEffectiveBalance may withdraw their entire balance.
//   - Deactivated validators may withdraw their entire balance.
//   - Active validators with balance > MaxEffectiveBalance may withdraw the
//     excess; everyone else is blocked.
func AvailableWithdrawalAmount(idx types.ValidatorIndex, st iface.BeaconState, cfg *Config) (uint64, error) {
	bal, err := st.BalanceAtIndex(idx)
	if err != nil {
		return 0, err
	}
	if bal == 0 {
		return 0, nil
	}
	val, err := st.ValidatorAtIndexReadOnly(idx)
	if err != nil {
		return 0, err
	}
	if val == nil {
		return 0, nil
	}

	currentEpoch := cfg.SlotToEpoch(st.Slot())

	// Refund of insufficient deposit — validator never activated.
	if val.ActivationEligibilityEpoch == cfg.FarFutureEpoch && bal < cfg.MaxEffectiveBalance {
		return bal, nil
	}
	// Deactivated validator — withdraw everything.
	if val.ExitEpoch <= currentEpoch {
		return bal, nil
	}
	// Active validator — only the balance above MaxEffectiveBalance.
	if bal <= cfg.MaxEffectiveBalance {
		return 0, nil
	}
	return bal - cfg.MaxEffectiveBalance, nil
}

// DecreaseBalance subtracts delta from currBalance, clamping at zero.
func DecreaseBalance(currBalance, delta uint64) uint64 {
	if delta > currBalance {
		return 0
	}
	return currBalance - delta
}
