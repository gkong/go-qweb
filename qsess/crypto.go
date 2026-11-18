// Copyright 2016 George S. Kong. All rights reserved.
// Use of this source code is governed by a license that can be found in the LICENSE.txt file.

package qsess

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"io"
)

func (st *Store) makeCiphers(cipherkeys ...[]byte) error {
	for i, key := range cipherkeys {
		blk, err := aes.NewCipher(key)
		if err != nil {
			return qsErr{"makeCiphers - NewCipher", err}
		}

		st.ciphers[i], err = cipher.NewGCM(blk)
		if err != nil {
			return qsErr{"makeCiphers - NewGCM", err}
		}
	}
	return nil
}

// encrypt, then base64-encode
func (st *Store) encrypt(data []byte) ([]byte, error) {
	var encrypted []byte
	var err error

	if st.Encrypt == nil {
		nonce := make([]byte, st.ciphers[0].NonceSize())
		if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
			return nil, qsErr{"encrypt - could not create nonce", err}
		}
		encrypted = st.ciphers[0].Seal(nonce, nonce, data, st.aad())
	} else {
		encrypted, err = st.Encrypt(data)
		if err != nil {
			return nil, qsErr{"encrypt - user-supplied Encrypt failed", err}
		}
	}

	encoded := make([]byte, base64.URLEncoding.EncodedLen(len(encrypted)))
	base64.URLEncoding.Encode(encoded, encrypted)
	return encoded, nil
}

// base64-decode, then decrypt
func (st *Store) decrypt(data []byte) ([]byte, error) {
	decoded := make([]byte, base64.URLEncoding.DecodedLen(len(data)))
	decodedsize, err := base64.URLEncoding.Decode(decoded, data)
	if err != nil {
		return nil, qsErr{"decrypt - base64 decode failure", nil}
	}
	decoded = decoded[:decodedsize]

	if st.Decrypt == nil {
		nsize := st.ciphers[0].NonceSize()
		if len(decoded) < nsize {
			return nil, qsErr{"decrypt - data too small", nil}
		}
		nonce := decoded[:nsize]
		sealed := decoded[nsize:]
		aad := st.aad()
		for _, c := range st.ciphers {
			if decrypted, err := c.Open(nil, nonce, sealed, aad); err == nil {
				return decrypted, nil
			}
			if aad != nil && st.AcceptUnbound {
				if decrypted, err := c.Open(nil, nonce, sealed, nil); err == nil {
					return decrypted, nil
				}
			}
		}
		return nil, qsErr{"decrypt - could not Open", nil}
	} else {
		decrypted, err := st.Decrypt(decoded)
		if err != nil {
			return nil, qsErr{"decrypt - user-supplied Decrypt failed", err}
		}
		return decrypted, nil
	}
}

// aad returns the AEAD additional data, Name + "\x00" + Binding, or nil when both are empty,
// which is the unbound encoding that values had before Name and Binding existed.
func (st *Store) aad() []byte {
	if st.Name == "" && st.Binding == "" {
		return nil
	}
	return []byte(st.Name + "\x00" + st.Binding)
}
