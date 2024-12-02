#!/bin/bash

filename="./sequence.csv"
data_list=()

while IFS= read -r line; do
    data_list+=("$line")
done < "$filename"

# 打印列表中的数据
for data in "${data_list[@]}"; do
    echo "$data"
done

