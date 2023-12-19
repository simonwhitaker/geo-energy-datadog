package energy

type EnergyDataReader interface {
	// GetLiveReeadings returns the current power reading for the commodity, in
	// watts.
	GetLiveReadings() ([]Reading, error)

	// GetMeterReadings returns the total energy consumption reading for the
	// commodity. This will typically be in kWh for electricity, m3 for gas.
	GetMeterReadings() ([]Reading, error)
}
