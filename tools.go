//go:build tools

// Package tools импортирует build-tools чтобы они попадали в go mod tidy.
// Подробнее: https://github.com/golang/go/wiki/Modules#how-can-i-track-tool-dependencies-for-a-module
package tools

import (
	_ "github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen"
)
