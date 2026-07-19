package utils

import (
	"archive/zip"
	"bytes"
	"crypto/aes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"io"
	"slices"
	"strings"

	"math/rand"

	"github.com/sandertv/gophertunnel/minecraft/resource"
)

type contents struct {
	Content []content `json:"content"`
}

type content struct {
	Path string `json:"path"`
	Key  string `json:"key"`
}

var ignoredFiles = []string{
	"manifest.json",
	"pack_icon.png",
}

// Encrypt encrypts a pack in place.
func Encrypt(pack *resource.Pack, key string) error {
	buf := bytes.NewBuffer([]byte{})
	err := EncryptToWriter(pack, buf, key)
	if err != nil {
		return err
	}
	updatePrivateField(pack, "contentKey", key)
	data := buf.Bytes()
	updatePrivateField(pack, "checksum", sha256.Sum256(data))
	updatePrivateField(pack, "content", bytes.NewReader(data))
	return nil
}

// EncryptToWriter encrypts a pack and writes the encrypted packs zip data to a writer.
func EncryptToWriter(pack *resource.Pack, writer io.Writer, key string) error {
	if pack.Encrypted() {
		return errors.New("cannot encrypt already encrypted pack")
	}
	packData := fetchPrivateField[*bytes.Reader](pack, "content")
	inputPack, err := zip.NewReader(packData, int64(pack.Len()))
	if err != nil {
		return err
	}

	newPack := zip.NewWriter(writer)

	var contents contents

	for _, f := range inputPack.File {
		if slices.Contains(ignoredFiles, strings.ToLower(f.Name)) {
			if err := newPack.Copy(f); err != nil {
				return err
			}
			continue
		}
		fileWriter, err := newPack.Create(f.Name)
		if err != nil {
			return err
		}

		fileKey := []byte(RandomString(32))
		reader, err := f.Open()
		if err != nil {
			return err
		}

		dataBytes, err := io.ReadAll(reader)
		if err != nil {
			return err
		}
		dataBytes, err = encryptCfb(dataBytes, fileKey)
		if err != nil {
			return err
		}
		if _, err := fileWriter.Write(dataBytes); err != nil {
			return err
		}
		contents.Content = append(contents.Content, content{Path: f.Name, Key: string(fileKey)})
	}

	contentJson, err := json.Marshal(&contents)
	if err != nil {
		return err
	}
	contentJsonEncrypted, err := encryptCfb(contentJson, []byte(key))
	if err != nil {
		return err
	}
	content, err := newPack.Create("contents.json")
	if err != nil {
		return err
	}

	header := bytes.NewBuffer(nil)
	header.Write([]byte{0, 0, 0, 0, 0xFC, 0xB9, 0xCF, 0x9B, 0, 0, 0, 0, 0, 0, 0, 0})
	header.Write([]byte("$" + pack.UUID().String()))
	header.Write(make([]byte, 256-header.Len()))
	if _, err := content.Write(header.Bytes()); err != nil {
		return err
	}
	if _, err := content.Write(contentJsonEncrypted); err != nil {
		return err
	}

	newPack.Close()

	return nil
}

func encryptCfb(data []byte, key []byte) ([]byte, error) {
	b, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	shiftRegister := make([]byte, 16)
	copy(shiftRegister, key[:16]) // prefill with iv
	_tmp := make([]byte, 16)
	off := 0
	for off < len(data) {
		b.Encrypt(_tmp, shiftRegister)
		data[off] ^= _tmp[0]
		shiftRegister = append(shiftRegister[1:], data[off])
		off++
	}
	return data, nil
}

var chars = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")

// RandomString is used for generating keys.
func RandomString(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = chars[rand.Intn(len(chars))]
	}
	return string(b)
}
