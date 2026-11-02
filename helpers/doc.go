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

// Package helpers provides consensus-layer utility functions used by the
// DAG-consensus and withdrawal processors — and by the prevote P2P routing
// layer — without importing Prysm or coordinator internals.
//
// All state-mutating helpers accept iface.BeaconState; config-dependent
// helpers receive a *Config rather than pulling from a global registry.
package helpers
