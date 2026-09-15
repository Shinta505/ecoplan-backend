package services

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"ecoplan-backend/config"
)

// BaseURLRajaOngkir mendefinisikan URL utama antarmuka pemanggilan API RajaOngkir Komerce v1.
const BaseURLRajaOngkir = "https://rajaongkir.komerce.id/api/v1"

// MetaResponse mendefinisikan struktur metadata respons API RajaOngkir.
type MetaResponse struct {
	Message string      `json:"message"`
	Code    int         `json:"code"`
	Status  interface{} `json:"status"`
}

// DomesticDestination mendefinisikan entitas data lokasi pengiriman domestik di Indonesia.
type DomesticDestination struct {
	ID              int    `json:"id"`
	Label           string `json:"label"`
	ProvinceName    string `json:"province_name"`
	CityName        string `json:"city_name"`
	DistrictName    string `json:"district_name"`
	SubdistrictName string `json:"subdistrict_name"`
	ZipCode         string `json:"zip_code"`
}

// DomesticDestinationResponse mendefinisikan struktur respons lengkap dari API pencarian lokasi domestik.
type DomesticDestinationResponse struct {
	Meta MetaResponse          `json:"meta"`
	Data []DomesticDestination `json:"data"`
}

// DomesticCostRequest mendefinisikan struktur data masukan untuk kalkulasi ongkos kirim domestik.
type DomesticCostRequest struct {
	Origin      int    `json:"origin"`
	Destination int    `json:"destination"`
	Weight      int    `json:"weight"` // Berat barang dalam satuan gram
	Courier     string `json:"courier"`
	Price       string `json:"price,omitempty"` // Opsi urutan harga: "lowest" atau "highest"
}

// DomesticCostItem mendefinisikan rincian opsi layanan pengiriman dan estimasi biaya per kurir.
type DomesticCostItem struct {
	Name        string  `json:"name"`
	Code        string  `json:"code"`
	Service     string  `json:"service"`
	Description string  `json:"description"`
	Cost        float64 `json:"cost"`
	ETD         string  `json:"etd"`
}

// DomesticCostResponse mendefinisikan struktur respons lengkap dari kalkulasi ongkos kirim.
type DomesticCostResponse struct {
	Meta MetaResponse       `json:"meta"`
	Data []DomesticCostItem `json:"data"`
}

// WaybillTrackingResponse mendefinisikan struktur generik untuk pembacaan data pelacakan resi real-time.
type WaybillTrackingResponse struct {
	Meta MetaResponse `json:"meta"`
	Data interface{}  `json:"data"`
}

// RajaOngkirService mendefinisikan struktur layanan integrasi API RajaOngkir dengan eksekusi HTTP client.
type RajaOngkirService struct {
	client *http.Client
}

// NewRajaOngkirService menginstansiasi objek RajaOngkirService dengan batas waktu koneksi HTTP.
func NewRajaOngkirService() *RajaOngkirService {
	return &RajaOngkirService{
		client: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

// SearchDomesticDestination mengeksekusi pencarian data lokasi pengiriman di Indonesia berdasarkan kata kunci.
func (s *RajaOngkirService) SearchDomesticDestination(search string, limit, offset int) (*DomesticDestinationResponse, error) {
	if strings.TrimSpace(search) == "" {
		return nil, errors.New("parameter kata kunci pencarian (search) tidak boleh kosong")
	}

	if limit <= 0 {
		limit = 10
	}

	endpoint := fmt.Sprintf("%s/destination/domestic-destination?search=%s&limit=%d&offset=%d",
		BaseURLRajaOngkir,
		url.QueryEscape(search),
		limit,
		offset,
	)

	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("gagal inisialisasi HTTP Request pencarian lokasi: %w", err)
	}

	req.Header.Set("key", config.ENV.RajaOngkirAPIKey)
	req.Header.Set("accept", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("gagal mengeksekusi panggilan API RajaOngkir: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("gagal membaca respons API RajaOngkir: %w", err)
	}

	// Cek status code dulu sebelum unmarshal agar aman dari error 500/400
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API RajaOngkir mengembalikan status error (%d): %s", resp.StatusCode, string(bodyBytes))
	}

	var result DomesticDestinationResponse
	if err := json.Unmarshal(bodyBytes, &result); err != nil {
		return nil, fmt.Errorf("gagal parsing JSON respons pencarian lokasi: %w (Body: %s)", err, string(bodyBytes))
	}

	return &result, nil
}

// CalculateDomesticCost mengeksekusi kalkulasi estimasi ongkos kirim paket secara real-time.
func (s *RajaOngkirService) CalculateDomesticCost(input DomesticCostRequest) (*DomesticCostResponse, error) {
	if input.Origin <= 0 || input.Destination <= 0 {
		return nil, errors.New("ID lokasi asal (origin) dan tujuan (destination) harus bernilai valid (> 0)")
	}
	if input.Weight <= 0 {
		return nil, errors.New("berat paket (weight) harus lebih besar dari 0 gram")
	}
	if strings.TrimSpace(input.Courier) == "" {
		return nil, errors.New("kode kurir (courier) tidak boleh kosong")
	}

	endpoint := fmt.Sprintf("%s/calculate/domestic-cost", BaseURLRajaOngkir)

	form := url.Values{}
	form.Set("origin", strconv.Itoa(input.Origin))
	form.Set("destination", strconv.Itoa(input.Destination))
	form.Set("weight", strconv.Itoa(input.Weight))
	form.Set("courier", input.Courier)
	if input.Price != "" {
		form.Set("price", input.Price)
	}

	req, err := http.NewRequest(http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("gagal inisialisasi HTTP Request kalkulasi ongkos kirim: %w", err)
	}

	req.Header.Set("key", config.ENV.RajaOngkirAPIKey)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("accept", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("gagal mengeksekusi panggilan API kalkulasi ongkos kirim: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("gagal membaca respons API kalkulasi ongkos kirim: %w", err)
	}

	var result DomesticCostResponse
	if err := json.Unmarshal(bodyBytes, &result); err != nil {
		return nil, fmt.Errorf("gagal parsing JSON respons kalkulasi ongkos kirim: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("kalkulasi ongkos kirim gagal dengan status (%d): %s", resp.StatusCode, result.Meta.Message)
	}

	return &result, nil
}

// TrackWaybill mengeksekusi pelacakan rekam jejak status pengiriman resi paket secara real-time.
func (s *RajaOngkirService) TrackWaybill(awb string, courier string) (*WaybillTrackingResponse, error) {
	if strings.TrimSpace(awb) == "" {
		return nil, errors.New("nomor resi pengiriman (awb) tidak boleh kosong")
	}

	endpoint := fmt.Sprintf("%s/track/waybill?awb=%s&courier=%s",
		BaseURLRajaOngkir,
		url.QueryEscape(awb),
		url.QueryEscape(courier),
	)

	req, err := http.NewRequest(http.MethodPost, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("gagal inisialisasi HTTP Request pelacakan resi: %w", err)
	}

	req.Header.Set("key", config.ENV.RajaOngkirAPIKey)
	req.Header.Set("accept", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("gagal mengeksekusi panggilan API pelacakan resi: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("gagal membaca respons API pelacakan resi: %w", err)
	}

	var result WaybillTrackingResponse
	if err := json.Unmarshal(bodyBytes, &result); err != nil {
		return nil, fmt.Errorf("gagal parsing JSON respons pelacakan resi: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("pelacakan resi gagal dengan status (%d): %s", resp.StatusCode, result.Meta.Message)
	}

	return &result, nil
}
