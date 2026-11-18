//go:build linux && !vivagtk

package main

// Viva requests native phases from IBus and falls back to untagged events on refusal.
const directX11Composition = true
