// +heroku install ./cmd/bbgo

module github.com/c9s/bbgo

go 1.21

toolchain go1.21.6

require (
	github.com/DATA-DOG/go-sqlmock v1.5.0
	github.com/Masterminds/squirrel v1.5.3
	github.com/adshao/go-binance/v2 v2.5.0
	github.com/c-bata/goptuna v0.8.1
	github.com/c9s/requestgen v1.3.6
	github.com/c9s/rockhopper/v2 v2.0.4
	github.com/cenkalti/backoff/v4 v4.2.0
	github.com/cheggaaa/pb/v3 v3.0.8
	github.com/codingconcepts/env v0.0.0-20200821220118-a8fbf8d84482
	github.com/evanphx/json-patch/v5 v5.6.0
	github.com/fatih/camelcase v1.0.0
	github.com/fatih/color v1.14.1
	github.com/gertd/go-pluralize v0.2.1
	github.com/gin-contrib/cors v1.3.1
	github.com/gin-gonic/gin v1.9.1
	github.com/go-redis/redis/v8 v8.8.0
	github.com/go-sql-driver/mysql v1.8.1
	github.com/gofrs/flock v0.8.1
	github.com/google/uuid v1.4.0
	github.com/gorilla/websocket v1.5.1
	github.com/heroku/rollrus v0.2.0
	github.com/jedib0t/go-pretty/v6 v6.5.8
	github.com/jmoiron/sqlx v1.3.4
	github.com/joho/godotenv v1.3.0
	github.com/leekchan/accounting v0.0.0-20191218023648-17a4ce5f94d4
	github.com/lestrrat-go/file-rotatelogs v2.2.0+incompatible
	github.com/mattn/go-shellwords v1.0.12
	github.com/muesli/clusters v0.0.0-20180605185049-a07a36e67d36
	github.com/muesli/kmeans v0.3.0
	github.com/pkg/errors v0.9.1
	github.com/pquerna/otp v1.3.0
	github.com/prometheus/client_golang v1.11.1
	github.com/rifflock/lfshook v0.0.0-20180920164130-b9218ef580f5
	github.com/robfig/cron/v3 v3.0.0
	github.com/sajari/regression v1.0.1
	github.com/sirupsen/logrus v1.9.3
	github.com/slack-go/slack v0.10.1
	github.com/spf13/cobra v1.8.0
	github.com/spf13/pflag v1.0.5
	github.com/spf13/viper v1.18.2
	github.com/stretchr/testify v1.9.0
	github.com/valyala/fastjson v1.5.1
	github.com/wcharczuk/go-chart/v2 v2.1.0
	github.com/webview/webview v0.0.0-20210216142346-e0bfdf0e5d90
	github.com/x-cray/logrus-prefixed-formatter v0.5.2
	github.com/zserge/lorca v0.1.9
	go.uber.org/mock v0.4.0
	go.uber.org/multierr v1.11.0
	golang.org/x/oauth2 v0.15.0
	golang.org/x/sync v0.7.0
	golang.org/x/time v0.5.0
	gonum.org/v1/gonum v0.8.2
	google.golang.org/api v0.153.0
	google.golang.org/grpc v1.59.0
	google.golang.org/protobuf v1.31.0
	gopkg.in/tucnak/telebot.v2 v2.5.0
	gopkg.in/yaml.v3 v3.0.1
)

