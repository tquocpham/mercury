module github.com/mercury/cmd/worker

go 1.25.5

require (
	github.com/gocql/gocql v1.7.0
	github.com/google/uuid v1.6.0
	github.com/mercury/pkg v0.0.0-00010101000000-000000000000
	github.com/segmentio/kafka-go v0.4.50
	github.com/sirupsen/logrus v1.9.4
)

require (
	github.com/golang/snappy v0.0.3 // indirect
	github.com/hailocab/go-hostpool v0.0.0-20160125115350-e80d13ce29ed // indirect
	github.com/klauspost/compress v1.15.9 // indirect
	github.com/pierrec/lz4/v4 v4.1.15 // indirect
	github.com/stretchr/testify v1.11.1 // indirect
	golang.org/x/sys v0.39.0 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
)

replace github.com/mercury/pkg => ../../pkg
