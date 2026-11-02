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

import "github.com/LFDT-Iguazu/iguazu-types/coordinator/types"

// Config carries the chain-configuration constants that helpers in this
// package consume.  It is populated by the coordinator or iguazu-coordinator
// at startup from params.BeaconConfig / params.BeaconNetworkConfig.
type Config struct {
	// DelegateForkSlot is the first slot at which withdrawal-op processing
	// is active (mirrors BeaconChainConfig.DelegateForkSlot).
	DelegateForkSlot types.Slot

	// SlotsPerEpoch is the number of slots in one epoch.
	SlotsPerEpoch types.Slot

	// CleanWithdrawalsAftEpochs is how many epochs of withdrawal-op history
	// to retain; older entries are pruned by ProcessWithdrawalOps.
	CleanWithdrawalsAftEpochs types.Epoch

	// SpinePublicationsPrefixSupport is the minimum number of publications a
	// spine must have across unpublished chains before it is included in the
	// new prefix (mirrors BeaconChainConfig.SpinePublicationsPefixSupport).
	SpinePublicationsPrefixSupport int

	// AttestationSubnetCount is the total number of attestation subnets.
	// Prevoting subnets start immediately after this range.
	AttestationSubnetCount uint64

	// TargetCommitteeSize is the target validator count per committee.
	TargetCommitteeSize uint64

	// MaxCommitteesPerSlot caps the number of committees per slot.
	MaxCommitteesPerSlot uint64

	// --- Withdrawal processor -----------------------------------------------

	// FarFutureEpoch is the sentinel epoch value for validators that have not
	// yet scheduled an exit or activation (math.MaxUint64).
	FarFutureEpoch types.Epoch

	// MaxEffectiveBalance is the maximum balance (in Gwei) that counts toward
	// a validator's effective stake.
	MaxEffectiveBalance uint64

	// WithdrawalOpsLimit caps the number of WithdrawalOp entries kept per
	// validator.  The oldest entry is evicted when the limit is reached.
	WithdrawalOpsLimit uint64

	// --- DAG-consensus processor --------------------------------------------

	// VotingRequiredSlots is the minimum number of block-voting entries that
	// must agree on a finalization candidate for it to be accepted.
	VotingRequiredSlots int

	// BlockVotingMinSupportPrc is the percentage (0-100) of committee
	// attestors that must vote for a candidate to count as "supported".
	BlockVotingMinSupportPrc int

	// PrefixFinForkSlot is the first slot for which the prefix-finalization
	// fork rules apply.
	PrefixFinForkSlot types.Slot

	// BlockVotingForkSlot is the first slot for which the block-voting fork
	// rules apply.
	BlockVotingForkSlot types.Slot

	// FcTgTreeForkSlot is the first slot for which the improved T(G)-tree
	// fork-choice algorithm applies (mirrors BeaconChainConfig.FcTgTreeForkSlot).
	FcTgTreeForkSlot types.Slot
}

// IsDelegatingStakeSlot returns true when withdrawal-op processing should
// run for the given slot.
func (c *Config) IsDelegatingStakeSlot(slot types.Slot) bool {
	return c.DelegateForkSlot <= slot
}

// IsPrefixFinForkSlot returns true when the prefix-finalization fork is active.
func (c *Config) IsPrefixFinForkSlot(slot types.Slot) bool {
	return c.PrefixFinForkSlot <= slot
}

// IsBlockVotingForkSlot returns true when the block-voting fork is active.
func (c *Config) IsBlockVotingForkSlot(slot types.Slot) bool {
	return c.BlockVotingForkSlot <= slot
}

// IsFcTgTreeForkSlot returns true when the improved T(G)-tree fork-choice
// algorithm should be used for the given slot.
func (c *Config) IsFcTgTreeForkSlot(slot types.Slot) bool {
	return c.FcTgTreeForkSlot <= slot
}

// SlotToEpoch converts a slot number to an epoch number.
func (c *Config) SlotToEpoch(slot types.Slot) types.Epoch {
	return types.Epoch(uint64(slot) / uint64(c.SlotsPerEpoch))
}

// EpochStartSlot returns the first slot of the given epoch.
func (c *Config) EpochStartSlot(epoch types.Epoch) types.Slot {
	return types.Slot(uint64(epoch) * uint64(c.SlotsPerEpoch))
}

// IsEpochStart reports whether slot is the first slot of its epoch.
func (c *Config) IsEpochStart(slot types.Slot) bool {
	return uint64(slot)%uint64(c.SlotsPerEpoch) == 0
}

// SlotCommitteeCount returns the number of crosslink committees per slot for
// the given active-validator count, clamped to [1, MaxCommitteesPerSlot].
func (c *Config) SlotCommitteeCount(activeValidatorCount uint64) uint64 {
	count := activeValidatorCount / uint64(c.SlotsPerEpoch) / c.TargetCommitteeSize
	if count > c.MaxCommitteesPerSlot {
		return c.MaxCommitteesPerSlot
	}
	if count == 0 {
		return 1
	}
	return count
}
