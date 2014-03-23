package triplesec

import (
	"bytes"
	"code.google.com/p/go.crypto/salsa20"
	"code.google.com/p/go.crypto/scrypt"
	"code.google.com/p/go.crypto/sha3"
	"code.google.com/p/go.crypto/twofish"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha512"
	"encoding/binary"
	"fmt"
)

type Cipher struct {
	passphrase []byte
}

// magic bytes + version + salt + 2 * MACs + 3 * IVS
var Overhead = 4 + 4 + 16 + 64 + 64 + 16 + 16 + 24
var MagicBytes = []byte("\x1c\x94\xd7\xde")

var (
	saltSize     = 16
	macKeyLen    = 48
	cipherKeyLen = 32
	dkSize       = 2*macKeyLen + 3*cipherKeyLen
)

func NewCipher(passphrase []byte) (*Cipher, error) {
	if len(passphrase) < 1 {
		return nil, fmt.Errorf("the passphrase cannot be empty")
	}

	c := new(Cipher)
	c.passphrase = append(c.passphrase, passphrase...)

	return c, nil
}

func (c *Cipher) Encrypt(dst, plain []byte) error {
	if len(plain) < 1 {
		return fmt.Errorf("the plaintext cannot be empty")
	}
	if len(dst) < len(plain)+Overhead {
		return fmt.Errorf("the destination is shorter than the plaintext plus Overhead")
	}

	buf := bytes.NewBuffer(dst[:0])

	_, err := buf.Write(MagicBytes)
	if err != nil {
		return err
	}

	// Write version
	err = binary.Write(buf, binary.BigEndian, uint32(3))
	if err != nil {
		return err
	}

	salt := make([]byte, saltSize)
	_, err = rand.Read(salt)
	if err != nil {
		return err
	}
	_, err = buf.Write(salt)
	if err != nil {
		return err
	}

	dk, err := scrypt.Key(c.passphrase, salt, 32768, 8, 1, dkSize)
	if err != nil {
		return err
	}
	macKeys := dk[:macKeyLen*2]
	cipherKeys := dk[macKeyLen*2:]

	// The allocation over here can be made better
	encryptedData, err := encrypt_data(plain, cipherKeys)
	if err != nil {
		return err
	}

	authenticatedData := make([]byte, 0, buf.Len()+len(encryptedData))
	authenticatedData = append(authenticatedData, buf.Bytes()...)
	authenticatedData = append(authenticatedData, encryptedData...)
	macsOutput := generate_macs(authenticatedData, macKeys)

	_, err = buf.Write(macsOutput)
	if err != nil {
		return err
	}
	_, err = buf.Write(encryptedData)
	if err != nil {
		return err
	}

	if buf.Len() != len(plain)+Overhead {
		panic(fmt.Errorf("something went terribly wrong: output size is not consistent"))
	}

	return nil
}

func encrypt_data(plain, keys []byte) ([]byte, error) {
	var iv, key []byte
	var block cipher.Block
	var stream cipher.Stream

	iv_offset := 16 + 16 + 24
	res := make([]byte, len(plain)+iv_offset)

	iv = res[iv_offset-24 : iv_offset]
	_, err := rand.Read(iv)
	if err != nil {
		return nil, err
	}
	// For some reason salsa20 API is different
	key_array := new([32]byte)
	copy(key_array[:], keys[cipherKeyLen*2:])
	salsa20.XORKeyStream(res[iv_offset:], plain, iv, key_array)
	iv_offset -= 24

	iv = res[iv_offset-16 : iv_offset]
	_, err = rand.Read(iv)
	if err != nil {
		return nil, err
	}
	key = keys[cipherKeyLen : cipherKeyLen*2]
	block, err = twofish.NewCipher(key)
	if err != nil {
		return nil, err
	}
	stream = cipher.NewCTR(block, iv)
	stream.XORKeyStream(res[iv_offset:], res[iv_offset:])
	iv_offset -= 16

	iv = res[iv_offset-16 : iv_offset]
	_, err = rand.Read(iv)
	if err != nil {
		return nil, err
	}
	key = keys[:cipherKeyLen]
	block, err = aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	stream = cipher.NewCTR(block, iv)
	stream.XORKeyStream(res[iv_offset:], res[iv_offset:])
	iv_offset -= 16

	if iv_offset != 0 {
		panic(fmt.Errorf("something went terribly wrong: iv_offset final value non-zero"))
	}

	return res, nil
}

