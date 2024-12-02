#!/usr/bin/env bash
# 每个机器上的docker数  机器数
CC_CNT=$1


#删除dist中的文件
rm -rf build/cc/*
echo "2. generate cc file"
python3 scripts/create_cc.py $CC_CNT 