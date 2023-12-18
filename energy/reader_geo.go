package energy

import (
	"github.com/olivercullimore/geo-energy-data-client"
)

type GeoEnergyDataReader struct {
	username string
	password string
	systemID string
}

func NewGeoEnergyDataReader(username, password string) *GeoEnergyDataReader {
	return &GeoEnergyDataReader{
		username: username,
		password: password,
		systemID: "",
	}
}

func (r *GeoEnergyDataReader) getAccessToken() (string, error) {
	return geo.GetAccessToken(r.username, r.password)
}

func (r *GeoEnergyDataReader) getSystemID(accessToken string) (string, error) {
	if r.systemID == "" {
		// Get device data to get the system ID
		deviceData, err := geo.GetDeviceData(accessToken)
		if err != nil {
			return "", err
		}

		geoSystemID := deviceData.SystemDetails[0].SystemID
		r.systemID = geoSystemID
	}
	return r.systemID, nil
}

func (r *GeoEnergyDataReader) GetLiveReadings() ([]Reading, error) {
	result := []Reading{}

	accessToken, err := r.getAccessToken()
	if err != nil {
		return result, err
	}

	systemId, err := r.getSystemID(accessToken)
	if err != nil {
		return result, err
	}

	liveData, err := geo.GetLiveMeterData(accessToken, systemId)
	if err != nil {
		return result, err
	}

	for _, v := range liveData.Power {
		if v.ValueAvailable {
			switch v.Type {
			case "GAS_ENERGY":
				result = append(result, Reading{
					Commodity:   GAS,
					ReadingType: LIVE,
					Value:       v.Watts,
				})
			case "ELECTRICITY":
				result = append(result, Reading{
					Commodity:   ELECTRICITY,
					ReadingType: LIVE,
					Value:       v.Watts,
				})
			}
		}
	}

	return result, nil
}

func (r *GeoEnergyDataReader) GetMeterReadings() ([]Reading, error) {
	result := []Reading{}

	accessToken, err := r.getAccessToken()
	if err != nil {
		return result, err
	}

	systemId, err := r.getSystemID(accessToken)
	if err != nil {
		return result, err
	}

	periodicData, err := geo.GetPeriodicMeterData(accessToken, systemId)
	if err != nil {
		return result, err
	}

	for _, v := range periodicData.TotalConsumptionList {
		if v.ValueAvailable {
			switch v.CommodityType {
			case "GAS_ENERGY":
				result = append(result, Reading{
					Commodity:   GAS,
					ReadingType: LIVE,
					Value:       v.TotalConsumption,
				})
			case "ELECTRICITY":
				result = append(result, Reading{
					Commodity:   ELECTRICITY,
					ReadingType: LIVE,
					Value:       v.TotalConsumption,
				})
			}
		}
	}

	return result, nil
}
