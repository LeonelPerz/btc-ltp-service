package kraken

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ===== CASOS DE ÉXITO - FUNCIONAMIENTO NORMAL =====

func TestKrakenTickerData_GetLastTradedPrice_Success(t *testing.T) {
	tickerData := KrakenTickerData{
		LastTradeClosed: []string{"50000.5", "1.0"},
		Ask:             []string{"50001.0", "1", "1"},
		Bid:             []string{"49999.0", "1", "1"},
	}

	price, err := tickerData.GetLastTradedPrice()

	require.NoError(t, err)
	assert.Equal(t, 50000.5, price)
}

func TestKrakenTickerData_GetTimestamp_ReturnsCurrentTime(t *testing.T) {
	tickerData := KrakenTickerData{}

	before := time.Now()
	timestamp := tickerData.GetTimestamp()
	after := time.Now()

	assert.True(t, timestamp.After(before.Add(-time.Second)))
	assert.True(t, timestamp.Before(after.Add(time.Second)))
}

func TestKrakenTickerData_GetAge_ReturnsZero(t *testing.T) {
	tickerData := KrakenTickerData{}

	age := tickerData.GetAge()

	assert.Equal(t, time.Duration(0), age)
}

func TestKrakenTickerResponse_ValidResponse(t *testing.T) {
	response := KrakenTickerResponse{
		Error: []string{},
		Result: map[string]KrakenTickerData{
			"XXBTZUSD": {
				LastTradeClosed:     []string{"50000.0", "1.0"},
				Ask:                 []string{"50001.0", "1", "1"},
				Bid:                 []string{"49999.0", "1", "1"},
				Volume:              []string{"100", "200"},
				VolumeWeightedPrice: []string{"50000.0", "50000.0"},
				NumberOfTrades:      []interface{}{10, 20},
				Low:                 []string{"49000.0", "49000.0"},
				High:                []string{"51000.0", "51000.0"},
				OpeningPrice:        "49500.0",
			},
		},
	}

	assert.Empty(t, response.Error)
	assert.Len(t, response.Result, 1)

	tickerData, exists := response.Result["XXBTZUSD"]
	assert.True(t, exists)

	price, err := tickerData.GetLastTradedPrice()
	require.NoError(t, err)
	assert.Equal(t, 50000.0, price)
}

// ===== CASOS DE ERROR - MANEJO DE ERRORES Y EXCEPCIONES =====

func TestKrakenTickerData_GetLastTradedPrice_EmptyData(t *testing.T) {
	tickerData := KrakenTickerData{
		LastTradeClosed: []string{}, // Empty array
	}

	_, err := tickerData.GetLastTradedPrice()

	assert.Error(t, err)
	assert.Equal(t, ErrInvalidTickerData, err)
}

func TestKrakenTickerData_GetLastTradedPrice_InvalidPrice(t *testing.T) {
	tickerData := KrakenTickerData{
		LastTradeClosed: []string{"invalid_price", "1.0"},
	}

	_, err := tickerData.GetLastTradedPrice()

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid syntax")
}

func TestKrakenTickerData_GetLastTradedPrice_NilData(t *testing.T) {
	tickerData := KrakenTickerData{
		LastTradeClosed: nil, // Nil slice
	}

	_, err := tickerData.GetLastTradedPrice()

	assert.Error(t, err)
	assert.Equal(t, ErrInvalidTickerData, err)
}

func TestKrakenTickerResponse_WithErrors(t *testing.T) {
	response := KrakenTickerResponse{
		Error:  []string{"EQuery:Invalid asset pair"},
		Result: map[string]KrakenTickerData{},
	}

	assert.NotEmpty(t, response.Error)
	assert.Contains(t, response.Error, "EQuery:Invalid asset pair")
	assert.Empty(t, response.Result)
}

func TestKrakenTickerResponse_EmptyResult(t *testing.T) {
	response := KrakenTickerResponse{
		Error:  []string{},
		Result: map[string]KrakenTickerData{},
	}

	assert.Empty(t, response.Error)
	assert.Empty(t, response.Result)
}

// ===== EDGE CASES - CASOS LÍMITE Y SITUACIONES EXTREMAS =====

