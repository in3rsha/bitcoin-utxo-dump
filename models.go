package main

type UTXO struct {
	TxID     string `json:"tx_id" bson:"tx_id"`
	Vout     int64  `json:"vout" bson:"vout"`
	Height   int64  `json:"height" bson:"height"`
	Coinbase bool   `json:"coinbase" bson:"coinbase"`
	Amount   int64  `json:"amount" bson:"amount"`
	Size     int64  `json:"size" bson:"size"`
	Script   string `json:"script" bson:"script"`
	Type     string `json:"type" bson:"type"` // TODO: maybe not string?
	Address  string `json:"address" bson:"address"`
}
