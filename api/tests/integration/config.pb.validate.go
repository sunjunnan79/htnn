// Code generated by protoc-gen-validate. DO NOT EDIT.
// source: api/tests/integration/config.proto

package integration

import (
	"bytes"
	"errors"
	"fmt"
	"net"
	"net/mail"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"google.golang.org/protobuf/types/known/anypb"
)

// ensure the imports are used
var (
	_ = bytes.MinRead
	_ = errors.New("")
	_ = fmt.Print
	_ = utf8.UTFMax
	_ = (*regexp.Regexp)(nil)
	_ = (*strings.Reader)(nil)
	_ = net.IPv4len
	_ = time.Duration(0)
	_ = (*url.URL)(nil)
	_ = (*mail.Address)(nil)
	_ = anypb.Any{}
	_ = sort.Sort
)

// Validate checks the field values on Config with the rules defined in the
// proto definition for this message. If any rules are violated, the first
// error encountered is returned, or nil if there are no violations.
func (m *Config) Validate() error {
	return m.validate(false)
}

// ValidateAll checks the field values on Config with the rules defined in the
// proto definition for this message. If any rules are violated, the result is
// a list of violation errors wrapped in ConfigMultiError, or nil if none found.
func (m *Config) ValidateAll() error {
	return m.validate(true)
}

func (m *Config) validate(all bool) error {
	if m == nil {
		return nil
	}

	var errors []error

	// no validation rules for NeedBuffer

	// no validation rules for Decode

	// no validation rules for Encode

	// no validation rules for Headers

	// no validation rules for Data

	// no validation rules for Trailers

	// no validation rules for ReplyMsg

	// no validation rules for InGrpcMode

	if len(errors) > 0 {
		return ConfigMultiError(errors)
	}

	return nil
}

// ConfigMultiError is an error wrapping multiple validation errors returned by
// Config.ValidateAll() if the designated constraints aren't met.
type ConfigMultiError []error

// Error returns a concatenation of all the error messages it wraps.
func (m ConfigMultiError) Error() string {
	var msgs []string
	for _, err := range m {
		msgs = append(msgs, err.Error())
	}
	return strings.Join(msgs, "; ")
}

// AllErrors returns a list of validation violation errors.
func (m ConfigMultiError) AllErrors() []error { return m }

// ConfigValidationError is the validation error returned by Config.Validate if
// the designated constraints aren't met.
type ConfigValidationError struct {
	field  string
	reason string
	cause  error
	key    bool
}

// Field function returns field value.
func (e ConfigValidationError) Field() string { return e.field }

// Reason function returns reason value.
func (e ConfigValidationError) Reason() string { return e.reason }

// Cause function returns cause value.
func (e ConfigValidationError) Cause() error { return e.cause }

// Key function returns key value.
func (e ConfigValidationError) Key() bool { return e.key }

// ErrorName returns error name.
func (e ConfigValidationError) ErrorName() string { return "ConfigValidationError" }

// Error satisfies the builtin error interface
func (e ConfigValidationError) Error() string {
	cause := ""
	if e.cause != nil {
		cause = fmt.Sprintf(" | caused by: %v", e.cause)
	}

	key := ""
	if e.key {
		key = "key for "
	}

	return fmt.Sprintf(
		"invalid %sConfig.%s: %s%s",
		key,
		e.field,
		e.reason,
		cause)
}

var _ error = ConfigValidationError{}

var _ interface {
	Field() string
	Reason() string
	Key() bool
	Cause() error
	ErrorName() string
} = ConfigValidationError{}

// Validate checks the field values on BadPluginConfig with the rules defined
// in the proto definition for this message. If any rules are violated, the
// first error encountered is returned, or nil if there are no violations.
func (m *BadPluginConfig) Validate() error {
	return m.validate(false)
}

// ValidateAll checks the field values on BadPluginConfig with the rules
// defined in the proto definition for this message. If any rules are
// violated, the result is a list of violation errors wrapped in
// BadPluginConfigMultiError, or nil if none found.
func (m *BadPluginConfig) ValidateAll() error {
	return m.validate(true)
}

func (m *BadPluginConfig) validate(all bool) error {
	if m == nil {
		return nil
	}

	var errors []error

	// no validation rules for PanicInFactory

	// no validation rules for PanicInParse

	// no validation rules for ErrorInInit

	// no validation rules for PanicInInit

	// no validation rules for ErrorInParse

	if len(errors) > 0 {
		return BadPluginConfigMultiError(errors)
	}

	return nil
}

// BadPluginConfigMultiError is an error wrapping multiple validation errors
// returned by BadPluginConfig.ValidateAll() if the designated constraints
// aren't met.
type BadPluginConfigMultiError []error

