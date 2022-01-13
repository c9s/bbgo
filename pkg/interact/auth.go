package interact

import (
	"errors"

	"github.com/pquerna/otp"
)

type AuthMode string

const (
	AuthModeOTP   AuthMode = "OTP"
	AuthModeToken AuthMode = "TOKEN"
)

var ErrAuthenticationFailed = errors.New("authentication failed")

type AuthInteract struct {
	Mode AuthMode `json:"authMode"`

	Token string `json:"authToken,omitempty"`

	OneTimePasswordKey *otp.Key `json:"otpKey,omitempty"`
}

func (i *AuthInteract) Commands(interact *Interact) {
	interact.Command("/auth", func(reply Reply) error {
		reply.Message("Enter your authentication code")
		return nil
	}).NamedNext(StateAuthenticated, func(reply Reply, code string) error {
		switch i.Mode {
		case AuthModeToken:
			if code == i.Token {
				reply.Message("Great! You're authenticated!")
				return nil
			} else {
				reply.Message("Incorrect authentication code")
				return ErrAuthenticationFailed
			}

		case AuthModeOTP:

		}

		return nil
	})
}
