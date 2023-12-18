package energy

type EnergyDataReader interface {
	GetLiveReadings() ([]Reading, error)
	GetMeterReadings() ([]Reading, error)
}
