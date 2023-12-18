package energy

import (
	"log"
)

type LoggerWriter struct {
	logger *log.Logger
}

func NewLoggerWriter(logger *log.Logger) LoggerWriter {
	return LoggerWriter{
		logger: logger,
	}
}

func (w LoggerWriter) WriteReadings(r []Reading) error {
	for _, el := range r {
		w.logger.Println(el)
	}
	return nil
}
