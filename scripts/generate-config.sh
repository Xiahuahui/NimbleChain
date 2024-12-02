#!/usr/bin/env bash
# 每个机器上的docker数  机器数
NODE_CNT=$1
HOST_CNT=$2

# 生成配置文件
rm -rf build/config_path/node*
echo "1. generate config for tendermint"

./build/tendermint testnet --config ./build/config-template.toml --v $NODE_CNT --o ./build/config_path --starting-ip-address 192.167.10.2
