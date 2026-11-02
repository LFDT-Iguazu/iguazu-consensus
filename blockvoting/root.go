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

package blockvoting

import (
	"encoding/binary"

	"github.com/pkg/errors"
	"github.com/prysmaticlabs/go-bitfield"

	"github.com/LFDT-Iguazu/iguazu-consensus/spine"
	"github.com/LFDT-Iguazu/iguazu-consensus/ssz"
	"github.com/LFDT-Iguazu/iguazu-types/coordinator/types"
)

// DataRoot computes the HashTreeRoot of a single BlockVoting value. The
// field order — root, slot, candidates, votes — matches the coordinator's
// stateutil.BlockVotingDataRootWithHasher so that roots remain
// byte-identical.
func DataRoot(bv *types.BlockVoting) ([32]byte, error) {
	if bv == nil {
		return [32]byte{}, errors.New("nil blockVoting data")
	}
	var rootRoot, slotRoot [32]byte
	if len(bv.Root) > 0 {
		rootRoot = ssz.ToBytes32(bv.Root)
	}
	binary.LittleEndian.PutUint64(slotRoot[:8], uint64(bv.Slot))

	candRoot, err := spine.BytesRoot(bv.Candidates)
	if err != nil {
		return [32]byte{}, errors.Wrap(err, "block voting candidates")
	}
	votesRoot, err := CommitteeVotesListRoot(bv.Votes)
	if err != nil {
		return [32]byte{}, errors.Wrap(err, "block voting votes")
	}

	fieldRoots := [][32]byte{rootRoot, slotRoot, candRoot, votesRoot}
	root, err := ssz.BitwiseMerkleize(fieldRoots, uint64(len(fieldRoots)), uint64(len(fieldRoots)))
	if err != nil {
		return [32]byte{}, err
	}
	return root, nil
}

// ListRoot computes the HashTreeRoot of a BlockVoting list under a given
// SSZ list capacity. The capacity is injected by the caller (rather than
// pulled from a global config) so that the package stays independent of
// the coordinator's fieldparams.
func ListRoot(votings []*types.BlockVoting, limit uint64) ([32]byte, error) {
	roots := make([][32]byte, len(votings))
	for i, bv := range votings {
		root, err := DataRoot(bv)
		if err != nil {
			return [32]byte{}, errors.Wrap(err, "could not compute blockVoting merkleization")
		}
		roots[i] = root
	}

	rootsRoot, err := ssz.BitwiseMerkleize(roots, uint64(len(roots)), limit)
	if err != nil {
		return [32]byte{}, errors.Wrap(err, "could not compute blockVoting votes merkleization")
	}
	return ssz.MixInLength(rootsRoot, ssz.Uint64LengthRoot(uint64(len(votings)))), nil
}

// CommitteeVotesListRoot returns the HashTreeRoot of a CommitteeVote list.
// The list is treated as unbounded — the "limit" passed to Merkleize is
// exactly the number of elements, matching the coordinator implementation.
func CommitteeVotesListRoot(votes []*types.CommitteeVote) ([32]byte, error) {
	roots := make([][32]byte, len(votes))
	for i, v := range votes {
		root, err := CommitteeVoteRoot(v)
		if err != nil {
			return [32]byte{}, err
		}
		roots[i] = root
	}
	rootsRoot, err := ssz.BitwiseMerkleize(roots, uint64(len(roots)), uint64(len(roots)))
	if err != nil {
		return [32]byte{}, errors.Wrap(err, "could not compute committee votes merkleization")
	}
	return ssz.MixInLength(rootsRoot, ssz.Uint64LengthRoot(uint64(len(roots)))), nil
}

// CommitteeVoteRoot returns the HashTreeRoot of a single CommitteeVote.
// AggregationBits is treated as a bitlist: its length-marker bit is
// stripped before hashing, mirroring bitfield.Bitlist.Bytes().
func CommitteeVoteRoot(v *types.CommitteeVote) ([32]byte, error) {
	if v == nil {
		return [32]byte{}, nil
	}
	var slotBuf, indexBuf [32]byte
	binary.LittleEndian.PutUint64(slotBuf[:8], uint64(v.Slot))
	binary.LittleEndian.PutUint64(indexBuf[:8], uint64(v.Index))

	bitsBytes := bitfield.Bitlist(v.AggregationBits).Bytes()
	aggrRoot, err := spine.BytesRoot(bitsBytes)
	if err != nil {
		return [32]byte{}, errors.Wrap(err, "could not compute committee vote aggr bits")
	}
	fieldRoots := [][32]byte{slotBuf, indexBuf, aggrRoot}
	root, err := ssz.BitwiseMerkleize(fieldRoots, uint64(len(fieldRoots)), uint64(len(fieldRoots)))
	if err != nil {
		return [32]byte{}, errors.Wrap(err, "could not compute committee vote merkleization")
	}
	return root, nil
}