func generate_macs(data, keys []byte) []byte {
	res := make([]byte, 0, 64*2)

	key := keys[:macKeyLen]
	mac := hmac.New(sha512.New, key)
	mac.Write(data)
	res = mac.Sum(res)

	key = keys[macKeyLen:]
	mac = hmac.New(sha3.NewKeccak512, key)
	mac.Write(data)
	res = mac.Sum(res)

	return res
}

func (c *Cipher) Decrypt(dst, ciphertext []byte) error {
	if len(ciphertext) <= Overhead {
		return fmt.Errorf("the ciphertext is too short to be a TripleSec ciphertext")
	}
	if len(dst) < len(ciphertext)-Overhead {
		return fmt.Errorf("the dst buffer is too short to hold the plaintext")
	}

	if !bytes.Equal(ciphertext[:4], MagicBytes) {
		return fmt.Errorf("the ciphertext does not look like a TripleSec ciphertext")
	}

	v := make([]byte, 4)
	v_b := bytes.NewBuffer(v[:0])
	err := binary.Write(v_b, binary.BigEndian, uint32(3))
	if err != nil {
		return err
	}
	if !bytes.Equal(ciphertext[4:8], v) {
		return fmt.Errorf("unknown version")
	}

	salt := ciphertext[8:24]
	dk, err := scrypt.Key(c.passphrase, salt, 32768, 8, 1, dkSize)
	if err != nil {
		return err
	}
	macKeys := dk[:macKeyLen*2]
	cipherKeys := dk[macKeyLen*2:]

	macs := ciphertext[24 : 24+64*2]
	encryptedData := ciphertext[24+64*2:]

	authenticatedData := make([]byte, 0, 24+len(encryptedData))
	authenticatedData = append(authenticatedData, ciphertext[:24]...)
	authenticatedData = append(authenticatedData, encryptedData...)

	if !hmac.Equal(macs, generate_macs(authenticatedData, macKeys)) {
		return fmt.Errorf("TripleSec ciphertext authentication FAILED")
	}

	err = decrypt_data(dst, encryptedData, cipherKeys)
	if err != nil {
		return err
	}

	return nil
}

func decrypt_data(dst, data, keys []byte) error {
	var iv, key []byte
	var block cipher.Block
	var stream cipher.Stream
	var err error

	buffer := append([]byte{}, data...)

	iv_offset := 16
	iv = buffer[:iv_offset]
	key = keys[:cipherKeyLen]
	block, err = aes.NewCipher(key)
	if err != nil {
		return err
	}
	stream = cipher.NewCTR(block, iv)
	stream.XORKeyStream(buffer[iv_offset:], buffer[iv_offset:])

	iv_offset += 16
	iv = buffer[iv_offset-16 : iv_offset]
	key = keys[cipherKeyLen : cipherKeyLen*2]
	block, err = twofish.NewCipher(key)
	if err != nil {
		return err
	}
	stream = cipher.NewCTR(block, iv)
	stream.XORKeyStream(buffer[iv_offset:], buffer[iv_offset:])

	iv_offset += 24
	iv = buffer[iv_offset-24 : iv_offset]
	key_array := new([32]byte)
	copy(key_array[:], keys[cipherKeyLen*2:])
	salsa20.XORKeyStream(dst, buffer[iv_offset:], iv, key_array)

	if len(buffer[iv_offset:]) != len(data)-(16+16+24) {
		panic(fmt.Errorf("something went terribly wrong: buffer size is not consistent"))
	}

	return nil
}