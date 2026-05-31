// Copyright 2026 The Kestrai Authors.
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

package cli

import (
	"fmt"
	"os"
	"time"
)

// colorEnabled reports whether to emit ANSI color: only when stderr is a
// terminal and NO_COLOR is unset (https://no-color.org). The hobbyist DX bar
// wants colorized output by default, plain when piped.
var colorEnabled = func() bool {
	if _, set := os.LookupEnv("NO_COLOR"); set {
		return false
	}
	info, err := os.Stderr.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}()

const (
	ansiReset = "\033[0m"
	ansiBold  = "\033[1m"
	ansiGreen = "\033[32m"
	ansiRed   = "\033[31m"
	ansiDim   = "\033[2m"
)

func colorize(code, s string) string {
	if !colorEnabled {
		return s
	}
	return code + s + ansiReset
}

// printError writes an actionable, colorized error to stderr.
func printError(err error) {
	fmt.Fprintln(os.Stderr, colorize(ansiRed, "error: ")+err.Error())
}

// statusf writes a green status line to stderr (progressive startup output).
func statusf(format string, args ...any) {
	fmt.Fprintln(os.Stderr, colorize(ansiGreen, "✓ ")+fmt.Sprintf(format, args...))
}

// dimf writes a dimmed informational line to stderr.
func dimf(format string, args ...any) {
	fmt.Fprintln(os.Stderr, colorize(ansiDim, fmt.Sprintf(format, args...)))
}

// durationShort renders the elapsed time since t in a compact kubectl-style
// form: "8s", "5m", "3h", "2d".
func durationShort(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	}
}
