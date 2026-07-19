package contracts

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

const maxJSONDepth = 64

func rejectDuplicateKeys(data []byte, recursive bool) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	if err := scanJSONValue(dec, recursive); err != nil {
		return err
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return invalidJSONBody()
	}
	return nil
}

func rejectBatchEnvelopeDuplicateKeys(data []byte) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	if err := beginJSONObject(dec); err != nil {
		return err
	}
	seen := map[string]struct{}{}
	for dec.More() {
		key, err := uniqueJSONKey(dec, seen)
		if err != nil {
			return err
		}
		if key == "events" {
			if err := skipJSONValue(dec); err != nil {
				return err
			}
			continue
		}
		if err := scanJSONValueAtDepth(dec, true, 0); err != nil {
			return err
		}
	}
	return finishJSONObject(dec)
}

func scanJSONValue(dec *json.Decoder, recursive bool) error {
	return scanJSONValueAtDepth(dec, recursive, 0)
}

func scanJSONValueAtDepth(dec *json.Decoder, recursive bool, depth int) error {
	if depth > maxJSONDepth {
		return nestingError()
	}
	token, err := dec.Token()
	if err != nil {
		return err
	}
	delim, ok := token.(json.Delim)
	if !ok {
		return nil
	}
	if delim == '{' {
		return scanJSONObject(dec, recursive, depth)
	}
	if delim == '[' {
		return scanJSONArray(dec, recursive, depth)
	}
	return fmt.Errorf("unexpected JSON delimiter")
}

func scanJSONObject(dec *json.Decoder, recursive bool, depth int) error {
	seen := map[string]struct{}{}
	for dec.More() {
		if _, err := uniqueJSONKey(dec, seen); err != nil {
			return err
		}
		if recursive {
			if err := scanJSONValueAtDepth(dec, true, depth+1); err != nil {
				return err
			}
		} else if err := skipJSONValue(dec); err != nil {
			return err
		}
	}
	_, err := dec.Token()
	return err
}

func scanJSONArray(dec *json.Decoder, recursive bool, depth int) error {
	for dec.More() {
		if err := scanJSONValueAtDepth(dec, recursive, depth+1); err != nil {
			return err
		}
	}
	_, err := dec.Token()
	return err
}

func skipJSONValue(dec *json.Decoder) error {
	return skipJSONValueAtDepth(dec, 0)
}

func skipJSONValueAtDepth(dec *json.Decoder, depth int) error {
	if depth > maxJSONDepth {
		return nestingError()
	}
	token, err := dec.Token()
	if err != nil {
		return err
	}
	delim, ok := token.(json.Delim)
	if !ok {
		return nil
	}
	for dec.More() {
		if delim == '{' {
			if _, err := dec.Token(); err != nil {
				return err
			}
		}
		if err := skipJSONValueAtDepth(dec, depth+1); err != nil {
			return err
		}
	}
	_, err = dec.Token()
	return err
}

func beginJSONObject(dec *json.Decoder) error {
	token, err := dec.Token()
	if err != nil {
		return err
	}
	if token != json.Delim('{') {
		return &ValidationError{Code: CodeInvalidRequest, Message: "batch envelope must be a JSON object"}
	}
	return nil
}

func uniqueJSONKey(dec *json.Decoder, seen map[string]struct{}) (string, error) {
	token, err := dec.Token()
	if err != nil {
		return "", err
	}
	key, ok := token.(string)
	if !ok {
		return "", fmt.Errorf("object key must be a string")
	}
	if _, exists := seen[key]; exists {
		return "", &ValidationError{Code: CodeInvalidRequest, Message: "duplicate JSON key: " + key}
	}
	seen[key] = struct{}{}
	return key, nil
}

func finishJSONObject(dec *json.Decoder) error {
	if _, err := dec.Token(); err != nil {
		return err
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return invalidJSONBody()
	}
	return nil
}

func nestingError() error {
	return &ValidationError{Code: CodeInvalidRequest, Message: "JSON nesting exceeds limit"}
}

func invalidJSONBody() error {
	return &ValidationError{Code: CodeInvalidRequest, Message: "body must contain exactly one JSON value"}
}
