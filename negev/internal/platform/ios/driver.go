package ios

import (
	"github.com/carlosrabelo/negev/negev/internal/domain/entities"
)

type Driver struct{}

func (d *Driver) Name() string {
	return "ios"
}

func (d *Driver) GetAuthenticationSequence() []entities.AuthPrompt {
	return []entities.AuthPrompt{
		{WaitFor: "Username:", SendCmd: "USERNAME_PLACEHOLDER\n"},
		{WaitFor: "Password:", SendCmd: "PASSWORD_PLACEHOLDER\n"},
		{WaitFor: ">", SendCmd: "enable\n"},
		{WaitFor: "Password:", SendCmd: "ENABLE_PASSWORD_PLACEHOLDER\n"},
		{WaitFor: "#", SendCmd: "terminal length 0\n"},
		{WaitFor: "#", SendCmd: ""},
	}
}
