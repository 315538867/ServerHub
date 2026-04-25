package crypto

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
)

// CopyAndSHA256 从 r 流式拷贝到 w，同时计算 SHA256。
// 返回 hex 字符串、字节数、错误。用于 Release 三维模型的 Artifact 上传去重。
func CopyAndSHA256(r io.Reader, w io.Writer) (string, int64, error) {
	h := sha256.New()
	n, err := io.Copy(io.MultiWriter(w, h), r)
	if err != nil {
		return "", n, err
	}
	return hex.EncodeToString(h.Sum(nil)), n, nil
}
