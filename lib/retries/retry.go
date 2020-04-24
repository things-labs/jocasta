package retries

import (
	"context"
	"errors"
	"strings"
	"time"
)

var ErrClosed = errors.New("use of closed network connection")

type Retries struct {
	Delay    time.Duration // 重试延时时间
	MaxCount int           // 为0表示无限重试
}

func (sf Retries) Do(ctx context.Context, f func() error) error {
	t := time.NewTimer(sf.Delay)
	defer t.Stop()
	count := 0
	for {
		err := f()
		if err == nil {
			return nil
		}
		if strings.Contains(err.Error(), "use of closed network connection") {
			return err
		}
		if count++; sf.MaxCount > 0 && count >= sf.MaxCount {
			return errors.New("reach max count")
		}

		t.Reset(sf.Delay)
		select {
		case <-ctx.Done():
			return ErrClosed
		case <-t.C:
		}
	}
}