func TestKrakenTickerData_GetLastTradedPrice_ZeroPrice(t *testing.T) {
	tickerData := KrakenTickerData{
		LastTradeClosed: []string{"0.0", "1.0"},
	}

	price, err := tickerData.GetLastTradedPrice()

	require.NoError(t, err)
	assert.Equal(t, 0.0, price)
}

func TestKrakenTickerData_GetLastTradedPrice_NegativePrice(t *testing.T) {
	tickerData := KrakenTickerData{
		LastTradeClosed: []string{"-1000.0", "1.0"},
	}

	price, err := tickerData.GetLastTradedPrice()

	require.NoError(t, err)
	assert.Equal(t, -1000.0, price)
}

func TestKrakenTickerData_GetLastTradedPrice_VeryLargePrice(t *testing.T) {
	tickerData := KrakenTickerData{
		LastTradeClosed: []string{"999999999999.99", "1.0"},
	}

	price, err := tickerData.GetLastTradedPrice()

	require.NoError(t, err)
	assert.Equal(t, 999999999999.99, price)
}

func TestKrakenTickerData_GetLastTradedPrice_VerySmallPrice(t *testing.T) {
	tickerData := KrakenTickerData{
		LastTradeClosed: []string{"0.00000001", "1.0"},
	}

	price, err := tickerData.GetLastTradedPrice()

	require.NoError(t, err)
	assert.Equal(t, 0.00000001, price)
}

func TestKrakenTickerData_GetLastTradedPrice_ScientificNotation(t *testing.T) {
	tickerData := KrakenTickerData{
		LastTradeClosed: []string{"1.23e+5", "1.0"},
	}

	price, err := tickerData.GetLastTradedPrice()

	require.NoError(t, err)
	assert.Equal(t, 123000.0, price)
}

func TestKrakenTickerData_CompleteDataStructure(t *testing.T) {
	tickerData := KrakenTickerData{
		Ask:                 []string{"50001.0", "10", "10"},
		Bid:                 []string{"49999.0", "15", "15"},
		LastTradeClosed:     []string{"50000.0", "2.5"},
		Volume:              []string{"100.5", "250.75"},
		VolumeWeightedPrice: []string{"49995.5", "50002.25"},
		NumberOfTrades:      []interface{}{25, 67},
		Low:                 []string{"49500.0", "49000.0"},
		High:                []string{"50500.0", "51000.0"},
		OpeningPrice:        "49750.0",
	}

	// Verificar que todos los campos están presentes
	assert.Len(t, tickerData.Ask, 3)
	assert.Len(t, tickerData.Bid, 3)
	assert.Len(t, tickerData.LastTradeClosed, 2)
	assert.Len(t, tickerData.Volume, 2)
	assert.Len(t, tickerData.VolumeWeightedPrice, 2)
	assert.Len(t, tickerData.NumberOfTrades, 2)
	assert.Len(t, tickerData.Low, 2)
	assert.Len(t, tickerData.High, 2)
	assert.NotEmpty(t, tickerData.OpeningPrice)

	// Verificar que el precio se puede extraer correctamente
	price, err := tickerData.GetLastTradedPrice()
	require.NoError(t, err)
	assert.Equal(t, 50000.0, price)
}

func TestKrakenTickerData_OpeningPriceAsString(t *testing.T) {
	tickerData := KrakenTickerData{
		OpeningPrice: "49500.0",
	}

	// Verificar que OpeningPrice puede ser string
	assert.IsType(t, "", tickerData.OpeningPrice)
	assert.Equal(t, "49500.0", tickerData.OpeningPrice)
}

func TestKrakenTickerData_OpeningPriceAsArray(t *testing.T) {
	tickerData := KrakenTickerData{
		OpeningPrice: []string{"49500.0", "49600.0"},
	}

	// Verificar que OpeningPrice puede ser array
	assert.IsType(t, []string{}, tickerData.OpeningPrice)
	openingArray := tickerData.OpeningPrice.([]string)
	assert.Len(t, openingArray, 2)
	assert.Equal(t, "49500.0", openingArray[0])
}

func TestKrakenTickerData_NumberOfTradesAsInts(t *testing.T) {
	tickerData := KrakenTickerData{
		NumberOfTrades: []interface{}{10, 25},
	}

	assert.Len(t, tickerData.NumberOfTrades, 2)

	// Verificar que los valores se pueden convertir a enteros
	if val, ok := tickerData.NumberOfTrades[0].(int); ok {
		assert.Equal(t, 10, val)
	} else {
		t.Error("First NumberOfTrades value is not an int")
	}
}

