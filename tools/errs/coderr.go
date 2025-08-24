package errs

import (
	"PProject/tools/errs/stack"
	"errors"
	"strconv"
	"strings"
)

const stackSkip = 4

var DefaultCodeRelation = newCodeRelation()

type CodeErrorI interface {
	ECode() int
	EMsg() string
	DDetail() string
	WithDetail(detail string) CodeError
	Error
}

func NewCodeError(code int, msg string) CodeError {
	return CodeError{
		Code: code,
		Msg:  msg,
	}
}

type CodeError struct {
	Code   int    `json:"code"`
	Msg    string `json:"msg"`
	Detail string `json:"detail,omitempty"`
}

func (e *CodeError) WithDetail(detail string) CodeError {
	var d string
	if e.Detail == "" {
		d = detail
	} else {
		d = e.Detail + ", " + detail
	}
	return CodeError{
		Code:   e.Code,
		Msg:    e.Msg,
		Detail: d,
	}
}

func (e *CodeError) Wrap() error {
	return stack.New(e, stackSkip)
}

func (e *CodeError) clone() *CodeError {
	return &CodeError{
		Code:   e.Code,
		Msg:    e.Msg,
		Detail: e.Detail,
	}
}

func (e *CodeError) WrapMsg(msg string, kv ...any) error {
	retErr := e.clone()
	if msg != "" || len(kv) > 0 {
		detail := toString(msg, kv)
		if retErr.Detail == "" {
			retErr.Detail = detail
		} else {
			retErr.Detail += ", " + detail
		}
	}
	return stack.New(retErr, stackSkip)
}

func (e *CodeError) Is(err error) bool {
	var codeErr CodeError
	ok := errors.As(Unwrap(err), &codeErr)
	if !ok {
		if err == nil && e == nil {
			return true
		}
		return false
	}
	if e == nil {
		return false
	}
	code := codeErr.Code
	if e.Code == code {
		return true
	}
	return DefaultCodeRelation.Is(e.Code, code)
}

const initialCapacity = 3

func (e *CodeError) Error() string {
	v := make([]string, 0, initialCapacity)
	v = append(v, strconv.Itoa(e.Code), e.Msg)

	if e.Detail != "" {
		v = append(v, e.Detail)
	}

	return strings.Join(v, " ")
}

func Unwrap(err error) error {
	for err != nil {
		unwrap, ok := err.(interface {
			error
			Unwrap() error
		})
		if !ok {
			break
		}
		err = unwrap.Unwrap()
		if err == nil {
			return unwrap
		}
	}
	return err
}

func Wrap(err error) error {
	if err == nil {
		return nil
	}
	return stack.New(err, stackSkip)
}

func WrapMsg(err error, msg string, kv ...any) error {
	if err == nil {
		return nil
	}
	err = NewErrorWrapper(err, toString(msg, kv))
	return stack.New(err, stackSkip)
}

type CodeRelation interface {
	Add(codes ...int) error
	Is(parent, child int) bool
}

func newCodeRelation() CodeRelation {
	return &codeRelation{m: make(map[int]map[int]struct{})}
}

type codeRelation struct {
	m map[int]map[int]struct{}
}

const minimumCodesLength = 2

func (r *codeRelation) Add(codes ...int) error {
	if len(codes) < minimumCodesLength {
		return New("codes length must be greater than 2", "codes", codes).Wrap()
	}
	for i := 1; i < len(codes); i++ {
		parent := codes[i-1]
		s, ok := r.m[parent]
		if !ok {
			s = make(map[int]struct{})
			r.m[parent] = s
		}
		for _, code := range codes[i:] {
			s[code] = struct{}{}
		}
	}
	return nil
}

func (r *codeRelation) Is(parent, child int) bool {
	if parent == child {
		return true
	}
	s, ok := r.m[parent]
	if !ok {
		return false
	}
	_, ok = s[child]
	return ok
}
