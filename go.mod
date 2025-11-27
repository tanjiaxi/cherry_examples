module github.com/cherry-game/examples

go 1.23.7

// 统一protobuf版本，解决冲突 - 使用兼容的旧版本
replace (
	// 降级其他可能冲突的库
	github.com/gin-gonic/gin => github.com/gin-gonic/gin v1.9.1
	github.com/golang/protobuf => github.com/golang/protobuf v1.4.3
	go.etcd.io/etcd/api/v3 => go.etcd.io/etcd/api/v3 v3.5.0
	go.etcd.io/etcd/client/pkg/v3 => go.etcd.io/etcd/client/pkg/v3 v3.5.0
	go.etcd.io/etcd/client/v3 => go.etcd.io/etcd/client/v3 v3.5.0
	google.golang.org/grpc => google.golang.org/grpc v1.40.0
	google.golang.org/protobuf => google.golang.org/protobuf v1.25.0
)

require (
	filippo.io/edwards25519 v1.1.0
	github.com/ahmetb/go-linq/v3 v3.2.0
	github.com/bytedance/sonic v1.14.0
	github.com/bytedance/sonic/loader v0.3.0
	github.com/cespare/xxhash/v2 v2.3.0
	github.com/cherry-game/cherry v1.4.10
	github.com/cherry-game/components/cron v1.4.1
	github.com/cherry-game/components/data-config v1.4.1
	github.com/cherry-game/components/etcd v1.4.1
	github.com/cherry-game/components/gin v1.4.1
	github.com/cherry-game/components/gops v1.4.1
	github.com/cloudwego/base64x v0.1.6
	github.com/coreos/go-semver v0.3.0
	github.com/coreos/go-systemd/v22 v22.3.2
	github.com/cpuguy83/go-md2man/v2 v2.0.7
	github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f
	github.com/gabriel-vasile/mimetype v1.4.8
	github.com/gin-contrib/sse v1.1.0
	github.com/gin-gonic/gin v1.11.0
	github.com/go-playground/locales v0.14.1
	github.com/go-playground/universal-translator v0.18.1
	github.com/go-playground/validator/v10 v10.27.0
	github.com/go-redis/redis/v8 v8.11.5
	github.com/go-sql-driver/mysql v1.8.1
	github.com/goburrow/cache v0.1.4
	github.com/goccy/go-json v0.10.2
	github.com/gogo/protobuf v1.3.2
	github.com/golang/protobuf v1.5.2
	github.com/google/gops v0.3.28
	github.com/google/uuid v1.6.0
	github.com/gorilla/websocket v1.5.0
	github.com/jackc/pgpassfile v1.0.0
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761
	github.com/jackc/pgx/v5 v5.6.0
	github.com/jackc/puddle/v2 v2.2.2
	github.com/jinzhu/inflection v1.0.0
	github.com/jinzhu/now v1.1.5
	github.com/json-iterator/go v1.1.12
	github.com/klauspost/compress v1.18.0
	github.com/klauspost/cpuid/v2 v2.3.0
	github.com/leodido/go-urn v1.4.0
	github.com/lestrrat-go/strftime v1.0.6
	github.com/mattn/go-isatty v0.0.20
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd
	github.com/modern-go/reflect2 v1.0.2
	github.com/nats-io/nats.go v1.47.0
	github.com/nats-io/nkeys v0.4.11
	github.com/nats-io/nuid v1.0.1
	github.com/pelletier/go-toml/v2 v2.2.4
	github.com/pkg/errors v0.9.1
	github.com/radovskyb/watcher v1.0.7
	github.com/robfig/cron/v3 v3.0.1
	github.com/russross/blackfriday/v2 v2.1.0
	github.com/spf13/cast v1.10.0
	github.com/tidwall/gjson v1.18.0
	github.com/twitchyliquid64/golang-asm v0.15.1
	github.com/ugorji/go/codec v1.3.0
	github.com/urfave/cli/v2 v2.27.7
	github.com/xrash/smetrics v0.0.0-20240521201337-686a1a2994c1
	go.etcd.io/etcd/api/v3 v3.5.9
	go.etcd.io/etcd/client/pkg/v3 v3.5.9
	go.etcd.io/etcd/client/v3 v3.5.9
	go.uber.org/multierr v1.11.0
	go.uber.org/zap v1.27.0
	golang.org/x/arch v0.20.0
	golang.org/x/crypto v0.40.0
	golang.org/x/mod v0.25.0
	golang.org/x/net v0.42.0
	golang.org/x/sync v0.16.0
	golang.org/x/sys v0.35.0
	golang.org/x/text v0.27.0
	golang.org/x/tools v0.34.0
	google.golang.org/genproto v0.0.0-20210602131652-f16073e35f0c
	google.golang.org/grpc v1.41.0
	google.golang.org/protobuf v1.36.10
	gopkg.in/yaml.v3 v3.0.1
	gorm.io/datatypes v1.2.4
	gorm.io/driver/mysql v1.6.0
	gorm.io/driver/postgres v1.6.0
	gorm.io/gen v0.3.27
	gorm.io/gorm v1.31.1
	gorm.io/hints v1.1.0
	gorm.io/plugin/dbresolver v1.6.2
)

require (
	github.com/DmitriyVTitov/size v1.5.0 // indirect
	github.com/jinzhu/copier v0.4.0 // indirect
	github.com/rogpeppe/go-internal v1.14.1 // indirect
	github.com/tidwall/match v1.1.1 // indirect
	github.com/tidwall/pretty v1.2.0 // indirect
)
