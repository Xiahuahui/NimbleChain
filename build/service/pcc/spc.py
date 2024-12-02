import logging
import numpy as np
import csv
import os
# #
# file_path = 'statistics.csv'
# para_file_path = 'para.csv'

# #
id = os.getenv("ID")
file_path='../config_path/node'+id+'/statistics.csv'
para_file_path='../config_path/node'+id+'/para.csv'

logging.basicConfig(level=logging.INFO, format='%(asctime)s - %(levelname)s - %(message)s')
# 专家经验值的验证
'''
    - 模式1:急剧上升、急剧下降对应(偏差大)b+；
    - 模式2:缓慢上升、缓慢下降对应（和近期的变化更相关）a+；
    - 模式3:抖动平缓(偏差小)b-；       
    - 模式4:抖动剧烈（需要更多经验值）a-。
'''
# code的取值分别 {b+:0,a+:1,b-:2,a-:3}
# 下面的前两个参数表示BCT的变化是（急升、急降）b+ 还是缓升缓降a+ 还是抖动的趋势，最后一个参数表示的抖动剧烈的程度是抖动平缓b-，还是抖动剧烈a-
threshold1 = 0.05605725463376267
threshold2 = 0.9
threshold3 = 1.3

mode = "fixed"


class Spc:
    def __init__(self, mi):
        logging.info("init SPC service Class, the Mi of SPC is: %s", mi)
        self.mi = mi
        self.alpha = 0.125
        self.beta = 0.25
        self.theta_1 = -1   #下面这两个是两个辅助量
        self.theta_2 = -1
        self.last_avg_bct_list = []
        self.smi_number = 0  # 当前经过的smi的个数是0个
        self.code = 2  # 这里的code表示alpha和beta具体怎样变化。

        #下面是加入一个辅助函数记录一下历史数据
        headstring = ["height", "is_leader","proposer", "is_byz", "round", "bct", "timeout", "byz_timeout", "vc-cost", "threat",
                      "discretionary", "cost", "crash", "reward", "latency", "tx_num", "tps"]
        with open(file_path, 'w', newline='') as file:
            writer = csv.writer(file)
            writer.writerow(headstring)
        #下面是记录一下参数的变化。
        paraheadstring = ["height", "smi","bct","flag","theta_1","theta_2","code", "alpha", "beta"]
        with open(para_file_path, 'w', newline='') as file2:
            writer = csv.writer(file2)
            writer.writerow(paraheadstring)

    # 根据Bct时间的对比
    def predictModel(self, smidata):
        if len(smidata.actaul_timeout_list) != self.mi:
            logging.info("height: %s",smidata.height)
            logging.info("error of the value of mi not equal the length of actual_timeout_list")
        # 这其实需要增加一个对自己是Leader节点时的数据的处理：
        bct_list = []
        for  k in range (len(smidata.actaul_timeout_list)):
            print(smidata.is_leader[k])
            # if smidata.is_leader[k] != True:       #TODO 在统计数据中增加一列当前节点是不是leader的数据，将这个条件替换掉。
            bct_list.append(smidata.actaul_timeout_list[k])
        print("bct_list: ",bct_list)
        avgbct = sum(bct_list) / float(len(bct_list))
        self.smi_number = self.smi_number + 1  # 又执行了一个mi
        self.last_avg_bct_list.append(avgbct)
        if self.smi_number != len(self.last_avg_bct_list):
            logging.info("error of the value of smi_number not equal the length of avg_bct_list")
        # 这的两个参数theta1、theta2的一个表示的是平均值的变化的大小，以及数据的抖动的程度
        # TODO
        if len(self.last_avg_bct_list) > 1:
            avg_bct_i = self.last_avg_bct_list[len(self.last_avg_bct_list) - 1]
            avg_bct_j = self.last_avg_bct_list[len(self.last_avg_bct_list) - 2]
            numerator = abs(avg_bct_j - avg_bct_i)
            denominator = max([avg_bct_j, avg_bct_i])
            theta_1 = numerator / denominator
            theta_2 = np.std(smidata.actaul_timeout_list)
            self.theta_1 = theta_1
            self.theta_2 = theta_2
            # if theta_1 >= threshold1:  # (急升、急降）
            #     self.code = 0
            # elif threshold2 < theta_1 < threshold1:  # （缓升、缓降）
            #     self.code = 1
            # elif theta_1 <= threshold2 and theta_2 <= threshold3:  # （平缓抖动）
            #     self.code = 2
            # else:  # (极具抖动)
            #     self.code = 3
            if theta_1 > threshold1: #TODO (升降模式)
                self.code = 0
                # self.beta = self.beta + 0.1
            else:                   #TODO(抖动模式)
                self.code = 1
                # self.beta = self.beta - 0.1

            if mode == "fixed":
                self.code = 2

            if self.code == 0:
                self.beta = self.beta + 0.1
                if self.beta >= 1.0:
                    self.beta = 0.99
            elif self.code == 1:
                self.beta = self.beta - 0.1
                if self.beta <= 0.0:
                    self.beta = 0.01
        else:
            logging.info("in the init state")

    def savedata(self, smidata):
        height_list = smidata.height
        is_leader = smidata.is_leader
        proposer_list = smidata.proposer_list
        is_byz_node = smidata.is_byz_node
        actual_value_list = smidata.actaul_timeout_list
        predict_value_list = smidata.predict_timeout_list
        byz_timeout_list = smidata.byz_timeout_list
        round_list = smidata.round
        cost_list = smidata.cost_list
        latency_list = smidata.latency_list
        tx_num_list = smidata.tx_num_list
        tps = smidata.tps
        logging.info("height_list: %s", height_list)
        logging.info("is_leader: %s",is_leader)
        logging.info("proposer_list: %s", proposer_list)
        logging.info("is_byz_node: %s", is_byz_node)
        logging.info("actual_value_list: %s", actual_value_list)
        logging.info("predict_value_list: %s", predict_value_list)
        logging.info("byz_timeout_list: %s", byz_timeout_list)
        logging.info("round_list: %s", round_list)
        logging.info("latency_list: %s", latency_list)
        logging.info("tx_num_list: %s", tx_num_list)
        logging.info("the tps of last MI: %s", tps)
        CostList = []
        view_change_cost_list = []
        threat_cost_list = []
        discretionary_list = []
        flag_list = []

        for cost in cost_list:
            CostList.append(cost.view_change_cost + int(cost.threat_cost / 3))
            view_change_cost_list.append(cost.view_change_cost)
            threat_cost_list.append(cost.threat_cost)
            discretionary_list.append(cost.discretionary)
            flag_list.append(cost.flag)
        # logging.info("the CostList of the mi: %s", CostList)

        save_data_list = []
        for k in range(len(actual_value_list)):
            sava_data = []
            sava_data.append(height_list[k])
            sava_data.append(is_leader[k])
            sava_data.append(proposer_list[k])
            sava_data.append(is_byz_node[k])
            sava_data.append(round_list[k])
            sava_data.append(actual_value_list[k])
            sava_data.append(predict_value_list[k])
            sava_data.append(byz_timeout_list[k])
            sava_data.append(view_change_cost_list[k])
            sava_data.append(threat_cost_list[k])
            sava_data.append(discretionary_list[k])
            sava_data.append(CostList[k])
            sava_data.append(flag_list[k])
            sava_data.append(tps)
            sava_data.append(latency_list[k])
            sava_data.append(tx_num_list[k])
            sava_data.append(tps)
            save_data_list.append(sava_data)
        with open(file_path, 'a', newline='') as file:
            writer = csv.writer(file)
            for save_data in save_data_list:
                writer.writerow(save_data)

    # 这个其实就是保存一下smi中对应的height,alpha、beta、和几个辅助值theta_1、theta_2
    def savePara(self,smidata):
        height_list = smidata.height
        bct_list = smidata.actaul_timeout_list
        is_leader = smidata.is_leader
        save_data_list = []
        for k in range(len(height_list)):
            sava_data = []
            sava_data.append(height_list[k])
            sava_data.append(self.smi_number-1)
            sava_data.append(bct_list[k])
            sava_data.append(is_leader[k])
            sava_data.append(self.theta_1)
            sava_data.append(self.theta_2)
            sava_data.append(self.code)
            sava_data.append(self.alpha)
            sava_data.append(self.beta)
            save_data_list.append(sava_data)
        with open(para_file_path, 'a', newline='') as file:
            writer = csv.writer(file)
            for save_data in save_data_list:
                writer.writerow(save_data)

    def run(self, smidata):
        self.savedata(smidata)
        self.savePara(smidata)
        self.predictModel(smidata)
        return self.code

