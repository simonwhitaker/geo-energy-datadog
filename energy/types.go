package energy

import "fmt"

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

func (r ReadingType) String() string {
	switch r {
	case LIVE:
		return "live"
	case METER:
		return "meter"
	}
	return "unknown"
}

type Reading struct {
	Commodity   CommodityType
	ReadingType ReadingType
	Value       float64
}

func (r Reading) String() string {
	return fmt.Sprintf("%v (%v): %.0f", r.Commodity, r.ReadingType, r.Value)
}
