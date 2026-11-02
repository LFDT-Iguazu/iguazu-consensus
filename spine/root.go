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

package spine

import (
	"github.com/pkg/errors"

	"github.com/LFDT-Iguazu/iguazu-consensus/ssz"
	"github.com/LFDT-Iguazu/iguazu-types/coordinator/types"
)

// DataRoot computes the HashTreeRoot of a SpineData value. The field
// order — spines, prefix, finalization, cpFinalized, parentSpines —
// matches the coordinator's stateutil.SpineDataRootWithHasher so that
// roots stay byte-identical across the two implementations.
func DataRoot(spineData *types.SpineData) ([32]byte, error) {
	if spineData == nil {
		return [32]byte{}, errors.New("nil spine data")
	}

	spinesRoot, err := BytesRoot(spineData.Spines)
	if err != nil {
		return [32]byte{}, errors.Wrap(err, "spines root failed")
	}
	prefixRoot, err := BytesRoot(spineData.Prefix)
	if err != nil {
		return [32]byte{}, errors.Wrap(err, "prefix root failed")
	}
	finalizationRoot, err := BytesRoot(spineData.Finalization)
	if err != nil {
		return [32]byte{}, errors.Wrap(err, "finalization root failed")
	}
	cpFinalizedRoot, err := BytesRoot(spineData.CpFinalized)
	if err != nil {
		return [32]byte{}, errors.Wrap(err, "cpFinalized root failed")
	}
	parentSpineRoot, err := parentSpinesRoot(spineData.ParentSpines)
	if err != nil {
		return [32]byte{}, errors.Wrap(err, "parent spines root failed")
	}

	fieldRoots := [][32]byte{
		spinesRoot,
		prefixRoot,
		finalizationRoot,
		cpFinalizedRoot,
		parentSpineRoot,
	}
	root, err := ssz.BitwiseMerkleize(fieldRoots, uint64(len(fieldRoots)), uint64(len(fieldRoots)))
	if err != nil {
		return [32]byte{}, errors.Wrap(err, "spines data merkleize failed")
	}
	return root, nil
}

// BytesRoot computes the SSZ HashTreeRoot of a variable-length byte list,
// splitting it into 32-byte chunks and mixing in its length.
func BytesRoot(bts []byte) ([32]byte, error) {
	btsLen := len(bts)
	chunksLen := btsLen / ssz.BytesPerChunk
	if btsLen%ssz.BytesPerChunk > 0 {
		chunksLen++
	}
	chunks := make([][32]byte, chunksLen)
	for i := 0; i < chunksLen; i++ {
		from := i * ssz.BytesPerChunk
		to := from + ssz.BytesPerChunk
		if to > btsLen {
			to = btsLen
		}
		chunks[i] = ssz.ToBytes32(bts[from:to])
	}
	chunksRoot, err := ssz.BitwiseMerkleize(chunks, uint64(len(chunks)), uint64(len(chunks)))
	if err != nil {
		return [32]byte{}, errors.Wrap(err, "could not compute bytes root merkleization")
	}
	return ssz.MixInLength(chunksRoot, ssz.Uint64LengthRoot(uint64(btsLen))), nil
}

func parentSpinesRoot(parentSpines []*types.SpinesSeq) ([32]byte, error) {
	allRoots := make([][32]byte, len(parentSpines))
	for i, ps := range parentSpines {
		root, err := BytesRoot(ps.Spines)
		if err != nil {
			return [32]byte{}, errors.Wrap(err, "parent spines root failed")
		}
		allRoots[i] = root
	}
	rootsRoot, err := ssz.BitwiseMerkleize(allRoots, uint64(len(allRoots)), uint64(len(allRoots)))
	if err != nil {
		return [32]byte{}, errors.Wrap(err, "could not compute bytes root ops merkleization")
	}
	return ssz.MixInLength(rootsRoot, ssz.Uint64LengthRoot(uint64(len(allRoots)))), nil
}
