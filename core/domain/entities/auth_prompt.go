package entities

// AuthPrompt represents a prompt-response pair during authentication
type AuthPrompt struct {
	WaitFor string // prompt to wait for
	SendCmd string // command to send (empty means just wait)
}
