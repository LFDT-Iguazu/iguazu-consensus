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

package dag

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"sort"

	"github.com/LFDT-Iguazu/iguazu-consensus/helpers"
	"github.com/LFDT-Iguazu/iguazu-types/common"
	"github.com/LFDT-Iguazu/iguazu-types/coordinator/iface"
	"github.com/LFDT-Iguazu/iguazu-types/coordinator/types"
)

type mapVoting map[common.Hash]int
type mapPriority map[int]common.HashArray
type mapCandidates map[common.Hash]common.HashArray

// ProcessDagConsensus is an operation performed on each beacon block to
// calculate the state transition data related to DAG consensus:
//  1. Calculate a new prefix of spines.
//  2. Collect attestations and calculate the consensus finalization sequence.
func ProcessDagConsensus(
	ctx context.Context,
	beaconState iface.DagBeaconState,
	signed iface.SignedBeaconBlock,
	cc iface.CommitteeCounter,
	cfg *helpers.Config,
) (iface.DagBeaconState, error) {
	if beaconState == nil {
		return nil, errors.New("nil state")
	}

	beaconBlock := signed.Block()
	candidates := beaconBlock.Body().Eth1Candidates()
	blockVoting := helpers.BlockVotingArrCopy(beaconState.BlockVoting())

	prefix, unpubChains, err := CalcPrefixAndParentSpines(beaconState.SpineData(), candidates, cfg)
	if err != nil {
		return nil, err
	}

	if cfg.IsPrefixFinForkSlot(beaconBlock.Slot()) {
		if len(prefix) > 0 {
			blockVoting = addBlockVoting(blockVoting, beaconBlock.ParentRoot(), beaconBlock.Slot()-1, prefix.ToBytes())
		}
	} else {
		if len(candidates) > 0 {
			blockVoting = addBlockVoting(blockVoting, beaconBlock.ParentRoot(), beaconBlock.Slot()-1, prefix.ToBytes())
		}
	}

	isBlockVotingForkSlot := cfg.IsBlockVotingForkSlot(beaconState.Slot())
	for _, att := range beaconBlock.Body().Attestations() {
		blockVoting = appendBlockVotingAtt(blockVoting, att, isBlockVotingForkSlot)
	}

	finalization, err := calcFinalization(ctx, beaconState, blockVoting, cc, cfg)
	if err != nil {
		return nil, err
	}

	prefix = prefix.Difference(finalization)

	parentSpines := make([]*types.SpinesSeq, 0, len(unpubChains))
	for _, chain := range unpubChains {
		dif := chain.Difference(finalization).Difference(prefix)
		if len(dif) > 0 {
			parentSpines = append(parentSpines, &types.SpinesSeq{Spines: dif.ToBytes()})
		}
	}

	if err = beaconState.SetEth1Candidates(candidates); err != nil {
		return nil, err
	}

	spineData := &types.SpineData{
		Spines:       candidates,
		Prefix:       prefix.ToBytes(),
		Finalization: finalization.ToBytes(),
		CpFinalized:  beaconState.SpineData().CpFinalized,
		ParentSpines: parentSpines,
	}
	if err = beaconState.SetSpineData(spineData); err != nil {
		return nil, err
	}

	deprecatedRoots := getBlockVotingsDeprecatedRoots(blockVoting, finalization)
	blockVoting = removeBlockVoting(blockVoting, deprecatedRoots)

	if blockVoting, err = handleBlockVotingVotesLimit(ctx, blockVoting, beaconState, cc, cfg); err != nil {
		return nil, err
	}

	if cfg.IsEpochStart(beaconBlock.Slot()) {
		if blockVoting, err = cleanBlockVotingStaleVotes(ctx, blockVoting, beaconState, cc, cfg); err != nil {
			return nil, err
		}

		cpEpoch := beaconState.FinalizedCheckpoint().Epoch
		cpSlot := cfg.EpochStartSlot(cpEpoch)
		staleRoots := getBlockVotingRootsLtSlot(blockVoting, cpSlot)
		blockVoting = removeBlockVoting(blockVoting, staleRoots)

		slog.Info("Block voting: removes stale at new epoch",
			"slot", beaconState.Slot(),
			"blockVoting", len(blockVoting),
			"staleRoots", fmt.Sprintf("%#x", staleRoots),
		)
	}

	blockVoting, err = helpers.BlockVotingArrStateOrder(blockVoting)
	if err != nil {
		return nil, err
	}

	if err = beaconState.SetBlockVoting(blockVoting); err != nil {
		return nil, err
	}
	return beaconState, nil
}

