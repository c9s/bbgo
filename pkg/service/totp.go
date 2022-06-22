package service

import (
	"os"

	"github.com/pkg/errors"
	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

func NewDefaultTotpKey() (*otp.Key, error) {
	if keyURL := viper.GetString("totp-key-url"); len(keyURL) > 0 {
		return otp.NewKeyFromURL(keyURL)
	}

	// The issuer parameter is a string value indicating the provider or service this account is associated with, URL-encoded according to RFC 3986.
	// If the issuer parameter is absent, issuer information may be taken from the issuer prefix of the label.
	// If both issuer parameter and issuer label prefix are present, they should be equal.
	// Valid values corresponding to the label prefix examples above would be: issuer=Example, issuer=Provider1, and issuer=Big%20Corporation.
	totpIssuer := viper.GetString("totp-issuer")
	totpAccountName := viper.GetString("totp-account-name")

	if len(totpIssuer) == 0 {
		hostname, err := os.Hostname()
		if err != nil {
			return nil, errors.Wrapf(err, "can not get hostname from os for totp issuer")
		}
		totpIssuer = hostname
	}

	if len(totpAccountName) == 0 {

		//unix like os
		user, ok := os.LookupEnv("USER")
		if !ok {
			user, ok = os.LookupEnv("USERNAME")
		}

		if !ok {
			log.Warnf("can not get USER or USERNAME env var, use default name 'bbgo' for totp account name")
			user = "bbgo"
		}

		totpAccountName = user
	}

	return totp.Generate(totp.GenerateOpts{
		Issuer:      totpIssuer,
		AccountName: totpAccountName,
	})
}
