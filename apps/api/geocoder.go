package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

// GeocodeResult represents an address found for coordinates
type GeocodeResult struct {
	Address    string
	City       string
	PostalCode string
}

// Geocoder abstraction for address lookup
type Geocoder interface {
	Geocode(ctx context.Context, lat, lng float64) (*GeocodeResult, error)
}

// MapboxGeocoder implements Geocoder using Mapbox API v6
type MapboxGeocoder struct {
	AccessToken string
	Client      *http.Client
}

func (g *MapboxGeocoder) Geocode(ctx context.Context, lat, lng float64) (*GeocodeResult, error) {
	if g.AccessToken == "" {
		return nil, errors.New("mapbox access token missing")
	}

	// Mapbox Geocoding v6 URL: /search/geocode/v6/reverse
	u := fmt.Sprintf("https://api.mapbox.com/search/geocode/v6/reverse?longitude=%f&latitude=%f&access_token=%s&types=address&limit=1", lng, lat, g.AccessToken)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}

	resp, err := g.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("mapbox error (%d): %s", resp.StatusCode, string(body))
	}

	var data struct {
		Features []struct {
			Properties struct {
				FullAddress string `json:"full_address"`
				Context     struct {
					Place struct {
						Name string `json:"name"`
					} `json:"place"`
					Postcode struct {
						Name string `json:"name"`
					} `json:"postcode"`
				} `json:"context"`
			} `json:"properties"`
		} `json:"features"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}

	if len(data.Features) == 0 {
		return nil, nil // Not found
	}

	feat := data.Features[0]
	return &GeocodeResult{
		Address:    feat.Properties.FullAddress,
		City:       feat.Properties.Context.Place.Name,
		PostalCode: feat.Properties.Context.Postcode.Name,
	}, nil
}

// NominatimGeocoder implements Geocoder using OSM Nominatim
// CAUTION: Requires User-Agent and has strict rate limits (1 req/sec)
type NominatimGeocoder struct {
	UserAgent string
	Client    *http.Client
	mu        sync.Mutex
	lastCall  time.Time
}

func (g *NominatimGeocoder) Geocode(ctx context.Context, lat, lng float64) (*GeocodeResult, error) {
	g.mu.Lock()
	elapsed := time.Since(g.lastCall)
	if elapsed < time.Second {
		time.Sleep(time.Second - elapsed)
	}
	g.lastCall = time.Now()
	g.mu.Unlock()

	// Nominatim uses lon/lat order in URL but params are lat/lon
	u := fmt.Sprintf("https://nominatim.openstreetmap.org/reverse?format=jsonv2&lat=%f&lon=%f&addressdetails=1", lat, lng)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", g.UserAgent)

	resp, err := g.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("nominatim error: %d", resp.StatusCode)
	}

	var data struct {
		Address struct {
			Road        string `json:"road"`
			HouseNumber string `json:"house_number"`
			City        string `json:"city"`
			Town        string `json:"town"`
			Village     string `json:"village"`
			Postcode    string `json:"postcode"`
		} `json:"address"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}

	city := data.Address.City
	if city == "" {
		city = data.Address.Town
	}
	if city == "" {
		city = data.Address.Village
	}

	addr := data.Address.Road
	if data.Address.HouseNumber != "" {
		addr = fmt.Sprintf("%s %s", addr, data.Address.HouseNumber)
	}

	if addr == "" && city == "" {
		return nil, nil
	}

	return &GeocodeResult{
		Address:    addr,
		City:       city,
		PostalCode: data.Address.Postcode,
	}, nil
}

// FallbackGeocoder prioritizes first, falls back to second
type FallbackGeocoder struct {
	Primary   Geocoder
	Secondary Geocoder
}

func (g *FallbackGeocoder) Geocode(ctx context.Context, lat, lng float64) (*GeocodeResult, error) {
	res, err := g.Primary.Geocode(ctx, lat, lng)
	if err != nil || res == nil {
		// Log error if primary failed? For now just fallback
		return g.Secondary.Geocode(ctx, lat, lng)
	}
	return res, nil
}
