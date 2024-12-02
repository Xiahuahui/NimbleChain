#!/usr/bin/env bash
# 每个机器上的docker数  机器数
NODE_CNT=$1
HOST_CNT=$2

#删除dist中的文件
rm -rf docker-compose/*
echo "2. generate docker-compose.yml"
echo "3. update config file"
python3 scripts/create_docker-compose.py $NODE_CNT $HOST_CNT
