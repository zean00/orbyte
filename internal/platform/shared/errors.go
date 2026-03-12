package shared

import "fmt"

type Kind string

const (
	KindValidation   Kind = "validation_error"
	KindConflict     Kind = "conflict_error"
	KindForbidden    Kind = "forbidden_error"
	KindUnauthorized Kind = "unauthorized_error"
	KindNotFound     Kind = "not_found_error"
)

type Error struct {
	Kind    Kind
	Message string
}

func (e Error) Error() string {
	return fmt.Sprintf("%s: %s", e.Kind, e.Message)
}

func Validation(message string) error {
	return Error{Kind: KindValidation, Message: message}
}

func Conflict(message string) error {
	return Error{Kind: KindConflict, Message: message}
}

func Forbidden(message string) error {
	return Error{Kind: KindForbidden, Message: message}
}

func Unauthorized(message string) error {
	return Error{Kind: KindUnauthorized, Message: message}
}

func NotFound(message string) error {
	return Error{Kind: KindNotFound, Message: message}
}
