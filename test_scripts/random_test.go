/*
 * @Author: t 921865806@qq.com
 * @Date: 2025-12-10 15:19:01
 * @LastEditors: t 921865806@qq.com
 * @LastEditTime: 2025-12-10 15:21:00
 * @FilePath: /examples/test_zap/rotatelogs/random.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package testscripts

import (
	"fmt"
	"math"
	"testing"
)

// 定义 LCG 结构体
type LCG struct {
	randomNext uint32 // 核心：必须是 uint32，依靠自然溢出
}

// 初始化函数
func NewLCG(seed uint32) *LCG {
	return &LCG{randomNext: seed}
}

// 1. Random: 基础生成函数
func (l *LCG) Random() uint32 {
	// JS原版: (this.randomNext * 1664525 + 1013904223) % LCG_M
	// Go实现: 不需要 %，uint32 溢出自动截断，且结果恒为正
	l.randomNext = l.randomNext*1664525 + 1013904223
	return l.randomNext
}

// 2. RandomInt: 生成 [min, max] 范围内的整数
func (l *LCG) RandomInt(min, max int) int {
	// 计算范围长度
	// 注意：这里假设 max >= min
	rangeLen := max - min + 1

	// 获取一个随机数
	rnd := l.Random()

	// JS原版: Math.abs(this.random() % (max - min + 1)) + min
	// Go实现:
	// 1. rnd 是 uint32，恒为正，不需要 Abs
	// 2. 将 rangeLen 转为 uint32 进行取模 (如果 rangeLen 很大，需注意类型匹配，通常游戏逻辑里int够用)
	// 3. 结果转回 int 加 min
	return int(rnd%uint32(rangeLen)) + min
}

// 3. RandomFloat: 生成 [min, max] 范围内的浮点数
func (l *LCG) RandomFloat(min, max float64) float64 {
	// JS原版: this.random() / LCG_M ...
	// Go实现:
	// LCG_M 就是 2^32 = 4294967296
	const LCG_M_FLOAT = 4294967296.0

	// 1. 归一化到 [0, 1)
	r := float64(l.Random()) / LCG_M_FLOAT

	// 2. 缩放并平移
	result := (max-min)*r + min

	// JS原版最后有个 Math.abs(r)，通常是因为 JS 的 random() 有时可能会被处理成有符号负数
	// 在 Go 里 uint32 转 float64 肯定是正数，除非 max < min，否则不需要 Abs
	return math.Abs(result)
}

func TestRandom(t *testing.T) {
	// 测试
	gen := NewLCG(123) // 种子 123

	fmt.Println("--- Random (uint32) ---")
	fmt.Println(gen.Random())

	fmt.Println("\n--- RandomInt [10, 20] ---")
	for i := 0; i < 5; i++ {
		fmt.Printf("%d ", gen.RandomInt(10, 20))
	}

	fmt.Println("\n\n--- RandomFloat [0.0, 1.0] ---")
	for i := 0; i < 5; i++ {
		fmt.Printf("%.4f ", gen.RandomFloat(0.0, 1.0))
	}
}
