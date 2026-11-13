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
	"bytes"
	"fmt"
	"log/slog"
	"math/big"

	"github.com/pkg/errors"

	"github.com/LFDT-Iguazu/iguazu-types/common"
	"github.com/LFDT-Iguazu/iguazu-types/coordinator/iface"
	"github.com/LFDT-Iguazu/iguazu-types/coordinator/types"
)

// ErrBadUnpublishedChains is returned when the unpublished-chains slice
// fails structural validation.
var ErrBadUnpublishedChains = errors.New("bad unpublished chains")

type mapPublications map[common.Hash]int

// ConsensusUpdateStateSpineFinalization updates SpineData when the
// justified or finalized checkpoint advances.  preJustRoot / preFinRoot are
// the checkpoint roots recorded before the current epoch processing round.
func ConsensusUpdateStateSpineFinalization(
	beaconState iface.BeaconState,
	preJustRoot, preFinRoot []byte,
) (iface.BeaconState, error) {
	finRoot := beaconState.FinalizedCheckpoint().Root[:]
	justRoot := beaconState.CurrentJustifiedCheckpoint().Root[:]

	if bytes.Equal(justRoot, preJustRoot) {
		return beaconState, nil
	}
	cpFinalized := beaconState.SpineData().CpFinalized
	finalization := beaconState.SpineData().Finalization
	if bytes.Equal(finRoot, preFinRoot) {
		cpFinalized = append(cpFinalized, finalization...)
		finalization = []byte{}
	} else {
		cpFinalized = append(cpFinalized[len(cpFinalized)-32:], finalization...)
		finalization = []byte{}
	}
	spineData := beaconState.SpineData()
	spineData.Finalization = finalization
	spineData.CpFinalized = cpFinalized
	err := beaconState.SetSpineData(spineData)
	return beaconState, err
}

// ProcessWithdrawalOps removes stale WithdrawalOps from every validator
// when the finalized checkpoint advances and the delegate-fork slot is active.
func ProcessWithdrawalOps(bState iface.BeaconState, cfg *Config, preFinRoot []byte) (iface.BeaconState, error) {
	if !cfg.IsDelegatingStakeSlot(bState.Slot()) {
		return bState, nil
	}
	finRoot := bState.FinalizedCheckpoint().Root[:]
	if bytes.Equal(finRoot, preFinRoot) {
		return bState, nil
	}

	staleAfterSlots := types.Slot(cfg.CleanWithdrawalsAftEpochs) * cfg.SlotsPerEpoch
	var minSlot types.Slot
	if bState.Slot() > staleAfterSlots {
		minSlot = bState.Slot() - staleAfterSlots
	}

	n := bState.NumValidators()
	for i := 0; i < n; i++ {
		idx := types.ValidatorIndex(i)
		val, err := bState.ValidatorAtIndex(idx)
		if err != nil {
			return bState, err
		}
		var upWops []*types.WithdrawalOp
		for j, wop := range val.WithdrawalOps {
			if j == 0 && wop.Slot > minSlot {
				break
			}
			if wop.Slot <= minSlot {
				if upWops == nil {
					upWops = make([]*types.WithdrawalOp, 0, len(val.WithdrawalOps)-1)
				}
				slog.Info("WithdrawalOps transition: rm stale (epoch proc)",
					"rm", wop.Slot < minSlot,
					"bState.Slot", fmt.Sprintf("%d", bState.Slot()),
					"w.Slot", fmt.Sprintf("%d", wop.Slot),
					"w.Amount", fmt.Sprintf("%d", wop.Amount),
					"w.Hash", fmt.Sprintf("%#x", wop.Hash),
					"val.Index", fmt.Sprintf("%d", idx),
					"val.PublicKey", fmt.Sprintf("%#x", val.PublicKey),
				)
			} else if upWops != nil {
				upWops = append(upWops, wop)
			}
		}
		if upWops != nil {
			newVal := val.Copy()
			newVal.WithdrawalOps = upWops
			if err := bState.UpdateValidatorAtIndex(idx, newVal); err != nil {
				return bState, err
			}
		}
	}
	return bState, nil
}

// CalculateCandidates derives the next set of candidate spines from
// optimistic spines, relative to the terminal spine in parentState.
func CalculateCandidates(parentState iface.BeaconState, optSpines []common.HashArray) common.HashArray {
	var terminalSpine common.Hash
	sd := parentState.SpineData()
	if len(sd.Prefix) > 0 {
		terminalSpine = common.BytesToHash(sd.Prefix[len(sd.Prefix)-common.HashLength:])
	} else {
		terminalSpine = GetTerminalFinalizedSpine(parentState)
	}
	candidates := make(common.HashArray, 0, len(optSpines))
	for _, spineList := range optSpines {
		if spineList.Has(terminalSpine) {
			candidates = make(common.HashArray, 0, len(optSpines))
			continue
		}
		if len(spineList) > 0 {
			candidates = append(candidates, spineList[0])
		}
	}
	return candidates
}

