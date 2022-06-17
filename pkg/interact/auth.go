package interact

import (
	"errors"
	"os"
	"time"

	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
	log "github.com/sirupsen/logrus"
)

type AuthMode string

const (
	AuthModeOTP   AuthMode = "OTP"
	AuthModeToken AuthMode = "TOKEN"
)

var ErrAuthenticationFailed = errors.New("authentication failed")

type Authorizer interface {
	StartAuthorizing()
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
		// generate a one-time-use otp
		// pragma: allowlist nextline secret
		if it.OneTimePasswordKey == nil {
			opts := totp.GenerateOpts{
				Issuer:      "interact",
				AccountName: os.Getenv("USER"),
				Period:      30,
			}
			log.Infof("[interact] one-time password key is not configured, generating one with %+v", opts)
			key, err := totp.Generate(opts)
			if err != nil {
				panic(err)
			}
			// pragma: allowlist nextline secret
			it.OneTimePasswordKey = key
		}
		interact.Command("/auth", "authorize", func(reply Reply, session Session) error {
			reply.Message("Please enter your authentication token")
			session.SetAuthorizing(true)
			return nil
		}).Next(func(token string, reply Reply) error {
			if token == it.Token {
				reply.Message("Token passed, please enter your one-time password")

				code, err := totp.GenerateCode(it.OneTimePasswordKey.Secret(), time.Now())
				if err != nil {
					return err
				}

				log.Infof("[interact] ======================================")
				log.Infof("[interact] your one-time password code: %s", code)
				log.Infof("[interact] ======================================")
				return nil
			}

			return ErrAuthenticationFailed
		}).NamedNext(StateAuthenticated, func(code string, reply Reply, session Session) error {
			if totp.Validate(code, it.OneTimePasswordKey.Secret()) {
				reply.Message("Great! You're authenticated!")
				session.SetOriginState(StateAuthenticated)
				session.SetAuthorized()
				return nil
			}

			reply.Message("Incorrect authentication code")
			return ErrAuthenticationFailed
		})
	} else {
		interact.Command("/auth", "authorize", func(reply Reply, session Session) error {
			switch it.Mode {
			case AuthModeToken:
				session.SetAuthorizing(true)
				reply.Message("Enter your authentication token")

			case AuthModeOTP:
				session.SetAuthorizing(true)
				reply.Message("Enter your one-time password")

			default:
				log.Warnf("unexpected auth mode: %s", it.Mode)
			}
			return nil
		}).NamedNext(StateAuthenticated, func(code string, reply Reply, session Session) error {
			switch it.Mode {
			case AuthModeToken:
				if code == it.Token {
					reply.Message("Great! You're authenticated!")
					session.SetOriginState(StateAuthenticated)
					session.SetAuthorized()
					return nil
				}
				reply.Message("Incorrect authentication token")

			case AuthModeOTP:
				if totp.Validate(code, it.OneTimePasswordKey.Secret()) {
					reply.Message("Great! You're authenticated!")
					session.SetOriginState(StateAuthenticated)
					session.SetAuthorized()
					return nil
				}
				reply.Message("Incorrect one-time pass code")

			default:
				log.Warnf("unexpected auth mode: %s", it.Mode)
			}

			return ErrAuthenticationFailed
		})
	}

}
