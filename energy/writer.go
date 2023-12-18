package energy

type EnergyDataWriter interface {
	WriteReadings(r []Reading) error
}
