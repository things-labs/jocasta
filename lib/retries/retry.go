package assist

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
	count    int           // 计数
}

func (sf Retries) Do(ctx context.Context, f func() error) error {
	t := time.NewTimer(sf.Delay)
	defer t.Stop()
	for {
		if err := f(); err == nil {
			return nil
		} else if strings.Contains(err.Error(), "use of closed network connection") {
			return err
		}
		if sf.count++; sf.MaxCount > 0 && sf.count >= sf.MaxCount {
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