// calcFinalization calculates the finalization sequence from the current
// BlockVoting entries.
func calcFinalization(
	ctx context.Context,
	beaconState iface.BeaconState,
	blockVotingArr []*types.BlockVoting,
	cc iface.CommitteeCounter,
	cfg *helpers.Config,
) (common.HashArray, error) {
	var (
		blockVotings    = helpers.BlockVotingArrCopy(blockVotingArr)
		supportedVotes  = make([]*types.BlockVoting, 0)
		candidatesParts = make([][]common.HashArray, 0)
		tabPriority     = mapPriority{}
		tabVoting       = mapVoting{}
		tabCandidates   = mapCandidates{}
		slotsToConfirm  = cfg.VotingRequiredSlots
		badVotes        = make([]*types.BlockVoting, 0)
		finalization    = common.HashArrayFromBytes(beaconState.SpineData().Finalization)
		lastFinSpine    = helpers.GetTerminalFinalizedSpine(beaconState)
		resFinalization = finalization.Copy()
		minSupportCache = map[types.Slot]int{}
	)

	for _, bv := range blockVotings {
		candidates := common.HashArrayFromBytes(bv.Candidates)
		if !candidates.IsUniq() {
			slog.Warn("calcFinalization: skip bad candidates (not uniq)",
				"slot", beaconState.Slot(),
				"root", fmt.Sprintf("%#x", bv.Root),
				"lastFinSpine", lastFinSpine.Hex(),
			)
			badVotes = append(badVotes, bv)
			continue
		}

		mapSlotAtt := map[types.Slot][]*types.CommitteeVote{}
		for _, vote := range bv.Votes {
			mapSlotAtt[vote.Slot] = append(mapSlotAtt[vote.Slot], vote)
		}
		for slot, atts := range mapSlotAtt {
			var minSupport int
			if v, ok := minSupportCache[slot]; ok {
				minSupport = v
			} else {
				var err error
				minSupport, err = BlockVotingMinSupport(ctx, beaconState, slot, cc, cfg)
				if err != nil {
					return nil, err
				}
				minSupportCache[slot] = minSupport
			}
			if helpers.CountCommitteeVotes(atts) >= uint64(minSupport) {
				supportedVotes = append(supportedVotes, bv.Copy())
				break
			}
		}
	}

	slog.Info("Voting info",
		"slot", beaconState.Slot(),
		"totalBlockVoting", len(blockVotings),
		"supportedBlockVoting", len(supportedVotes),
		"votingRequiredSlots", cfg.VotingRequiredSlots,
	)

	_ = badVotes

	for _, bv := range supportedVotes {
		candidates := common.HashArrayFromBytes(bv.Candidates)
		fullLen := len(candidates)
		if i := candidates.IndexOf(lastFinSpine); i >= 0 {
			candidates = candidates[i+1:]
		}
		if len(candidates) == 0 && fullLen > len(candidates) {
			slog.Warn("calcFinalization: skip no candidates after trim",
				"slot", beaconState.Slot(),
				"root", fmt.Sprintf("%#x", bv.Root),
				"lastFinSpine", lastFinSpine.Hex(),
			)
			badVotes = append(badVotes, bv)
			continue
		}

		reductedParts := []common.HashArray{candidates}
		for i := len(candidates) - 1; i > 0; i-- {
			reductedParts = append(reductedParts, candidates[:i])
		}
		candidatesParts = append(candidatesParts, reductedParts)
	}

	for i, candidatesList := range candidatesParts {
		var restList []common.HashArray
		for _, rp := range candidatesParts[i+1:] {
			restList = append(restList, rp...)
		}
		for _, cands := range candidatesList {
			for _, rc := range restList {
				if cands.IsEqualTo(rc) {
					key := cands.Key()
					tabCandidates[key] = cands
					tabPriority[len(cands)] = append(tabPriority[len(cands)], key).Uniq()
					tabVoting[key]++
				}
			}
		}
	}

	priorities := make([]int, 0, len(tabPriority))
	for p := range tabPriority {
		priorities = append(priorities, p)
	}
	sort.Sort(sort.Reverse(sort.IntSlice(priorities)))

	resKey := common.Hash{}
	for _, p := range priorities {
		// Consensus-critical: maxVotes is deliberately never updated, so the
		// condition degenerates to "votes > 0" and the *last* qualifying key
		// of the priority group wins, not the one with the most votes. This
		// mirrors the live coordinator; tracking the maximum here would pick
		// a different finalization sequence and fork the chain.
		maxVotes := 0
		for _, key := range tabPriority[p] {
			votes := tabVoting[key]
			if votes >= slotsToConfirm && votes > maxVotes {
				resKey = key
			}
		}
		if resKey != (common.Hash{}) {
			break
		}
	}

	slog.Info("Calculation of finalization sequence",
		"slot", beaconState.Slot(),
		"lastFinSpine", lastFinSpine.Hex(),
		"finalization", tabCandidates[resKey],
	)

	if resKey != (common.Hash{}) {
		resFinalization = append(resFinalization, tabCandidates[resKey]...)
	}

	return resFinalization, nil
}

