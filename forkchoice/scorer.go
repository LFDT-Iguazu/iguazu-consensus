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

package forkchoice

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/LFDT-Iguazu/iguazu-consensus/helpers"
	"github.com/LFDT-Iguazu/iguazu-types/common"
	"github.com/LFDT-Iguazu/iguazu-types/coordinator/iface"
)

// GetParentByOptimisticSpines selects the best head block root from the
// fork-choice tree that is compatible with the provided optimistic spine sets.
//
// It mirrors coordinator/beacon-chain/forkchoice/protoarray.GetParentByOptimisticSpines
// but delegates the LMD-GHOST head computation to fc.HeadBySubset so that
// the protoarray implementation stays in the coordinator.
func GetParentByOptimisticSpines(
	ctx context.Context,
	fc iface.ForkChoice,
	optSpines []common.HashArray,
	jCpRoot [32]byte,
	cfg *helpers.Config,
) ([32]byte, error) {
	var headRoot [32]byte

	defer func(start time.Time) {
		slog.Info("FC: GetParentByOptimisticSpines end",
			"elapsed", time.Since(start),
			"optSpines", len(optSpines),
			"headRoot", fmt.Sprintf("%#x", headRoot),
			"jCpRoot", fmt.Sprintf("%#x", jCpRoot),
		)
	}(time.Now())

	// drop empty chains
	filtered := make([]common.HashArray, 0, len(optSpines))
	for _, ha := range optSpines {
		if len(ha) > 0 {
			filtered = append(filtered, ha)
		}
	}

	acceptableRoots := CollectTgTreeNodes(fc, filtered, jCpRoot, cfg)

	slog.Info("FC: TG Tree", "acceptableRoots", len(acceptableRoots))

	if len(acceptableRoots) == 0 {
		return [32]byte{}, nil
	}

	var err error
	headRoot, err = fc.HeadBySubset(ctx, acceptableRoots, jCpRoot)
	if err != nil {
		return [32]byte{}, err
	}
	return headRoot, nil
}