// Error returns a concatenation of all the error messages it wraps.
func (m BadPluginConfigMultiError) Error() string {
	var msgs []string
	for _, err := range m {
		msgs = append(msgs, err.Error())
	}
	return strings.Join(msgs, "; ")
}

// AllErrors returns a list of validation violation errors.
func (m BadPluginConfigMultiError) AllErrors() []error { return m }

// BadPluginConfigValidationError is the validation error returned by
// BadPluginConfig.Validate if the designated constraints aren't met.
type BadPluginConfigValidationError struct {
	field  string
	reason string
	cause  error
	key    bool
}

// Field function returns field value.
func (e BadPluginConfigValidationError) Field() string { return e.field }

// Reason function returns reason value.
func (e BadPluginConfigValidationError) Reason() string { return e.reason }

// Cause function returns cause value.
func (e BadPluginConfigValidationError) Cause() error { return e.cause }

// Key function returns key value.
func (e BadPluginConfigValidationError) Key() bool { return e.key }

// ErrorName returns error name.
func (e BadPluginConfigValidationError) ErrorName() string { return "BadPluginConfigValidationError" }

// Error satisfies the builtin error interface
func (e BadPluginConfigValidationError) Error() string {
	cause := ""
	if e.cause != nil {
		cause = fmt.Sprintf(" | caused by: %v", e.cause)
	}

	key := ""
	if e.key {
		key = "key for "
	}

	return fmt.Sprintf(
		"invalid %sBadPluginConfig.%s: %s%s",
		key,
		e.field,
		e.reason,
		cause)
}

var _ error = BadPluginConfigValidationError{}

var _ interface {
	Field() string
	Reason() string
	Key() bool
	Cause() error
	ErrorName() string
} = BadPluginConfigValidationError{}

// Validate checks the field values on ConsumerConfig with the rules defined in
// the proto definition for this message. If any rules are violated, the first
// error encountered is returned, or nil if there are no violations.
func (m *ConsumerConfig) Validate() error {
	return m.validate(false)
}

// ValidateAll checks the field values on ConsumerConfig with the rules defined
// in the proto definition for this message. If any rules are violated, the
// result is a list of violation errors wrapped in ConsumerConfigMultiError,
// or nil if none found.
func (m *ConsumerConfig) ValidateAll() error {
	return m.validate(true)
}

func (m *ConsumerConfig) validate(all bool) error {
	if m == nil {
		return nil
	}

	var errors []error

	// no validation rules for Name

	if len(errors) > 0 {
		return ConsumerConfigMultiError(errors)
	}

	return nil
}

// ConsumerConfigMultiError is an error wrapping multiple validation errors
// returned by ConsumerConfig.ValidateAll() if the designated constraints
// aren't met.
type ConsumerConfigMultiError []error

// Error returns a concatenation of all the error messages it wraps.
func (m ConsumerConfigMultiError) Error() string {
	var msgs []string
	for _, err := range m {
		msgs = append(msgs, err.Error())
	}
	return strings.Join(msgs, "; ")
}

// AllErrors returns a list of validation violation errors.
func (m ConsumerConfigMultiError) AllErrors() []error { return m }

// ConsumerConfigValidationError is the validation error returned by
// ConsumerConfig.Validate if the designated constraints aren't met.
type ConsumerConfigValidationError struct {
	field  string
	reason string
	cause  error
	key    bool
}

// Field function returns field value.
func (e ConsumerConfigValidationError) Field() string { return e.field }

// Reason function returns reason value.
func (e ConsumerConfigValidationError) Reason() string { return e.reason }

// Cause function returns cause value.
func (e ConsumerConfigValidationError) Cause() error { return e.cause }

// Key function returns key value.
func (e ConsumerConfigValidationError) Key() bool { return e.key }

// ErrorName returns error name.
func (e ConsumerConfigValidationError) ErrorName() string { return "ConsumerConfigValidationError" }

// Error satisfies the builtin error interface
func (e ConsumerConfigValidationError) Error() string {
	cause := ""
	if e.cause != nil {
		cause = fmt.Sprintf(" | caused by: %v", e.cause)
	}

	key := ""
	if e.key {
		key = "key for "
	}

	return fmt.Sprintf(
		"invalid %sConsumerConfig.%s: %s%s",
		key,
		e.field,
		e.reason,
		cause)
}

var _ error = ConsumerConfigValidationError{}

var _ interface {
	Field() string
	Reason() string
	Key() bool
	Cause() error
	ErrorName() string
} = ConsumerConfigValidationError{}
