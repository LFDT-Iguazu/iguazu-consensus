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

import "github.com/LFDT-Iguazu/iguazu-types/common"

// Spines is a newtype over []byte that carries DAG spine hashes encoded as a
// contiguous byte slice (32 bytes per hash). It mirrors the coordinator's
// proto/prysm/v1alpha1/wrapper.Spines but without gwat dependencies.
type Spines []byte

// HashArray decodes the byte slice into a typed HashArray.
func (s Spines) HashArray() common.HashArray {
	if len(s) == 0 {
		return common.HashArray{}
	}
	return common.HashArrayFromBytes(s)
}

// Key returns a canonical deduplication key for the spine set, suitable for
// use as a map key or for comparing two Spines values for equality.
func (s Spines) Key() common.Hash {
	return s.HashArray().Key()
}
