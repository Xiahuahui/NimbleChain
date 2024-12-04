package tendermint

import (
	"bytes"
	"fmt"
	"math"
	"os/exec"
	"regexp"
	"strconv"
	"testing"
	"time"
)

func ping(host string, duration time.Duration) (string, error) {
	var cmd *exec.Cmd
	cmd = exec.Command("ping", "-i", "0.5", host)

	var out bytes.Buffer
	cmd.Stdout = &out

	// 启动 ping 命令
	if err := cmd.Start(); err != nil {
		return "", err
	}

	// 等待指定的时间
	time.Sleep(duration)

	// 结束命令
	if err := cmd.Process.Kill(); err != nil {
		return "", err
	}

	return out.String(), nil
}

func calculateStats(times []float64) (min, max, avg, std float64) {
	if len(times) == 0 {
		return 0, 0, 0, 0
	}

	min = times[0]
	max = times[0]
	sum := 0.0

	// 计算 min, max 和 sum
	for _, value := range times {
		if value < min {
			min = value
		}
		if value > max {
			max = value
		}
		sum += value
	}

	avg = sum / float64(len(times))

	// 计算标准差
	var variance float64
	for _, value := range times {
		variance += (value - avg) * (value - avg)
	}
	variance /= float64(len(times))
	std = math.Sqrt(variance)

	return min, max, avg, std
}

func TestPing(t *testing.T) {
	host := "www.baidu.com"
	duration := 3 * time.Second // 设置持续时间为 3 秒

	result, err := ping(host, duration)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Println(result)

	// 使用正则表达式提取响应时间
	re := regexp.MustCompile(`time=([\d.]+) ms`)
	fmt.Println(re)
	matches := re.FindAllStringSubmatch(result, -1)
	fmt.Println(matches)
	if len(matches) == 0 {
		fmt.Println("No matches found. Please check the output format.")
		return
	}
	var times []float64
	for _, match := range matches {
		if len(match) > 1 {
			if t, err := strconv.ParseFloat(match[1], 64); err == nil {
				times = append(times, t)
			}
		}
	}

	// 计算统计值
	min, max, avg, std := calculateStats(times)

	fmt.Printf("Min: %f ms\n", min)
	fmt.Printf("Max: %f ms\n", max)
	fmt.Printf("Avg: %f ms\n", avg)
	fmt.Printf("Std: %f ms\n", std)
}
