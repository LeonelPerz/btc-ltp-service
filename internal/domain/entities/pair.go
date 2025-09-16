package entities

type Pair struct {
	Symbol string `json:"symbol"`
	Price  Price  `json:"price"`
}

func NewPair(symbol string) *Pair {

	return &Pair{
		Symbol: symbol,
	}
}
