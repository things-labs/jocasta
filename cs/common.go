package cs

import (
	"net"
	"strconv"

	"github.com/thinkgos/jocasta/lib/logger"
)

type common struct {
	ip     string
	port   int
	status chan error
	log    logger.Logger
}

func newCommon(addr string) (common, error) {
	h, port, err := net.SplitHostPort(addr)
	if err != nil {
		return common{}, err
	}
	p, _ := strconv.Atoi(port)
	return common{
		h,
		p,
		make(chan error, 1),
		logger.NewDiscard(),
	}, nil
}

func (s *common) Status() <-chan error {
	return s.status
}

func (s *common) SetLogger(log logger.Logger) {
	if log != nil {
		s.log = log
	}
}
