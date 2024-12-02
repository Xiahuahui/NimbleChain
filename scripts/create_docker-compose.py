# -*-coding:utf-8 -*-
import sys
import yaml
import math
from collections import OrderedDict
from json import loads

config_file = "./config.yaml"
first_file_name = "./templates/first_dcoker_compose.yaml"    #新的
old_first_file_name = "./templates/old_first_docker_compose.yaml"
last_file_name = "./templates/last_docker_compose.yaml"  #最后的网络
template_filename = "./templates/docker_compose_template.yaml"  # 读取模板文件
old_template_filename = "./templates/old_docker_compose_template.yaml"
head_template_filename = "./templates/head_docker_compose_template.yaml" #头部

filename = "./docker-compose.yml"  # 输出文件名
#主机的列表
hostsList = ["10.77.110.164"]

#生成新的docker-compose模版
def new_compose_template(old_file,new_file,node_hosts):
    new_template = ""
    with open(old_file, "r") as f:
        template = f.read()
        first = template.split("# extra_hosts:")[0]
        last = template.split("# extra_hosts:")[1]
        # 这几行代码是说要不要跨机的问题
        first = first + "# extra_hosts:"
        # for node_host in node_hosts:
        #     first = first + "\n" +str("      - ") + '"'+node_host + '"'
        new_template = first + last
        f.close()
    with open(new_file, "w") as output:
        output.write(new_template)
# 生成docker-compose.yaml文件
#input：node_cnt docker数、node_hosts：host的list、num_host:host的数量，tag：imgaes ，core_num 核数
#output：
def genYaml(node_cnt,node_hosts,num_host,tag,core_num,binary_tag,delay_flag,delay_time,jitter_flag,jitter_time,app,testtc_flag, cc_flag, cc_space,cc_delay, cc_dis, multiple,service):
    template = ""
    with open(template_filename, "r") as f:
        template = f.read()
    head = ""
    with open(head_template_filename, "r") as f4:
        head = f4.read()
        f4.close()
    node_host_list = {}
    for node_host in node_hosts:
        node = node_host.split(":")[0]
        host = node_host.split(":")[1]
        node_host_list[node] = host
        with open("./docker-compose/"+host+".yaml","w") as ff3:
            ff3.write(head)
            ff3.close()

    with open(filename, "w") as output:
        with open(first_file_name,"r") as f1:
            frist = f1.read()
            parameters = {
                "tag": tag,
                "LOG": "{LOG:-tendermint.log}",
                "cpus": core_num,
                "binary-tag": binary_tag,
                "delay_flag":delay_flag[0],
                "delay_time":delay_time[0],
                "jitter_flag":jitter_flag[0],
                "jitter_time":jitter_time[0],
                "testtc_flag":testtc_flag, 
                "cc_flag":cc_flag, 
                "cc_space":cc_space,
                "cc_delay":cc_delay,
                "cc_dis":cc_dis, 
                "multiple":multiple,
                "Application":app,
                "service":service
            }
            old = ""
            with open("./docker-compose/"+node_host_list["node" + str(0)]+".yaml","r") as ff1:
                old = ff1.read()
                ff1.close()
            with open("./docker-compose/"+node_host_list["node" + str(0)]+".yaml","w") as ff2:
                ff2.write(old + frist.format(**parameters))
                ff2.close()
        for i in range(node_cnt):
            parameters = {
                "tag": tag,
                "node": "node" + str(i+1),
                "id": i+1,
                # # "node_name": "node{}".format(i+1),
                "LOG": "{LOG:-tendermint.log}",
                "ip_adresss": "192.167.10.{}".format(3 + i),
                "port1": 26659 + i * 2,
                "port2": 26660 + i * 2,
                "cpus": core_num,
                "binary-tag": binary_tag,
                "delay_flag":delay_flag[i+1],
                "delay_time":delay_time[i+1],
                "jitter_flag":jitter_flag[i+1],
                "jitter_time":jitter_time[i+1],
                "testtc_flag":testtc_flag, 
                "cc_flag":cc_flag, 
                "cc_space":cc_space,
                "cc_delay":cc_delay,
                "cc_dis":cc_dis, 
                "multiple":multiple,
                "Application":app,
                "service":service
            }
            old = ""
            with open("./docker-compose/"+node_host_list["node" + str(i+1)]+".yaml","r") as ff1:
                old = ff1.read()
                ff1.close()
            with open("./docker-compose/"+node_host_list["node" + str(i+1)]+".yaml","w") as ff2:
                ff2.write(old + template.format(**parameters))
                ff2.close()
            output.write(template.format(**parameters))
            output.write("\n")
        with open(last_file_name,"r") as f2 :
            last = f2.read()
            output.write(last)

    testIndex = 0
    for host in hostsList :
        if testIndex >= int(num_host):
            continue
        old2 = ""
        with open("./docker-compose/"+host+".yaml","r") as ff1:
            old2 = ff1.read()
            ff1.close()
        with open("./docker-compose/"+host+".yaml","w") as ff2:
            last = ""
            with open(last_file_name,"r") as f5 :
                last = f5.read()
            ff2.write(old2 + last)
            ff2.close()
        testIndex = testIndex + 1

# 创建docker与主机间的映射
# input: node_cnt: docker的数量, num_host: host的数量
# output： ["node1:host1","node2:host2","node3:host3","node4:host1"]
def gen_node_host(node_cnt,num_host):
    String = "node"
    node_hosts = []
    for i in range(node_cnt):
        node_hosts.append(String+str(int(i))+str(":")+hostsList[(int(i) % int(num_host))])
    return node_hosts

