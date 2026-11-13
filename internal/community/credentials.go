package community

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"io"
	"os"
	"path/filepath"
)

// The local encryption key is outside the app bundle, private to the OS user.
// This protects plaintext database exports, not access by the same OS account.
func credentialCipher(dir string) (cipher.AEAD, error) {
	path := filepath.Join(dir, "credentials.key")
	key, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		key = make([]byte, 32)
		if _, err = rand.Read(key); err != nil {
			return nil, err
		}
		f, e := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if errors.Is(e, os.ErrExist) {
			return credentialCipher(dir)
		}
		if e != nil {
			return nil, e
		}
		_, err = f.Write(key)
		if err == nil {
			err = f.Sync()
		}
		closeErr := f.Close()
		if err == nil {
			err = closeErr
		}
	}
	if err != nil {
		return nil, err
	}
	if len(key) != 32 {
		return nil, errors.New("本地凭据密钥文件无效，请恢复 credentials.key 备份")
	}
	if err = os.Chmod(path, 0600); err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}
func (s *Store) sealKey(owner, key string) ([]byte, error) {
	nonce := make([]byte, s.credentials.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	return s.credentials.Seal(nonce, nonce, []byte(key), []byte(owner)), nil
}
func (s *Store) openKey(owner string, data []byte) (string, error) {
	size := s.credentials.NonceSize()
	if len(data) < size {
		return "", errors.New("本地 AppKey 数据损坏")
	}
	plain, err := s.credentials.Open(nil, data[:size], data[size:], []byte(owner))
	if err != nil {
		return "", errors.New("无法解密本地 AppKey，请确认 credentials.key 与数据库来自同一份备份")
	}
	return string(plain), nil
}
