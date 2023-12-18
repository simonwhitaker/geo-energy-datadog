package energy

type ReadingType int
type CommodityType int

const (
	GAS CommodityType = iota
	ELECTRICITY
)

func (c CommodityType) String() string {
	switch c {
	case GAS:
		return "gas"
	case ELECTRICITY:
		return "electricity"
	}
	return "unknown"
}

const (
	LIVE ReadingType = iota
	METER
)

type Reading struct {
	Commodity   CommodityType
	ReadingType ReadingType
	Value       float64
}
