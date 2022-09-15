package args

import (
	"sync"
)

var horusBuilder = &builder{holder: horusHolder}

type builder struct {
	l sync.RWMutex
	*holder
}

func (b *builder) SetKubeconfig(kubeconfig string) *builder {
	b.l.Lock()
	defer b.l.Unlock()
	b.kubeconfig = kubeconfig
	return b
}

func (b *builder) SetLogLevel(logLevel string) *builder {
	b.l.Lock()
	defer b.l.Unlock()
	b.logLevel = logLevel
	return b
}

func (b *builder) SetLogFormat(logFormat string) *builder {
	b.l.Lock()
	defer b.l.Unlock()
	b.logFormat = logFormat
	return b
}

func (b *builder) SetLogFile(logFile string) *builder {
	b.l.Lock()
	defer b.l.Unlock()
	b.logFile = logFile
	return b
}
func NewBuilder() *builder { return horusBuilder }
