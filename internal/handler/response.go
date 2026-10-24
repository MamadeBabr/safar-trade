// Package handler implements the HTTP transport layer (controllers and routing).
//
// Node comparison:
// In Express, you write `res.status(200).json(data)` or `res.status(400).json({ error })`.
// In Go net/http, response formatting is handled by composing standard functions:
// setting headers, writing status codes, and encoding structs with encoding/json.
package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/go-playground/validator/v10"
)

// StandardErrorResponse represents the uniform JSON error format across all endpoints.
type StandardErrorResponse struct {
	Error ErrorDetails `json:"error"`
}

// ErrorDetails contains error message, HTTP status code, and optional field-level validation details.
type ErrorDetails struct {
	Message string `json:"message"`
	Code    int    `json:"code"`
	Details any    `json:"details,omitempty"`
}

// WriteJSON serializes any data structure to the response as JSON with the given HTTP status code.
func WriteJSON(w http.ResponseWriter, status int, data any) error {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)

	// In Go, streaming the JSON directly to the ResponseWriter via json.NewEncoder is more
	// memory-efficient than buffering with json.Marshal.
	return json.NewEncoder(w).Encode(data)
}

// WriteError formats and writes a consistent JSON error response.
func WriteError(w http.ResponseWriter, status int, message string, details any) {
	resp := StandardErrorResponse{
		Error: ErrorDetails{
			Message: message,
			Code:    status,
			Details: details,
		},
	}
	_ = WriteJSON(w, status, resp)
}

// ReadJSON safely decodes an incoming JSON request body into the target pointer.
// It limits request payload size to 1MB to prevent Denial of Service (OOM) attacks.
func ReadJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	maxBytes := int64(1048576) // 1MB
	r.Body = http.MaxBytesReader(w, r.Body, maxBytes)

	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields() // Reject unexpected JSON properties

	if err := dec.Decode(dst); err != nil {
		var syntaxError *json.SyntaxError
		var unmarshalTypeError *json.UnmarshalTypeError

		switch {
		case errors.As(err, &syntaxError):
			return fmt.Errorf("malformed JSON at character %d", syntaxError.Offset)
		case errors.Is(err, io.ErrUnexpectedEOF):
			return errors.New("malformed JSON payload")
		case errors.As(err, &unmarshalTypeError):
			return fmt.Errorf("invalid type for field %q at character %d", unmarshalTypeError.Field, unmarshalTypeError.Offset)
		case errors.Is(err, io.EOF):
			return errors.New("request body cannot be empty")
		default:
			return err
		}
	}

	// Verify that the body only contains a single JSON value
	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("request body must contain only a single JSON object")
	}

	return nil
}

// FormatValidationErrors converts validator.ValidationErrors into a clean map of field -> message.
func FormatValidationErrors(err error) map[string]string {
	fieldErrors := make(map[string]string)
	var valErrors validator.ValidationErrors
	if errors.As(err, &valErrors) {
		for _, fe := range valErrors {
			switch fe.Tag() {
			case "required":
				fieldErrors[fe.Field()] = fmt.Sprintf("%s is required", fe.Field())
			case "min":
				fieldErrors[fe.Field()] = fmt.Sprintf("%s must be at least %s characters", fe.Field(), fe.Param())
			case "max":
				fieldErrors[fe.Field()] = fmt.Sprintf("%s must be at most %s characters", fe.Field(), fe.Param())
			case "email":
				fieldErrors[fe.Field()] = fmt.Sprintf("%s must be a valid email", fe.Field())
			case "gt":
				fieldErrors[fe.Field()] = fmt.Sprintf("%s must be greater than %s", fe.Field(), fe.Param())
			default:
				fieldErrors[fe.Field()] = fmt.Sprintf("%s failed on rule '%s'", fe.Field(), fe.Tag())
			}
		}
	}
	return fieldErrors
}
