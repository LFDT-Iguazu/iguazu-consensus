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

package prevote

import (
	"context"
	"time"

	"github.com/LFDT-Iguazu/iguazu-types/coordinator/types"
)

// Pool is the prevote-pool contract consumed by the sync layer, the RPC
// layer and the block-proposer path. Mirrors iface.PreVotePool and is
// re-declared here so that iguazu-consensus consumers do not have to import the
// iguazu-types interface package directly.
type Pool interface {
	HasPrevote(pv *types.PreVote) (bool, error)
	SavePrevote(pv *types.PreVote) error
	GetPrevoteBySlot(ctx context.Context, slot types.Slot) []*types.PreVote
	PurgeOutdatedPrevote(curSlot types.Slot) error
}

// NewPool constructs the default PrevoteCache implementation with the given
// seen-bits TTL and cleanup interval. Callers are expected to derive the
// TTL from the network's slots-per-epoch * seconds-per-slot product.
func NewPool(seenBitsTTL, cleanupInterval time.Duration) *PrevoteCache {
	return NewPrevoteCache(seenBitsTTL, cleanupInterval)
}
