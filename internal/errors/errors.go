// Package errors defines sentinel errors for pgoctl internal packages.
package errors

import "errors"

// ErrFlatProfile is a format string: it expects the minimum stack depth value.
const ErrFlatProfile = "flat/cold profile: avg stack depth < %.1f"

// Sentinel errors returned by profile I/O and parsing.
var (
	ErrReadFile        = errors.New("failed to read profile file")
	ErrParseProfile    = errors.New("failed to parse profile data")
	ErrNoCPUSampleType = errors.New("no cpu sample type")
)
