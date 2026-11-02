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

package helpers

import (
	"github.com/prysmaticlabs/go-bitfield"

	"github.com/LFDT-Iguazu/iguazu-types/coordinator/types"
)

// ComputeSubnetForPrevote returns the P2P subnet on which pv should be
// broadcast, based on the total active-validator count.
func ComputeSubnetForPrevote(activeValCount uint64, pv *types.PreVote, cfg *Config) uint64 {
	return ComputeSubnetPrevotingBySlot(activeValCount, pv.Data.Slot, cfg)
}

// IsAggregatedPrevote reports whether pv carries more than one aggregated vote.
func IsAggregatedPrevote(pv *types.PreVote) bool {
	return bitfield.Bitlist(pv.AggregationBits).Count() > 1
}

// ComputeSubnetPrevotingBySlot returns the prevote subnet number for the
// given slot.  Prevoting subnets occupy the range immediately after the
// attestation subnets: [AttestationSubnetCount, 2*AttestationSubnetCount).
func ComputeSubnetPrevotingBySlot(activeValCount uint64, slot types.Slot, cfg *Config) uint64 {
	// Prevoting uses a single committee index (0) per slot.
	var comIdx types.CommitteeIndex = 0
	computed := computeSubnetFromCommitteeAndSlot(activeValCount, comIdx, slot, cfg)
	return computed + cfg.AttestationSubnetCount
}

// computeSubnetFromCommitteeAndSlot mirrors
// coordinator/beacon-chain/core/helpers.ComputeSubnetFromCommitteeAndSlot.
func computeSubnetFromCommitteeAndSlot(
	activeValCount uint64,
	comIdx types.CommitteeIndex,
	slot types.Slot,
	cfg *Config,
) uint64 {
	slotSinceStart := uint64(slot.Mod(uint64(cfg.SlotsPerEpoch)))
	comCount := cfg.SlotCommitteeCount(activeValCount)
	commsSinceStart := slotSinceStart * comCount
	return (commsSinceStart + uint64(comIdx)) % cfg.AttestationSubnetCount
}
