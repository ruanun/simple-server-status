// Package randx 生成密码学安全的随机字符串。
package randx

import "crypto/rand"

const alphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

// String 生成长度为 n 的字母数字随机串
func String(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic("读取系统随机数失败: " + err.Error())
	}
	for i := range b {
		b[i] = alphabet[int(b[i])%len(alphabet)]
	}
	return string(b)
}
