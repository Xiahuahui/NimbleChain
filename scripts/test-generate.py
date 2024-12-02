from string import Template
import sys
# command = "./tm-bench/tm-load-test -c 1 -T ${time} -r ${rate} -s 250 --broadcast-tx-method async --endpoints ${endpoints}"
command = "/home/xhh/tendemrint_env/build/tm-load-test -c 1 -T ${time} -r ${rate} -s 250 --broadcast-tx-method async --endpoints ${endpoints}"
hosts_list = ["localhost","10.77.110.164"]

def generate_command(node_cnt,docker_cnt,time,rate):
    template = Template(command)

    command_list = []
    protocol = "ws://"
    route = "/websocket"
    url = protocol + hosts_list[0] +":26657"+ route
    command_list.append(url)
    for k in range(docker_cnt - 1):
        k = k+1
        host = hosts_list[k%node_cnt]
        port = 26660 + (k-1) * 2
        url = protocol+host+":"+str(port)+route
        command_list.append(url)
        k = k + 1
    endpoint = ""
    for com in command_list:
        endpoint = endpoint + com + ","
    endpoint = endpoint[:-1]
    d = {"time": time, "rate": rate, "endpoints":endpoint}
    command_new = template.substitute(d)
    return command_new

if __name__ == '__main__':
    argv = sys.argv[1:]
    # host的数量
    node_host = int(argv[0])
    # docker 的数量
    docker_cnt = int(argv[1])
    # 发送时间
    time = int(argv[2])
    # 发送速率
    rate = int(argv[3])
    command = generate_command(node_host,docker_cnt,time,rate)
    with open("./build/start-test.sh",mode='w',encoding='utf-8') as f:
        f.write(command)