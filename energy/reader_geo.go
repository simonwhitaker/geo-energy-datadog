package energy

import (
	"fmt"
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
	
	token, err := callAPI("GetAccessToken", func() (string, error) {
		return geo.GetAccessToken(r.username, r.password)
	})
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
	deviceData, err := callAPI("GetDeviceData", func() (geo.DeviceData, error) {
		return geo.GetDeviceData(accessToken)
	})
	if err != nil {
		return "", err
	}

	// A degraded API can answer with an empty or partial payload, so don't
	// assume there's a system to read.
	if len(deviceData.SystemDetails) == 0 {
		return "", fmt.Errorf("no system details returned for account")
	}

	systemID := deviceData.SystemDetails[0].SystemID
	if systemID == "" {
		return "", fmt.Errorf("empty system ID returned for account")
	}

	r.systemID = systemID
	return r.systemID, nil
}

// invalidateToken drops the cached access token so the next call logs in
// again. Used when the API rejects the token we're holding.
func (r *GeoEnergyDataReader) invalidateToken() {
	r.tokenMu.Lock()
	defer r.tokenMu.Unlock()

	r.cachedToken = ""
	r.tokenExpiry = time.Time{}
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

	liveData, err := callAPI("GetLiveMeterData", func() (geo.LiveMeterData, error) {
		return geo.GetLiveMeterData(accessToken, systemId)
	})
	if err != nil {
		if isAuthError(err) {
			r.invalidateToken()
		}
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

	periodicData, err := callAPI("GetPeriodicMeterData", func() (geo.PeriodicMeterData, error) {
		return geo.GetPeriodicMeterData(accessToken, systemId)
	})
	if err != nil {
		if isAuthError(err) {
			r.invalidateToken()
		}
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
