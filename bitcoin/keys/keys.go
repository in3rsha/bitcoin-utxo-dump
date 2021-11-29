package keys

import (
	"math/big"

	"github.com/ABMatrix/bitcoin-utxo/bitcoin/crypto"
	"github.com/akamensky/base58"
)

func Hash160ToAddress(hash160 []byte, prefix []byte) string {
	//
	// prefix   hash160                                                                   checksum
	//     \           \                                                                          \
	//    [00] [203 194 152 111 249 174 214 130 89 32 174 206 20 170 111 83 130 202 85 128] [56 132 221 179]
	//    \                                                                                                / base58 encode
	//     ------------------------------------------address-----------------------------------------------

	hash160_with_prefix := append(prefix, hash160...)                                        // prepend prefix to hash160pubkey (... unpacks the slice)
	hash160_prepared := append(hash160_with_prefix, crypto.Checksum(hash160_with_prefix)...) // add checksum to the end
	address := base58.Encode(hash160_prepared)
	return address
}

func PublicKeyToAddress(publickey []byte, prefix []byte) string {
	hash160 := crypto.Hash160(publickey)
	address := Hash160ToAddress(hash160, prefix)
	return address
}

func DecompressPublicKey(publickey []byte) []byte { // decompressing public keys from P2PK scripts
	// first byte (indicates whether y is even or odd)
	prefix := publickey[0:1]

	// remaining bytes (x coordinate)
	x := publickey[1:]

	// y^2 = x^3 + 7 mod p
	p, _ := new(big.Int).SetString("0xfffffffffffffffffffffffffffffffffffffffffffffffffffffffefffffc2f", 0)
	x_int := new(big.Int).SetBytes(x)
	x_3 := new(big.Int).Exp(x_int, big.NewInt(3), p)
	y_sq := new(big.Int).Add(x_3, big.NewInt(7))
	y_sq = new(big.Int).Mod(y_sq, p)

	// square root of y - secp256k1 is chosen so that the square root of y is y^((p+1)/4)
	y := new(big.Int).Exp(y_sq, new(big.Int).Div(new(big.Int).Add(p, big.NewInt(1)), big.NewInt(4)), p)

	// determine if the y we have caluclated is even or odd
	y_mod_2 := new(big.Int).Mod(y, big.NewInt(2))

	// if prefix is even (indicating an even y value) and y is odd, use other y value
	if (int(prefix[0])%2 == 0) && (y_mod_2.Cmp(big.NewInt(0)) != 0) { // Cmp returns 0 if equal
		y = new(big.Int).Mod(new(big.Int).Sub(p, y), p)
	}

	// if prefix is odd (indicating an odd y value) and y is even, use other y value
	if (int(prefix[0])%2 != 0) && (y_mod_2.Cmp(big.NewInt(0)) == 0) { // Cmp returns 0 if equal
		y = new(big.Int).Mod(new(big.Int).Sub(p, y), p)
	}

	// convert y to byte array
	y_bytes := y.Bytes()

	// make sure y value is 32 bytes in length
	if len(y_bytes) < 32 {
		y_bytes = make([]byte, 32)
		copy(y_bytes[32-len(y.Bytes()):], y.Bytes())
	}

	// return full x and y coordinates (with 0x04 prefix) as a byte array
	uncompressed := []byte{0x04}
	uncompressed = append(uncompressed, x...)
	uncompressed = append(uncompressed, y_bytes...)

	return uncompressed
}