// BlockVotingMinSupport calculates the minimum number of committee votes
// required to consider a BlockVoting entry as having enough support
// (cfg.BlockVotingMinSupportPrc percent of the slot's attestors, plus one).
func BlockVotingMinSupport(
	ctx context.Context,
	state iface.BeaconState,
	slot types.Slot,
	cc iface.CommitteeCounter,
	cfg *helpers.Config,
) (int, error) {
	slotAtts, err := cc.CountSlotAttestors(ctx, slot)
	if err != nil {
		return 0, err
	}
	val := int(math.Ceil((float64(slotAtts)/100)*float64(cfg.BlockVotingMinSupportPrc))) + 1
	if val > slotAtts {
		val = slotAtts
	}
	return val, nil
}

// CalcPrefixAndParentSpines computes the new spine prefix and the list of
// unpublished parent spine chains from the current state's SpineData and the
// incoming block's candidates.
func CalcPrefixAndParentSpines(
	stSpineData *types.SpineData,
	blCandidates []byte,
	cfg *helpers.Config,
) (prefix common.HashArray, parentSpines []common.HashArray, err error) {
	parentPrefix := common.HashArrayFromBytes(stSpineData.Prefix)
	finalized := common.HashArrayFromBytes(stSpineData.Finalization)
	spines := common.HashArrayFromBytes(blCandidates)

	parentUnpubChains := make([]common.HashArray, len(stSpineData.ParentSpines))
	for i, spseq := range stSpineData.ParentSpines {
		chain := common.HashArrayFromBytes(spseq.Spines)
		if len(chain) > 0 {
			parentUnpubChains[i] = chain
		}
	}

	resUnpubChains := parentUnpubChains
	if len(spines) > 0 {
		resUnpubChains = []common.HashArray{spines}
		for _, chain := range parentUnpubChains {
			if len(chain) == 0 {
				continue
			}
			chainDif := chain.Difference(parentPrefix)
			if len(chainDif) > 0 && chainDif[0] == spines[0] {
				resUnpubChains = append(resUnpubChains, chain)
			}
		}
	}

	prefixExtension, err := helpers.ConsensusCalcPrefix(resUnpubChains, cfg)
	if err != nil {
		return nil, nil, err
	}
	resPrefix := append(parentPrefix, prefixExtension...).Uniq()
	resPrefix = resPrefix.Difference(finalized)

	return resPrefix, resUnpubChains, nil
}

// ---- BlockVoting mutation helpers -------------------------------------------

func isBlockVotingExists(votes []*types.BlockVoting, root []byte) bool {
	for _, bv := range votes {
		if bytes.Equal(bv.Root, root) {
			return true
		}
	}
	return false
}

