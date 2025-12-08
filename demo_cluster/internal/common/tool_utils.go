/*
 * @Author: t 921865806@qq.com
 * @Date: 2025-11-26 17:03:16
 * @LastEditors: t 921865806@qq.com
 * @LastEditTime: 2025-12-03 17:41:39
 * @FilePath: /examples/demo_cluster/internal/common/tool_utils.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package common

import (
	"bytes"
	"compress/zlib"
	"encoding/base64"
	"fmt"
	"io"
	"runtime"
	"strconv"
	"strings"
)

// DecompressBase64Zlib 接收一个 base64 字符串，返回解压后的字节切片
func DecompressBase64Zlib(encodedStr string) ([]byte, error) {
	// 1. Base64 解码
	compressedData, err := base64.StdEncoding.DecodeString(encodedStr)
	if err != nil {
		return nil, fmt.Errorf("base64 decode error: %w", err)
	}

	// 2. Zlib 解压
	// 创建一个 bytes.Reader 读取二进制数据
	bReader := bytes.NewReader(compressedData)
	// 创建 zlib Reader
	zReader, err := zlib.NewReader(bReader)
	if err != nil {
		return nil, fmt.Errorf("zlib reader creation error: %w", err)
	}
	defer zReader.Close()

	// 3. 读取所有解压后的数据
	decompressedData, err := io.ReadAll(zReader)
	if err != nil {
		return nil, fmt.Errorf("read all error: %w", err)
	}

	return decompressedData, nil
}

// SplitToInts 将 "10,20,50" 格式的字符串转换为 []int 切片
// s: 源字符串
// sep: 分隔符 (例如 ",")
func SplitNumber(str, sep string) ([]int, error) {
	if str == "" {
		return []int{}, nil
	}
	// 1. 分割字符串
	parts := strings.Split(str, sep)

	result := make([]int, 0, len(parts))
	for _, part := range parts {
		// 2. 去除首尾空格 (防止配置写成 "10, 20" 导致转换失败)
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			continue // 跳过空项，比如 "10,,20"
		}
		val, err := strconv.Atoi(trimmed)
		if err != nil {
			return nil, err // 如果配置有误（比如写了 "10,abc"），返回错误
		}

		result = append(result, val)
	}
	return result, nil
}
func PrintMemUsage() {
	var m runtime.MemStats
	// 读取当前的内存状态到 m 中
	runtime.ReadMemStats(&m)

	// 打印关键指标
	// Alloc:      当前堆上正在使用的对象内存 (最关键的指标)
	// TotalAlloc: 程序启动以来分配的总内存 (累计值，可以看出分配速率)
	// Sys:        从操作系统申请的总内存 (包含堆、栈、全局变量等)
	// NumGC:      GC 发生的次数
	fmt.Printf("Alloc = %v kb", bToMb(m.Alloc))
	fmt.Printf("\tTotalAlloc = %v kb", bToMb(m.TotalAlloc))
	fmt.Printf("\tSys = %v kb", bToMb(m.Sys))
	fmt.Printf("\tNumGC = %v\n", m.NumGC)

	fmt.Printf("----------- Object Stats -----------\n")
	// 1. 累计创建了多少个对象 (用来衡量分配速率)
	fmt.Printf("Total Created (Mallocs): %d\n", m.Mallocs)

	// 2. 累计清理了多少个对象 (这就是已经被回收的垃圾总数)
	fmt.Printf("Total Freed   (Frees)  : %d\n", m.Frees)

	// 3. 当前内存里还剩多少个对象 (存活 + 待清理)
	fmt.Printf("Current Live  (Objects): %d\n", m.HeapObjects)

	// 4. 计算垃圾回收率 (仅作参考)
	if m.Mallocs > 0 {
		fmt.Printf("GC Rate: %.2f%%\n", float64(m.Frees)/float64(m.Mallocs)*100)
	}
	fmt.Printf("------------------------------------\n")
}
func bToMb(b uint64) uint64 {
	return b / 1024
}