func TestKrakenTickerData_NumberOfTradesAsFloats(t *testing.T) {
	tickerData := KrakenTickerData{
		NumberOfTrades: []interface{}{10.0, 25.5},
	}

	assert.Len(t, tickerData.NumberOfTrades, 2)

	// Verificar que los valores se pueden convertir a floats
	if val, ok := tickerData.NumberOfTrades[0].(float64); ok {
		assert.Equal(t, 10.0, val)
	} else {
		t.Error("First NumberOfTrades value is not a float64")
	}
}

// ===== CONCURRENCIA - ACCESO CONCURRENTE Y THREAD-SAFETY =====

func TestKrakenTickerData_ConcurrentAccess(t *testing.T) {
	tickerData := KrakenTickerData{
		LastTradeClosed: []string{"50000.0", "1.0"},
	}

	const numGoroutines = 100
	results := make(chan float64, numGoroutines)
	errors := make(chan error, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func() {
			price, err := tickerData.GetLastTradedPrice()
			if err != nil {
				errors <- err
				return
			}
			results <- price
		}()
	}

	// Recoger todos los resultados
	for i := 0; i < numGoroutines; i++ {
		select {
		case price := <-results:
			assert.Equal(t, 50000.0, price)
		case err := <-errors:
			t.Errorf("Unexpected error in concurrent access: %v", err)
		case <-time.After(time.Second):
			t.Error("Timeout waiting for goroutine result")
		}
	}
}

func TestKrakenTickerData_ConcurrentTimestamp(t *testing.T) {
	tickerData := KrakenTickerData{}

	const numGoroutines = 100
	timestamps := make(chan time.Time, numGoroutines)

	before := time.Now()

	for i := 0; i < numGoroutines; i++ {
		go func() {
			timestamps <- tickerData.GetTimestamp()
		}()
	}

	after := time.Now()

	// Verificar que todos los timestamps están en el rango esperado
	for i := 0; i < numGoroutines; i++ {
		select {
		case timestamp := <-timestamps:
			assert.True(t, timestamp.After(before.Add(-time.Second)))
			assert.True(t, timestamp.Before(after.Add(time.Second)))
		case <-time.After(time.Second):
			t.Error("Timeout waiting for timestamp")
		}
	}
}

// ===== PERFORMANCE TESTS =====

func TestKrakenTickerData_GetLastTradedPrice_Performance(t *testing.T) {
	tickerData := KrakenTickerData{
		LastTradeClosed: []string{"50000.123456789", "1.0"},
	}

	// Medir el tiempo de múltiples llamadas
	const numCalls = 10000
	start := time.Now()

	for i := 0; i < numCalls; i++ {
		_, err := tickerData.GetLastTradedPrice()
		require.NoError(t, err)
	}

	duration := time.Since(start)

	// Verificar que el tiempo por llamada es razonable (menos de 1ms por llamada)
	avgTimePerCall := duration / numCalls
	assert.Less(t, avgTimePerCall, time.Millisecond,
		"GetLastTradedPrice is too slow: %v per call", avgTimePerCall)
}

func TestKrakenTickerResponse_LargeResult(t *testing.T) {
	// Crear una respuesta con muchos pares para probar el rendimiento
	result := make(map[string]KrakenTickerData)

	for i := 0; i < 1000; i++ {
		pairName := fmt.Sprintf("PAIR%d", i)
		result[pairName] = KrakenTickerData{
			LastTradeClosed: []string{fmt.Sprintf("%d.0", 50000+i), "1.0"},
		}
	}

	response := KrakenTickerResponse{
		Error:  []string{},
		Result: result,
	}

	assert.Empty(t, response.Error)
	assert.Len(t, response.Result, 1000)

	// Verificar que podemos acceder a los datos eficientemente
	for pairName, tickerData := range response.Result {
		price, err := tickerData.GetLastTradedPrice()
		require.NoError(t, err)
		assert.Greater(t, price, 0.0)
		assert.Contains(t, pairName, "PAIR")
	}
}