func addBlockVoting(votes []*types.BlockVoting, root []byte, slot types.Slot, candidates []byte) []*types.BlockVoting {
	cpy := helpers.BlockVotingArrCopy(votes)
	if !isBlockVotingExists(cpy, root) {
		cpy = append(cpy, &types.BlockVoting{
			Root:       append([]byte(nil), root...),
			Votes:      []*types.CommitteeVote{},
			Slot:       slot,
			Candidates: append([]byte(nil), candidates...),
		})
		return cpy
	}
	// Consensus-critical: when an entry for root already exists the update is
	// applied to the copy and then thrown away — the caller's original slice
	// is returned untouched. That is what the live coordinator does, so an
	// existing BlockVoting never has its Slot/Candidates refreshed. Returning
	// cpy here would write the refreshed values into the state and fork.
	for _, bv := range cpy {
		if bytes.Equal(bv.Root, root) {
			bv.Slot = slot
			bv.Candidates = append([]byte(nil), candidates...)
		}
	}
	return votes
}

func appendBlockVotingAtt(votes []*types.BlockVoting, att iface.Attestation, isBlockVotingForkSlot bool) []*types.BlockVoting {
	root := att.BeaconBlockRoot()
	if !isBlockVotingExists(votes, root) {
		return votes
	}
	newVote := &types.CommitteeVote{
		AggregationBits: append([]byte(nil), att.AggregationBits()...),
		Slot:            att.AttestationSlot(),
		Index:           att.CommitteeIndex(),
	}
	cpy := helpers.BlockVotingArrCopy(votes)
	for _, bv := range cpy {
		if bytes.Equal(bv.Root, root) {
			if isBlockVotingForkSlot {
				bv.Votes = helpers.AddAggregateCommitteeVote(bv.Votes, newVote)
			} else {
				// Pre-fork slots keep the legacy aggregation to preserve consensus.
				bv.Votes = helpers.AggregateCommitteeVote(append(bv.Votes, newVote)) //nolint:staticcheck // SA1019: required before the block voting fork slot
			}
			break
		}
	}
	return cpy
}

func removeBlockVoting(votes []*types.BlockVoting, roots [][]byte) []*types.BlockVoting {
	if len(roots) == 0 {
		return votes
	}
	res := make([]*types.BlockVoting, 0, len(votes))
	for _, bv := range votes {
		if helpers.IndexOfRoot(roots, bv.Root) == -1 {
			res = append(res, bv)
		}
	}
	return res
}

// ---- BlockVoting pruning helpers --------------------------------------------

func getBlockVotingsDeprecatedRoots(blockVoting []*types.BlockVoting, finalization common.HashArray) [][]byte {
	seen := map[common.Hash]bool{}
	roots := make([][]byte, 0)
	for _, bv := range blockVoting {
		candidates := common.HashArrayFromBytes(bv.Candidates)
		if len(candidates) == 0 {
			continue
		}
		last := candidates[len(candidates)-1]
		if finalization.IndexOf(last) > -1 {
			h := common.BytesToHash(bv.Root)
			if !seen[h] {
				seen[h] = true
				roots = append(roots, append([]byte(nil), bv.Root...))
			}
		}
	}
	return roots
}

func getBlockVotingRootsLtSlot(blockVoting []*types.BlockVoting, slot types.Slot) [][]byte {
	seen := map[common.Hash]bool{}
	roots := make([][]byte, 0)
	for _, bv := range blockVoting {
		if bv.Slot <= slot {
			h := common.BytesToHash(bv.Root)
			if !seen[h] {
				seen[h] = true
				roots = append(roots, append([]byte(nil), bv.Root...))
			}
		}
	}
	return roots
}

