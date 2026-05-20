# Copyright 2026 The Kestrai Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

"""Smoke test so `pytest` has something to find before Phase 1 lands."""

import kestrai


def test_version_is_a_string() -> None:
    assert isinstance(kestrai.__version__, str)
    assert kestrai.__version__