// CollectTgTreeNodes returns the set of beacon block roots that form the T(G)
// tree — i.e. all nodes whose spine data is compatible with at least one of the
// optimistic spine chains provided by the GWAT execution layer.
//
// jCpRoot is the justified-checkpoint block root; only forks that include it
// are considered.
func CollectTgTreeNodes(
	fc iface.ForkChoice,
	optSpines []common.HashArray,
	jCpRoot [32]byte,
	cfg *helpers.Config,
) map[[32]byte]struct{} {
	forks := fc.GetForks()
	acceptableRoots := make(map[[32]byte]struct{})

	// pre-compute set of all spines present across all optSpines chains
	existedSpines := make(map[[32]byte]bool)
	for _, chain := range optSpines {
		for _, s := range chain {
			existedSpines[s] = true
		}
	}

	for frkNr, frk := range forks {
		roots := frk.Roots()
		if len(roots) == 0 {
			continue
		}

		// skip forks that do not pass through the justified checkpoint
		isJustified := false
		for _, r := range roots {
			if r == jCpRoot {
				isJustified = true
				break
			}
		}
		if !isJustified {
			slog.Warn("CollectTgTreeNodes: skip not-justified fork",
				"frkNr", frkNr,
				"jCpRoot", fmt.Sprintf("%#x", jCpRoot),
			)
			continue
		}

		for i, r := range roots {
			node := frk.NodeByRoot(r)
			if node == nil {
				continue
			}

			cpFinalized := node.SpinesFinalized()
			if len(cpFinalized) == 0 {
				slog.Error("CollectTgTreeNodes: cpFinalized empty",
					"frkNr", frkNr,
					"nodeIdx", i,
					"slot", node.Slot(),
					"root", fmt.Sprintf("%#x", node.Root()),
				)
				continue
			}

			// trim optSpines: drop chains up to and including lastFinSpine
			lastFinHash := cpFinalized[len(cpFinalized)-1]
			lastFinIndex := IndexOfOptimisticSpines(lastFinHash, optSpines)
			forkOptSpines := optSpines
			if lastFinIndex > -1 {
				forkOptSpines = optSpines[lastFinIndex+1:]
			}

			// check finalization is compatible with optSpines
			finalization := node.SpinesFinalization()
			var ok bool
			if cfg.IsFcTgTreeForkSlot(node.Slot()) {
				ok = true
				for _, s := range finalization {
					if !existedSpines[s] {
						ok = false
						break
					}
				}
			} else {
				ok = IsSequenceMatchOptimisticSpines(finalization, forkOptSpines)
			}

			slog.Info("CollectTgTreeNodes: check finalization",
				"ok", ok,
				"frkNr", frkNr,
				"nodeIdx", i,
				"slot", node.Slot(),
				"isTgTreeFork", cfg.IsFcTgTreeForkSlot(node.Slot()),
				"root", fmt.Sprintf("%#x", node.Root()),
				"finalization", len(finalization),
				"forkOptSpines", len(forkOptSpines),
			)

			if !ok {
				continue
			}

			// compute prefix-relevant optSpines window
			var prefOptSpines []common.HashArray
			if len(forkOptSpines) > len(finalization) {
				if len(finalization) > 0 {
					termFin := finalization[len(finalization)-1]
					termIndex := IndexOfOptimisticSpines(termFin, forkOptSpines)
					if termIndex >= 0 {
						prefOptSpines = forkOptSpines[termIndex+1:]
					}
				} else {
					prefOptSpines = forkOptSpines
				}
			}

			// check prefix is compatible with optSpines
			prefix := node.SpinesPrefix()
			if cfg.IsFcTgTreeForkSlot(node.Slot()) {
				ok = IsPrefixMatchOptimisticSpines(prefix, prefOptSpines)
			} else {
				ok = IsSequenceMatchOptimisticSpines(prefix, prefOptSpines)
			}

			slog.Info("CollectTgTreeNodes: check prefix",
				"ok", ok,
				"frkNr", frkNr,
				"nodeIdx", i,
				"slot", node.Slot(),
				"root", fmt.Sprintf("%#x", node.Root()),
			)

			if !ok {
				continue
			}

			// accept when: prefix ∩ published ≠ ∅, finalization ∩ published ≠ ∅,
			// or no spines published yet
			published := node.SpinesPublished()
			isExtended := len(prefix.Intersection(published)) > 0 ||
				len(finalization.Intersection(published)) > 0
			if isExtended || len(published) == 0 {
				for _, root := range roots[i:] {
					acceptableRoots[root] = struct{}{}
				}
				slog.Info("CollectTgTreeNodes: prefix extension accepted",
					"frkNr", frkNr,
					"nodeIdx", i,
					"slot", node.Slot(),
					"root", fmt.Sprintf("%#x", node.Root()),
				)
				break
			}

			// compute published-relevant optSpines window
			var pubOptSpines []common.HashArray
			if len(prefOptSpines) > len(prefix) {
				if len(prefix) > 0 {
					termPref := prefix[len(prefix)-1]
					termIndex := IndexOfOptimisticSpines(termPref, prefOptSpines)
					if termIndex >= 0 {
						pubOptSpines = prefOptSpines[termIndex+1:]
					}
				} else {
					pubOptSpines = prefOptSpines
				}
			}

			if len(pubOptSpines) == 0 {
				continue
			}

			// first published spine must appear in the first optSpines chain slot
			if !pubOptSpines[0].Has(published[0]) {
				continue
			}

			for _, root := range roots[i:] {
				acceptableRoots[root] = struct{}{}
			}
			slog.Info("CollectTgTreeNodes: first published spine accepted",
				"frkNr", frkNr,
				"nodeIdx", i,
				"slot", node.Slot(),
				"root", fmt.Sprintf("%#x", node.Root()),
			)
			break
		}
	}
	return acceptableRoots
}

// IsSequenceMatchOptimisticSpines reports whether every spine in seq is present
// in the corresponding optSpines slot (positional match).
func IsSequenceMatchOptimisticSpines(seq common.HashArray, optSpines []common.HashArray) bool {
	if len(seq) > len(optSpines) {
		return false
	}
	for i, h := range seq {
		if !optSpines[i].Has(h) {
			return false
		}
	}
	return true
}

// IsPrefixMatchOptimisticSpines reports whether every spine in prefix appears
// somewhere in optSpines (non-positional, order-preserving scan).
func IsPrefixMatchOptimisticSpines(prefix common.HashArray, optSpines []common.HashArray) bool {
	if len(prefix) == 0 {
		return true
	}
	if len(prefix) > len(optSpines) {
		return false
	}
	j := 0
	for _, os := range optSpines {
		if os.Has(prefix[j]) {
			j++
			if j == len(prefix) {
				break
			}
		}
	}
	return j == len(prefix)
}

// IndexOfOptimisticSpines returns the index of the first optSpines chain that
// contains hash, or -1 if not found.
func IndexOfOptimisticSpines(hash common.Hash, optSpines []common.HashArray) int {
	for i, chain := range optSpines {
		if chain.Has(hash) {
			return i
		}
	}
	return -1
}
