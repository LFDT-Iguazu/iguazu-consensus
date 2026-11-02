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

package prevote

import (
	"crypto/sha256"
	"encoding/binary"
	"sync"
	"time"

	"github.com/patrickmn/go-cache"

	"github.com/LFDT-Iguazu/iguazu-types/coordinator/types"
)

// PrevoteCache is the default in-memory implementation of Pool.
//
// Two stores back it:
//   - prevoteCache: slot → list of live prevotes to include in the next
//     proposal
//   - seenPrevote:  dataRoot → list of aggregation bits already observed,
//     evicted after a configurable TTL (typically one epoch)
type PrevoteCache struct {
	prevoteCacheLock sync.RWMutex
	prevoteCache     map[types.Slot][]*types.PreVote
	seenPrevote      *cache.Cache
}

// NewPrevoteCache constructs an empty cache. The seenBitsTTL is the default
// retention for the seen-bits index; cleanupInterval is how often expired
// entries are pruned (typically 2 * seenBitsTTL).
func NewPrevoteCache(seenBitsTTL, cleanupInterval time.Duration) *PrevoteCache {
	return &PrevoteCache{
		prevoteCache: make(map[types.Slot][]*types.PreVote),
		seenPrevote:  cache.New(seenBitsTTL, cleanupInterval),
	}
}

// dataRoot deterministically hashes a PreVoteData value. Used as the key in
// the seen-bits index; the exact function is an implementation detail and
// is never persisted outside the process.
func dataRoot(d *types.PreVoteData) [32]byte {
	if d == nil {
		return [32]byte{}
	}
	var buf [16]byte
	binary.LittleEndian.PutUint64(buf[0:8], uint64(d.Slot))
	binary.LittleEndian.PutUint64(buf[8:16], uint64(d.Index))
	h := sha256.New()
	h.Write(buf[:])
	h.Write(d.Candidates)
	var out [32]byte
	h.Sum(out[:0])
	return out
}
