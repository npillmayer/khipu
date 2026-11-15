package khipu

import (
	"errors"

	"github.com/npillmayer/khipu/contracts"
)

var (
	// ErrIllegalArguments retains identity with contracts.ErrIllegalArguments.
	ErrIllegalArguments = contracts.ErrIllegalArguments
	// ErrVoidKhipukamayuq identifies an operation on a nil owner. It does not
	// classify every incomplete configuration; those may return ErrIllegalArguments.
	// Use errors.Is when the error may have been wrapped.
	ErrVoidKhipukamayuq = errors.New("khipu: void khipukamayuq")
)