class SmiData:
    def __init__(self, height_list, is_leader,proposer_list, is_byz_node, actaul_timeout_list, predict_timeout_list,
                 byz_timeout_list, round_list, cost_list, latency_list, tx_num_list, tps):
        self.height = height_list
        self.is_leader = is_leader
        self.proposer_list = proposer_list
        self.is_byz_node = is_byz_node
        self.actaul_timeout_list = actaul_timeout_list
        self.predict_timeout_list = predict_timeout_list
        self.byz_timeout_list = byz_timeout_list
        self.round = round_list
        self.cost_list = cost_list
        self.latency_list = latency_list
        self.tx_num_list = tx_num_list
        self.tps = tps


class Cost:
    def __init__(self, view_change_cost, threat_cost, discretionary, flag):
        self.view_change_cost = view_change_cost
        self.threat_cost = threat_cost
        self.discretionary = discretionary
        self.flag = flag


if __name__ == '__main__':
    # 给出响应的测试程序.
    spc = Spc(8)
    sample_data_list = [[[2]], [[4]], [[8]], [[16]], [[13]], [[7], [3], [5], [9]], [[13]], [[14]], [[16]], [[15]],
                        [[1], [2], [4], [5]]]  # 这个说的是效用值
    height = 1  # 高度是1
    for sample_data in sample_data_list:
        test_data_list = []
        for j in range(len(sample_data)):  # 一个sample_data中可能包含多个Mi
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
            for k in range(spc.mi):
                if k == spc.mi - 1:
                    height_list.append(height)
                    is_leader.append(False)
                    proposer_list.append(height % 16)
                    is_byz_node.append(False)
                    actaul_timeout_list.append(2)
                    predict_timeout_list.append(2)
                    byz_timeout_list.append(1)
                    round_list.append(3)
                    latency_list.append(2)
                    tx_num_list.append(3)
                    cost = Cost(sample_data[j][0], 0, 5, True)
                    cost_list.append(cost)
                else:
                    height_list.append(height)
                    is_leader.append(False)
                    proposer_list.append(height % 16)
                    is_byz_node.append(False)
                    actaul_timeout_list.append(0)
                    predict_timeout_list.append(0)
                    byz_timeout_list.append(0)
                    round_list.append(0)
                    latency_list.append(2)
                    tx_num_list.append(3)
                    cost = Cost(0, 0, 0, False)
                    cost_list.append(cost)
                height = height + 1
            smi_data = SmiData(height_list, is_leader,proposer_list, is_byz_node, actaul_timeout_list, predict_timeout_list,
                               byz_timeout_list, round_list, cost_list, latency_list, tx_num_list, sample_data[j][0])
            spc.run(smi_data)
            # test_data_list.append(test_data)
        # pcc.run(test_data_list)
        # logging.info("next_init_timeout: %s",pcc.init_timeout)
