package specialerror

import (
	"PProject/tools/errs"
	"errors"
)

var handlers []func(err error) errs.CodeError

func AddErrHandler(h func(err error) errs.CodeError) (err error) {
	if h == nil {
		return errs.New("nil handler")
	}
	handlers = append(handlers, h)
	return nil
}

func ErrString(err error) errs.Error {
	var codeErr errs.Error
	if errors.As(err, &codeErr) {
		return codeErr
	}
	return nil
}

func ErrWrapper(err error) errs.ErrWrapper {
	var codeErr errs.ErrWrapper
	if errors.As(err, &codeErr) {
		return codeErr
	}
	return nil
}
