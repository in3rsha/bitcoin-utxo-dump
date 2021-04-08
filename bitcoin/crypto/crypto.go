package crypto

import "crypto/sha256"
import "golang.org/x/crypto/ripemd160"


func Hash256(bytes []byte) []byte {
    hash1 := sha256.Sum256(bytes[:]) // hash the byte slice twice through sha256
    hash2 := sha256.Sum256(hash1[:]) // need to coerce the [32]byte{} result above to a []byte{}
    return hash2[:] // return slice []byte{}, not [32]byte{}
}

func Hash160(bytes []byte) []byte {
    hash1 := sha256.Sum256(bytes[:]) // hash the byte slice through sha256 first
    hash2 := ripemd160.New() // ripemd160 hash function
    hash2.Write(hash1[:]) // need to coerce the [32]byte{} result above to a []byte{}
    result := hash2.Sum(nil)
    return result[:] // return slice []byte{}, not [32]byte{}
}

func Checksum(bytes []byte) []byte {
    hash := Hash256(bytes) // hash256 the bytes
    cksum := hash[:4]      // get last 4 bytes
    return cksum
}
