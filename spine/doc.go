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

// Package spine computes the SSZ HashTreeRoot of SpineData — the
// Iguazu-specific field that threads DAG finalization hashes through
// the beacon block. The roots produced here must be byte-identical to
// those of the coordinator's stateutil.SpineDataRoot; see the tests in
// this package and in iguazu-consensus/ssz for the compatibility anchors.
package spine
