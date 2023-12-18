package energy

import (
	"log"
	"os"
)

type WriterStdout struct {
	logger *log.Logger
}

func NewWriterStdout() WriterStdout {
	return WriterStdout{
		logger: log.New(os.Stdout, "", log.LstdFlags),
	}
}

func (w WriterStdout) WriteReadings(r []Reading) error {
	for _, el := range r {
		w.logger.Println(el)
	}
	return nil
}
