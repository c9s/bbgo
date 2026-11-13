package bbgo

import (
	"context"
	"os"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/service"
)

// resetNotifierViperFlags clears the viper flags that
// ConfigureNotificationSystem reads to decide whether any notifier is
// configured, and restores them afterwards so other tests are not affected.
func resetNotifierViperFlags(t *testing.T) {
	keys := []string{"telegram-bot-token", "slack-bot-token", "slack-token"}
	original := make(map[string]interface{}, len(keys))
	for _, k := range keys {
		original[k] = viper.Get(k)
		viper.Set(k, "")
	}

	t.Cleanup(func() {
		for _, k := range keys {
			viper.Set(k, original[k])
		}
	})
}

// TestConfigureNotificationSystem_NoNotifierConfigured covers issue #2209:
// when no notifier env (telegram/slack token) and no notifications config
// block are present, the notification bootstrap must not initialize the
// interaction/OTP subsystem, since that subsystem only exists to authorize a
// messenger, and there is no messenger without a telegram bot token. In
// particular it must not touch the persistence layer, since doing so on a
// misconfigured or unavailable persistence backend is what previously
// crashed with "dial tcp :0".
func TestConfigureNotificationSystem_NoNotifierConfigured(t *testing.T) {
	resetNotifierViperFlags(t)

	mem := service.NewMemoryService()
	isolation := NewIsolation(&service.PersistenceServiceFacade{Memory: mem})
	ctx := NewContextWithIsolation(context.Background(), isolation)

	environ := NewEnvironment()
	userConfig := &Config{}

	err := environ.ConfigureNotificationSystem(ctx, userConfig)
	assert.NoError(t, err)
	assert.Empty(t, mem.Slots, "notification bootstrap should not touch persistence when no notifier is configured")
}

// TestSetupInteraction_PersistsOTPKey ensures the interaction (OTP/auth)
// subsystem, when it does run (i.e. the path taken when telegram is
// configured), still persists the OTP auth key as before. This guards
// against over-broad gating that would silently disable the auth flow for
// users who do use telegram notifications. It exercises setupInteraction
// directly, bypassing setupTelegram/telebot.NewBot, which performs a real
// network call to the Telegram API and is out of scope for this fix.
func TestSetupInteraction_PersistsOTPKey(t *testing.T) {
	otpImagePath := "otp.png"
	t.Cleanup(func() { os.Remove(otpImagePath) })

	mem := service.NewMemoryService()
	environ := NewEnvironment()

	err := environ.setupInteraction(mem)
	assert.NoError(t, err)
	assert.NotEmpty(t, mem.Slots, "OTP auth key should be persisted when the interaction subsystem runs")
}
