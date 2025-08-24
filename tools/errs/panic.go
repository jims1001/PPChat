package errs

import (
	"PProject/tools/errs/stack"
	"fmt"
)

func ErrPanic(r any) error {
	return ErrPanicMsg(r, ServerInternalError, "panic error", 9)
}

func ErrPanicMsg(r any, code int, msg string, skip int) error {
	if r == nil {
		return nil
	}
	err := &CodeError{
		Code:   code,
		Msg:    msg,
		Detail: fmt.Sprint(r),
	}
	return stack.New(err, skip)
}
