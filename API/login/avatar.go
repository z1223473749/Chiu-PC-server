package login

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math/rand"
	"os"
	"path/filepath"
)

// colors 预定义的头像背景颜色（柔和色系）
var colors = []color.RGBA{
	{79, 172, 254, 255},  // 蓝
	{252, 161, 78, 255},  // 橙
	{131, 214, 121, 255}, // 绿
	{249, 117, 148, 255}, // 粉
	{163, 129, 255, 255}, // 紫
	{255, 193, 82, 255},  // 黄
	{83, 201, 195, 255},  // 青
	{243, 138, 173, 255}, // 玫红
	{161, 195, 232, 255}, // 淡蓝
	{206, 163, 255, 255}, // 淡紫
	{255, 165, 108, 255}, // 杏
	{128, 222, 164, 255}, // 薄荷
	{255, 133, 127, 255}, // 珊瑚
	{159, 168, 218, 255}, // 薰衣草
	{248, 199, 114, 255}, // 香槟
	{130, 204, 221, 255}, // 天蓝
	{229, 159, 187, 255}, // 樱花
	{175, 215, 157, 255}, // 草绿
	{249, 176, 143, 255}, // 蜜桃
	{192, 172, 230, 255}, // 鸢尾
	{255, 210, 130, 255}, // 淡金
	{167, 199, 231, 255}, // 雾蓝
}

// InitAvatars 初始化默认头像
// 在 public/avatar/ 目录下生成 22 张默认头像（1.png ~ 22.png）
func InitAvatars(avatarDir string) error {
	// 确保目录存在
	if err := os.MkdirAll(avatarDir, 0755); err != nil {
		return fmt.Errorf("创建头像目录失败: %w", err)
	}

	// 检查是否已经有头像文件
	existing, _ := os.ReadDir(avatarDir)
	if len(existing) >= 10 {
		// 已有头像，跳过生成
		return nil
	}

	for i := 0; i < len(colors); i++ {
		filePath := filepath.Join(avatarDir, fmt.Sprintf("%d.png", i+1))
		if _, err := os.Stat(filePath); err == nil {
			continue
		}
		if err := generateAvatar(filePath, colors[i]); err != nil {
			return fmt.Errorf("生成头像 %d.png 失败: %w", i+1, err)
		}
	}

	fmt.Printf("[头像] 已生成 %d 张默认头像到 %s\n", len(colors), avatarDir)
	return nil
}

// generateAvatar 生成单个头像文件
func generateAvatar(filePath string, bgColor color.RGBA) error {
	const size = 128

	img := image.NewRGBA(image.Rect(0, 0, size, size))

	// 填充背景色
	draw.Draw(img, img.Bounds(), &image.Uniform{bgColor}, image.Point{}, draw.Src)

	// 用稍深的颜色画一个随机圆形/装饰点（让每张图有差异）
	r := rand.New(rand.NewSource(int64(hashColor(bgColor))))
	for i := 0; i < 20; i++ {
		x := r.Intn(size)
		y := r.Intn(size)
		rSize := r.Intn(15) + 3
		darken := 30
		dotColor := color.RGBA{
			R: clamp(bgColor.R - uint8(r.Intn(darken))),
			G: clamp(bgColor.G - uint8(r.Intn(darken))),
			B: clamp(bgColor.B - uint8(r.Intn(darken))),
			A: 180,
		}
		drawCircle(img, x, y, rSize, dotColor)
	}

	f, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer f.Close()

	return png.Encode(f, img)
}

func hashColor(c color.RGBA) int64 {
	return int64(c.R)*1000000 + int64(c.G)*1000 + int64(c.B)
}

func clamp(v uint8) uint8 {
	if v > 255 {
		return 255
	}
	return v
}

func drawCircle(img *image.RGBA, cx, cy, radius int, c color.Color) {
	for y := -radius; y <= radius; y++ {
		for x := -radius; x <= radius; x++ {
			if x*x+y*y <= radius*radius {
				px := cx + x
				py := cy + y
				if px >= 0 && px < img.Bounds().Max.X && py >= 0 && py < img.Bounds().Max.Y {
					img.Set(px, py, c)
				}
			}
		}
	}
}
