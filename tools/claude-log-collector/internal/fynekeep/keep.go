// Package fynekeep pins fyne.io/fyne/v2 in go.mod so GUI builds stay reproducible
// even when cmd/gui is excluded by the cgo build tag.
package fynekeep

import _ "fyne.io/fyne/v2"
