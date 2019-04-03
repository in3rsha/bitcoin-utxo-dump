package keys

import "github.com/in3rsha/bitcoin-utxo-dump/bitcoin/crypto"
import "github.com/akamensky/base58"

func Hash160ToAddress(hash160 []byte, prefix []byte) string {
    //
    // prefix   hash160                                                                   checksum
    //     \           \                                                                          \
    //    [00] [203 194 152 111 249 174 214 130 89 32 174 206 20 170 111 83 130 202 85 128] [56 132 221 179]
    //    \                                                                                                / base58 encode
    //     ------------------------------------------address-----------------------------------------------

    hash160_with_prefix := append(prefix, hash160...) // prepend prefix to hash160pubkey (... unpacks the slice)
    hash160_prepared := append(hash160_with_prefix, crypto.Checksum(hash160_with_prefix)...) // add checksum to the end
    address := base58.Encode(hash160_prepared)
    return address
}