func cleanBlockVotingStaleVotes(
	ctx context.Context,
	blockVoting []*types.BlockVoting,
	bState iface.BeaconState,
	cc iface.CommitteeCounter,
	cfg *helpers.Config,
) ([]*types.BlockVoting, error) {
	stSlot := bState.Slot()
	targetEpoch := cfg.SlotToEpoch(stSlot)
	if targetEpoch <= 2 {
		return blockVoting, nil
	}
	staleVotesSlot := cfg.EpochStartSlot(targetEpoch - 2)

	isBlockVotingFork := cfg.IsBlockVotingForkSlot(stSlot)
	for i, bv := range blockVoting {
		if bv.Slot >= staleVotesSlot {
			continue
		}
		slotMap := map[types.Slot][]*types.CommitteeVote{}
		slotsArr := make([]types.Slot, 0, len(bv.Votes))
		for _, vote := range bv.Votes {
			slot := vote.Slot
			if isBlockVotingFork {
				if slotMap[slot] == nil {
					slotMap[slot] = nil
					slotsArr = append(slotsArr, slot)
				}
			} else {
				slotsArr = append(slotsArr, slot)
				if slotMap[slot] == nil {
					slotMap[slot] = nil
				}
			}
			slotMap[slot] = append(slotMap[slot], vote)
		}
		sort.Slice(slotsArr, func(a, b int) bool { return slotsArr[a] < slotsArr[b] })

		for _, slot := range slotsArr {
			if slot >= staleVotesSlot {
				continue
			}
			votes := slotMap[slot]
			minSupport, err := BlockVotingMinSupport(ctx, bState, slot, cc, cfg)
			if err != nil {
				return nil, err
			}
			if helpers.CountCommitteeVotes(votes) < uint64(minSupport) {
				slog.Info("BlockVoting votes clean stale votes",
					"staleVotesSlot", staleVotesSlot,
					"rmSlot", slot,
					"bv", helpers.PrintBlockVoting(bv),
				)
				delete(slotMap, slot)
			}
		}

		resVotes := make([]*types.CommitteeVote, 0)
		for _, slot := range slotsArr {
			if len(slotMap[slot]) > 0 {
				resVotes = append(resVotes, slotMap[slot]...)
			}
		}
		blockVoting[i].Votes = resVotes
	}

	res := make([]*types.BlockVoting, 0, len(blockVoting))
	for _, bv := range blockVoting {
		if bv.Slot >= staleVotesSlot || len(bv.Votes) > 0 {
			res = append(res, bv)
		} else {
			slog.Info("BlockVoting votes clean empty item",
				"staleVotesSlot", staleVotesSlot,
				"bv", helpers.PrintBlockVoting(bv),
			)
		}
	}
	return res, nil
}

func handleBlockVotingVotesLimit(
	ctx context.Context,
	blockVoting []*types.BlockVoting,
	bState iface.BeaconState,
	cc iface.CommitteeCounter,
	cfg *helpers.Config,
) ([]*types.BlockVoting, error) {
	stSlot := bState.Slot()
	isBlockVotingFork := cfg.IsBlockVotingForkSlot(stSlot)
	minSupportMap := map[types.Slot]int{}

	for i, bv := range blockVoting {
		if len(bv.Votes) <= 64 {
			continue
		}
		slotMap := map[types.Slot][]*types.CommitteeVote{}
		slotsArr := make([]types.Slot, 0, len(bv.Votes))
		for _, vote := range bv.Votes {
			slot := vote.Slot
			if isBlockVotingFork {
				if slotMap[slot] == nil {
					slotMap[slot] = nil
					slotsArr = append(slotsArr, slot)
				}
			} else {
				slotsArr = append(slotsArr, slot)
				if slotMap[slot] == nil {
					slotMap[slot] = nil
				}
			}
			slotMap[slot] = append(slotMap[slot], vote)
		}
		sort.Slice(slotsArr, func(a, b int) bool { return slotsArr[a] < slotsArr[b] })

		for _, slot := range slotsArr {
			votes := slotMap[slot]
			var minSupport int
			if v, ok := minSupportMap[slot]; ok {
				minSupport = v
			} else {
				var err error
				minSupport, err = BlockVotingMinSupport(ctx, bState, slot, cc, cfg)
				if err != nil {
					return nil, err
				}
				minSupportMap[slot] = minSupport
			}
			if helpers.CountCommitteeVotes(votes) < uint64(minSupport) {
				delete(slotMap, slot)
				break
			}
		}

		resVotes := make([]*types.CommitteeVote, 0)
		for _, slot := range slotsArr {
			if len(slotMap[slot]) > 0 {
				resVotes = append(resVotes, slotMap[slot]...)
			}
		}
		blockVoting[i].Votes = resVotes

		slog.Info("BlockVoting votes reduced",
			"bv", helpers.PrintBlockVoting(bv),
		)
	}
	return blockVoting, nil
}
