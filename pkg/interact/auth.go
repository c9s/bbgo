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

type Authorizer interface {
	Authorize() error
}

type AuthInteract struct {
	Strict bool `json:"strict,omitempty"`

	Mode AuthMode `json:"authMode"`

	Token string `json:"authToken,omitempty"`

	OneTimePasswordKey *otp.Key `json:"otpKey,omitempty"`
}

func (it *AuthInteract) Commands(interact *Interact) {
	if it.Strict {
		interact.Command("/auth", func(reply Reply) error {
			reply.Message("Enter your authentication token")
			return nil
		}).Next(func(token string, reply Reply) error {
			if token == it.Token {
				reply.Message("Token passed, please enter your one-time password")
				return nil
			}
			return ErrAuthenticationFailed
		}).NamedNext(StateAuthenticated, func(code string, reply Reply, authorizer Authorizer) error {
			if totp.Validate(code, it.OneTimePasswordKey.Secret()) {
				reply.Message("Great! You're authenticated!")
				return authorizer.Authorize()
			}

			reply.Message("Incorrect authentication code")
			return ErrAuthenticationFailed
		})
	} else {
		interact.Command("/auth", func(reply Reply) error {
			reply.Message("Enter your authentication code")
			return nil
		}).NamedNext(StateAuthenticated, func(code string, reply Reply, authorizer Authorizer) error {
			switch it.Mode {
			case AuthModeToken:
				if code == it.Token {
					reply.Message("Great! You're authenticated!")
					return authorizer.Authorize()
				}

			case AuthModeOTP:
				if totp.Validate(code, it.OneTimePasswordKey.Secret()) {
					reply.Message("Great! You're authenticated!")
					return authorizer.Authorize()
				}
			}

			reply.Message("Incorrect authentication code")
			return ErrAuthenticationFailed
		})
	}

}
