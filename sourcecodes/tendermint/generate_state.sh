#!/bin/bash

# 运行 ping 命令，限制时长为 3 秒
ping_results=$(ping -i 10 -c 10 www.baidu.com)

# 提取 RTT 统计信息
rtt_info=$(echo "$ping_results" | grep 'round-trip')

# 使用 awk 提取 min, avg, max, stddev
min=$(echo "$rtt_info" | awk -F'/' '{print $4}')
avg=$(echo "$rtt_info" | awk -F'/' '{print $5}')
max=$(echo "$rtt_info" | awk -F'/' '{print $6}')
stddev=$(echo "$rtt_info" | awk -F'/' '{print $7}' | awk '{print $1}')

# 输出结果
echo "Min: $min ms"
echo "Avg: $avg ms"
echo "Max: $max ms"
echo "Stddev: $stddev ms"