require (
	cloud.google.com/go/compute v1.23.3 // indirect
	cloud.google.com/go/compute/metadata v0.2.3 // indirect
	filippo.io/edwards25519 v1.1.0 // indirect
	github.com/VividCortex/ewma v1.1.1 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/bitly/go-simplejson v0.5.1 // indirect
	github.com/boombuler/barcode v1.0.1-0.20190219062509-6c824513bacc // indirect
	github.com/bytedance/sonic v1.9.1 // indirect
	github.com/cespare/xxhash/v2 v2.2.0 // indirect
	github.com/chenzhuoyu/base64x v0.0.0-20221115062448-fe3a3abad311 // indirect
	github.com/cockroachdb/apd v1.1.0 // indirect
	github.com/cpuguy83/go-md2man/v2 v2.0.3 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/denisenkom/go-mssqldb v0.12.3 // indirect
	github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f // indirect
	github.com/fastly/go-utils v0.0.0-20180712184237-d95a45783239 // indirect
	github.com/fsnotify/fsnotify v1.7.0 // indirect
	github.com/gabriel-vasile/mimetype v1.4.2 // indirect
	github.com/gin-contrib/sse v0.1.0 // indirect
	github.com/go-playground/locales v0.14.1 // indirect
	github.com/go-playground/universal-translator v0.18.1 // indirect
	github.com/go-playground/validator/v10 v10.14.0 // indirect
	github.com/go-test/deep v1.0.6 // indirect
	github.com/goccy/go-json v0.10.2 // indirect
	github.com/golang-sql/civil v0.0.0-20220223132316-b832511892a9 // indirect
	github.com/golang-sql/sqlexp v0.1.0 // indirect
	github.com/golang/freetype v0.0.0-20170609003504-e2365dfdc4a0 // indirect
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/golang/protobuf v1.5.3 // indirect
	github.com/google/s2a-go v0.1.7 // indirect
	github.com/googleapis/enterprise-certificate-proxy v0.3.2 // indirect
	github.com/googleapis/gax-go/v2 v2.12.0 // indirect
	github.com/hashicorp/hcl v1.0.0 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/jehiah/go-strftime v0.0.0-20171201141054-1d33003b3869 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/klauspost/cpuid/v2 v2.2.4 // indirect
	github.com/lann/builder v0.0.0-20180802200727-47ae307949d0 // indirect
	github.com/lann/ps v0.0.0-20150810152359-62de8c46ede0 // indirect
	github.com/leodido/go-urn v1.2.4 // indirect
	github.com/lestrrat-go/strftime v1.0.0 // indirect
	github.com/lib/pq v1.10.9 // indirect
	github.com/magiconair/properties v1.8.7 // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/mattn/go-runewidth v0.0.15 // indirect
	github.com/mattn/go-sqlite3 v1.14.22 // indirect
	github.com/matttproud/golang_protobuf_extensions v1.0.1 // indirect
	github.com/mgutz/ansi v0.0.0-20200706080929-d51e80ef957d // indirect
	github.com/mitchellh/mapstructure v1.5.0 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/nxadm/tail v1.4.8 // indirect
	github.com/pelletier/go-toml/v2 v2.2.0 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/prometheus/client_model v0.2.0 // indirect
	github.com/prometheus/common v0.32.1 // indirect
	github.com/prometheus/procfs v0.7.3 // indirect
	github.com/rivo/uniseg v0.4.7 // indirect
	github.com/rollbar/rollbar-go v1.4.5 // indirect
	github.com/russross/blackfriday/v2 v2.1.0 // indirect
	github.com/sagikazarmark/locafero v0.4.0 // indirect
	github.com/sagikazarmark/slog-shim v0.1.0 // indirect
	github.com/shopspring/decimal v1.2.0 // indirect
	github.com/sourcegraph/conc v0.3.0 // indirect
	github.com/spf13/afero v1.11.0 // indirect
	github.com/spf13/cast v1.6.0 // indirect
	github.com/subosito/gotenv v1.6.0 // indirect
	github.com/tebeka/strftime v0.1.3 // indirect
	github.com/twitchyliquid64/golang-asm v0.15.1 // indirect
	github.com/ugorji/go/codec v1.2.11 // indirect
	github.com/ziutek/mymysql v1.5.4 // indirect
	go.opencensus.io v0.24.0 // indirect
	go.opentelemetry.io/otel v0.19.0 // indirect
	go.opentelemetry.io/otel/metric v0.19.0 // indirect
	go.opentelemetry.io/otel/trace v0.19.0 // indirect
	golang.org/x/arch v0.3.0 // indirect
	golang.org/x/crypto v0.22.0 // indirect
	golang.org/x/exp v0.0.0-20240404231335-c0f41cb1a7a0 // indirect
	golang.org/x/image v0.5.0 // indirect
	golang.org/x/mod v0.17.0 // indirect
	golang.org/x/net v0.24.0 // indirect
	golang.org/x/sys v0.19.0 // indirect
	golang.org/x/term v0.19.0 // indirect
	golang.org/x/text v0.14.0 // indirect
	golang.org/x/tools v0.20.0 // indirect
	google.golang.org/appengine v1.6.7 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20231120223509-83a465c0220f // indirect
	gopkg.in/ini.v1 v1.67.0 // indirect
)
