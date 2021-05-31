// +heroku install ./cmd/bbgo

module github.com/c9s/bbgo

go 1.13

require (
	github.com/DATA-DOG/go-sqlmock v1.5.0
	github.com/adshao/go-binance/v2 v2.2.2
	github.com/c-bata/go-prompt v0.2.6 // indirect
	github.com/c9s/rockhopper v1.2.1-0.20210217093258-2661955904a9
	github.com/codingconcepts/env v0.0.0-20200821220118-a8fbf8d84482
	github.com/fastly/go-utils v0.0.0-20180712184237-d95a45783239 // indirect
	github.com/gin-contrib/cors v1.3.1
	github.com/gin-gonic/gin v1.6.3
	github.com/go-playground/validator/v10 v10.4.1 // indirect
	github.com/go-redis/redis/v8 v8.8.0
	github.com/go-sql-driver/mysql v1.5.0
	github.com/go-test/deep v1.0.6 // indirect
	github.com/golang/protobuf v1.4.3 // indirect
	github.com/google/uuid v1.1.2
	github.com/gorilla/websocket v1.4.2
	github.com/jehiah/go-strftime v0.0.0-20171201141054-1d33003b3869 // indirect
	github.com/jmoiron/sqlx v1.2.0
	github.com/joho/godotenv v1.3.0
	github.com/json-iterator/go v1.1.10 // indirect
	github.com/leekchan/accounting v0.0.0-20191218023648-17a4ce5f94d4
	github.com/leodido/go-urn v1.2.1 // indirect
	github.com/lestrrat-go/file-rotatelogs v2.2.0+incompatible
	github.com/lestrrat-go/strftime v1.0.0 // indirect
	github.com/magefile/mage v1.11.0 // indirect
	github.com/magiconair/properties v1.8.4 // indirect
	github.com/mattn/go-sqlite3 v2.0.3+incompatible // indirect
	github.com/mitchellh/mapstructure v1.4.1 // indirect
	github.com/pelletier/go-toml v1.8.1 // indirect
	github.com/pkg/errors v0.9.1
	github.com/pquerna/otp v1.3.0
	github.com/rifflock/lfshook v0.0.0-20180920164130-b9218ef580f5
	github.com/robfig/cron/v3 v3.0.0
	github.com/shopspring/decimal v1.2.0 // indirect
	github.com/sirupsen/logrus v1.7.1
	github.com/slack-go/slack v0.6.6-0.20200602212211-b04b8521281b
	github.com/spf13/afero v1.5.1 // indirect
	github.com/spf13/cast v1.3.1 // indirect
	github.com/spf13/cobra v1.1.1
	github.com/spf13/jwalterweatherman v1.1.0 // indirect
	github.com/spf13/pflag v1.0.5
	github.com/spf13/viper v1.7.1
	github.com/stretchr/testify v1.7.0
	github.com/tebeka/strftime v0.1.3 // indirect
	github.com/ugorji/go v1.2.3 // indirect
	github.com/valyala/fastjson v1.5.1
	github.com/webview/webview v0.0.0-20210216142346-e0bfdf0e5d90
	github.com/x-cray/logrus-prefixed-formatter v0.5.2
	github.com/zserge/lorca v0.1.9
	golang.org/x/crypto v0.0.0-20201221181555-eec23a3978ad // indirect
	golang.org/x/net v0.0.0-20210119194325-5f4716e94777 // indirect
	golang.org/x/sys v0.0.0-20210217090653-ed5674b6da4a // indirect
	golang.org/x/text v0.3.5 // indirect
	golang.org/x/time v0.0.0-20201208040808-7e3f01d25324
	gonum.org/v1/gonum v0.8.1
	google.golang.org/protobuf v1.25.0 // indirect
	gopkg.in/check.v1 v1.0.0-20190902080502-41f04d3bba15 // indirect
	gopkg.in/ini.v1 v1.62.0 // indirect
	gopkg.in/tucnak/telebot.v2 v2.3.5
	gopkg.in/yaml.v2 v2.4.0 // indirect
	gopkg.in/yaml.v3 v3.0.0-20210107192922-496545a6307b
)

//  replace (
//  	github.com/adshao/go-binance/v2 => ../../adshao/go-binance/v2
//  	github.com/c9s/rockhopper => ../rockhopper
//  )
