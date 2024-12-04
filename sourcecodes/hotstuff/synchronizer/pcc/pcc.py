# 定义PCC服务
# phase = ["starting_state","decision_making_state","rate_adjusting_state"]
import logging
import os
import numpy as np
import csv
# numpy-1.26.4
# global_head = 0

file_path = 'K.csv'

# id = os.getenv("ID")
# file_path='../config_path/node'+id+'/K.csv'

logging.basicConfig(level=logging.INFO, format='%(asctime)s - %(levelname)s - %(message)s')
class Pcc:
    # 初始化PCC算法需要给出MI的大小，以及初始化的epoch_size,epoc_size是一个需要学习的量
    def __init__(self,MI,epoch_size,init_timeout):
        logging.info("enter Pcc init")
        self.MI = MI  #当前统计的MI的大小，初始化
        self.MiIndex = 0 # 当前Mi的索引是多少

        # phase = ["starting_state","decision_making_state","rate_adjusting_state"] 学习的3个阶段.
        self.phase = "starting_state"
        self.epoch_size = epoch_size # 当前epoch_size的大小，用来初始化epoch_size=2
        self.utility_value = 0.0  #当前MI对应到效用值函数的值
        self.utility_value_list = []  #所有Mi的效用值集合
        self.init_timeout = init_timeout   #TODO 这个是下一个Mi开始时的timeout（或者是变化epoch_size）
        self.step = 1           #学习阶段的目标
        self.dir = 0            #当前epoch_size的变化方向  当前的方向
        self.mi_of_number = 1   #当前需要连续做几个MI在继续
        self.epoch_size_list = [self.epoch_size] #用来告诉系统每个MI的epoch_size的大小，比如在学习阶段是长度4的epoch_size列表。

        #下面是加入一个辅助函数记录一下历史数据
        headstring = ["height", "mi", "k", "tps"]
        with open(file_path, 'w', newline='') as file:
            writer = csv.writer(file)
            writer.writerow(headstring)

    # 起始阶段
    '''
     starting_state pcc的起始阶段
     以2倍的速率的快速达到较优的点附近，当出现效用值下降时，进入决策阶段。
    '''
    def starting_state(self,data):
        utility_value = self.predict(self.epoch_size_list[0],data[0]) #预测当前MI的对应的效用值
        self.utility_value_list.append(utility_value)
        if utility_value > self.utility_value:
            self.utility_value = utility_value
            self.epoch_size = 2*self.epoch_size
            self.mi_of_number = 1
            self.epoch_size_list=[self.epoch_size]
        else:
            self.phase = "decision_making_state"
            logging.info("enter %s phase",self.phase)
            self.epoch_size = int(self.epoch_size/2)
            self.mi_of_number = 4
            self.epoch_size_list=[self.epoch_size+self.step,self.epoch_size-self.step,self.epoch_size-self.step,self.epoch_size+self.step]

    # 学习阶段 这的data应该和pair对应的
    def decision_making_state(self,data):

        # 下面是RCTs
        reward1 = self.predict(self.epoch_size_list[0],data[0])
        self.utility_value_list.append(reward1)
        reward2 = self.predict(self.epoch_size_list[1],data[1])
        self.utility_value_list.append(reward2)
        reward3 = self.predict(self.epoch_size_list[2],data[2])
        self.utility_value_list.append(reward3)
        reward4 = self.predict(self.epoch_size_list[3],data[3])
        self.utility_value_list.append(reward4)

        if reward1 > reward2 and reward4 > reward3:
            self.step = 1
            self.dir = 1
            self.mi_of_number = 1
            self.epoch_size = self.epoch_size + self.dir
            self.epoch_size_list = [self.epoch_size]
            self.utility_value = 0.0
            self.phase = "rate_adjusting_state"
            logging.info("enter %s phase",self.phase)
            return
        if reward1 < reward2 and reward4 < reward3:
            self.step = 1
            self.dir = -1
            self.mi_of_number = 1
            self.epoch_size = self.epoch_size + self.dir
            self.epoch_size_list = [self.epoch_size]
            self.utility_value = 0.0
            self.phase = "rate_adjusting_state"
            logging.info("enter %s phase",self.phase)
            return
        self.dir = 0
        self.step = self.step*2
        self.mi_of_number = 4
        self.epoch_size_list = [self.epoch_size+self.step,self.epoch_size-self.step,self.epoch_size-self.step,self.epoch_size+self.step]

    # 策略适应阶段
    def rate_adjusting_state(self,data):
        utility_value = self.predict(self.epoch_size_list[0],data[0]) #预测当前MI的对应的效用值
        self.utility_value_list.append(utility_value)
        if utility_value > self.utility_value:
            self.utility_value = utility_value
            self.epoch_size = self.epoch_size + self.dir
            self.mi_of_number = 1
            self.epoch_size_list = [self.epoch_size]
        else:
            self.phase = "decision_making_state"
            logging.info("enter %s phase",self.phase)
            self.epoch_size = self.epoch_size - self.dir
            self.mi_of_number = 4
            self.epoch_size_list = [self.epoch_size+self.step,self.epoch_size-self.step,self.epoch_size-self.step,self.epoch_size+self.step]

    # 执行PCC,执行完run函数需要通知系统需要连续做几个MI,并且每个MI的epoch_size的大小是多少
    def run(self,data):
        # 提前保存一下MI的学习信息
        dlen = len(data)
        klen = len(self.epoch_size_list)
        if dlen != klen:
            logging.debug("error")
        for index in range(len(data)):
            self.savaKdata(self.MiIndex,self.epoch_size_list[index],data[index])
            self.MiIndex = self.MiIndex + 1
        if self.phase == "starting_state":
            if len(data) != 1:
                print("error stating_state phase data length != 1")
            self.starting_state(data)

        elif self.phase == "decision_making_state":
            if len(data) !=4:
                print("error decision_making_state phase data length != 4")
            self.decision_making_state(data)

        elif self.phase == "rate_adjusting_state":
            if len(data) != 1:
                print("error rate_adjusting_state phase data length != 1")
            self.rate_adjusting_state(data)
        # 计算下一个Mi第一个timeout
        self.compute_first_to_next_mi(data[len(data)-1])
        logging.info("utility_value_list: %s",self.utility_value_list)

    #这个函数其实没有真正的被用到
    def compute_first_to_next_mi(self,data):
        self.init_timeout = int(3*1000)

    #根据高度存储大MI对应的tps,其实是要存储两个数据 一个是需要学的数据epoch（不一定是epoch）
    def savaKdata(self,mi_index,kvalue,data):
        height_list = data.height
        save_data_list = []
        for k in range(len(height_list)):
            sava_data = []
            sava_data.append(height_list[k])
            sava_data.append(mi_index)
            sava_data.append(kvalue)
            sava_data.append(data.tps)
            save_data_list.append(sava_data)
        with open(file_path, 'a', newline='') as file:
            writer = csv.writer(file)
            for save_data in save_data_list:
                writer.writerow(save_data)

    #predict 用来计算每个MI对应的效用值函数 data是一个MI中的数据。
    #epoch_size 当前MI对应的epoch_size的大小
    #init_timeout 初始的timeout的值
    def predict(self,epoch_size,data):
        global file_path
        # utility_value = sum(view_change_cost_list) + int(sum(threat_cost_list)/3)
        utility_value = data.tps
        save_data_list = []
        logging.info("the epoch_size of mi: %s,the tps of the mi: %s",epoch_size,data.tps)
        return utility_value