#创建docker与端口间的映射
#input: node_cnt docker数
#output: ["node0:26656","node1:26659","node2:26661"]
def gen_node_port(node_cnt):
    node_port_list = []
    String = "node"
    node_port_list.append("node0:26656")
    for i in range(node_cnt-1):
        port = 26657 + i * 2
        node_port_list.append(String+str(i)+":"+str(port))
    return node_port_list

# 修改配置文件
def change_config_file(node_cnt,propose_time_list):
    String = "node"
    ip0 = "192.167.10.2"
    node_ip = {ip0:String + "0:26656"}

    for k in range(node_cnt-1):
        k = k + 1
        nodeId = String + str(k)
        ipId = int(ip0[-1]) + k
        ip = ip0[:-1]+str(ipId)
        port = 26657 + k * 2
        node_ip[ip] = nodeId + ":"+str(port)
    for i in range(node_cnt):
        file_dir = "./build/config_path/node"+str(i)+"/config/config.toml"
        #需要修改ip的时候放开即可
        # with open(file_dir, "r") as f:
        #     template = f.read()
        #     first = template.split("# Comma separated list of nodes to keep persistent connections to")[0]
        #     second = template.split("# Comma separated list of nodes to keep persistent connections to")[1]
        #     chnge_sring = second.split("# UPNP port forwarding")[0][1:][:-2]
        #     for key in node_ip:
        #         if len(chnge_sring.split(key+":26656")) > 1 :
        #             chnge_sring = chnge_sring.split(key+":26656")[0] + node_ip[key] + chnge_sring.split(key+":26656")[1]
        #     # print(chnge_sring)
        #     f.close()  
        # with open(file_dir,"w") as f2:
        #     f2.write(first + "# Comma separated list of nodes to keep persistent connections to" + "\n"+ chnge_sring+"\n"+"\n"+"# UPNP port forwarding" +second.split("# UPNP port forwarding")[1])
        #     f2.close()
        
        with open(file_dir, "r") as f3:
            template = f3.read()
            first = template.split('timeout_propose = "3s"')[0]
            second = template.split('timeout_propose = "3s"')[1]
            # print("first: ",second)
            f3.close()
        with open(file_dir,"w") as f4:
            f4.write(first+'timeout_propose = "' +str(propose_time_list[i]) +'s"'+second )
            f4.close()
            
        with open(file_dir, "r") as f5:
            template = f5.read()
            first = template.split('flush_throttle_timeout = "100ms"')[0]
            second = template.split('flush_throttle_timeout = "100ms"')[1]
            # print("first: ",second)
            f5.close()
        with open(file_dir,"w") as f6:
            f6.write(first+'flush_throttle_timeout = "' +str(10) +'ms"'+second )
            f6.close()
        
# def split_file(node_hosts):
#

def get_config():
    tag =  ""
    core_nmu = 0
    binary_tag = ""
    # open方法打开直接读出来
    with open(config_file, 'r') as f:
        config = f.read()
        d = yaml.load(config,Loader=yaml.FullLoader)  # 用load方法转字典
        tag = d["tag"]
        core_nmu = int(d["core_num"])
        binary_tag = d["binary_tag"]
        delay_flag =d["network_delay_flag_list"]
        delay_time=d["network_delay_list"]
        jitter_flag =d["jitter"]
        jitter_time=d["jitter_list"]
        app=d["app"]
        testtc_flag=d["testtc_flag"]
        cc_flag=d["cc_flag"]
        cc_space=d["cc_space"]
        cc_delay=d["cc_delay"]
        cc_dis=d["cc_dis"]
        multiple=d["multiple"]
        propose_time_list=d["propose_timeout_list"]
        service = d["service"]
    return tag,core_nmu,binary_tag,delay_flag,delay_time,jitter_flag,jitter_time,app,\
        testtc_flag, cc_flag, cc_space,cc_delay, cc_dis, multiple,propose_time_list,service

if __name__ == "__main__":
    argv = sys.argv[1:]
    # docker 的数量
    node_cnt = int(argv[0])
    # host的数量
    num_host = argv[1]           #几台主机
    print("the nums of docker：", node_cnt)
    print("the nums of hosts：",num_host)
    # 做一个node和host上的映射
    node_hosts = gen_node_host(node_cnt,num_host)

    # 生成新的docker-compose 模版
    new_compose_template(old_first_file_name,first_file_name,node_hosts)
    new_compose_template(old_template_filename,template_filename,node_hosts)

    # #修改系统的配置文件
    # gen_node_port(node_cnt)
    # change_config_file(node_cnt)

    #读取配置文件生成docker-compose.yaml
    tag,core_num,binary_tag,delay_flag,delay_time,jitter_flag,jitter_time,app, \
    testtc_flag, cc_flag, cc_space,cc_delay, cc_dis, multiple,propose_time_list,service = get_config()
    print("版本",tag,"核数",core_num)
    print("延迟标志位：",delay_flag)
    print("延迟时间",delay_time)
    print("抖动标志位：",jitter_flag)
    print("抖动时间",jitter_time)
    print("ABCI端的应用：",app)
    print("当前是否是测试节点",testtc_flag)
    print("当前是否是拥塞模式",cc_flag) 
    print("拥塞的长度",cc_space)
    print("拥塞时的延迟",cc_delay)  
    print("延迟的分布",cc_dis)
    print("拥塞的程度",multiple)
    print("提案超时列表",propose_time_list)
    print("python提供的服务",service)
    
    
    #修改系统的配置文件
    gen_node_port(node_cnt)
    change_config_file(node_cnt,propose_time_list)
    
    genYaml(node_cnt-1,node_hosts,num_host,tag,core_num,binary_tag,delay_flag,delay_time,jitter_flag,jitter_time,app,testtc_flag, cc_flag, cc_space,cc_delay, cc_dis, multiple,service)


