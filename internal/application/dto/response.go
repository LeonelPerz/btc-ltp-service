package dto

import (
	"btc-ltp-service/internal/domain/entities"
	"time"
)

// PriceData represents an individual price in the response
// @Description Last traded price data for a cryptocurrency pair
type PriceData struct {
	Pair   string  `json:"pair" example:"BTC/USD" validate:"required"`          // Trading pair (e.g., BTC/USD)
	Amount float64 `json:"amount" example:"45123.45" validate:"required,min=0"` // Price in the quoted currency
}

// PriceError represents an error for a specific pair
// @Description Error when retrieving price for a specific pair
type PriceError struct {
	Pair    string `json:"pair" example:"ETH/USD" validate:"required"`                        // Trading pair that caused the error
	Error   string `json:"error" example:"price not available" validate:"required"`           // Main error message
	Code    string `json:"code,omitempty" example:"PRICE_NOT_FOUND"`                          // Optional error code
	Message string `json:"message,omitempty" example:"Price data is temporarily unavailable"` // Additional descriptive message
}

// GetLTPResponse represents the response from /api/v1/ltp endpoint
// @Description Main response with last traded prices
type GetLTPResponse struct {
	LTP    []PriceData  `json:"ltp" validate:"required"` // List of successfully retrieved prices
	Errors []PriceError `json:"errors,omitempty"`        // Errors for specific pairs (optional)
}

// GetLTPPartialResponse represents a response with partial successes and errors
type GetLTPPartialResponse struct {
	Success []PriceData   `json:"success"`
	Errors  []PriceError  `json:"errors"`
	Stats   ResponseStats `json:"stats"`
}

// ResponseStats provides statistics about the response
type ResponseStats struct {
	Total     int `json:"total"`
	Succeeded int `json:"succeeded"`
	Failed    int `json:"failed"`
}

// ErrorResponse represents a standard error response for endpoints
// @Description Standard error response for endpoints
type ErrorResponse struct {
	Error   string `json:"error" example:"INVALID_PARAMETER" validate:"required"`                  // Main error message
	Message string `json:"message,omitempty" example:"The provided trading pair is not supported"` // Detailed error description
	Code    string `json:"code,omitempty" example:"400"`                                           // HTTP error code or internal code
}

// HealthResponse represents the health check response with service status
// @Description Health check response with service status
type HealthResponse struct {
	Status    string            `json:"status" example:"healthy" validate:"required" enums:"healthy,degraded,unhealthy"` // Overall service status
	Timestamp time.Time         `json:"timestamp" example:"2023-12-01T10:30:00Z" validate:"required"`                    // When the health check was performed
	Services  map[string]string `json:"services,omitempty" example:"cache:healthy,exchange:healthy"`                     // Individual service statuses
}

// NewGetLTPResponse creates a new response from a list of prices
func NewGetLTPResponse(prices []*entities.Price) *GetLTPResponse {
	priceData := make([]PriceData, len(prices))

	for i, price := range prices {
		priceData[i] = PriceData{
			Pair:   price.Pair,
			Amount: price.Amount,
		}
	}

	return &GetLTPResponse{
		LTP: priceData,
	}
}

// NewGetLTPResponseWithErrors creates a response that includes partial errors
func NewGetLTPResponseWithErrors(successPrices []*entities.Price, errors []PriceError) *GetLTPResponse {
	priceData := make([]PriceData, len(successPrices))

	for i, price := range successPrices {
		priceData[i] = PriceData{
			Pair:   price.Pair,
			Amount: price.Amount,
		}
	}

	return &GetLTPResponse{
		LTP:    priceData,
		Errors: errors,
	}
}

// NewGetLTPPartialResponse creates a response with detailed statistics
func NewGetLTPPartialResponse(successPrices []*entities.Price, errors []PriceError) *GetLTPPartialResponse {
	successData := make([]PriceData, len(successPrices))

	for i, price := range successPrices {
		successData[i] = PriceData{
			Pair:   price.Pair,
			Amount: price.Amount,
		}
	}

	total := len(successPrices) + len(errors)

	return &GetLTPPartialResponse{
		Success: successData,
		Errors:  errors,
		Stats: ResponseStats{
			Total:     total,
			Succeeded: len(successPrices),
			Failed:    len(errors),
		},
	}
}

// NewPriceError creates an error for a specific pair
func NewPriceError(pair, error, code, message string) PriceError {
	return PriceError{
		Pair:    pair,
		Error:   error,
		Code:    code,
		Message: message,
	}
}

// NewErrorResponse creates a new error response
func NewErrorResponse(error string, message string) *ErrorResponse {
	return &ErrorResponse{
		Error:   error,
		Message: message,
	}
}

// NewErrorResponseWithCode creates an error response with code
func NewErrorResponseWithCode(error string, message string, code string) *ErrorResponse {
	return &ErrorResponse{
		Error:   error,
		Message: message,
		Code:    code,
	}
}

// NewHealthResponse creates a health check response
func NewHealthResponse(status string, services map[string]string) *HealthResponse {
	return &HealthResponse{
		Status:    status,
		Timestamp: time.Now(),
		Services:  services,
	}
}
