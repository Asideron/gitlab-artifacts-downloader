package app

import "errors"

var (
	errNotAllRequiredFlagsSet = errors.New("not all required flags were specified")
)