package entities

import "time"

type Price struct {
	Pair      string        `json:"pair"`
	Amount    float64       `json:"amount"`
	Timestamp time.Time     `json:"timestamp"`
	Age       time.Duration `json:"age"`
}

func NewPrice(pair string, amount float64, timestamp time.Time, age time.Duration) *Price {
	return &Price{
		Pair:      pair,
		Amount:    amount,
		Timestamp: timestamp,
		Age:       age,
	}
}

func (p *Price) setAge() time.Duration {
	return time.Since(p.Timestamp)
}
