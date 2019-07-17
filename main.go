package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"math/big"
	"os"
	"os/signal"
	"time"

	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/accounts/keystore"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/console"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/yondonfu/eth-tx-filler/gasprice"
	"github.com/yondonfu/eth-tx-filler/tx"
)

func run() error {
	// sender flags
	senderAddr := flag.String("senderAddr", "", "tx sender address")
	keystoreDir := flag.String("keystoreDir", "", "keystore directory")
	chainID := flag.Int("chainID", -1, "chain ID")
	provider := flag.String("provider", "http://localhost:8545", "ETH provider URL")
	sendInterval := flag.Int("sendInterval", 5, "time interval to send tx in seconds")
	// gas price flags
	randomizeInterval := flag.Int("randomizeInterval", 30, "time interval to randomize gas price in seconds")
	maxGasPrice := flag.String("maxGasPrice", "1000", "max for randomizing gas price")
	minGasPrice := flag.String("minGasPrice", "100", "min for randomizing gas price")

	flag.Parse()

	if *senderAddr == "" {
		return errors.New("need -senderAddr")
	}

	if *keystoreDir == "" {
		return errors.New("need -keystoreDir")
	}

	if *chainID == -1 {
		return errors.New("need -chainID")
	}

	if *provider == "" {
		return errors.New("need -provider")
	}

	if *sendInterval <= 0 {
		return errors.New("-senderInterval must be > 0")
	}

	if *randomizeInterval <= 0 {
		return errors.New("-randomizeInterval must be > 0")
	}

	bigMaxGasPrice, ok := new(big.Int).SetString(*maxGasPrice, 10)
	if !ok || bigMaxGasPrice.Cmp(big.NewInt(0)) < 0 {
		return errors.New("-maxGasPrice must be >= 0")
	}

	bigMinGasPrice, ok := new(big.Int).SetString(*minGasPrice, 10)
	if !ok || bigMinGasPrice.Cmp(big.NewInt(0)) < 0 {
		return errors.New("-minGasPrice must be >= 0")
	}

	randomizer := gasprice.NewRandomizer(time.Duration(*randomizeInterval)*time.Second, bigMaxGasPrice, bigMinGasPrice)

	randomizer.Start()
	defer randomizer.Stop()

	addr := common.HexToAddress(*senderAddr)
	ks := keystore.NewKeyStore(*keystoreDir, keystore.StandardScryptN, keystore.StandardScryptP)
	acct, err := ks.Find(accounts.Account{Address: addr})
	if err != nil {
		return err
	}

	wallet, err := findWallet(ks, acct)
	if err != nil {
		return err
	}

	passphrase, err := console.Stdin.PromptPassword("Passphrase: ")
	if err != nil {
		return err
	}

	if err := ks.Unlock(acct, passphrase); err != nil {
		return err
	}

	client, err := ethclient.Dial(*provider)
	if err != nil {
		return err
	}

	sender := tx.NewSender(acct, wallet, big.NewInt(int64(*chainID)), client, randomizer, time.Duration(*sendInterval)*time.Second)
	sender.Start()
	defer sender.Stop()

	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt)
	<-c

	return nil
}

func findWallet(ks *keystore.KeyStore, acct accounts.Account) (accounts.Wallet, error) {
	wallets := ks.Wallets()
	for _, w := range wallets {
		accts := w.Accounts()
		if len(accts) > 0 && accts[0] == acct {
			return w, nil
		}
	}

	return nil, fmt.Errorf("wallet for %x not found", acct.Address)
}

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}
