# Base58  [![Go Report Card](https://goreportcard.com/badge/github.com/akamensky/base58)](https://goreportcard.com/report/github.com/akamensky/base58)

This is a Go language package implementing base58 encoding/decoding algorithms. This implementation attempts to optimize performance of encoding and decoding data to be used with other applications.

There are a few existing implementations and this package takes an inspiration (and hints on optimizations) from a few of them (see Acknowledgements for details).

## Why base58?

There are other widely used methods to encode/decode raw data into printable format. Most common onces are HEX and base64. While those are good approaches in some situations, each of them has own limitations. HEX tends to be long and base64 is hard to understand and read, they still have a place to be used when storage and readability are not of concern. Base58 encoding serves double purpose - 1st it allows the long data to be presented in short format (compression of sorts) and 2nd it produces human readable output by removing unnecessary and ambigous characters.

The alphabet of this encoding can be presented as below:

```
Alphabet:       0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz
Base58:          123456789ABCDEFGH JKLMN PQRSTUVWXYZabcdefghijk mnopqrstuvwxyz

Base64:         ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/=
```

## Installation

To install and use this package simply use standard go tools:

```
$ go get -x -u github.com/akamensky/base58
```

And then include this package in your project

```
import "github.com/akamensky/base58"
```

## Usage

`base58.Encode` takes slice of bytes. The reason for this is to be able to encode ANY types of data (as long as it is possible to present it as a slice of bytes):

```
    var unencoded []byte = ...
    var encoded string = base58.Encode(unencoded)
```

`base58.Decode` takes string as an input and returns slice of bytes, which can be converted/cast to original type. If the passed string contains characters invalid for base58 encoding then the returned error is not `nil`:

```
    var encoded string = ...
    decoded, err := base58.Decode(encoded)
    if err != nil {
        ...
    }
```

## Example

Below is example of encoding and decoding random bytes:

```
package main

import (
    "crypto/rand"
    "log"
    "fmt"
    "encoding/hex"
    "github.com/akamensky/base58"
)

func main() {
    // Generate random data to be encoded
    data := make([]byte, 64)
    _, err := rand.Read(data)
    if err != nil {
        log.Fatal(err)
    }
    
    // Print generated data in HEX format
    fmt.Println("Random data to encode: ", hex.EncodeToString(data))
    
    // Encode generated data to base58 and print it
    encoded := base58.Encode(data)
    fmt.Println("Base58 encoded data  : ", encoded)
    
    // Decode base58 string to bytes and print it in HEX format for inspection
    decoded, err := base58.Decode(encoded)
    if err != nil {
        log.Fatal(err)
    }
    fmt.Println("Decoded base58 data  : ", hex.EncodeToString(decoded))
}

```

## Running the tests

To run tests and benchmarks simply use:

```
$ cd $(go env GOPATH)/src/github.com/akamensky/base58
$ go test
$ go test -bench=.
```

## Contributing

I welcome any contribution to this library, especially those targeting performance improvements.

Please read [CONTRIBUTING](CONTRIBUTING) beforehand.

## License

This project is licensed under the UNLICENSE License - see the [LICENSE](LICENSE) file for details

## Acknowledgments

* Developers of [github.com/btcsuite/btcutil/base58](https://github.com/btcsuite/btcutil/tree/master/base58). For the hints on optimizing base58.Decode function.
