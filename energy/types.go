package energy

import "fmt"

type ReadingType string
type CommodityType string

const (
	GAS         CommodityType = "gas"
	ELECTRICITY               = "electricity"
)

const (
	LIVE  ReadingType = "live"
	METER             = "meter"
)

type Reading struct {
	Commodity   CommodityType
	ReadingType ReadingType
	Value       float64
}

func (r Reading) String() string {
	return fmt.Sprintf("%v (%v): %.0f", r.Commodity, r.ReadingType, r.Value)
}
