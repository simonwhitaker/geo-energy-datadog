package energy

import (
	"sync"
	"time"
	
	"github.com/olivercullimore/geo-energy-data-client"
)

type GeoEnergyDataReader struct {
	username string
	password string
	
	// Cache for system ID
	systemMu sync.RWMutex
	systemID string
	
	// Cache for access token
	tokenMu          sync.RWMutex
	cachedToken      string
	tokenExpiry      time.Time
}

func NewGeoEnergyDataReader(username, password string) *GeoEnergyDataReader {
	return &GeoEnergyDataReader{
		username: username,
		password: password,
	}
}


func (r *GeoEnergyDataReader) getAccessToken() (string, error) {
	r.tokenMu.RLock()
	if r.cachedToken != "" && time.Now().Before(r.tokenExpiry) {
		token := r.cachedToken
		r.tokenMu.RUnlock()
		return token, nil
	}
	r.tokenMu.RUnlock()
	
	// Need to get a new token
	r.tokenMu.Lock()
	defer r.tokenMu.Unlock()
	
	// Double-check in case another goroutine got the token
	if r.cachedToken != "" && time.Now().Before(r.tokenExpiry) {
		return r.cachedToken, nil
	}
	
	token, err := geo.GetAccessToken(r.username, r.password)
	if err != nil {
		return "", err
	}
	
	// Parse JWT to get actual expiration
	expiry, err := parseJWTExpiry(token)
	if err != nil {
		// Fall back to 55 minutes if we can't parse the JWT
		expiry = time.Now().Add(55 * time.Minute)
	} else {
		// Use a 5-minute buffer before the actual expiration
		expiry = expiry.Add(-5 * time.Minute)
	}
	
	r.cachedToken = token
	r.tokenExpiry = expiry
	
	return token, nil
}

func (r *GeoEnergyDataReader) getSystemID(accessToken string) (string, error) {
	r.systemMu.RLock()
	if r.systemID != "" {
		id := r.systemID
		r.systemMu.RUnlock()
		return id, nil
	}
	r.systemMu.RUnlock()
	
	// Need to fetch system ID
	r.systemMu.Lock()
	defer r.systemMu.Unlock()
	
	// Double-check in case another goroutine got it
	if r.systemID != "" {
		return r.systemID, nil
	}
	
	// Get device data to get the system ID
	deviceData, err := geo.GetDeviceData(accessToken)
	if err != nil {
		return "", err
	}

	r.systemID = deviceData.SystemDetails[0].SystemID
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
					ReadingType: METER,
					Value:       v.TotalConsumption,
				})
			case "ELECTRICITY":
				result = append(result, Reading{
					Commodity:   ELECTRICITY,
					ReadingType: METER,
					Value:       v.TotalConsumption,
				})
			}
		}
	}

	return result, nil
}
