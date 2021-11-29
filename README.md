# Bitcoin UTXO Dump

**Warning:** Stop the bitcoin node prior to running this tool.
If there's any error related to a corrupted database,
you will need to run `bitcoind -reindex-chainstate` the next time you run bitcoin, and this usually takes around a day to complete.

-----
Get a **list of every unspent bitcoin** in the blockchain.

The program iterates over each entry in Bitcoin Core's `chainstate` [LevelDB](http://leveldb.org/) database.
It decompresses and decodes the data,
and produces a human-readable text dump of all the
[UTXO](http://learnmeabitcoin.com/glossary/utxo)s (unspent transaction outputs).

## Install

First of all, you need to have a full copy of the blockchain. You also need to install LevelDB:

```
sudo apt install bitcoind
sudo apt install libleveldb-dev
```

After that, if you have [Go](https://golang.org/) installed you can do:

```
go get github.com/ABMatrix/bitcoin-utxo
```

This will create a binary called `bitcoin-utxo`, which you can call from the command line:

```
$ export MONGO_URI='mongodb://user:password@<ip>:<port>/?authSource=admin&authMechanism=SCRAM-SHA-1[&ssl=true]'
$ export MONGO_UTXO_DB_NAME=<db_name>
$ bitcoin-utxo --db /path/to/chainstate/ --testnet > /root/bioin-utxo-testnet.log 2>&1 &
```
`[&ssl=true]` is optional depending on how you connect to your mongodb.
This will start dumping all the UTXO from the testnet to mongodb.
After that, you will see utxos residing in either `utxo-testnet` or `utxo-mainnet` depending on which network you specified.

**NOTE:** This program reads the chainstate LevelDB database created by `bitcoind`, so you will need to download and sync `bitcoind` for this script to work. In other words, this script reads your own local copy of the blockchain.

**NOTE:** LevelDB wasn't designed to be accessed by multiple programs at the same time, so make sure `bitcoind` isn't running before you start (`bitcoin-cli stop` should do it).


## Usage

The basic command is:

```
$ export MONGO_URI='mongodb://user:password@<ip>:<port>/?authSource=admin&authMechanism=SCRAM-SHA-1[&ssl=true]'
$ export MONGO_UTXO_DB_NAME=<db_name>
$ bitcoin-utxo
```

You **must** set the 2 environment variables prior to running the `bitcoin-utxo` command.

If you know that the `chainstate` LevelDB folder is in a different location to the default (e.g. you want to get a UTXO dump of the _Testnet_ blockchain), use the `-db` option:

```
$ bitcoin-utxo-dump  --db /home/bitcoin/.bitcoin/testnet3/chainstate/ --testnet
```

By default this script does not convert the public keys inside P2PK locking scripts to addresses (because technically they do not have an address). However, sometimes it may be useful to get addresses for them anyway for use with other APIs, so the following option allows you to return the "address" for UTXOs with P2PK locking scripts:

```
$ bitcoin-utxo -p2pkaddresses
```

### Fields for each mongodb document
* **txid** - [Transaction ID](http://learnmeabitcoin.com/glossary/txid) for the output.
* **vout** - The index number of the transaction output (which output in the transaction is it?).
* **height** - The height of the block the transaction was mined in.
* **coinbase** - Whether the output is from a coinbase transaction (i.e. claiming a block reward).
* **amount** - The value of the output in _satoshis_.
* **script** - The locking script placed on the output (this is just the hash160 public key or hash160 script for a P2PK, P2PKH, or P2SH)
* **type** - The type of locking script (e.g. P2PK, P2PKH, P2SH, P2MS, P2WPKH, P2WSH, or non-standard)
* **address** - The address the output is locked to (this is generally just the locking script in a shorter format with user-friendly characters).


All other options can be found with `-h`:

```
$ bitcoin-utxo-dump -h
```

## FAQ

### How long does this script take to run?

It takes me about **20 minutes** to get all the UTXOs.

This obviously this depends on how big the UTXO database is and how fast your computer is. For me, the UTXO database had 52 million entries, and I'm using a Thinkpad X220 (with a SSD).

Either way, I'd probably make a cup of tea after it starts running.

### How big is the file?

The resultant mongodb size should be around **7GB** (roughly **2.5 times the size** of the LevelDB database: `du -h ~/.bitcoin/chainstate/`).

### What versions of bitcoin does this tool work with?

This tool works for Bitcoin Core [0.22.1](https://bitcoincore.org/en/releases/22.0/) and above. You can check your version with `bitcoind --version`.

Older versions of bitcoind have a different chainstate LevelDB structure. The structure was updated in 0.22.0 to make reading from the database more memory-efficient. Here's an interesting talk by [Chris Jeffrey](https://youtu.be/0WCaoGiAOHE?t=8936) that explains how you could crash Bitcoin Core with the old chainstate database structure.

Nonetheless, if you really want to parse an _old-style_ chainstate database, try one of the _similar tools_ at the bottom of this page.

### How does this program work?

This program just iterates through all the entries in the LevelDB database at `~/.bitcoin/chainstate`.

However, the data inside `~/.bitcoin/chainstate` has been _obfuscated_ (to prevent triggering anti-virus software) and _compressed_ (to reduce the size on disk), so it's far from being human-readable. This script just deobfuscates each entry and decodes/decompresses the data to get human-readable data for each UTXO in the database.

![](assets/bitcoin-utxo-dump.png)

### Can I parse the chainstate LevelDB myself?

Sure. Most programming languages seem to have libraries for reading a LevelDB database.

* [Go](https://github.com/syndtr/goleveldb)
* [Ruby](https://github.com/wmorgan/leveldb-ruby)
* [Python](https://github.com/wbolster/plyvel)

The trickier part is decoding the data for each UTXO in the database:

```
       type                          txid (little-endian)                      index (varint)
           \                               |                                  /
           <><--------------------------------------------------------------><>
    key:   430000155b9869d56c66d9e86e3c01de38e3892a42b99949fe109ac034fff6583900

    value: 71a9e87d62de25953e189f706bcf59263f15de1bf6c893bda9b045 <- obfuscated
           b12dcefd8f872536b12dcefd8f872536b12dcefd8f872536b12dce <- extended obfuscateKey
           c0842680ed5900a38f35518de4487c108e3810e6794fb68b189d8b <- deobfuscated (XOR)
           <----><----><><-------------------------------------->
            /      |    \                   |
       varint   varint   varint          script <- P2PKH/P2SH hash160, P2PK public key, or complete script
          |        |     nSize
          |        |
          |     amount (compressesed)
          |
          |
   100000100001010100110
   <------------------> \
          height         coinbase
```

## Thanks

 * This script was inspired by the [bitcoin_tools](https://github.com/sr-gi/bitcoin_tools) repo made by [Sergi Delgado Segura](https://github.com/sr-gi). I wanted to see if I could get a faster dump of the UTXO database by writing the program in Go, in addition to getting the **addresses** for each of the UTXOs. The decoding and decompressing code in his repo helped me to write this tool.

### Similar Tools

 * [github.com/sr-gi/bitcoin_tools](https://github.com/sr-gi/bitcoin_tools)
 * [github.com/in3rsha/bitcoin-chainstate-parser](https://github.com/in3rsha/bitcoin-chainstate-parser)
 * [github.com/in3rsha/bitcoin-utxo-dump](https://github.com/in3rsha/bitcoin-utxo-dump)
 * [github.com/mycroft/chainstate](https://github.com/mycroft/chainstate)
 * [laanwj (unfinished)](https://github.com/bitcoin/bitcoin/pull/7759)

### Links

 * <https://github.com/syndtr/goleveldb>
 * <https://github.com/bitcoin/bitcoin/blob/master/src/compressor.cpp>
 * <https://bitcoin.stackexchange.com/questions/51387/how-does-bitcoin-read-from-write-to-leveldb/52167#52167>
 * <https://bitcoin.stackexchange.com/questions/52257/chainstate-leveldb-corruption-after-reading-from-the-database>
 * <https://bitcoin.stackexchange.com/questions/85710/does-the-chainstate-leveldb-only-contain-addresses-for-p2pkh-and-p2sh>
 * <https://github.com/bitcoin/bitcoin/issues/14584>
