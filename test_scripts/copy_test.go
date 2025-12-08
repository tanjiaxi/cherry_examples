/*
 * @Author: t 921865806@qq.com
 * @Date: 2025-12-03 14:29:53
 * @LastEditors: t 921865806@qq.com
 * @LastEditTime: 2025-12-03 14:59:09
 * @FilePath: /examples/test_scripts/copy_test.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package testscripts

//比较copy的性能
//手动 > Jinzhu > Mohae。
//go test -bench=. -benchmem
// BenchmarkManual-8         415198              2896 ns/op            4608 B/op         28 allocs/op
// BenchmarkJinzhu-8          80043             15386 ns/op            1320 B/op         89 allocs/op
// BenchmarkMohae-8           75674             15606 ns/op            7920 B/op        204 allocs/op
import (
	"fmt"
	"testing"

	"github.com/jinzhu/copier"
	"github.com/mohae/deepcopy"
)

type BenchStruct struct {
	Name   string
	Age    int
	Scores []int
	Meta1  map[string]string
	Meta2  map[string]string
	Meta3  map[string]string
	Meta4  map[string]string
	Meta5  map[string]string
	Meta6  map[string]string
	Meta7  map[string]string
	Meta8  map[string]string
	Meta9  map[string]string
	Meta10 map[string]string
	Meta11 map[string]string
	Meta12 map[string]string
	Meta13 map[string]string
}

// 模拟数据
var src = &BenchStruct{
	Name:   "Test",
	Age:    100,
	Scores: []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 0},
	Meta1:  map[string]string{"key": "value", "foo": "bar"},
	Meta2:  map[string]string{"key": "value", "foo": "bar"},
	Meta3:  map[string]string{"key": "value", "foo": "bar"},
	Meta4:  map[string]string{"key": "value", "foo": "bar"},
	Meta5:  map[string]string{"key": "value", "foo": "bar"},
	Meta6:  map[string]string{"key": "value", "foo": "bar"},
	Meta7:  map[string]string{"key": "value", "foo": "bar"},
	Meta8:  map[string]string{"key": "value", "foo": "bar"},
	Meta9:  map[string]string{"key": "value", "foo": "bar"},
	Meta10: map[string]string{"key": "value", "foo": "bar"},
	Meta11: map[string]string{"key": "value", "foo": "bar"},
	Meta12: map[string]string{"key": "value", "foo": "bar"},
	Meta13: map[string]string{"key": "value", "foo": "bar"},
}

// 浅拷贝
func (s *BenchStruct) Clone() *BenchStruct {
	copy1 := *s
	fmt.Printf("[Clone内部] s 的地址(原件): %p\n", s)
	fmt.Printf("[Clone内部] copy1 的地址(副本): %p\n", &copy1)
	return &copy1
}

// 1. 手写深拷贝 (性能基准)
func (s *BenchStruct) ManualDeepCopy() *BenchStruct {
	dst := &BenchStruct{
		Name:   s.Name,
		Age:    s.Age,
		Meta1:  make(map[string]string, len(s.Meta1)),
		Meta2:  make(map[string]string, len(s.Meta2)),
		Meta3:  make(map[string]string, len(s.Meta3)),
		Meta4:  make(map[string]string, len(s.Meta4)),
		Meta5:  make(map[string]string, len(s.Meta5)),
		Meta6:  make(map[string]string, len(s.Meta6)),
		Meta7:  make(map[string]string, len(s.Meta7)),
		Meta8:  make(map[string]string, len(s.Meta8)),
		Meta9:  make(map[string]string, len(s.Meta9)),
		Meta10: make(map[string]string, len(s.Meta10)),
		Meta11: make(map[string]string, len(s.Meta11)),
		Meta12: make(map[string]string, len(s.Meta12)),
		Meta13: make(map[string]string, len(s.Meta13)),

		Scores: make([]int, len(s.Scores)),
	}
	copy(dst.Scores, s.Scores)
	for k, v := range s.Meta1 {
		dst.Meta1[k] = v
	}
	for k, v := range s.Meta2 {
		dst.Meta2[k] = v
	}
	for k, v := range s.Meta3 {
		dst.Meta3[k] = v
	}
	for k, v := range s.Meta4 {
		dst.Meta4[k] = v
	}
	for k, v := range s.Meta5 {
		dst.Meta5[k] = v
	}
	for k, v := range s.Meta6 {
		dst.Meta6[k] = v
	}
	for k, v := range s.Meta7 {
		dst.Meta7[k] = v
	}
	for k, v := range s.Meta8 {
		dst.Meta8[k] = v
	}
	for k, v := range s.Meta9 {
		dst.Meta9[k] = v
	}
	for k, v := range s.Meta10 {
		dst.Meta10[k] = v
	}
	for k, v := range s.Meta11 {
		dst.Meta11[k] = v
	}
	for k, v := range s.Meta12 {
		dst.Meta12[k] = v
	}
	for k, v := range s.Meta13 {
		dst.Meta13[k] = v
	}
	return dst
}

// Benchmark: 手写
func BenchmarkManual(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = src.ManualDeepCopy()
	}
}

// Benchmark: Jinzhu Copier
func BenchmarkJinzhu(b *testing.B) {
	for i := 0; i < b.N; i++ {
		dst := &BenchStruct{}
		_ = copier.Copy(dst, src)
	}
}

// Benchmark: Mohae Deepcopy
func BenchmarkMohae(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = deepcopy.Copy(src)
	}
}
func BenchmarkClone(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = src.Clone()
	}
}
