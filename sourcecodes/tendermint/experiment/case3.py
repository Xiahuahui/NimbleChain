#构造一个例子说明adjust的方法，也不能应对
import random
import statistics
import time
import csv
import logging
import numpy as np
import copy
import matplotlib.pyplot as plt
from sklearn.ensemble import RandomForestRegressor   #导入随机森林的包

best_block_index = 0   #TODO 这个是什么意思
best_bct_list = []
head = 0

default_alpha = 0.1
default_beta = 0.1
default_K = 4

leader = 4
def ema(sample, RCPT,alpha):  # rtt是根据五个变量预测下一个时间的值
    RCPT = (1 - alpha) * RCPT + alpha * float(sample)  # 低通滤波计算下一个RCPT
    timeout = RCPT
    return RCPT, timeout

def adjust(fixed_data,leader_partion,cold_start_phase,init_timeout,index):
    byz_bct_list = []
    timeout = init_timeout
    byz_timeout = init_timeout

    tps = []
    vc_num = 0
    fixed_timeout_list = []
    byz_fixed_timeout_list = []

    fixed_timeout_list.append(timeout)
    byz_fixed_timeout_list.append(byz_timeout)

    RCPT = 1000.0
    RCPT2 = 1000.0
    alpha = 0.5

    fixed_index = 0
    fixed_bct = []
    i = 0
    for bct in fixed_data:
        i = i + 1
        byz_bct = bct
        if fixed_index % leader_partion == index:
            if cold_start_phase > i:
                byz_bct = bct+2000.0
            else:
                byz_bct = bct + byz_timeout - timeout

        # pdt = byz_bct
        # test_bct = byz_bct
        # test_to = byz_fixed_timeout_list[-1]
        # if test_bct > test_to:
        #     while test_bct > test_to:
        #         vc_num = vc_num + 1
        #         pdt = pdt + test_to
        #         test_to = test_to *2
        # tps.append(10000/pdt*300)
        byz_bct_list.append(byz_bct)
        fixed_bct.append(bct)
        if bct > byz_bct :
            print("bct:",bct,"byz_bct",byz_bct)
            time.sleep(1)
        RCPT,timeout = ema(bct,RCPT,alpha)
        RCPT2,byz_timeout = ema(byz_bct,RCPT2,alpha)
        # if cold_start_phase > i:
        #     timeout = init_timeout
        #     byz_timeout = init_timeout
        fixed_timeout_list.append(timeout)
        byz_fixed_timeout_list.append(byz_timeout)
        fixed_index = fixed_index + 1
    fixed_timeout_list = fixed_timeout_list[:-1]
    byz_fixed_timeout_list=byz_fixed_timeout_list[:-1]
    return fixed_timeout_list,tps,vc_num,byz_bct_list,byz_fixed_timeout_list,byz_bct_list

def geneateBCT():
    # 设置正态分布的参数
    mean = 970       # 均值
    std_dev = 30  # 标准差
    mean2 = 970      # 均值
    std_dev2 = 30   # 标准差
    # # 生成正态分布数据
    bct_data = []
    for k in range(12):
        data = np.random.normal(mean, std_dev, 25)
        data2 = np.random.normal(mean2, std_dev2, 25)
        for i in range(len(data)):
            bct_data.append(data[i])
        for i in range(len(data2)):
            bct_data.append(data2[i])

    return bct_data
def read_col_data(filename,index):
    data_list = []
    with open(filename, 'r') as file:
        #创建CSV读取器
        reader = csv.reader(file)
        # 跳过第一行
        next(reader)
        # i =0
        for row in reader:
            data_list.append(100*float(row[index]))
    return data_list

