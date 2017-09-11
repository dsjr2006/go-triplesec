# TripleSec
---

Golang implementation of the layered encryption scheme TripleSec

A fork of Fillipo's TripleSec

See [TripleSec Homepage](https://keybase.io/triplesec/) for more info.

--- 

## Installation

    go get github.com/keybase/go-triplesec

## Usage

    import "github.com/keybase/go-triplesec"

### Example
```go
    package main

    import (
    	"crypto/rand"
    	"fmt"
    	"github.com/keybase/go-triplesec"
    	"log"
    )

    func main() {
    	// Make Random 16 byte salt
    	passphrase := []byte("password1234567890")
    	salt := make([]byte, 16)
    	_, err := rand.Read(salt)
    	if err != nil {
    		log.Fatal("Could not create random salt")
    	}
    	// Create Cipher
    	cipher, err := triplesec.NewCipher(passphrase, salt)
    	if err != nil {
    		log.Fatal("Error creating new triple sec cipher")
    	}
    	fileBytes := []byte("testdata")
    	encryptedFileBytes, err := cipher.Encrypt(fileBytes)
        if err != nil {
            log.Fatal("Encryption error")
        }
    
    	// Do something with encrypted data such as save the file/send to remote
    }
```