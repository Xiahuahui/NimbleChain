#!/bin/bash

# ..................
param1=$1
param2=$2
param3=$3
param4=$4

echo "the cct size is $param1"
echo "the cc delay is $param2"
echo "the normal delay is $param3"
echo "the param4 is $param4"

filename="/config_path/ping-result/sequence.csv"  # CSV.........
read_csv() {
    local delay_array=()
    while IFS= read -r line; do
        echo "$line" | tr -d '\r'
    done < "$filename"
}
delay_array=($(read_csv))
#tc qdisc add dev eth0 root netem delay 25ms
tc qdisc add dev eth0 root handle 1: htb default 11

tc class add dev eth0 parent 1: classid 1:1 htb rate 100mbit
tc qdisc add dev eth0 parent 1:1 handle 10: netem delay 50ms
tc filter add dev eth0 protocol ip parent 1: prio 1 u32 match ip protocol 6 0xff flowid 1:1

tc class add dev eth0 parent 1: classid 1:2 htb rate 100mbit
tc qdisc add dev eth0 parent 1:2 handle 20: netem delay 50ms
tc filter add dev eth0 protocol ip parent 1: prio 2 u32 match ip protocol 1 0xff flowid 1:2
index=0
prev_value=""
# ..................Height..............................
while true; do
        value="${delay_array[index]}"
        if [ "$value" != "$prev_value" ]; then
            echo "$value"
            value=$((value * 2))
            #tc qdisc change dev eth0 root netem delay "${value}"ms
            tc qdisc change dev eth0 parent 1:1 handle 10: netem delay "${value}"ms
            tc qdisc change dev eth0 parent 1:2 handle 20: netem delay "${value}"ms
            tc qdisc show dev eth0
            prev_value="$value"
        fi  
        (( index++ ))
        index=$(( index % ${#delay_array[@]} ))
        sleep 2s # ..................
done
