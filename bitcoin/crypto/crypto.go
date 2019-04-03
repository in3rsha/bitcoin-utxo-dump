package crypto

import "crypto/sha256"


func Hash256(bytes []byte) []byte {
    hash1 := sha256.Sum256(bytes[:]) // hash the byte slice twice through sha256
    hash2 := sha256.Sum256(hash1[:]) // need to coerce the [32]byte{} result above to a []byte{}
    return hash2[:] // return slice []byte{}, not [32]byte{}
}

func Checksum(bytes []byte) []byte {
    hash := Hash256(bytes) // hash256 the bytes
    cksum := hash[:4]      // get last 4 bytes
    return cksum
}
