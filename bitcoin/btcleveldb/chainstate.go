package btcleveldb

import "math"          // math.Pow() in decompress_value()

func Varint128Read(bytes []byte, offset int) ([]byte, int) { // take a byte array and return (byte array and number of bytes read)

    // store bytes
    result := []byte{} // empty byte slice

    // loop through bytes
    for _, v := range bytes[offset:] { // start reading from an offset

        // store each byte as you go
        result = append(result, v)

        // Bitwise AND each of them with 128 (0b10000000) to check if the 8th bit has been set
        set := v & 128 // 0b10000000 is same as 1 << 7

        // When you get to one without the 8th bit set, return that byte slice
        if set == 0 {
            return result, len(result)
            // Also return the number of bytes read
        }
    }

    // Return zero bytes read if we haven't managed to read bytes properly
    return result, 0

}

func Varint128Decode(bytes []byte) int64 { // takes a byte slice, returns an int64 (makes sure it work on 32 bit systems)

    // total
    var n int64 = 0

    for _, v := range bytes {

        // 1. shift n left 7 bits (add some extra bits to work with)
        //                             00000000
        n = n << 7

        // 2. set the last 7 bits of each byte in to the total value
        //    AND extracts 7 bits only 10111001  <- these are the bits of each byte
        //                              1111111
        //                              0111001  <- don't want the 8th bit (just indicated if there were more bytes in the varint)
        //    OR sets the 7 bits
        //                             00000000  <- the result
        //                              0111001  <- the bits we want to set
        //                             00111001
        n = n | int64(v & 127)

        // 3. add 1 each time (only for the ones where the 8th bit is set)
        if (v & 128 != 0) { // 0b10000000 <- AND to check if the 8th bit is set
                            // 1 << 7     <- could always bit shift to get 128
            n++
        }

    }

    return n
    // 11101000000111110110

}

func DecompressValue(x int64) int64 {

    var n int64 = 0      // decompressed value

    // Return value if it is zero (nothing to decompress)
    if x == 0 {
        return 0
    }

    // Decompress...
    x = x - 1    // subtract 1 first
    e := x % 10  // remainder mod 10
    x = x / 10   // quotient mod 10 (reduce x down by 10)

    // If the remainder is less than 9
    if e < 9 {
        d := x % 9 // remainder mod 9
        x = x / 9  // (reduce x down by 9)
        n = x * 10 + d + 1 // work out n
    } else {
        n = x + 1
    }

    // Multiply n by 10 to the power of the first remainder
    result := float64(n) * math.Pow(10, float64(e)) // math.Pow takes a float and returns a float

    // manual exponentiation
    // multiplier := 1
    // for i := 0; i < e; i++ {
    //     multiplier *= 10
    // }
    // fmt.Println(multiplier)

    return int64(result)

}