// GetTerminalFinalizedSpine returns the last spine in the finalization or
// cp-finalized sequence stored in beaconState.
func GetTerminalFinalizedSpine(beaconState iface.BeaconState) common.Hash {
	finalization := beaconState.SpineData().Finalization
	if len(finalization) >= 32 {
		return common.BytesToHash(finalization[len(finalization)-32:])
	}
	cpFinalized := beaconState.SpineData().CpFinalized
	if len(cpFinalized) >= 32 {
		return common.BytesToHash(cpFinalized[len(cpFinalized)-32:])
	}
	return common.Hash{}
}

// GetFinalizationSequence returns the full finalization spine sequence
// (cp-finalized + finalization), trimmed so that the base spine is excluded.
func GetFinalizationSequence(beaconState iface.BeaconState) common.HashArray {
	cpFinalized := common.HashArrayFromBytes(beaconState.SpineData().CpFinalized)
	finalization := common.HashArrayFromBytes(beaconState.SpineData().Finalization)
	baseSpine := cpFinalized[0]
	finalizationSeq := append(cpFinalized, finalization...)
	if baseIx := finalizationSeq.IndexOf(baseSpine); baseIx > -1 {
		finalizationSeq = finalizationSeq[baseIx+1:]
	}
	return finalizationSeq
}

// GetBaseSpine returns the base spine stored in the cp-finalized sequence.
func GetBaseSpine(beaconState iface.BeaconState) common.Hash {
	cpFinalized := common.HashArrayFromBytes(beaconState.SpineData().CpFinalized)
	return cpFinalized[0]
}

// ConsensusCalcPrefix computes the new prefix from a set of unpublished
// chain proposals.  A spine is included when it appears in the common
// intersection AND has at least cfg.SpinePublicationsPrefixSupport votes.
func ConsensusCalcPrefix(unpublishedChains []common.HashArray, cfg *Config) (common.HashArray, error) {
	if err := ConsensusValidateUnpublishedChains(unpublishedChains); err != nil {
		return common.HashArray{}, err
	}
	var (
		publicationsMap = mapPublications{}
		commonChain     = common.HashArray{}
		prefix          = common.HashArray{}
	)
	for i, chain := range unpublishedChains {
		if i == 0 {
			commonChain = chain
		} else {
			commonChain = commonChain.SequenceIntersection(chain)
		}
		for _, spine := range chain {
			publicationsMap[spine]++
		}
	}
	for _, spine := range commonChain {
		if publicationsMap[spine] >= cfg.SpinePublicationsPrefixSupport {
			prefix = append(prefix, spine)
		}
	}
	slog.Info("Calculate prefix",
		"calcPrefix", prefix,
		"unpublishedChains", unpublishedChains,
		"commonChain", commonChain,
		"publicationsMap", publicationsMap,
	)
	return prefix, nil
}

// ConsensusValidateUnpublishedChains returns an error if any chain in the
// slice is empty, non-unique, or has a different first element than the rest.
func ConsensusValidateUnpublishedChains(unpublishedChains []common.HashArray) error {
	var firstVal common.Hash
	for _, chain := range unpublishedChains {
		if len(chain) == 0 {
			return errors.Wrap(ErrBadUnpublishedChains, "contains empty chain")
		}
		if !chain.IsUniq() {
			return errors.Wrap(ErrBadUnpublishedChains, "chain is not uniq")
		}
		if firstVal == (common.Hash{}) {
			firstVal = chain[0]
			continue
		}
		if chain[0] != firstVal {
			return errors.Wrap(ErrBadUnpublishedChains, "the first values of chain are not equal")
		}
	}
	return nil
}

// ConsensusCopyUnpublishedChains returns a deep copy of the chains slice.
func ConsensusCopyUnpublishedChains(unpublishedChains []common.HashArray) []common.HashArray {
	cpy := make([]common.HashArray, len(unpublishedChains))
	for i, chain := range unpublishedChains {
		cpy[i] = chain.Copy()
	}
	return cpy
}

// GweiToWei converts a Gwei amount to Wei.
func GweiToWei(gwei uint64) *big.Int {
	return new(big.Int).Mul(new(big.Int).SetUint64(gwei), new(big.Int).SetUint64(1_000_000_000))
}

// CountUniqSpines returns the number of distinct spine hashes across the
// finalization, prefix and spines sequences in beaconState.
func CountUniqSpines(beaconState iface.BeaconState) int {
	finSeq := GetFinalizationSequence(beaconState)
	prefixSeq := common.HashArrayFromBytes(beaconState.SpineData().Prefix)
	spineSeq := common.HashArrayFromBytes(beaconState.SpineData().Spines)
	fullSeq := make(common.HashArray, 0, len(finSeq)+len(prefixSeq)+len(spineSeq))
	fullSeq = append(fullSeq, finSeq...)
	fullSeq = append(fullSeq, prefixSeq...)
	fullSeq = append(fullSeq, spineSeq...)
	fullSeq.Deduplicate()
	return len(fullSeq)
}
