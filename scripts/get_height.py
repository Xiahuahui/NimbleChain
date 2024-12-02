'''
    这个脚本是用来测试tendermint是否存在抖动的问题，大致思路是并发request到每个节点上，拿取该节点刚刚提交完的高度
    其中需要改动的参数是node_cnt真实的物理机器数，docker_cnt启动的docker节点数，hosts_lists真实的物理机ip列表
'''
import csv
import time
import sys
import requests
import threading

hosts_list = ["10.77.110.164","10.77.110.165"]
def send_requests(url,result,i):
    response = requests.get(url).json()
    height = response["result"]["sync_info"]["latest_block_height"]
    # time = response["result"]["sync_info"]["latest_block_time"]
    result[i] = height

def getBlockHeight(urls):
    threads = []
    result = [0]*len(urls)
    i = 0
    for url in urls:
        thread = threading.Thread(target=send_requests,args=(url,result,i))
        thread.start()
        threads.append(thread)
        i = i+1
    for thread in threads:
        thread.join()
    return result

def generate_url(node_cnt,docker_cnt):
    urls = []
    protocol = "http://"
    route = "/status"
    url = protocol + hosts_list[0] +":26657"+ route
    urls.append(url)
    for k in range(docker_cnt - 1):
        k = k+1
        host = hosts_list[k%node_cnt]
        port = 26660 + (k-1) * 2
        url = protocol+host+":"+str(port)+route
        urls.append(url)
        k = k + 1
    return urls

def test(node_cnt,docker_cnt,filepath):
    urls = generate_url(node_cnt,docker_cnt)
    print("urls: ", urls)
    data = []
    i = 0
    for url in urls:
        data.append("node" + str(i))
        i = i + 1
    with open(filepath, 'a') as file:
        writer = csv.writer(file)
        writer.writerow(data)
    while True:
        result = getBlockHeight(urls)
        print("result: ", result)
        with open(filepath, 'a') as file:
            writer = csv.writer(file)
            writer.writerow(result)
        time.sleep(0.5)


if __name__ == '__main__':
    argv = sys.argv[1:]
    # host的数量
    node_host = int(argv[0])
    # docker 的数量
    docker_cnt = int(argv[1])
    result_file = (argv[2])
    print("node_host: ",node_host,"docker_cnt: ",node_host,"result_file: ",result_file)
    test(node_host,docker_cnt,result_file)