class MIdata:
    def __init__(self,height_list,is_leader,proposer_list,is_byz_node,actaul_timeout_list,predict_timeout_list,byz_timeout_list,round_list,cost_list,latency_list,tx_num_list,tps):
        self.height = height_list
        self.is_leader = is_leader
        self.proposer_list = proposer_list
        self.is_byz_node = is_byz_node
        self.actaul_timeout_list =  actaul_timeout_list
        self.predict_timeout_list = predict_timeout_list
        self.byz_timeout_list = byz_timeout_list
        self.round = round_list
        self.cost_list = cost_list
        self.latency_list = latency_list
        self.tx_num_list = tx_num_list
        self.tps = tps

class Cost:
    def __init__(self,view_change_cost,threat_cost,discretionary,flag):
        self.view_change_cost = view_change_cost
        self.threat_cost = threat_cost
        self.discretionary = discretionary
        self.flag = flag

if __name__ == '__main__':
    # 给出响应的测试程序
    pcc = Pcc(5,2,3000)
    sample_data_list = [[[2]],[[4]],[[8]],[[16]],[[13]],[[7],[3],[5],[9]],[[13]],[[14]],[[16]],[[15]],[[1],[2],[4],[5]]]
    height = 1  #高度是1
    for sample_data in sample_data_list:
        test_data_list = []
        for j in range(len(sample_data)):  #一个sample_data中可能包含多个Mi
            height_list = []
            is_leader = []
            proposer_list = []
            is_byz_node = []
            actaul_timeout_list = []
            predict_timeout_list = []
            byz_timeout_list = []
            latency_list = []
            tx_num_list = []
            round_list = []
            cost_list = []
            for k in range(pcc.MI):
                if k == pcc.MI -1:
                    height_list.append(height)
                    is_leader.append(False)
                    proposer_list.append(height%16)
                    is_byz_node.append(False)
                    actaul_timeout_list.append(2)
                    predict_timeout_list.append(2)
                    byz_timeout_list.append(1)
                    round_list.append(3)
                    latency_list.append(2)
                    tx_num_list.append(3)
                    cost = Cost(sample_data[j][0],0,5,True)
                    cost_list.append(cost)
                else:
                    height_list.append(height)
                    is_leader.append(False)
                    proposer_list.append(height%16)
                    is_byz_node.append(False)
                    actaul_timeout_list.append(0)
                    predict_timeout_list.append(0)
                    byz_timeout_list.append(0)
                    round_list.append(0)
                    latency_list.append(2)
                    tx_num_list.append(3)
                    cost = Cost(0,0,0,False)
                    cost_list.append(cost)
                height = height + 1
            test_data = MIdata(height_list,is_leader,proposer_list,is_byz_node,actaul_timeout_list,predict_timeout_list,byz_timeout_list,round_list,cost_list,latency_list,tx_num_list,sample_data[j][0])
            test_data_list.append(test_data)
        pcc.run(test_data_list)
        # logging.info("next_init_timeout: %s",pcc.init_timeout)





