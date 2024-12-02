# -*- coding: utf-8 -*-
'''
   这个文件是的Model是用来预测下一个区块高度的timeout
'''

import logging
import os
import random
import time
import math
import numpy as np
import csv
import statistics
import copy
import matplotlib.pyplot as plt
from sklearn.ensemble import RandomForestRegressor

LOG_FORMAT = '%(asctime)s - %(levelname)s - %(message)s'
logging.basicConfig(level=logging.DEBUG, format=LOG_FORMAT)

class Model:
    # 初始化模型这个模型不是学习模型。
    # cold_start_phase: 冷启动阶段，这个是为了后面拜占庭节点作恶设置的
    def __init__(self,version,height,mi,learning_model,init_timeout,input_data,cold_start_phase,experiences_window,method,para_a):
        id = os.getenv("ID")
        self.index = '../config_path/node'+id+'/'
        # self.index = ''
        self.version = version
        self.init_timeout = init_timeout
        self.cold_start_phase = cold_start_phase
        self.Height = height                    #当前区块的高度
        self.learning_model = learning_model    #当前使用的学习模型
        self.trainFlag = False                  #表示当前模型中学习模型是否被训练过
        self.input_data = input_data            #当前区块的Timeout以及计算下一个区块所需要的一些历史信息
        self.experience_pool = ExperiencePool(experiences_window=experiences_window*mi) #存储一些经验值
        self.input_data_list = {}               #存储一些历史信息，为了投毒的时候做测试集
        self.Mode = False                        #表示是否是在企图作恶
        self.outlier_dict_lable = {}
        self.outlier_dict ={}
        self.MI = mi
        self.wait_write_data = []
        self.Method = method        #选择的方法  ["mal-para mal-to Jac Fixed EMA"]
        self.Para_a = para_a


    # 保存相应高度的数据
    def savedataofheight(self,height_data):
        cost = height_data.cost
        Cost = cost.view_change_cost + int(cost.threat_cost / 3)
        view_change_cost = cost.view_change_cost
        threat_cost = cost.threat_cost
        discretionary = cost.discretionary
        flag = cost.flag
        save_data = []
        save_data.append(height_data.height)
        save_data.append(height_data.is_leader)
        save_data.append(height_data.proposer)
        save_data.append(height_data.is_byz_node)
        save_data.append(height_data.is_crash_node)
        save_data.append(height_data.round)
        # actual_time = height_data.actaul_timeout-height_data.delay_time
        # if height_data.delay_time<0.0:
        #     actual_time = height_data.actaul_timeout
        # save_data.append(actual_time)
        save_data.append(height_data.actaul_timeout)
        save_data.append(height_data.pdt)
        save_data.append(height_data.predict_timeout)
        save_data.append(height_data.byz_timeout)
        save_data.append(view_change_cost)
        save_data.append(threat_cost)
        save_data.append(discretionary)
        save_data.append(Cost)
        save_data.append(flag)
        save_data.append(height_data.tps)
        save_data.append(height_data.latency)
        save_data.append(height_data.tx_num)
        save_data.append(height_data.tps)
        test_delay_time = 0.0
        if height_data.delay_time >0.0:
            test_delay_time = height_data.delay_time
        save_data.append(test_delay_time)
        save_data.append(height_data.min)
        save_data.append(height_data.max)
        save_data.append(height_data.avg)
        save_data.append(height_data.std)
        save_data.append(height_data.deliver_time)
        self.hight_data_writer.writerow(save_data)
        self.hight_data_store.flush()

    # 初始化模型训练和预测过程中的一些辅助信息，比如确定一下(x,y)的形式
    # 这个只是初始化了一个随机森林的输入，和输出。
    def initLearningModel(self,node_type):
        #下面是加入一个辅助函数记录一下历史数据
        self.hight_data_store = open( self.index+node_type+'_height_data.csv', 'w')
        self.hight_data_writer = csv.writer(self.hight_data_store)
        self.hight_data_writer.writerow(["height", "is_leader","proposer", "is_byz", "is_crash","round", "bct", "pdt","timeout", "byz_timeout", "vc-cost", "threat",
                  "discretionary", "cost", "crash", "reward", "latency", "tx_num", "tps","delay_time","min","max","avg","std","deliver_time"])
        self.hight_data_store.flush()

        #下面是加入一个辅助函数记录一下历史数据
        self.data_store = open( self.index+node_type+'_experiences.csv', 'w')
        self.csv_writer = csv.writer(self.data_store)
        self.csv_writer.writerow(
            ["start_height",'avg_rcpt', 'avg_rcpt_var', 'avg_bct', 'std_bct','label', 'alpha', 'beta',"K",
             'utility_value','TO','PDT','TO-PDT','Differ','Differ_Normal','compute_utility_overhead (s)', 'defense_overhead(s)','training_overhead (s)', 'feature_extraction_overhead (s)', 'inference_overhead (s)',
              'episode_duration (s)'])
        self.data_store.flush()

        # 记录一些辅助信息, 作恶的过程。 这个是个拜占庭的byz
        self.byz_info = open(self.index+node_type+'_byz_data.csv','w')
        self.byz_csv_writer = csv.writer(self.byz_info)
        self.byz_csv_writer.writerow(['bct','timeout','byz_bct'])
        self.byz_info.flush()

        # 记录每个区块高度的信息，这个是每个高度对应的辅助
        self.input_data_info = open(self.index + node_type+'_input_data_info.csv','w')
        self.input_data_writer = csv.writer(self.input_data_info)
        self.input_data_writer.writerow(['height','rcpt', 'rcpt_var','pre_bct','timeout','bct'])
        self.input_data_info.flush()


        self.outlier_Info = open(self.index + node_type + 'distance.csv', mode='w', newline='')
        self.outlier_Info_writer = csv.writer(self.outlier_Info)
        self.outlier_Info_writer.writerow(['height', 'point1_x','point1_y','point2_x','point2_y','point3_x','point3_y','distance','lable','pre_lable'])
        self.outlier_Info.flush()

        alpha_list = None
        beta_list =  None
        k_list = None

        if self.Method == "mal-para":
            alpha_list = [i/10 for i in range(1, 10)]
            beta_list = [i/10 for i in range(1, 10)]
            k_list = [3,4,5,6,7,8]
        if self.Method == 'mal-to':
            alpha_list = [0]
            beta_list = [0]
            k_list = np.linspace(0,self.init_timeout,num=9*9*9+1)
        if self.Method == 'Jac':
            alpha_list = [0.125]
            beta_list = [0.25]
            k_list = [4]
        if self.Method == 'EMA':
            alpha_list = [0.1]
            beta_list = [0.25]
            k_list = [4]
        if self.Method == 'Fixed':
            alpha_list = [0]
            beta_list = [0]
            k_list = [self.init_timeout]

        actions = []
        for dim_1 in alpha_list:
            for dim_2 in beta_list:
                for dim_3 in k_list:
                    actions.append(np.array([dim_1, dim_2, dim_3]))
        actions_matrix = np.vstack(actions)
        self.enumeration_matrix = np.hstack((np.zeros((actions_matrix.shape[0], 5)), actions_matrix))
        # logging.info("the enumeration_matrix is: %s",self.enumeration_matrix)
        logging.info("%s learning agent has been initialized",node_type)

    #设置BCT的时间
    def setBct(self,bct):
        # TODO 应该加一个检查项，查看当前高度对应的input_data是否在input_data_list中。
        self.input_data.setBCT(bct)
        if str(self.Height) in self.input_data_list:
            logging.info("the input of height %s is already exists!",str(self.Height))
        else:
            self.input_data_list[str(self.Height)] = self.input_data

    # 设置一下Pdt的最终的有效的时间
    def setPdt(self,pdt):
        if str(self.Height) not in self.input_data_list:
            logging.info("set pdt error")
        else:
            self.input_data_list[str(self.Height)].setPDT(pdt)

    def setState(self,min_value,max_value,avg_value,std_value):
        if str(self.Height) not in self.input_data_list:
            logging.info("set state error")
        else:
             self.input_data_list[str(self.Height)].setState(min_value,max_value,avg_value,std_value)

    def save_input_data(self):
        self.input_data_list[str(self.Height)] = self.input_data

    def del_input_data(self):
        del self.input_data_list[str(self.Height)]

    #训练模型
    def traningLearningModel(self,training_x_set,training_y_set):

        assert (len(training_x_set)==len(training_y_set))
        # logging.info("traning_x_set: %s",training_x_set)
        # logging.info("traning_y_set: %s",training_y_set)
        if len(training_x_set) != 0:
            self.learning_model.fit(training_x_set,training_y_set)
            if not self.trainFlag :
                self.trainFlag = True

    def extractFeatures(self,height):
        s = None
        if self.Height >= self.cold_start_phase:
            # logging.info("the hestory height len is: %s the now height is: %s",len(self.input_data_list),self.Height)
            rcpt_list,rcpt_var_list,current_bct_data = self.get_MI_info(start_height=height)
            # compute the avg_rcpt
            avg_rcpt = statistics.mean(rcpt_list)
            # compute the avg_rcpt_var
            avg_rcpt_var = statistics.mean(rcpt_var_list)
            # compute the avg_bct
            avg_bct = statistics.mean(current_bct_data)
            # compute the std_bct
            std_bct = 0.0
            if self.MI == 1:
                std_bct = 0.0
            else:
                std_bct = statistics.stdev(current_bct_data)

            # compute lable
            # label = (self.Height) % 4   #TODO
            label = 0
            # assert (rcpt_list[-1]== avg_rcpt)
            # assert (rcpt_var_list[-1]==avg_rcpt_var)
            # assert (current_bct_data[-1]==avg_bct)
            # assert (std_bct==0)
            s = State(self.Height,rcpt_list[-1],rcpt_var_list[-1],current_bct_data[-1],1000,0)
            # s = State(self.Height,rcpt_list[-1],rcpt_var_list[-1],avg_bct,std_bct,label)
            # min_value,max_value,avg_value,std_value = self.get_State(start_height=height)
            # s = State(self.Height,min_value,max_value,avg_value,std_value,label)
            s = State(self.Height,avg_rcpt,avg_rcpt_var,avg_bct,std_bct,label)
        return s

    # 预测过程：
    # 通过列举action 选择最佳的action
    # feature_idx是说选择哪些特征值的，包括state和action
    #
    def Predict(self,state):

        # TODO 为什么要加个mode
        if state is not None and not self.Mode:
            assert (state.lastHeight == self.Height-1)
        action = None
        if self.trainFlag:
            enumeration_matrix = copy.deepcopy(self.enumeration_matrix)
            enumeration_matrix[:, 0:5] = np.array(state.tolist())
            feature_idx = np.array([True, True, True, True, True, True ,True, True])
            prediction = self.learning_model.predict(enumeration_matrix[:, feature_idx])
            while True:
                # 这tampson采样的部分(可能不是 只是选择最佳的策略)
                best_index = np.random.choice(np.flatnonzero(np.isclose(prediction, prediction.max())), replace=True)
                #更新下一个区块用到的action
                last_height = self.Height
                alpha = enumeration_matrix[best_index, 5]
                beta = enumeration_matrix[best_index, 6]
                K = enumeration_matrix[best_index, 7]
                action = Action(last_height,alpha,beta,K)
                break
        else:
            if self.Method == 'mal-para':
                last_height = self.Height
                alpha = 0.1     #alpha = 0.125
                beta = 0.1      #beta = 0.25
                K = 4           #K = 4
                action = Action(last_height,alpha,beta,K)
            if self.Method == 'mal-to':
                last_height = self.Height
                alpha = 0     #alpha = 0.125
                beta = 0     #beta = 0.25
                K = self.init_timeout          #K = 4
                action = Action(last_height,alpha,beta,K)
            if self.Method == 'Jac':
                last_height = self.Height
                alpha = 0.125     #alpha = 0.125
                beta = 0.25     #beta = 0.25
                K = 4          #K = 4
                action = Action(last_height,alpha,beta,K)
            if self.Method == 'Fixed':
                last_height = self.Height
                alpha = 0     #alpha = 0.125
                beta = 0     #beta = 0.25
                K = self.init_timeout          #K = 4
                action = Action(last_height,alpha,beta,K)
            if self.Method == 'EMA':
                last_height = self.Height
                alpha = 0.1     #alpha = 0.125
                beta = 0     #beta = 0.25
                K = 4          #K = 4
                action = Action(last_height,alpha,beta,K)
        # if self.Method == "Jac":
        #     action = Action(self.Height,0.125,0.25,4)
        return action

    # 计算过去MI的reward
    def computeReward(self):  #计算效用值函数
        reward_value = None
        if self.Height >= self.cold_start_phase:
            # logging.info("the compute reward height: %s",self.Height)
            bct_list = self.getMIPDTList(start_height=self.Height)
            to_list = self.getMITOList(start_height=self.Height)
            logging.info("the bct_list: %s,to_list: %s",bct_list,to_list)
            r ,r1,r2 = reward(self.version,bct_list,to_list,self.Para_a)
            reward_value = Reward(self.Height,r,bct_list[-1],to_list[-1],r1,r2)
        return reward_value

    #增加经验值，到经验池中
    def addExp(self,state,action,reward):
        if state is not None:
            if reward is not None:
                # 下面两个断言是为了不同MI上的state、action、reward对应一下
                assert (action.last_height == reward.last_height)
                assert (state.lastHeight == (reward.last_height-1))
                self.experience_pool.add_exp_x(state,action)
                self.experience_pool.add_exp_y(reward)
                self.experience_pool.pop_exp()

    '''
        这个是个作恶的过程，这个是个双层优化问题。
        1.现有训练集上获取最好的参数（随机森林的参数）
        2.在测试集上获取最大的timeout的值
    '''
    # 数据投毒的步骤,t1是真实的bct数据，t2是真实的timeout，利用timeout和bct之间的数据差
    # 其实这两个都是一个预测值，我们认为timeout就是bct在计算reward
    # 一定要注意这是一个不会发生view-change的作恶，delay view-change attack
    def dataPoisoning(self,t1,t2,state,action):
        self.Mode = True
        #作恶的几种操作
        byz_bct_list = np.linspace(t1, t2, 5)
        max_bct = byz_bct_list[0]
        max_timeout_sum = 0
        # TODO 这其实是个冷启动
        if self.Height >= self.cold_start_phase:
            training_set_x,training_set_y = self.get_training_set()
            testing_set = self.get_testing_set()
            for byz_bct in byz_bct_list:
                self.inner(byz_bct,state,action,training_set_x,training_set_y)
                timeout_sum = self.outer(testing_set)
                if timeout_sum > max_timeout_sum:
                    max_timeout_sum = timeout_sum
                    max_bct = byz_bct
        return max_bct

    # TODO
    def defense(self,byz_node_flag,is_leader_flag):
        pass
        # # 这个下面表明当前节点是真的
        # if byz_node_flag and is_leader_flag:
        #     self.outlier_dict_lable[str(self.Height)] = ''
        # logging.info("the outlier dict is: %s",self.outlier_Info)
        # #这个需要设置一个异常点检测的域值
        # threshold = 3
        # if len (self.input_data_list) >= 3:  # TODO
        #     save_data = []
        #     # 从高度2开始
        #     point_height = self.Height - 1
        #     save_data.append(point_height)
        #     point_bct = self.input_data_list[str(point_height)].BCT
        #
        #     low_point_height = point_height - 1
        #     low_point_bct = self.input_data_list[str(low_point_height)].BCT
        #
        #     high_point_height = point_height + 1
        #     high_point_bct = self.input_data_list[str(high_point_height)].BCT
        #
        #     point = (point_height,point_bct)
        #     low_point = (low_point_height,low_point_bct)
        #     high_point = (high_point_height,high_point_bct)
        #
        #     m,b = line_equation(low_point,high_point)
        #
        #     distance = point_to_line_distance(point,m,b)
        #
        #     save_data.extend([low_point_height,low_point_bct])
        #     save_data.extend([point_height,point_bct])
        #     save_data.extend([high_point_height,high_point_bct])
        #     save_data.append(distance)
        #     # 增加一列lable列
        #     if str(point_height) in self.outlier_dict_lable:
        #         save_data.append(1)
        #         self.outlier_dict[str(point_height)] = ''  #将异常点加入到字典中。
        #     else:
        #         save_data.append(0)
        #     #增加一列预测的lable列
        #     if distance > threshold:
        #         save_data.append(1)
        #     else:
        #         save_data.append(0)
        #
        #     self.outlier_Info_writer.writerow(save_data)  #TODO 添加一些辅助信息 有利于计算后面的统计信息，精确率、准确率、召回率等

    #内部优化新生成的模型的效果
    def outer(self,testing_set):
        timeout_sum = 0
        for sample in testing_set:
            state = self.extractFeatures(sample.Height)
            action = self.Predict(state)
            _,_,new_timeout = self.timeout(sample.BCT,action)
            timeout_sum = timeout_sum + new_timeout
        return timeout_sum

    #外部优化训练一个新learning_model，更新模型参数，不是(alpha、beta、K)
    def inner(self,byz_bct,state,action,training_set_x,training_set_y):
        logging.info("training_set_x'len is: %s,training_set_y'len is: %s",len(training_set_x),len(training_set_y))
        assert (len(training_set_x) == len(training_set_y))
        # TODO 还是要处理一下,怎样提前计算reward,先加入在删除掉？
        self.setBct(byz_bct)   #TODO 这个要看一下是不是新增加的高度
        self.save_input_data()
        poisoning_sample_x = np.array(state.tolist() + action.tolist())
        reward = self.computeReward()
        self.del_input_data()
        new_poisoning_sample_x = poisoning_sample_x.reshape(1, -1)
        training_set_x = np.vstack((training_set_x,new_poisoning_sample_x))
        training_set_y = np.append(training_set_y,np.array([reward.reward]))
        self.traningLearningModel(training_set_x,training_set_y)


    # 用来计算timeout
    def timeout(self,bct,action):
        rcpt,rcpt_var,timeout = rtt(self.Method,bct,self.input_data.RCPT,self.input_data.RCPTVAR,action.alpha,action.beta,action.K)
        return rcpt,rcpt_var,timeout

    # 用来估计诚实节点的timeout,用来作恶
    def byz_timeout(self):
        pass

    # 获取历史数据用于测试集的选取
    def get_input_data_list(self):
        return self.input_data_list

    # 获取训练集根据重复抽样，TODO 在这可以考虑深拷贝的问题（experience）
    def get_training_set(self):
        training_x_set = []
        training_y_set = []
        if self.experience_pool.isNotNull():
            feature_idx = np.array([True, True, True, True, True, True, True,True])
            # 抽样整体，抽样样本数，是否重复抽样
            bootstrapped_idx = np.random.choice(self.experience_pool.Len(), self.experience_pool.Len(), replace=True)
            # bootstrapped_idx = self.experience_pool.samplewithoutrepeats(self.MI,self.outlier_dict_lable)   #这个其实就是又放回的采样
            # logging.info("抽样到的bootstrapped_idx %s",bootstrapped_idx)
            # 垂直折叠
            training_x_set = np.vstack(self.experience_pool.experience_X)[bootstrapped_idx, :][:, feature_idx]
            training_y_set = np.array(self.experience_pool.experience_Y)[bootstrapped_idx]

        return training_x_set,training_y_set

    #获取测试集
    def get_testing_set(self):
        testing_set = []
        #TODO 这个随机数的开始的值,其实要保证随机数的大段要比小端要大，那其实也是要有个冷启动的阶段
        random_numbers = [random.randint(self.MI+1, self.Height-1) for _ in range(int(self.Height)//int(3) + 1)]
        logging.info("the random_numbers is: %s",random_numbers)
        for random_number in random_numbers:
            testing_set.append(self.input_data_list[str(random_number)])
        return testing_set


    # 获取当前MI需要用到的数据结构
    def get_State(self,start_height):
        min_value = 0.0
        max_value = 0.0
        avg_value = 0.0
        std_value = 0.0
        for t in range(self.MI):
            if str(start_height-t) in self.input_data_list:
                min_value = self.input_data_list[str(start_height-t)].Min
                max_value = self.input_data_list[str(start_height-t)].Max
                avg_value = self.input_data_list[str(start_height-t)].Avg
                std_value = self.input_data_list[str(start_height-t)].Std
            else:
                raise Exception("the height of "+str(start_height-t)+ " input_data is not in the input_data_list")
        return min_value,max_value,avg_value,std_value

    # 获取当前MI需要用到的数据结构
    def get_MI_info(self,start_height):
        rcpt_list = []
        rcpt_var_list = []
        current_bct_data = []    #TODO 起始的高度
        for t in range(self.MI):
            if str(start_height-t) in self.input_data_list:
                rcpt_list.append(self.input_data_list[str(start_height-t)].RCPT)
                rcpt_var_list.append(self.input_data_list[str(start_height-t)].RCPTVAR)
                current_bct_data.append(self.input_data_list[str(start_height-t)].BCT)
            else:
                raise Exception("the height of "+str(start_height-t)+ " input_data is not in the input_data_list")
        return rcpt_list,rcpt_var_list,current_bct_data

    def getMIBCTList(self,start_height):
        bct_list = []
        for t in range(self.MI):
            if str(start_height-t) in self.input_data_list:
                bct_list.append(self.input_data_list[str(start_height-t)].BCT)
            else:
                raise Exception("the height of "+str(start_height-t)+ " input_data is not in the input_data_list")
        return bct_list

    def getMIPDTList(self,start_height):
        pdt_list = []
        for t in range(self.MI):
            if str(start_height-t) in self.input_data_list:
                pdt_list.append(self.input_data_list[str(start_height-t)].PDT)
            else:
                raise Exception("the height of "+str(start_height-t)+ " input_data is not in the input_data_list")
        return pdt_list

    def getMITOList(self,start_height):
        to_list = []
        for t in range(self.MI):
            if str(start_height-t) in self.input_data_list:
                to_list.append(self.input_data_list[str(start_height-t)].Timeout)
            else:
                raise Exception("the height of "+str(start_height-t)+ " input_data is not in the input_data_list")
        return to_list

    def getFullTOandBCT(self):
        bct_list = []
        to_list = []
        for h in range(self.Height-1):
            bct_list.append(self.input_data_list[str(h+1)].BCT)
        for h in range(self.Height-1):
            to_list.append(self.input_data_list[str(h+1)].Timeout)
        return bct_list,to_list

    #给一个新的高度
    '''
    在进入一个新的高度后：
    1设置这个高度的真实bct
    2计算一下当前MI（height-MI）对应的reward
    3将经验值放入经验池
    4抽样
    5重新训练模型
    6预测action，
    7计算下一个高度对应的timeout
    '''
    #在统计时间的时候 能不能在下一个高度再保存上一个高度的数据
    def new_height(self,bct_data,pdt_data,state,action,byz_node_flag,is_leader_flag,last_latency,min_value,max_value,avg_value,std_value):
        if self.Height > self.cold_start_phase+1:
            self.wait_write_data.append(last_latency)
            self.csv_writer.writerow(self.wait_write_data)
            self.data_store.flush()
        self.wait_write_data = []

        self.setBct(bct_data)  #顺便将Input_data放入input_data_list中
        self.setPdt(pdt_data)
        self.setState(min_value,max_value,avg_value,std_value)
        self.input_data_writer.writerow(self.input_data.tolist())
        self.input_data_info.flush()



        # TODO 其实这块要等当前MI执行完，才能计算出来。

        start_compute_reward_time = time.time()
        r = self.computeReward() #拿到真实的Label 这个是个新的样本的y  TODO 这个其实是要等当前MI执行完之后才能有的,哪个步骤都行。
        end_compute_reward_time = time.time()
        compute_reward_time = end_compute_reward_time - start_compute_reward_time
        self.wait_write_data.append(compute_reward_time)


        self.addExp(state,action,r)

        if state is not None:
            if r is not None:
                write_data1 = [self.Height]
                write_data2 = state.tolist()+action.tolist()+r.tolist()
                write_data1.extend(write_data2)
                write_data = write_data1
                write_data.extend(self.wait_write_data)
                self.wait_write_data = write_data


        start_defense_time = time.time()
        self.defense(byz_node_flag,is_leader_flag)
        end_defense_time  = time.time()
        defense_time = end_defense_time - start_defense_time
        self.wait_write_data.append(defense_time)


        start_training_time = time.time()
        training_set_x,training_set_y = self.get_training_set()
        self.traningLearningModel(training_set_x,training_set_y)  #重新训练模型
        end_training_time = time.time()
        training_time = end_training_time - start_training_time
        self.wait_write_data.append(training_time)

        start_extractFeatures_time = time.time()
        state = self.extractFeatures(self.Height)
        end_extractFeatures_time = time.time()
        extractFeatures_time = end_extractFeatures_time - start_extractFeatures_time
        self.wait_write_data.append(extractFeatures_time)
        height = self.Height
        self.Height = self.Height + 1
        self.Mode = False

        start_Predict_time = time.time()
        action = self.Predict(state)   #根据现有的模型进行预测 这个是个新的样本的x
        end_Perdict_time = time.time()
        Predict_time = end_Perdict_time - start_Predict_time
        self.wait_write_data.append(Predict_time)

        logging.info("the height is: %s,the compute reward time: %s, the defense time: %s, the training time: %s, the extraFeature time: %s, the Predict time: %s",height,compute_reward_time,defense_time,training_time,extractFeatures_time,Predict_time)
        rcpt = 0
        rcpt_var = 0
        new_timeout = 0
        if is_leader_flag and byz_node_flag:
        # if test_flag:
            if str(self.Height-1) in self.input_data_list:
                input_data = self.input_data_list[str(self.Height-1)]
                rcpt = input_data.RCPT
                rcpt_var = input_data.RCPTVAR
                new_timeout =input_data.Timeout  #这个其实是去验证timeout的好坏，用来生成后续的timeout  TODO 这个其实可以是每个MI多次
            else:
                rcpt = 0
                rcpt_var = 0
                new_timeout = self.init_timeout
        else:
            rcpt,rcpt_var,new_timeout = self.timeout(bct_data,action)  #这个其实是去验证timeout的好坏，用来生成后续的timeout  TODO 这个其实可以是每个MI多次
        # rcpt,rcpt_var,new_timeout = self.timeout(bct_data,action)  #这个其实是去验证timeout的好坏，用来生成后续的timeout  TODO 这个其实可以是每个MI多次
        if new_timeout > self.init_timeout:
            new_timeout = self.init_timeout
        if self.Height > self.cold_start_phase:
            new_input_data = InputData(height=self.Height,rcpt=rcpt,rcpt_var=rcpt_var,timeout=new_timeout,pre_bct=new_timeout)
        else:
            new_input_data = InputData(height=self.Height,rcpt=rcpt,rcpt_var=rcpt_var,timeout=self.init_timeout,pre_bct=new_timeout)
        self.input_data = new_input_data
        return state,action

# 记录一下区块的开始的信息
class InputData:
    def __init__(self,height,rcpt,rcpt_var,pre_bct,timeout):
        self.Height = height     #当前区块高度
        self.RCPT = rcpt         #计算下一个区块Timeout需要使用到的辅助数据
        self.RCPTVAR = rcpt_var  #计算下一个区块Timeout需要使用到的辅助数据
        self.PreBct = pre_bct    #这个是为了有一个冷启动环节，timeout和pre_bct可能不一致,这个是一个冷启动的环节
        self.Timeout = timeout   #当前区块对应的Timeout
    def setBCT(self,bct):
        self.BCT = bct
    def setPDT(self,pdt):
            self.PDT = pdt
    def setState(self,min_value,max_value,avg_value,std_value):
        self.Min = min_value
        self.Max = max_value
        self.Avg = avg_value
        self.Std = std_value
    def tolist(self):
        return [self.Height,self.RCPT,self.RCPTVAR,self.PreBct,self.Timeout,self.BCT]
# 状态
class State:
    def __init__(self,last_height,avg_rcpt,avg_rcpt_var,avg_bct,std_bct,label):
        self.lastHeight = last_height
        self.avg_rcpt = avg_rcpt
        self.avg_rcpt_var = avg_rcpt_var
        self.avg_bct = avg_bct
        self.std_bct = std_bct
        self.label= label
    def tolist(self):
        s_list = [self.avg_rcpt,self.avg_rcpt_var,self.avg_bct,self.std_bct,self.label]
        return s_list
# 动作
class Action:
    def __init__(self,last_height,alpha,beta,k):
        self.last_height = last_height
        self.alpha = alpha
        self.beta = beta
        self.K = k
    def tolist(self):
        a_list = [self.alpha,self.beta,self.K]
        return a_list


class Reward:
    def __init__(self,last_height,reward,pdt,to,differ,differ_normal):
        self.last_height = last_height
        self.reward = reward
        self.PDT = pdt
        self.TO = to
        self.differ = differ
        self.differ_normal = differ_normal
    def tolist(self):
        return [self.reward,self.TO,self.PDT,self.TO-self.PDT,self.differ,self.differ_normal]

class ExperiencePool:

    def __init__(self,experiences_window):
        self.experience_X = []
        self.experience_Y = []
        self.experiences_window=experiences_window
        self.start_height = 1
    #
    def add_exp_x(self,state,action):
        x_list = []
        s_list = state.tolist()
        x_list.extend(s_list)
        a_list = action.tolist()
        x_list.extend(a_list)
        self.experience_X.append(x_list)

    def add_exp_y(self,reward):
        y = reward.reward
        self.experience_Y.append(y)

    def isNotNull(self):
        if len(self.experience_X) == 0:
            return False
        return True
    # 使得经验池保持恒定的长度
    def pop_exp(self):
        assert (len(self.experience_X) == len(self.experience_Y))
        while len(self.experience_X) > self.experiences_window:
            self.experience_X.pop(0)
            self.experience_Y.pop(0)
            self.start_height = self.start_height + 1

    def Len(self):
        return len(self.experience_X)

    def samplewithoutrepeats(self,mi,outlier_dict):
        samples = [[self.start_height + i + j  for j in range(mi)] for i in range(len(self.experience_X))]
        logging.info("the outlier_dict is: %s",outlier_dict)
        # logging.info("the samples is: %s",samples)
        selected_samples = []
        # logging.info("the experience_X's length is: %s,10mi is: %s",len(self.experience_X),10*mi)
        if len(self.experience_X)>(10*mi): #TODO 抽样的开始大小
            used_numbers = set()
            for out in outlier_dict:
                used_numbers.add(int(out))
            samples_dict = {}
            while True:
                # logging.info("find true sample")
                sample = random.choice(samples)
                key = '-'.join(str(num) for num in sample)
                if key in samples_dict:
                    selected_samples.append(sample)
                else:
                    # 检查样本中的数字是否已经使用
                    if all(num not in used_numbers for num in sample):
                        selected_samples.append(sample)
                        used_numbers.update(sample)
                        samples_dict[key] = ''
                # 停止条件：如果没有可用的样本
                if len(selected_samples) >= (len(self.experience_X) // mi) :  # 因为数字范围是1到103
                    break
        else:
            index = len(self.experience_X)-1
            while index >= 0:
                for test_node in samples[index]:
                    if str(test_node) in outlier_dict:
                        continue
                selected_samples.append(samples[index])
                index = index - mi

        # logging.info("the select samples is: %s",selected_samples)
        bootstrapped_idx = []
        for selected_sample in selected_samples:
            bootstrapped_idx.append(selected_sample[0]-self.start_height)
        bootstrapped_idx = np.array(bootstrapped_idx)

        return bootstrapped_idx


class Config:
    def __init__(self,k,high,low,high_std,low_std,step):
        self.K = k  #生成几个循环的数据
        self.high = high
        self.low = low
        self.high_std = high_std
        self.low_std = low_std
        self.step = step

def rtt(method,bct, RCPT, RCPTVAR, alpha, beta,K):  # rtt是根据五个变量预测下一个时间的值

    if method == 'mal-para' or method == 'Jac':
        RCPT = (1 - alpha) * RCPT + alpha * float(bct)  # 低通滤波计算下一个RCPT
        timeVar = float(bct) - RCPT
        RCPTVAR = (1 - beta) * RCPTVAR + beta * abs(timeVar)
        timeout = RCPT + K * RCPTVAR
    if method == 'mal-to' or method == 'Fixed':
        RCPT = 0.0
        RCPTVAR = 0.0
        timeout = K
    if method == "EMA":
        RCPT = (1 - alpha) * RCPT + alpha * float(bct)
        RCPTVAR = 0.0
        timeout = RCPT
    # print("data",sample,"timeout",timeout,"RCPT",RCPT,"RCPTVAR",RCPTVAR,"timeVar",abs(timeVar))
    return RCPT, RCPTVAR, timeout


# 怎样产生网络状况
def generatedata( high, low, std1, std2, step):
    # 第一阶段，从low经过步数step提升到high
    step1 = random.randint(1,step)
    step1 = step
    data1 = np.linspace(low, high, step1)

    # 第二阶段，在high经过步数step在均值为high方差为std1上正态分布
    step2 = random.randint(1,step)
    step2 = step
    data2 = np.random.normal(high, std1, step2)

    # 第三阶段，从high经过步数step降低到low
    step3 = random.randint(1,step)
    step3 = step
    data3 = np.linspace(high, low, step3)

    # 第四阶段，在low经过步数step在均值为low方差为std2上正态分布
    step4 = random.randint(1,step)
    step4 = step
    data4 = np.random.normal(low, std2, step4)

    # data1 = np.random.normal(high, std1, step)
    # data2 = np.random.normal(high, std1, step)
    # data3 = np.random.normal(high, std1, step)
    # data4 = np.random.normal(high, std1, step)
    return data1, data2, data3, data4

# 生成模拟的数据，并将模拟数据存到文件中
def model_data(config,ReNewFlag,datafile):

    bct_data_list =  []
    for k in range(config.K):
        d1,d2,d3,d4 = generatedata(config.high,config.low,config.high_std,config.low_std,config.step)
        bct_data_list.extend(d1)
        bct_data_list.extend(d2)
        bct_data_list.extend(d3)
        bct_data_list.extend(d4)
    # 这设置一个能够控制重复原来波形的控制
    if ReNewFlag:
        save_model_data(bct_data_list, datafile)
    bct_data_list = read_model_data(datafile)
    return bct_data_list

#下面这两个函数，是为了复现实验结果所做的
def save_model_data(bct_data, file_path):
    with open(file_path, 'w', newline='') as csvfile:
        writer = csv.writer(csvfile)
        writer.writerow(bct_data)

def read_model_data(file_path):
    bct_data = []
    with open(file_path, 'r') as csvfile:
        reader = csv.reader(csvfile)
        for row in reader:
            bct_data.extend([float(val) for val in row])
    return bct_data

# 线性归一化
def linear_normalization(value, min_value=-5000.0, max_value=5000.0, a=-5, b=5):
    if value < -5000.0:
        value = -5000.0
    if value > 5000.0:
        value = 5000.0
    if max_value == min_value:
        raise ValueError("max_value must be greater than min_value")

    normalized_value = (value - min_value) / (max_value - min_value) * (b - a) + a
    return normalized_value

# 定义变换函数
def transform(x, a):
    logging.info("the para a is：%s",a)
    return 4 / (math.exp(x + a)+ math.exp(-x - a))**2

# 激励函数的作用
# 这个epoch的含义是错过多少个数据不参与计算reward
# data1表示的PDT
# data2表示的是timeout
# 其实我们想要的激励函数是TPS的大小z
def reward(version, data1, data2,para_a):
    differ = 0.0
    normal_differ = 0.0
    reward = 0
    view_change = 0
    threat_space = 0
    assert (len(data1) == len(data2))

    if version == 'v1':
        if data1[-1] > data2[-1]:
            reward = -1
        else:
            reward = reward + 1 #v1
    if version == 'v2':
        if data2[-1] > data1[-1]:
            reward = reward + data1[-1] + 1/3*(data2[-1]-data1[-1])
        else:
            reward = reward + data1[-1]
    if version == 'v3':
        differ = data2[-1] - data1[-1]
        normal_differ = linear_normalization(differ)
        reward = transform(normal_differ,para_a)
        reward = 1/reward
    if version == 'v4':
        reward = 1/data2[-1]


    return 1/reward, differ,normal_differ

# 两个点确定的距离
def line_equation(point1, point2):
    x1, y1 = point1
    x2, y2 = point2
    # 计算斜率
    m = (y2 - y1) / (x2 - x1)

    # 计算y轴截距
    b = y1 - m * x1
    return m, b

# 点到直线的剧烈
def point_to_line_distance(point, m, b):
    x0, y0 = point
    distance = abs(m * x0 - y0 + b) / ((m**2 + 1)**0.5)
    return distance

#这个是画出生成个的bct_data_list 查看
def print_bct_data_list(bct_data_list):
    plt.plot(bct_data_list,label= "bct_data_list")
    plt.legend()
    plt.figure()

#画两条曲线 bct和timeout
#TODO 这的byz_height是什么意思(需要解释一下)
def print_data(timeout,bct,title):
    plt.plot(bct,label= "bct")
    plt.plot(timeout,label= "timeout")
    # x = []
    # y = []
    # for key in byz_height:
    #     x.append(int(key))
    #     y.append(float(byz_height[key]))
    # plt.scatter(x, y, marker='o', color='red')
    # 设置标题
    plt.title(title)
    plt.legend()
    plt.figure()
    plt.title

#测试一下观测点的距离
def print_distance(distance_dict,height):
    distance = []
    x = []
    y = []
    for h in range (height-3):
        distance.append(float(distance_dict[str(h+2)]))
        if (h+2) % 5 == 0:
            x.append(h)
            y.append(float(distance_dict[str(h+2)]))
    plt.plot(distance,label= "dis")
    plt.scatter(x, y, marker='o', color='red')
    plt.legend()
    plt.show()
def save_distance(csv_file,data):
    with open(csv_file, mode='w', newline='') as file:
        writer = csv.writer(file)

        # 写入字典的键值对
        for key, value in data.items():
            writer.writerow([key, value])

# 这个函数中可以调节的参数有MI,Pre_Bct,也就是init_timeout
# 第一高度上是没有state的
#
def generatemodel(node_type,version,MI,init_timeout,cold_start_phase,experiences_window,method,para_a):
    height = 1  #开始的高度是从1开始的
    # 后面这两个参数是同时设置的，一个是预测的bct，而是timeout
    input_data = InputData(height=height,rcpt=0.0,rcpt_var=0.0,pre_bct = init_timeout,timeout=init_timeout)
    state = None
    action = Action(last_height=1,alpha=0.1,beta=0.1,k=4)
    learning_model = RandomForestRegressor(max_depth=5)     #这个是使用的随机森林的模型
    model = Model(version=version,height=height,mi=MI,learning_model=learning_model,init_timeout=init_timeout,input_data=input_data,cold_start_phase=cold_start_phase,experiences_window=experiences_window,method=method,para_a=para_a)
    logging.info("the node type is: %s",node_type)
    model.initLearningModel(node_type)
    return model,state,action

#下面是主程序
'''
作恶节点作恶的空间需要两个
    这两个timeout的差值就是作恶节点的作恶空间
    现在基于两个假设：（TODO 网络是扁平的，那么每个节点之间的延迟是相同的，and 作恶节点只有一个）
    1诚实节点设置的timeout
    还是基于每个高度bct的来计算，但是这两个点是特殊的，
    1）估计点在做leader时,bct的取值（上帝视角） 在真实的系统的怎样实现，使用ack/
    2）作恶节点在做leader时，bct的取值就是两个model所计算出来的bct差值，中间的一个取值
    其他节点的bct，通过网络是扁平的，其实是可以拿到的

    2真实的网络bct
    在作恶节点上，其实我们可以拿到其他诚实节点发送给当前节点的数据，当前节点是作恶节点是leader时，通过上帝视角
    在自己做leader时,bct的取值（上帝视角） 在真实的系统的怎样实现
'''
class HeightData:
    def __init__(self,height,
                 is_leader,
                 proposer,
                 is_byz_node,
                 is_crash_node,
                 actaul_timeout,
                 predict_timeout,
                 byz_timeout,
                 round,
                 timeout_value,
                 latency,
                 tx_num,
                 tps,
                 delay_time,
                 pdt,
                 min,
                 max,
                 avg,
                 std):
        self.height = height
        self.is_leader = is_leader
        self.proposer = proposer
        self.is_byz_node = is_byz_node
        self.is_crash_node = is_crash_node
        self.actaul_timeout = actaul_timeout
        self.predict_timeout = predict_timeout
        self.byz_timeout = byz_timeout
        self.round = round
        self.cost = Cost(0,0,0,0)
        self.timeout_value =  timeout_value
        self.latency = latency
        self.tx_num = tx_num
        self.tps = tps
        self.delay_time = delay_time
        self.pdt = pdt
        self.min = min
        self.max = max
        self.avg = avg
        self.std = std

class Cost:
    def __init__(self, view_change_cost, threat_cost, discretionary, flag):
        self.view_change_cost = view_change_cost
        self.threat_cost = threat_cost
        self.discretionary = discretionary
        self.flag = flag

def generteheightdata(data):
    height_dtat = HeightData(height=int(data[0]),is_leader=data[1],proposer=int(data[2]),is_byz_node=data[3],is_crash_node=data[4],round=data[5],actaul_timeout=float(data[6]),
                             pdt=float(data[7]),predict_timeout=float(data[8]),byz_timeout=float(data[9]),tps = float(data[15]),latency=float(data[16]),tx_num=int(data[17]),delay_time= float(data[19]),
                                                                                                 timeout_value=float(data[18]),min = float(data[20]),max=float(data[21]),avg=float(data[22]),std=float(data[23])
                             )
    return height_dtat
def generateNewHeightData(height):
    height_data = HeightData(height=height,is_leader=0,proposer=0,is_byz_node=0,is_crash_node=0,round=0,actaul_timeout=0,
                             pdt=0,predict_timeout=0,byz_timeout=0,tps = 0,latency=0,tx_num=0,delay_time= 0,
                             timeout_value=0,min = 0,max=0,avg=0,std=0
                             )
    return height_data

# 产生需要训练模型的height数据
def read_csv_file(filename):
    height_data_list = []
    with open(filename, 'r') as file:
        #创建CSV读取器
        reader = csv.reader(file)
        # 跳过第一行
        next(reader)
        # i =0
        for row in reader:
            height_data = generteheightdata(row)
            height_data_list.append(height_data)
        # 打印结果
    return height_data_list

# 产生需要训练模型的height数据
def read_col_data(filename,index):
    data_list = []
    with open(filename, 'r') as file:
        #创建CSV读取器
        reader = csv.reader(file)
        # 跳过第一行
        next(reader)
        # i =0
        for row in reader:
            data_list.append(float(row[index]))
    return data_list

# def generatedata( high, low, std1, std2, step):
#
#
#     # data1 = np.random.normal(high, std1, step)
#     # data2 = np.random.normal(high, std1, step)
#     # data3 = np.random.normal(high, std1, step)
#     # data4 = np.random.normal(high, std1, step)
#     return data1, data2, data3, data4

'''
学不出来和把错的学出来道理其实是一样的，只要在作恶节点那个位置抬高即可，其他节点低点就低点
效用函数要平滑一点，lable就是节点索引
'''
# 产生BCT波形
def geneateBCT():
    # 设置正态分布的参数
    mean = 500       # 均值
    std_dev = 100  # 标准差
    mean2 = 500      # 均值
    std_dev2 = 100   # 标准差
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

def print_data(data,data2,label,label2,title):
    plt.plot(data,label= label)
    plt.plot(data2,label= label2)
    plt.tick_params(axis='both', labelsize=30)  # x轴和y轴刻度标签字体大小
    # 添加标题和标签
    plt.title(title)
    plt.xlabel('Block Index',fontsize=30)
    plt.ylabel('Time (ms)',fontsize=30)
    plt.grid(True, linestyle='--', linewidth=0.5)
    plt.legend(fontsize=30)
    plt.figure()
    # plt.show()
def read_bct_to(filename):
    bct_list = []
    to_list = []
    pdt_list = []
    with open(filename, 'r') as file:
        #创建CSV读取器
        reader = csv.reader(file)
        # 跳过第一行
        next(reader)
        # i =0
        for row in reader:
            bct_list.append(float(row[6]))
            pdt_list.append(float(row[7]))
            to_list.append(float(row[8]))
        # 打印结果
    return bct_list,to_list,pdt_list



def test():
    crash = False
    bct_list = geneateBCT()
    bct_list = read_col_data('../../../github.com/consensus/tendermint/analyze_icde/mal-para-delay/node0/honest_height_data.csv',6)
    # print("bct_list",bct_list)
    test_bct_list  = []
    bct_list = bct_list[:300]
    # for tt in bct_list:
    #     test_bct_list.append(tt/2)
    # bct_list = test_bct_list
    MI = 1   #每个MI的大小
    next_byz_timeout = 3000.0  #初始化的timeout
    next_honest_timeout = 3000.0
    cold_start_phase = 30  #冷启动阶段的长度
    experiences_window = 300  #经验池的大小
    method = "mal-para"
    para_a = -2
    # 这个是诚实节点设置偏大的TOu

    # 初始化两个模型 产生model 以及 state、action
    model_with_posioned,state_with_posioned,action_with_posioned = generatemodel('byz',version="v3",MI=MI,init_timeout=next_byz_timeout,cold_start_phase=cold_start_phase,experiences_window = experiences_window,method=method,para_a=para_a)

    model_without_posioned,state_without_posioned,action_without_posioned = generatemodel('honest',version="v3",MI=MI,init_timeout=next_honest_timeout,cold_start_phase=cold_start_phase,experiences_window = experiences_window,method=method,para_a=para_a)

    for i in range(len(bct_list)):
        #初始化下一个高度数据
        honest_height_data = generateNewHeightData(i+1)
        byz_height_data = generateNewHeightData(i+1)
        #初始化下一个高度的timeout
        honest_height_data.predict_timeout = next_honest_timeout
        byz_height_data.predict_timeout = next_byz_timeout

        #不作恶的数据是不延误的
        honest_height_data.delay_time = 0.0

        #作恶数据的延误时间
        if (i + 1) % 4 == 0:
            if i < cold_start_phase:
                byz_height_data.delay_time = 2000.0
                if crash:
                    byz_height_data.delay_time = 0

            else:
                delay_time = next_byz_timeout-next_honest_timeout
                # delay_time = 2000.0
                if delay_time < 0.0:
                    delay_time = 0.0
                if crash :
                    delay_time = 0.0
                byz_height_data.delay_time = delay_time

        # 拿取bct数据
        bct_data = bct_list[i]
        byz_bct_data = bct_data + byz_height_data.delay_time

        honest_height_data.actaul_timeout = bct_data
        byz_height_data.actaul_timeout = byz_bct_data

        pdt = bct_data
        byz_pdt = byz_bct_data

        if crash:
            if (i + 1) % 4 == 0:
                pdt = bct_data + next_honest_timeout
                byz_pdt = byz_bct_data + next_honest_timeout

        if pdt >= next_honest_timeout:
            pdt = pdt*2
            honest_height_data.round = 1

        else:
            pdt = pdt

        if byz_pdt >= next_byz_timeout:
            byz_pdt = byz_pdt*10
            byz_height_data.round = 1
        else:
            byz_pdt = byz_pdt

        state_with_posioned,action_with_posioned = model_with_posioned.new_height(bct_data=byz_bct_data,pdt_data=byz_pdt,state=state_with_posioned,action=action_with_posioned,byz_node_flag=False,is_leader_flag=False,last_latency=byz_height_data.latency,min_value=byz_height_data.min,max_value=byz_height_data.max,avg_value=byz_height_data.actaul_timeout,std_value=byz_height_data.std)
        state_without_posioned,action_without_posioned = model_without_posioned.new_height(bct_data=bct_data,pdt_data=pdt,state=state_without_posioned,action=action_without_posioned,byz_node_flag=False,is_leader_flag=False,last_latency=honest_height_data.latency,min_value=honest_height_data.min,max_value=honest_height_data.max,avg_value=honest_height_data.actaul_timeout,std_value=honest_height_data.std)

        next_byz_timeout = model_with_posioned.input_data.Timeout
        next_honest_timeout = model_without_posioned.input_data.Timeout

        honest_height_data.pdt =  pdt
        byz_height_data.pdt = byz_pdt

        model_with_posioned.savedataofheight(byz_height_data)
        model_without_posioned.savedataofheight(honest_height_data)
    # print(timeout_list)

# TODO
if __name__ == '__main__':
   test()
   #
   bct_list,to_list,pdt_list = read_bct_to("honest_height_data.csv")
   byz_bct_list,byz_to_list,byz_pdt_list = read_bct_to("byz_height_data.csv")
   vc = read_col_data('honest_height_data.csv',5)
   tps = read_col_data('honest_height_data.csv',7)
   byz_tps = read_col_data('byz_height_data.csv',7)
   byz_vc = read_col_data('byz_height_data.csv',5)

   byz_tps_list = []
   tps_list = []
   for tt in range(len(tps)):
       byz_tps_list.append(10000/byz_tps[tt])
       tps_list.append(10000/tps[tt])

   chazhi1 = []
   for i in range(len(bct_list)):
       if i+30 >= len(bct_list):
           break
       chazhi1.append(to_list[i+30]-bct_list[i+30])
   avg_Difference = statistics.mean(chazhi1)
   avg_Difference = f"{avg_Difference:.2f}"
   avg_bct =  statistics.mean(pdt_list[30:])
   avg_bct = f"{avg_bct:.2f}"
   vc_num = sum(vc)

   chazhi2 = []
   for i in range(len(byz_bct_list)):
       if i+30 >= len(byz_bct_list):
           break
       chazhi2.append(byz_to_list[i+30]-byz_bct_list[i+30])
   avg_Difference2 = statistics.mean(chazhi2)
   avg_Difference2 = f"{avg_Difference2:.2f}"
   avg_byz_bct =  statistics.mean(byz_pdt_list[30:])
   avg_byz_bct = f"{avg_byz_bct:.2f}"
   byz_vc_num = sum(byz_vc)

   delay_time = read_col_data('byz_height_data.csv',19)
   chazhi3 = []
   for i in range(len(delay_time)):
       if i+30 >= len(delay_time):
           break
       chazhi3.append(delay_time[i+3])
   avg_Difference3 = statistics.mean(chazhi3)
   avg_Difference3 = f"{avg_Difference3:.2f}"

   print("hoest TO-BCT",avg_Difference,"byz TO-BCT",avg_Difference2,"delay_time",avg_Difference)

   print_data(byz_to_list,to_list,"Byz_TO","TO","Delay_time="+str(avg_Difference3))
   print_data(bct_list,to_list,"BCT","TO","to-bct="+str(avg_Difference)+"ms  "+"avg_pdt="+str(avg_bct)+"ms  "+"vc_num="+str(vc_num))
   print_data(byz_bct_list,byz_to_list,"Byz_BCT","Byz_TO","Byz_to-Byz_bct="+str(avg_Difference2)+"ms  "+"avg_Byz_pdt="+str(avg_byz_bct)+"ms  "+"Byz_vc_num="+str(byz_vc_num))
   print_data(byz_tps_list[30:],tps_list[30:],'Byz_TPS','TPS',"TPS")


   plt.show()