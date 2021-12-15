package main

// local packages
import (
	"bufio"
	"context"
	"encoding/hex"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"strings"
	"sync"
	"syscall"

	"github.com/ABMatrix/bitcoin-utxo/bitcoin/bech32"

	"github.com/syndtr/goleveldb/leveldb/opt"

	"github.com/ABMatrix/bitcoin-utxo/bitcoin/btcleveldb"
	"github.com/ABMatrix/bitcoin-utxo/bitcoin/keys"
	"github.com/btcsuite/btcd/txscript"
	"github.com/syndtr/goleveldb/leveldb"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Version
const (
	Version                     = "1.0.1"
	ENV_MONGO_URI               = "MONGO_URI"
	ENV_MONGO_BITCOIN_DB_NAME   = "MONGO_UTXO_DB_NAME"
	UTXO_COLLECTION_NAME_PREFIX = "utxo"
	BUF_SIZE                    = 1024

	NONSTANDARD           string = "nonstandard"
	PUBKEY                string = "pubkey"
	PUBKEYHASH            string = "pubkeyhash"
	SCRIPTHASH            string = "scripthash"
	MULTISIG              string = "multisig"
	WITNESS_V0_KEYHASH    string = "witness_v0_keyhash"
	WITNESS_V0_SCRIPTHASH string = "witness_v0_scripthash"
	WITNESS_V1_TAPROOT    string = "witness_v1_taproot"
	WITNESS_UNKNOWN       string = "witness_unknown"
	NULLDATA              string = "nulldata"
)

func insertUTXO(ctx context.Context, buf []UTXO, wg *sync.WaitGroup, totalProcessedSoFar int64, utxoCollection *mongo.Collection) {
	defer wg.Done()
	log.Printf("%d utxos processed\n", totalProcessedSoFar) // Show progress at intervals.
	// convert to mongo-acceptable arguments...
	var docs []interface{}
	for _, utxo := range buf {
		docs = append(docs, utxo)
	}
	_, err := utxoCollection.InsertMany(ctx, docs)
	if err != nil {
		log.Println("failed to insert many with error: ", err.Error())
		return
	}
}

func main() {
	// Set default chainstate LevelDB and output file
	defaultFolder := fmt.Sprintf("%s/.bitcoin/chainstate/", os.Getenv("HOME")) // %s = string

	// Command Line Options (Flags)
	chainstate := flag.String("db", defaultFolder, "Location of bitcoin chainstate db.") // chainstate folder
	testnetFlag := flag.Bool("testnet", false, "Is the chainstate leveldb for testnet?") // true/false
	version := flag.Bool("version", false, "Print version.")
	p2pkaddresses := flag.Bool("p2pkaddresses", false, "Convert public keys in P2PK locking scripts to addresses also.") // true/false
	nowarnings := flag.Bool("nowarnings", false, "Ignore warnings if bitcoind is running in the background.")            // true/false
	flag.Parse()                                                                                                         // execute command line parsing for all declared flags

	// Check bitcoin isn't running first
	if !*nowarnings {
		cmd := exec.Command("bitcoin-cli", "getnetworkinfo")
		_, err := cmd.Output()
		if err == nil {
			log.Println("Bitcoin is running. You should shut it down with `bitcoin-cli stop` first. We don't want to access the chainstate LevelDB while Bitcoin is running.")
			log.Println("Note: If you do stop bitcoind, make sure that it won't auto-restart (e.g. if it's running as a systemd service).")

			// Ask if you want to continue anyway (e.g. if you've copied the chainstate to a new location and bitcoin is still running)
			reader := bufio.NewReader(os.Stdin)
			log.Printf("%s [y/n] (default n): ", "Do you wish to continue anyway?")
			response, _ := reader.ReadString('\n')
			response = strings.ToLower(strings.TrimSpace(response))

			if response != "y" && response != "yes" {
				return
			}
		}
	}

	// Check if OS type is Mac OS, then increase ulimit -n to 4096 filehandler during runtime and reset to 1024 at the end
	// Mac OS standard is 1024
	// Linux standard is already 4096 which is also "max" for more edit etc/security/limits.conf
	if runtime.GOOS == "darwin" {
		cmd2 := exec.Command("ulimit", "-n", "4096")
		log.Println("setting ulimit 4096")
		_, err := cmd2.Output()
		if err != nil {
			log.Printf("setting new ulimit failed with %s\n", err)
		}
		defer exec.Command("ulimit", "-n", "1024")
	}

	// Show Version
	if *version {
		log.Println(Version)
		os.Exit(0)
	}

	ctx := context.Background()

	// MongoDB version of API service
	mongoURI := os.Getenv(ENV_MONGO_URI) // "mongodb://username@password:<ip>:port/"
	if mongoURI == "" {
		log.Fatalln("mongo URI is unset")
	}

	mongoDBName := os.Getenv(ENV_MONGO_BITCOIN_DB_NAME) // "bitcoin"
	if mongoDBName == "" {
		log.Fatalln("mongo db name is unset")
	}

	// initialize mongodb
	clientOptions := options.Client().ApplyURI(mongoURI)

	// connect to MongoDB
	mongoCli, err := mongo.Connect(ctx, clientOptions)
	if err != nil {
		log.Fatalln("failed to connect with error:", err)
	}
	// check connection
	err = mongoCli.Ping(ctx, nil)
	if err != nil {
		log.Fatalln("failed to ping mongodb with error: ", err)
	}

	log.Println("mongo connection is OK...")

	// Mainnet or Testnet (for encoding addresses correctly)
	testnet := false
	utxoCollectionName := UTXO_COLLECTION_NAME_PREFIX + "-mainnet"
	if *testnetFlag || strings.Contains(*chainstate, "testnet") { // check testnet flag
		testnet = true
		utxoCollectionName = UTXO_COLLECTION_NAME_PREFIX + "-testnet"
	}

	// Check chainstate LevelDB folder exists
	if _, err := os.Stat(*chainstate); os.IsNotExist(err) {
		log.Println("Couldn't find", *chainstate)
		return
	}

	// Select bitcoin chainstate leveldb folder
	// open leveldb without compression to avoid corrupting the database for bitcoin
	opts := &opt.Options{
		Compression: opt.NoCompression,
	}
	// https://bitcoin.stackexchange.com/questions/52257/chainstate-leveldb-corruption-after-reading-from-the-database
	// https://github.com/syndtr/goleveldb/issues/61
	// https://godoc.org/github.com/syndtr/goleveldb/leveldb/opt

	db, err := leveldb.OpenFile(*chainstate, opts) // You have got to dereference the pointer to get the actual value
	if err != nil {
		log.Println("Couldn't open LevelDB with error: ", err.Error())
		return
	}
	defer db.Close()

	// Stats - keep track of interesting stats as we read through leveldb.
	var totalAmount int64 = 0 // total amount of satoshis
	scriptTypeCount := map[string]int{
		NONSTANDARD:           0,
		PUBKEY:                0,
		PUBKEYHASH:            0,
		SCRIPTHASH:            0,
		MULTISIG:              0,
		WITNESS_V0_KEYHASH:    0,
		WITNESS_V0_SCRIPTHASH: 0,
		WITNESS_V1_TAPROOT:    0,
		WITNESS_UNKNOWN:       0,
		NULLDATA:              0,
	} // count each script type

	// Declare obfuscateKey (a byte slice)
	var obfuscateKey []byte // obfuscateKey := make([]byte, 0)

	// Iterate over LevelDB keys
	iter := db.NewIterator(nil, nil)
	// NOTE: iter.Release() comes after the iteration (not deferred here)
	if err := iter.Error(); err != nil {
		fmt.Println("failed to iterate over level DB keys", err)
		return
	}

	// Catch signals that interrupt the script so that we can close the database safely (hopefully not corrupting it)
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() { // goroutine
		<-c // receive from channel
		log.Println("Interrupt signal caught. Shutting down gracefully.")
		// iter.Release() // release database iterator
		db.Close() // close database
		os.Exit(0) // exit
	}()

	utxoDB := mongoCli.Database(mongoDBName)
	utxoCollection := utxoDB.Collection(utxoCollectionName)

	utxoBuf := make([]UTXO, BUF_SIZE)
	var i int64
	wg := &sync.WaitGroup{}
	for iter.Next() {
		// Output Fields - build output from flags passed in
		output := UTXO{} // we will add to this as we go through each utxo in the database

		key := iter.Key()
		value := iter.Value()

		// first byte in key indicates the type of key we've got for leveldb
		prefix := key[0]

		// obfuscateKey (first key)
		if prefix == 14 { // 14 = obfuscateKey
			obfuscateKey = value
		}

		// utxo entry
		if prefix == 67 { // 67 = 0x43 = C = "utxo"

			// ---
			// Key
			// ---

			//      430000155b9869d56c66d9e86e3c01de38e3892a42b99949fe109ac034fff6583900
			//      <><--------------------------------------------------------------><>
			//      /                               |                                  \
			//  type                          txid (little-endian)                      index (varint)

			// txid
			txidLE := key[1:33] // little-endian byte order

			// txid - reverse byte order
			txid := make([]byte, 0)                 // create empty byte slice (dont want to mess with txid directly)
			for i := len(txidLE) - 1; i >= 0; i-- { // run backwards through the txid slice
				txid = append(txid, txidLE[i]) // append each byte to the new byte slice
			}
			output.TxID = hex.EncodeToString(txid) // add to output results map

			// vout
			index := key[33:]

			// convert varint128 index to an integer
			output.Vout = btcleveldb.Varint128Decode(index)

			// -----
			// Value
			// -----

			// Only deobfuscate and get data from the Value if something is needed from it (improves speed if you just want the txid:vout)

			// Copy the obfuscateKey ready to extend it
			obfuscateKeyExtended := obfuscateKey[1:] // ignore the first byte, as that just tells you the size of the obfuscateKey

			// Extend the obfuscateKey so it's the same length as the value
			for i, k := len(obfuscateKeyExtended), 0; len(obfuscateKeyExtended) < len(value); i, k = i+1, k+1 {
				// append each byte of obfuscateKey to the end until it's the same length as the value
				obfuscateKeyExtended = append(obfuscateKeyExtended, obfuscateKeyExtended[k])
				// Example
				//   [8 175 184 95 99 240 37 253 115 181 161 4 33 81 167 111 145 131 0 233 37 232 118 180 123 120 78]
				//   [8 177 45 206 253 143 135 37 54]                                                                  <- obfuscate key
				//   [8 177 45 206 253 143 135 37 54 8 177 45 206 253 143 135 37 54 8 177 45 206 253 143 135 37 54]    <- extended
			}

			// XOR the value with the obfuscateKey (xor each byte) to de-obfuscate the value
			var xor []byte // create a byte slice to hold the xor results
			for i := range value {
				result := value[i] ^ obfuscateKeyExtended[i]
				xor = append(xor, result)
			}

			// -----
			// Value
			// -----

			//   value: 71a9e87d62de25953e189f706bcf59263f15de1bf6c893bda9b045 <- obfuscated
			//          b12dcefd8f872536b12dcefd8f872536b12dcefd8f872536b12dce <- extended obfuscateKey (XOR)
			//          c0842680ed5900a38f35518de4487c108e3810e6794fb68b189d8b <- deobfuscated
			//          <----><----><><-------------------------------------->
			//           /      |    \                   |
			//      varint   varint   varint          script <- P2PKH/P2SH hash160, P2PK public key, or complete script
			//         |        |     nSize
			//         |        |
			//         |     amount (compressesed)
			//         |
			//         |
			//  100000100001010100110
			//  <------------------> \
			//         height         coinbase

			offset := 0

			// First Varint
			// ------------
			// b98276a2ec7700cbc2986ff9aed6825920aece14aa6f5382ca5580
			// <---->
			varint, bytesRead := btcleveldb.Varint128Read(xor, 0) // start reading at 0
			offset += bytesRead
			varintDecoded := btcleveldb.Varint128Decode(varint)

			// Height (first bits)
			output.Height = varintDecoded >> 1 // right-shift to remove last bit

			// Coinbase (last bit)
			output.Coinbase = varintDecoded&1 == 1 // AND to extract right-most bit

			// Second Varint
			// -------------
			// b98276a2ec7700cbc2986ff9aed6825920aece14aa6f5382ca5580
			//       <---->
			varint, bytesRead = btcleveldb.Varint128Read(xor, offset) // start after last varint
			offset += bytesRead
			varintDecoded = btcleveldb.Varint128Decode(varint)

			// Amount
			amount := btcleveldb.DecompressValue(varintDecoded) // int64
			output.Amount = amount
			totalAmount += amount // add to stats

			// Third Varint
			// ------------
			// b98276a2ec7700cbc2986ff9aed6825920aece14aa6f5382ca5580
			//             <>
			//
			// nSize - byte to indicate the type or size of script - helps with compression of the script data
			//  - https://github.com/bitcoin/bitcoin/blob/master/src/compressor.cpp

			//  0  = P2PKH <- hash160 public key
			//  1  = P2SH  <- hash160 script
			//  2  = P2PK 02publickey <- nsize makes up part of the public key in the actual script
			//  3  = P2PK 03publickey
			//  4  = P2PK 04publickey (uncompressed - but has been compressed in to leveldb) y=even
			//  5  = P2PK 04publickey (uncompressed - but has been compressed in to leveldb) y=odd
			//  6+ = [size of the upcoming script] (subtract 6 though to get the actual size in bytes, to account for the previous 5 script types already taken)
			varint, bytesRead = btcleveldb.Varint128Read(xor, offset) // start after last varint
			offset += bytesRead
			nsize := btcleveldb.Varint128Decode(varint) //

			// Script (remaining bytes)
			// ------
			// b98276a2ec7700cbc2986ff9aed6825920aece14aa6f5382ca5580
			//               <-------------------------------------->
			if nsize > 1 && nsize < 6 { // either 2, 3, 4, 5
				// move offset back a byte if script type is 2, 3, 4, or 5 (because this forms part of the P2PK public key along with the actual script)
				offset--
			}

			script := xor[offset:]
			output.Script = hex.EncodeToString(script)

			// Addresses - Get address from script (if possible), and set script type (P2PK, P2PKH, P2SH, P2MS, P2WPKH, or P2WSH)
			// ---------

			var address string           // initialize address variable
			var scriptType = NONSTANDARD // initialize script type

			if script[0] == 0x6a { // OP_RETURN = 0x6a
				// nulldata
				scriptTypeCount[NULLDATA]++
				continue
			}

			if isTrue, _ := txscript.IsMultisigScript(script); isTrue { // P2MS
				scriptType = MULTISIG
				scriptTypeCount[MULTISIG]++
			} else if nsize < 6 { // legacy txout types
				if nsize == 0 { // P2PKH
					if testnet == true {
						address = keys.Hash160ToAddress(script, []byte{0x6f}) // (m/n)address - testnet addresses have a special prefix
					} else {
						address = keys.Hash160ToAddress(script, []byte{0x00}) // 1address
					}
					scriptType = PUBKEYHASH
					scriptTypeCount[PUBKEYHASH]++
				} else if nsize == 1 { // P2SH
					if testnet == true {
						address = keys.Hash160ToAddress(script, []byte{0xc4}) // 2address - testnet addresses have a special prefix
					} else {
						address = keys.Hash160ToAddress(script, []byte{0x05}) // 3address
					}
					scriptType = SCRIPTHASH
					scriptTypeCount[SCRIPTHASH]++
				} else { // P2PK
					// 2, 3, 4, 5
					//  2 = P2PK 02publickey <- nsize makes up part of the public key in the actual script (e.g. 02publickey)
					//  3 = P2PK 03publickey <- y is odd/even (0x02 = even, 0x03 = odd)
					//  4 = P2PK 04publickey (uncompressed)  y = odd  <- actual script uses an uncompressed public key, but it is compressed when stored in this db
					//  5 = P2PK 04publickey (uncompressed) y = even
					// "The uncompressed pubkeys are compressed when they are added to the db. 0x04 and 0x05 are used to indicate that the key is supposed to be uncompressed and those indicate whether the y value is even or odd so that the full uncompressed key can be retrieved."
					//
					// if nsize is 4 or 5, you will need to uncompress the public key to get it's full form
					// if nsize == 4 || nsize == 5 {
					//     // uncompress (4 = y is even, 5 = y is odd)
					//     script = decompress(script)
					// }
					scriptType = PUBKEY
					scriptTypeCount[PUBKEY]++

					if *p2pkaddresses { // if we want to convert public keys in P2PK scripts to their corresponding addresses (even though they technically don't have addresses)

						// Decompress if starts with 0x04 or 0x05
						if (nsize == 4) || (nsize == 5) {
							script = keys.DecompressPublicKey(script)
						}

						if testnet == true {
							address = keys.PublicKeyToAddress(script, []byte{0x6f}) // (m/n)address - testnet addresses have a special prefix
						} else {
							address = keys.PublicKeyToAddress(script, []byte{0x00}) // 1address
						}
					}
				}
			} else if txscript.IsWitnessProgram(script) { // witness program
				version := script[0]
				program := script[2:]

				// bech32 function takes an int array and not a byte array, so convert the array to integers
				var programint []int // initialize empty integer array to hold the new one
				for _, v := range program {
					programint = append(programint, int(v)) // cast every value to an int
				}

				if testnet == true {
					address, _ = bech32.SegwitAddrEncode("tb", int(version), programint) // hrp (string), version (int), program ([]int)
				} else {
					address, _ = bech32.SegwitAddrEncode("bc", int(version), programint) // hrp (string), version (int), program ([]int)
				}

				if nsize == 28 && script[0] == 0 && script[1] == 20 { // P2WPKH (script type is 28, which means length of script is 22 bytes)
					scriptType = WITNESS_V0_KEYHASH
					scriptTypeCount[WITNESS_V0_KEYHASH]++
				} else if nsize == 40 && script[0] == 0 && script[1] == 32 { // P2WSH (script type is 40, which means length of script is 34 bytes)
					scriptType = WITNESS_V0_SCRIPTHASH
					scriptTypeCount[WITNESS_V0_SCRIPTHASH]++
				} else if nsize == 40 && script[0] == 0x51 && script[1] == 32 { // P2TR
					scriptType = WITNESS_V1_TAPROOT
					scriptTypeCount[WITNESS_V1_TAPROOT]++
				} else { // P2W?
					scriptType = WITNESS_UNKNOWN
					scriptTypeCount[WITNESS_UNKNOWN]++
				}
			}

			// Non-Standard (if the script type hasn't been identified and set then it remains as an unknown "non-standard" script)
			if scriptType == "non-standard" {
				scriptTypeCount["non-standard"]++
			}

			// add address and script type to results map
			output.Address = address
			output.Type = scriptType

			// -------
			// Results
			// -------

			// Print Results
			// -------------
			if i > 0 && i%BUF_SIZE == 0 {
				wg.Add(1)
				go insertUTXO(ctx, utxoBuf, wg, i, utxoCollection)
			}

			// Write to File
			// -------------
			// Write to buffer (use bufio for faster writes)
			utxoBuf[i%BUF_SIZE] = output

			// Increment Count
			i++
		}
	}

	wg.Add(1)

	go insertUTXO(ctx, utxoBuf[:i%BUF_SIZE], wg, i, utxoCollection)

	iter.Release() // Do not defer this, want to release iterator before closing database

	wg.Wait()

	// Final Progress Report
	// ---------------------
	log.Printf("Total spendable UTXOs: %d\n", i)

	// Can only show total btc amount if we have requested to get the amount for each entry with the -f fields flag
	log.Printf("Total BTC:   %.8f\n", float64(totalAmount)/float64(100000000)) // convert satoshis to BTC (float with 8 decimal places)

	// Can only show script type stats if we have requested to get the script type for each entry with the -f fields flag
	log.Println("Script Types:")
	for k, v := range scriptTypeCount {
		log.Printf(" %-12s %d\n", k, v) // %-12s = left-justify padding
	}
}
