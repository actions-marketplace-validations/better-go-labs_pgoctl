package errors

import "errors"

// ErrFlatProfile is a format string: it expects the minimum stack depth value.
const ErrFlatProfile = "flat/cold profile: avg stack depth < %.1f"

var (
	ErrReadFile        = errors.New("failed to read profile file")
	ErrParseProfile    = errors.New("failed to parse profile data")
	ErrNoCPUSampleType = errors.New("no cpu sample type")
)
