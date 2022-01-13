package interact

import (
	"errors"

	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
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

func (it *AuthInteract) Commands(interact *Interact) {
	interact.Command("/auth", func(reply Reply) error {
		reply.Message("Enter your authentication code")
		return nil
	}).NamedNext(StateAuthenticated, func(reply Reply, code string) error {
		switch it.Mode {
		case AuthModeToken:
			if code == it.Token {
				reply.Message("Great! You're authenticated!")
				return nil
			}

		case AuthModeOTP:
			if totp.Validate(code, it.OneTimePasswordKey.Secret()) {
				reply.Message("Great! You're authenticated!")
				return nil
			}
		}

		reply.Message("Incorrect authentication code")
		return ErrAuthenticationFailed
	})
}