if __name__ == '__main__':

    with open('data.csv', 'w', newline='') as file:
        file.truncate(0)
    LOG_FORMAT = '%(asctime)s - %(levelname)s - %(message)s'
    logging.basicConfig(level=logging.DEBUG, format=LOG_FORMAT)
    """ Set the same random seed for each peer """
    np.random.seed(0)
    # 这个多久更新一次alpha、beta
    '''
    ***************************模拟生成数据***************************
    '''
    data = read_col_data('../analyze_icde/output/column_0.csv',2)

    fixed_data = data[:150]
    logging.info("the bct_list'lenght is: %s,the bct_list is: %s",len(fixed_data),fixed_data)
    tps_list = []
    tps_list2 = []
    x_list = []
    # for to in range(200,3200,200):
    #     x_list.append(to)
        # timeout3,tps1,vc_num1,byz_bct_list = adjust(fixed_data,88888,"Fixed",to,8888)
    timeout_list,tps2,vc_num2,byz_bct_list2,byz_timeout_list,byz_fixed_data = adjust(fixed_data,4,30,3000.0,0)
    logging.info("timeout_list: %s",timeout_list)
    logging.info("byz_timeout_list: %s",byz_timeout_list)
    logging.info("byz_fixed_data: %s",byz_fixed_data)
    assert (len(timeout_list) == len(byz_timeout_list))
    i = 0
    i_list = []
    for kk in range(len(timeout_list)):
        logging.info("kk: =%s,timeout:= %s, byz_timeout:= %s",kk,timeout_list[kk],byz_timeout_list[kk])
        if timeout_list[kk] > byz_timeout_list[kk]:

            logging.info("timeout:= %s, byz_timeout:= %s",timeout_list[kk],byz_timeout_list[kk])
            i_list.append(kk)
            i = i +1
    logging.info("i:= %s,i_list = %s",i,i_list)
    tps_list.append(10000/(sum(byz_bct_list2)/len(byz_bct_list2))*300)
    # tps_list2.append(10000/(sum(byz_bct_list2)/len(byz_bct_list2))*300)
    # print("byz_bct_list: ",byz_bct_list)
    # print("byz_bct_list: ",byz_bct_list2)
    fig, ax = plt.subplots()

    # plt.spines['top'].set_linewidth(2)    # 上边框加宽
    # plt.spines['right'].set_linewidth(2)  # 右边框加宽RUC&info500
    # plt.spines['left'].set_linewidth(2)   # 左边框加宽
    # plt.spines['bottom'].set_linewidth(2)  # 下边框加宽

    # ax1.plot(x_list,tps_list,label='Normal',color='r',marker='^',markersize=10,linewidth=5)
    # ax.plot(fixed_data,label = 'PDT',color='green',markersize=15,linewidth=5)
    ax.plot(byz_fixed_data,label = 'PDT',markersize=15,linewidth=5)
    # ax.plot(timeout_list,label = 'EMA-TO',color='blue',markersize=15,linewidth=5)
    ax.plot(byz_timeout_list,label = 'byz_EMA-TO',color='red',markersize=15,linewidth=5)
    logging.info("timeout_list: %s",timeout_list)
    logging.info("byz_timeout_list: %s",byz_timeout_list)
    ax.set_ylabel('TPS',fontsize=30)
    # ax1.set_ylim(900, 1900)
    # ax1.set_yticks(np.arange(1000, 1900, 200))  # 每隔 0.3 设置一个刻度

    ax.tick_params(axis='y',labelsize=30,width=2)
    ax.tick_params(axis='x',labelsize=30,width=2)
    # for label in ax1.get_xticklabels():  # 对于 x 轴的刻度标签
    #     label.set_fontweight('bold')     # 设置字体加粗
    # for label in ax1.get_yticklabels():  # 对于 y 轴的刻度标签
    #     label.set_fontweight('bold')     # 设置字体加粗
    ax.set_xlabel('block index',fontsize=30)
    ax.set_xticks(np.arange(0, 180, 30))
    ax.spines['top'].set_linewidth(2)    # 上边框加宽
    ax.spines['right'].set_linewidth(2)  # 右边框加宽
    ax.spines['left'].set_linewidth(2)   # 左边框加宽
    ax.spines['bottom'].set_linewidth(2)  # 下边框加宽
    ax.legend(prop={'size': 20, 'weight': 'bold'},loc='upper left')




    plt.show()
