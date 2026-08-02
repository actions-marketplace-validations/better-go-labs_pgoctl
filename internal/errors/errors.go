package errors

import "errors"

var (
	ErrReadFile        = errors.New("failed to read profile file")
	ErrParseProfile    = errors.New("failed to parse profile data")
	ErrNoCPUSampleType = errors.New("no cpu sample type")
	ErrFlatProfile     = errors.New("flat/cold profile: avg stack depth < 2")
)
