#!/usr/bin/env bash
set -e

#将节点的信息重定向到log/node_info.txt
docker  ps  | grep 'node' | awk -F"node" '{print $2}' > /home/xhh/tendemrint_env/build/log/node_info.txt


# rm -rf result
# mkdir result
#获取高度的其实时间
# cat /home/xhh/tendemrint_env/build/log/node_info.txt | while read NODE
# do
# #    echo "start"
#     echo node$NODE
# #    echo "end"
#     docker logs node$NODE > /home/xhh/tendemrint_env/build/log/node$NODE.log

# done
dir=$1

mkdir /home/xhh/tendemrint_env/build/result/$dir

cat /home/xhh/tendemrint_env/build/log/node_info.txt | while read NODE
do
#    echo "start"
     echo node$node
#    echo "end"
    
    mkdir /home/xhh/tendemrint_env/build/result/$dir/node$NODE
    cp -r /home/xhh/tendemrint_env/build/config_path/node$NODE/cost_time.csv  /home/xhh/tendemrint_env/build/result/$dir/node$NODE/
    cp -r /home/xhh/tendemrint_env/build/config_path/node$NODE/commit_round.csv  /home/xhh/tendemrint_env/build/result/$dir/node$NODE/
    cp -r /home/xhh/tendemrint_env/build/config_path/node$NODE/statistics.csv  /home/xhh/tendemrint_env/build/result/$dir/node$NODE/
    cp -r /home/xhh/tendemrint_env/build/config_path/node$NODE/latency_time.csv  /home/xhh/tendemrint_env/build/result/$dir/node$NODE/
    cp -r /home/xhh/tendemrint_env/build/config_path/node$NODE/propose_time.csv  /home/xhh/tendemrint_env/build/result/$dir/node$NODE/
    cp -r /home/xhh/tendemrint_env/build/config_path/node$NODE/tendermint.log  /home/xhh/tendemrint_env/build/result/$dir/node$NODE/
done