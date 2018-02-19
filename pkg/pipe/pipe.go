package pipe

import (
	"fmt"
	"time"

	"github.com/playnet-public/libs/log"

	"go.uber.org/zap"
)

// Message provides a generic message type
type Message struct {
	Author    string
	Timestamp time.Time
	Source    string
	Content   string
}

func (m *Message) String() string {
	return fmt.Sprintf("%s | %s: (%s) %s", m.Timestamp.String(), m.Source, m.Author, m.Content)
}

// End provides an interface for channel endpoints
type End interface {
	Connect() error
	Write(*Message) error
	Listen(Hook)
}

// Hook for MessageHandling
type Hook func(*Message) error

// Pipe syncs messages between to points
type Pipe struct {
	Input  End
	Output End

	log *log.Logger
}

// New Pipe for syncing all input events to define hooks
func New(log *log.Logger, input, output End) *Pipe {
	return &Pipe{
		Input:  input,
		Output: output,
		log:    log,
	}
}

// Open the pipe to allow messages to pass
func (p *Pipe) Open() (err error) {
	p.log.Info("connecting input")
	err = p.Input.Connect()
	if err != nil {
		p.log.Debug("connection error", zap.String("end", "input"), zap.Error(err))
		return err
	}
	p.log.Info("connecting output")
	err = p.Output.Connect()
	if err != nil {
		p.log.Debug("connection error", zap.String("end", "output"), zap.Error(err))
		return err
	}
	p.log.Info("listening")
	p.Input.Listen(p.Output.Write)

	return nil
}
