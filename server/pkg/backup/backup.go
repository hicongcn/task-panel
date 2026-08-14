// Package backup 提供加密备份的打包/加密与解密/解包能力。
//
// 备份文件格式:magic(8B) || nonce(12B) || AES-256-GCM(ciphertext || tag)
// 明文内容为 tar.gz 归档,内含数据库与脚本目录。
package backup

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const magic = "TPBACK1\x00"

// Encrypt 使用 32 字节密钥 AES-256-GCM 加密,前缀 magic 作为附加认证数据。
func Encrypt(plaintext, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	sealed := gcm.Seal(nil, nonce, plaintext, []byte(magic))
	out := make([]byte, 0, len(magic)+len(nonce)+len(sealed))
	out = append(out, magic...)
	out = append(out, nonce...)
	out = append(out, sealed...)
	return out, nil
}

// Decrypt 解密 Encrypt 的输出,校验 magic 与 GCM 认证。
func Decrypt(data, key []byte) ([]byte, error) {
	if len(data) < len(magic) {
		return nil, errors.New("备份文件格式错误")
	}
	if string(data[:len(magic)]) != magic {
		return nil, errors.New("不是有效的 task-panel 备份文件")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	rest := data[len(magic):]
	if len(rest) < gcm.NonceSize()+gcm.Overhead() {
		return nil, errors.New("备份文件损坏或密钥不匹配")
	}
	nonce := rest[:gcm.NonceSize()]
	ciphertext := rest[gcm.NonceSize():]
	return gcm.Open(nil, nonce, ciphertext, []byte(magic))
}

// TarGz 将 sources(归档内路径 -> 本地路径)打包为 tar.gz 字节流。
// 目录会被递归打包。
func TarGz(sources map[string]string) ([]byte, error) {
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)

	for arcPath, localPath := range sources {
		info, err := os.Stat(localPath)
		if err != nil {
			return nil, fmt.Errorf("读取 %s 失败: %w", localPath, err)
		}
		if info.IsDir() {
			err = filepath.Walk(localPath, func(p string, fi os.FileInfo, walkErr error) error {
				if walkErr != nil {
					return walkErr
				}
				if fi.IsDir() {
					return nil
				}
				rel, err := filepath.Rel(localPath, p)
				if err != nil {
					return err
				}
				name := filepath.ToSlash(filepath.Join(arcPath, rel))
				return addFile(tw, name, p, fi)
			})
			if err != nil {
				return nil, err
			}
		} else {
			if err := addFile(tw, arcPath, localPath, info); err != nil {
				return nil, err
			}
		}
	}

	if err := tw.Close(); err != nil {
		return nil, err
	}
	if err := gw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func addFile(tw *tar.Writer, name, path string, fi os.FileInfo) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	hdr := &tar.Header{
		Name:    name,
		Mode:    int64(fi.Mode().Perm()),
		Size:    fi.Size(),
		ModTime: fi.ModTime(),
	}
	if err := tw.WriteHeader(hdr); err != nil {
		return err
	}
	_, err = io.Copy(tw, f)
	return err
}

// UntarGz 将 tar.gz 字节流解包到 destDir,带路径穿越防护。
func UntarGz(data []byte, destDir string) error {
	gr, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return err
	}
	defer gr.Close()

	tr := tar.NewReader(gr)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		name := filepath.Clean(filepath.FromSlash(hdr.Name))
		if name == "." || name == ".." || strings.HasPrefix(name, ".."+string(filepath.Separator)) || filepath.IsAbs(name) {
			return fmt.Errorf("非法归档路径: %s", hdr.Name)
		}
		target := filepath.Join(destDir, name)
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			f, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(hdr.Mode))
			if err != nil {
				return err
			}
			if _, err := io.Copy(f, tr); err != nil {
				f.Close()
				return err
			}
			f.Close()
		default:
			// 忽略符号链接/设备等,避免解包引入不安全项。
			continue
		}
	}
	return nil
}

// CreateBackup 打包并加密 sources 到 dest 文件。
func CreateBackup(dest string, key []byte, sources map[string]string) error {
	tgz, err := TarGz(sources)
	if err != nil {
		return err
	}
	enc, err := Encrypt(tgz, key)
	if err != nil {
		return err
	}
	return os.WriteFile(dest, enc, 0o600)
}

// ExtractBackup 解密并解包 src 到 destDir。
func ExtractBackup(src string, key []byte, destDir string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	plain, err := Decrypt(data, key)
	if err != nil {
		return err
	}
	return UntarGz(plain, destDir)
}
