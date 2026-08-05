package main

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"sync"
	_ "image/gif" // 注册 gif 解码器
)

// thumbnailCache 内存缩略图缓存：key = path|maxDim|size|mtime，改图/换图自动失效。
var thumbnailCache = struct {
	sync.RWMutex
	m map[string][]byte
}{m: map[string][]byte{}}

// thumbCacheMax 缓存条目上限（超限直接清空，简单可靠）
const thumbCacheMax = 256

// imageThumbnail 读取 path 图片并缩放（最长边 <= maxDim），按原文件类型重编码返回。
// 返回 (内容, Content-Type, 是否需要原图回退)。
// png/jpeg/gif 用标准库解码；其他格式（如 webp/animated）解码失败时 ok=false，调用方回退原图。
func imageThumbnail(path string, maxDim int) ([]byte, string, bool) {
	if maxDim < 16 {
		maxDim = 16
	}
	if maxDim > 2000 {
		maxDim = 2000
	}
	fi, err := os.Stat(path)
	if err != nil || fi.IsDir() {
		return nil, "", false
	}
	key := fmt.Sprintf("%s|%d|%d|%d", path, maxDim, fi.Size(), fi.ModTime().UnixNano())

	thumbnailCache.RLock()
	if data, hit := thumbnailCache.m[key]; hit {
		thumbnailCache.RUnlock()
		return data, thumbnailContentType(path), true
	}
	thumbnailCache.RUnlock()

	f, err := os.Open(path)
	if err != nil {
		return nil, "", false
	}
	defer f.Close()

	img, format, err := image.Decode(f)
	if err != nil {
		return nil, "", false
	}
	thumb := resizeImage(img, maxDim)

	var buf bytes.Buffer
	var ct string
	// PNG 可能带透明，保持原格式编码；其余统一 JPEG（体积小、内容类型稳定）
	if strings.EqualFold(format, "png") {
		if err := png.Encode(&buf, thumb); err != nil {
			return nil, "", false
		}
		ct = "image/png"
	} else {
		if err := jpeg.Encode(&buf, thumb, &jpeg.Options{Quality: 82}); err != nil {
			return nil, "", false
		}
		ct = "image/jpeg"
	}
	out := buf.Bytes()

	thumbnailCache.Lock()
	if len(thumbnailCache.m) > thumbCacheMax {
		thumbnailCache.m = map[string][]byte{}
	}
	thumbnailCache.m[key] = out
	thumbnailCache.Unlock()

	return out, ct, true
}

// thumbnailContentType 按扩展名返回缩略图内容类型（与重编码结果一致，供命中缓存时使用）
func thumbnailContentType(path string) string {
	if strings.EqualFold(filepath.Ext(path), ".png") {
		return "image/png"
	}
	return "image/jpeg"
}

// resizeImage 将 src 缩放为最长边 <= maxDim 的新图（双线性采样，返回 NRGBA，避免预乘色偏）
func resizeImage(src image.Image, maxDim int) image.Image {
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	if w <= 0 || h <= 0 {
		return src
	}
	if w <= maxDim && h <= maxDim {
		return src
	}
	scale := float64(maxDim) / float64(w)
	if h > w {
		scale = float64(maxDim) / float64(h)
	}
	nw := int(float64(w)*scale + 0.5)
	nh := int(float64(h)*scale + 0.5)
	if nw < 1 {
		nw = 1
	}
	if nh < 1 {
		nh = 1
	}
	dst := image.NewNRGBA(image.Rect(0, 0, nw, nh))
	step := 0.8 // 采样间步子像素，避免大缩小整体偏移
	for y := 0; y < nh; y++ {
		fy := float64(y)/scale + step
		if fy > float64(h-1) {
			fy = float64(h - 1)
		}
		for x := 0; x < nw; x++ {
			fx := float64(x)/scale + step
			if fx > float64(w-1) {
				fx = float64(w - 1)
			}
			dst.SetNRGBA(x, y, sampleBilinear(src, b, fx, fy))
		}
	}
	return dst
}

// sampleBilinear 对 src 在 (fx,fy) 处做双线性采样（工作于非预乘色空间）
func sampleBilinear(src image.Image, b image.Rectangle, fx, fy float64) color.NRGBA {
	x0 := int(fx)
	y0 := int(fy)
	if x0 < b.Min.X {
		x0 = b.Min.X
	}
	if y0 < b.Min.Y {
		y0 = b.Min.Y
	}
	x1 := x0 + 1
	if x1 >= b.Max.X {
		x1 = x0
	}
	y1 := y0 + 1
	if y1 >= b.Max.Y {
		y1 = y0
	}
	n00 := toNRGBA(src.At(x0, y0))
	n10 := toNRGBA(src.At(x1, y0))
	n01 := toNRGBA(src.At(x0, y1))
	n11 := toNRGBA(src.At(x1, y1))
	dx := fx - float64(x0)
	dy := fy - float64(y0)
	if dx < 0 {
		dx = 0
	}
	if dy < 0 {
		dy = 0
	}
	if dx > 1 {
		dx = 1
	}
	if dy > 1 {
		dy = 1
	}
	fr0 := float32(n00.R)
	fg0 := float32(n00.G)
	fb0 := float32(n00.B)
	fa0 := float32(n00.A)
	fr := fr0 + float32(dx)*(float32(n10.R)-fr0)
	fg := fg0 + float32(dx)*(float32(n10.G)-fg0)
	fb := fb0 + float32(dx)*(float32(n10.B)-fb0)
	fa := fa0 + float32(dx)*(float32(n10.A)-fa0)
	fr += float32(dy) * ((float32(n01.R) + float32(dx)*(float32(n11.R)-float32(n01.R))) - fr)
	fg += float32(dy) * ((float32(n01.G) + float32(dx)*(float32(n11.G)-float32(n01.G))) - fg)
	fb += float32(dy) * ((float32(n01.B) + float32(dx)*(float32(n11.B)-float32(n01.B))) - fb)
	fa += float32(dy) * ((float32(n01.A) + float32(dx)*(float32(n11.A)-float32(n01.A))) - fa)
	return color.NRGBA{R: clamp8(fr), G: clamp8(fg), B: clamp8(fb), A: clamp8(fa)}
}

func toNRGBA(c color.Color) color.NRGBA {
	if n, ok := c.(color.NRGBA); ok {
		return n
	}
	return color.NRGBAModel.Convert(c).(color.NRGBA)
}

func clamp8(v float32) uint8 {
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return uint8(v)
}