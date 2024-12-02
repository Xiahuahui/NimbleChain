# -*-coding:utf-8 -*-
import sys
import shutil
cc_file_name = "./templates/create-network-congestion.sh"    #新的

def create_cc_file(cc_cnt):
    for i in range(cc_cnt):
        destination_file = "./build/cc/create-network-congestion"+str(i)+".sh"
        shutil.copy(cc_file_name, destination_file)

if __name__ == "__main__":
    argv = sys.argv[1:]
    # docker 的数量
    cc_cnt = int(argv[0])
    #几个docker
    print("the nums of cc files",cc_cnt,type(cc_cnt))
    create_cc_file(cc_cnt)