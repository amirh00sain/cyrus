package ui

import (
	"log"

	"fyne.io/fyne/v2"
)

// uiDo schedules fn on Fyne's UI thread and recovers panics. On Android an
// unrecovered panic on any thread terminates the app process, so every async
// UI update goes through this wrapper.
func uiDo(fn func()) {
	fyne.Do(func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("recovered panic in UI update: %v", r)
			}
		}()
		fn()
	})
}
