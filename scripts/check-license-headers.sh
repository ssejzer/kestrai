#!/usr/bin/env bash
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
#
# Verifies that every source file in the repository starts with the
# standard Apache 2.0 header. Intended to run in CI and locally via
# `make lint-headers`.
#
# Detection rule: the first 20 lines must contain the substring
#   "Licensed under the Apache License, Version 2.0"
#
# Tracked extensions:
#   .go .py .proto .ts .tsx .js .jsx .sh
#
# Excluded paths:
#   - generated code under gen/
#   - vendor/, node_modules/, .git/, .venv/, dist/, build/, bin/
#   - empty __init__.py files (zero-byte package markers are exempt)

set -euo pipefail

MARKER='Licensed under the Apache License, Version 2.0'

# Use git ls-files so we only ever inspect tracked files (or files that
# would be tracked once added). This avoids walking node_modules etc.
if ! command -v git >/dev/null 2>&1; then
    echo "check-license-headers.sh: git is required" >&2
    exit 2
fi

# shellcheck disable=SC2207
FILES=($(git ls-files --cached --others --exclude-standard \
    '*.go' '*.py' '*.proto' '*.ts' '*.tsx' '*.js' '*.jsx' '*.sh'))

missing=()
checked=0
for file in "${FILES[@]}"; do
    # Skip generated subtree.
    case "$file" in
        gen/*|*/node_modules/*|.venv/*|vendor/*) continue ;;
    esac

    # Empty __init__.py files are exempt — they are pure package markers.
    if [[ "$file" == *"__init__.py" ]] && [[ ! -s "$file" ]]; then
        continue
    fi

    checked=$((checked + 1))
    if ! head -n 20 "$file" 2>/dev/null | grep -q -F "$MARKER"; then
        missing+=("$file")
    fi
done

if (( ${#missing[@]} > 0 )); then
    echo "Missing Apache 2.0 header in ${#missing[@]} file(s):" >&2
    for f in "${missing[@]}"; do
        echo "  $f" >&2
    done
    echo >&2
    echo "Add the header from any existing source file, e.g. cmd/kestrai/main.go." >&2
    exit 1
fi

echo "License headers OK ($checked files checked)."
