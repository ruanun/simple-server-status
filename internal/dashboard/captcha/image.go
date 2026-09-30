// Package captcha 提供登录验证码：自绘的图形验证码与 Cloudflare Turnstile 核验。
package captcha

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	mrand "math/rand/v2"

	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"
)

// codeAlphabet 验证码字符集：去掉容易混淆的 0/O、1/I/L
const codeAlphabet = "23456789ABCDEFGHJKMNPQRSTUVWXYZ"

// CodeLen 验证码长度
const CodeLen = 4

const (
	imgW  = 140
	imgH  = 48
	scale = 3 // basicfont 字形为 7×13，放大后更易辨认
)

// randomCode 生成随机验证码（使用密码学安全随机数）
func randomCode() string {
	b := make([]byte, CodeLen)
	if _, err := rand.Read(b); err != nil {
		panic("读取系统随机数失败: " + err.Error())
	}
	for i := range b {
		b[i] = codeAlphabet[int(b[i])%len(codeAlphabet)]
	}
	return string(b)
}

// render 把验证码绘制为 PNG 并返回 data URL：每个字符随机偏移、倾斜与配色，叠加干扰线和噪点
func render(code string) (string, error) {
	img := image.NewRGBA(image.Rect(0, 0, imgW, imgH))
	draw.Draw(img, img.Bounds(), &image.Uniform{color.RGBA{245, 246, 248, 255}}, image.Point{}, draw.Src)

	for range 4 {
		line(img, jitter(imgW), jitter(imgH), jitter(imgW), jitter(imgH), randColor(120, 190))
	}

	face := basicfont.Face7x13
	cell := imgW / (len(code) + 1)
	for i, ch := range code {
		glyph := image.NewAlpha(image.Rect(0, 0, 7, 13))
		d := font.Drawer{Dst: glyph, Src: image.Opaque, Face: face, Dot: fixed.P(0, face.Ascent)}
		d.DrawString(string(ch))

		c := randColor(20, 110)
		x0 := cell/2 + i*cell + jitter(7) - 3
		y0 := (imgH-13*scale)/2 + jitter(7) - 3
		shear := float64(jitter(61)-30) / 100 // 每个字符单独倾斜
		for gy := range 13 * scale {
			dx := int(shear * float64(13*scale/2-gy))
			for gx := range 7 * scale {
				if glyph.AlphaAt(gx/scale, gy/scale).A == 0 {
					continue
				}
				setPixel(img, x0+gx+dx, y0+gy, c)
			}
		}
	}

	for range 2 {
		line(img, 0, jitter(imgH), imgW-1, jitter(imgH), randColor(40, 120))
	}
	for range 120 {
		setPixel(img, jitter(imgW), jitter(imgH), randColor(60, 200))
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return "", err
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}

// jitter 返回 [0, n) 的随机数，只用于位置、颜色等图形扰动；验证码内容由 randomCode 用 crypto/rand 生成
func jitter(n int) int {
	return mrand.IntN(n) //nolint:gosec // 图形扰动不需要密码学安全随机数
}

func randColor(lo, hi int) color.RGBA {
	n := func() uint8 { return uint8(lo + jitter(hi-lo)) } //nolint:gosec // 调用方保证 0 ≤ lo < hi ≤ 256
	return color.RGBA{n(), n(), n(), 255}
}

func setPixel(img *image.RGBA, x, y int, c color.RGBA) {
	if image.Pt(x, y).In(img.Bounds()) {
		img.SetRGBA(x, y, c)
	}
}

// line 用 Bresenham 算法画一条 1 像素宽的直线
func line(img *image.RGBA, x0, y0, x1, y1 int, c color.RGBA) {
	dx, dy := abs(x1-x0), -abs(y1-y0)
	sx, sy := sign(x1-x0), sign(y1-y0)
	e := dx + dy
	for {
		setPixel(img, x0, y0, c)
		if x0 == x1 && y0 == y1 {
			return
		}
		e2 := 2 * e
		if e2 >= dy {
			e += dy
			x0 += sx
		}
		if e2 <= dx {
			e += dx
			y0 += sy
		}
	}
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func sign(v int) int {
	switch {
	case v > 0:
		return 1
	case v < 0:
		return -1
	}
	return 0
}
