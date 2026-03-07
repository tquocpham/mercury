package config

import (
	"context"
	"log"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
)

// LiveInt is an integer config value backed by an SSM parameter.
// It polls SSM on a fixed interval so changes take effect without a restart.
// Reads are lock-free via atomic — safe to call on every request.
type LiveInt struct {
	val atomic.Int64
}

// NewLiveInt fetches the SSM parameter immediately, then polls on interval.
// If the initial fetch fails, defaultVal is used. If a subsequent poll fails,
// the previous value is kept and a warning is logged.
// Polling stops when ctx is cancelled.
func NewLiveInt(
	ctx context.Context,
	client *ssm.Client,
	param string,
	defaultVal int,
	interval time.Duration,
) *LiveInt {
	l := &LiveInt{}
	l.val.Store(int64(defaultVal))

	if v, err := ssmGetInt(ctx, client, param); err == nil {
		l.val.Store(int64(v))
	} else {
		log.Printf("[config] initial fetch of %s failed, using default %d: %v", param, defaultVal, err)
	}

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if v, err := ssmGetInt(ctx, client, param); err == nil {
					l.val.Store(int64(v))
				} else {
					log.Printf("[config] poll of %s failed, keeping current value: %v", param, err)
				}
			}
		}
	}()

	return l
}

// Get returns the current value. Safe to call concurrently.
func (l *LiveInt) Get() int {
	return int(l.val.Load())
}

func ssmGetInt(ctx context.Context, client *ssm.Client, param string) (int, error) {
	out, err := client.GetParameter(ctx, &ssm.GetParameterInput{
		Name: aws.String(param),
	})
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(aws.ToString(out.Parameter.Value))
}
