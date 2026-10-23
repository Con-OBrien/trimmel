package main

import (
	"os"

	"git.sr.ht/~jackmordaunt/go-toast/v2"
)

// showToast is the alerts.Notifier. The optional button just opens Trimmel
// (protocol activation on the exe path); it never cleans anything by itself.
func showToast(title, body string, withOpenButton bool) error {
	n := toast.Notification{AppID: "Trimmel", Title: title, Body: body}
	if exe, err := os.Executable(); err == nil && withOpenButton {
		n.Actions = []toast.Action{{Type: toast.Protocol, Content: "Open Trimmel", Arguments: exe}}
	}
	return n.Push()
}
