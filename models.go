package main

type UTXO struct {
	ID       string `json:"id,omitempty" bson:"_id,omitempty"`
	TxID     string `json:"tx_id" bson:"tx_id"`
	Vout     int64  `json:"vout" bson:"vout"`
	Height   int64  `json:"height" bson:"height"`
	Coinbase bool   `json:"coinbase" bson:"coinbase"`
	Amount   int64  `json:"amount" bson:"amount"`
	Script   string `json:"script" bson:"script"`
	Type     string `json:"type" bson:"type"`
	Address  string `json:"address" bson:"address"`
}
