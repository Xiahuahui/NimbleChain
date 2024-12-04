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
def rtt(sample, RCPT, RCPTVAR, alpha, beta,K):  # rtt是根据五个变量预测下一个时间的值
    RCPT = (1 - alpha) * RCPT + alpha * float(sample)  # 低通滤波计算下一个RCPT
    timeVar = float(sample) - RCPT
    RCPTVAR = (1 - beta) * RCPTVAR + beta * abs(timeVar)
    timeout = RCPT + K * RCPTVAR
    # print("data",sample,"timeout",timeout,"RCPT",RCPT,"RCPTVAR",RCPTVAR,"timeVar",abs(timeVar))
    return RCPT, RCPTVAR, timeout

# 这个文件记录的每个区块对应的timeout的变化，类似于后边更新后的input_data
def save_data(data1,data5,data2,data3,data4):
    global head
    data_store = open('data.csv', 'a')   #清理每个区块的信息。
    csv_writer = csv.writer(data_store)
    if head == 0:
        head = head + 1
        csv_writer.writerow(['bct','byz-bct','timeout', 'rcpt', 'rcpt_var'])

    for i in range(len(data1)):
        sava_d = []
        sava_d.append(str(data1[i]))
        sava_d.append(str(data5[i]))
        sava_d.append(str(data2[i]))
        sava_d.append(str(data3[i]))
        sava_d.append(str(data4[i]))
        csv_writer.writerow(sava_d)
    data_store.flush()

# 从文件中读取数据，读取的是bct、byz-bct以及timeout那三列
def read_data():
    bct_list = []
    timeout_list = []
    byz_bct = []
    with open('data.csv', 'r') as file:
        #创建CSV读取器
        reader = csv.reader(file)
        # 跳过第一行
        next(reader)
        # 读取特定列的数据
        for row in reader:
            bct_list.append(float(row[0]))
            byz_bct.append(float(row[1]))
            timeout_list.append(float(row[2]))
    return bct_list,byz_bct,timeout_list

# 激励函数的作用
# 这个epoch的含义是错过多少个数据不参与计算reward
# data1表示的真实的bct
# data2表示的预测值timeout
# 其实我们想要的激励函数是TPs的大小
# TODO 解释一下这个函数 这个函数为什么要这么实现,这个要仔细检查一下是个错的目前
# data1 = bct data2 = timeout
def reward(epoch, data1, data2):
    reward = 0
    view_change = 0
    threat_space = 0
    for k in range(len(data2)):
        if k < epoch:
            continue
        bct = data1[k]
        to = data2[k]
        if bct >= to:
            temp = to
            reward = reward + temp
            view_change = view_change + temp
            while temp < bct:
                temp = temp * 2
                reward = reward + temp
                view_change = view_change + temp
        reward = reward + data1[k]
    return 1/reward, view_change, threat_space

# 产生不同的网络波形的脚本
def generatedata( high, low, std1, std2, step):
    # 第一阶段，从low经过步数step提升到high
    step1 = random.randint(1,step)  #TODO 产生不均匀的波形
    # step1 = step
    data1 = np.linspace(low, high, step1)
    data1 = []

    # 第二阶段，在high经过步数step在均值为high方差为std1上正态分布
    step2 = random.randint(1,step)
    # step2 = step
    data2 = np.random.normal(high, std1, step2)

    # 第三阶段，从high经过步数step降低到low
    step3 = random.randint(1,step)
    # step3 = step
    data3 = np.linspace(high, low, step3)
    data3 = []

    # 第四阶段，在low经过步数step在均值为low方差为std2上正态分布
    step4 = random.randint(1,step)
    # step4 = step
    data4 = np.random.normal(low, std2, step4)

    # data1 = np.random.normal(high, std1, step)
    # data2 = np.random.normal(high, std1, step)
    # data3 = np.random.normal(high, std1, step)
    # data4 = np.random.normal(high, std1, step)
    return data1, data2, data3, data4

# 遍历每一个组合得出最优的alpha和beta 所谓的最优是说根据效用值计算出来的最优(遍历出最优的组合)
def test_best_rtt(data,timeout0,RCPT,RCPTVAR):
    # TODO 下面的变量是说明当前区块的索引值
    global best_block_index,best_bct_list #在最优的alpha和beta组合下，真实的bct的组合
    logging.info("timeout0: %s",timeout0)  #开始的timeout
    rcpt = RCPT    # 当前开始的rcpt
    rcptvar = RCPTVAR   # 当前开始的 rcpt_var
    timeout = timeout0  # 第一个block对应的timeout

    # 下面的这几个辅助量是用来返回最优值。
    best_alpha=0
    best_beta=0
    best_k = 0
    best_rcpt = 0
    best_rcpt_var = 0
    best_timeout = 0
    best_byz_bct = []
    best_timeout_list =[]

    trace_r = 0

    my_dict = {}  #TODO 这个字典的用处在哪里
    alpha = 0
    while alpha < 0.99:
        beta = 0
        alpha+=0.1
        # alpha = 0.125
        while beta < 0.99:
            beta+=0.1
            k_list = [3,4,5,6,7,8,9]
            for k in k_list :
                trace_predict = []
                trace_predict.append(timeout0)
                alpha_new = alpha
                beta_new  = beta
                if "alpha"+str(alpha_new)+"beta_new"+str(beta_new)+"k"+str(k)  in my_dict:
                    continue
                else:
                    my_dict["alpha"+str(alpha_new)+"beta_new"+str(beta_new)+"k"+str(k)] = ""
                # print("#############################alpha",alpha_new,"beta",beta_new,"k",k,"####################################")
                byz_bct = []
                for j in range (len(data)):
                    bct = float(data[j])
                    block_index = best_block_index + j
                    if block_index % leader == 0:
                        bct = trace_predict[-1]
                    # 这是因为要遍历多次 所以rcpt,rcpt_var都会变
                    byz_bct.append(bct)
                    if j == 0 :
                        rcpt,rcptvar,timeout = rtt(float(bct),RCPT,RCPTVAR,alpha_new,beta_new,k)
                    else:
                        rcpt,rcptvar,timeout = rtt(float(bct),rcpt,rcptvar,alpha_new,beta_new,k)
                    trace_predict.append(timeout)
                # logging.info("byz_bct: %s,timeout: %s",byz_bct,trace_predict)
                r,r1,r2=reward(0,byz_bct,trace_predict[:-1])
                if r > trace_r:
                    trace_r = r
                    best_alpha = alpha_new
                    best_beta = beta_new
                    best_k = k
                    best_rcpt = rcpt
                    best_rcpt_var = rcptvar
                    best_timeout = timeout
                    best_byz_bct = byz_bct
                    best_timeout_list = trace_predict
    best_bct_list.extend(copy.deepcopy(best_byz_bct))
    logging.info("best_alpha: %s,best_beta: %s,best_k: %s ,best_timeout: %s",best_alpha,best_beta,best_k,best_timeout_list[:-1])
    best_block_index = best_block_index + len(data)
    return best_alpha,best_beta,best_k,best_rcpt,best_rcpt_var,best_timeout,best_timeout_list[:-1],trace_r

# 这个是将最优的timeout,MI的大小
def find_best(data_list,MI):
    best_rcpt= 0.0
    best_rcpt_list  = []
    best_rcpt_list.append(best_rcpt)

    best_rcpt_var = 0.0
    best_rcpt_var_list = []
    best_rcpt_var_list.append(best_rcpt_var)

    best_timeout = 3000.0
    best_alpha_list = []
    best_bate_list = []

    best_alpha_list2 = []
    best_beta_list2 = []
    best_k_list2 = []

    timeout_list_3 = []
    reward_list =[]
    i = 0
    while True:
        logging.info("the Mi is: %s",i)
        start_index = i*MI
        end_index = (i+1)*MI
        if end_index > len(data_list):
            break
        b_s_l = data_list[start_index:end_index]
        logging.info("the bct time list: %s",b_s_l)
        best_alpha,best_beta,best_k,best_rcpt,best_rcpt_var,best_timeout,best_timeout_list,best_reward = test_best_rtt(b_s_l,best_timeout,best_rcpt,best_rcpt_var)

        best_alpha_list2.append(best_alpha)
        best_beta_list2.append(best_beta)
        best_k_list2.append(best_k)
        timeout_list_3.extend(best_timeout_list)
        reward_list.append(best_reward)

        i = i+1
    logging.info("the best reward_list's length is: %s",len(reward_list))
    return timeout_list_3,sum(reward_list),reward_list,[best_alpha_list2,best_beta_list2,best_k_list2]

# 这个代码是预测
# 不管用不用low还是high
# 第一次都会是没有训练就预测一次
# 我们当前的做法是删除第一次MI的经验值
# TODO MI上的第一个timeout究竟是取哪个alpha和beta比较好

def run_agent(bct_data,episodes_len,num_episodes,experiences_window=30):
    global default_alpha, default_beta,default_K
    """ Init 经验池 """
    experiences_X = []
    experiences_y = []
    # 第一次的action选择
    initial_alpha = default_alpha
    initial_beta = default_beta
    initial_K  = default_K

    # 下面这三个数其实是初始值
    last_rcpt = 0.0
    last_rcpt_var = 0.0
    last_timeout = 3000.0

    # 下面这个是默认的空间
    best_alpha = initial_alpha
    best_beta = initial_beta
    best_K = initial_K

    current_bct_data = None
    byz_bct_data = None
    timeout_list = None

    avg_rcpt = None
    avg_rcpt_var = None
    avg_bct  = None
    std_bct =None
    label = None

    last_utility_value = None
    reward_list =[]


    # 记录一些辅助信息
    data_store = open('experiences.csv', 'w')
    csv_writer = csv.writer(data_store)
    csv_writer.writerow(
        ['avg_rcpt', 'avg_rcpt_var', 'avg_bct', 'std_bct','label', 'alpha', 'beta',"K",
         'utility_value', 'feature_extraction_overhead (s)', 'training_overhead (s)', 'inference_overhead (s)',
         'compute_utility_overhead (s)', 'episode_duration (s)'])


    # set the enumeration matrix as input to the predictor
    rf = RandomForestRegressor(max_depth=5)

    alpha_list = [i/10 for i in range(1, 10)]
    beta_list = [i/10 for i in range(1, 10)]
    k_list = [3,4,5,6,7,8,9]

    actions = []
    for dim_1 in alpha_list:
        for dim_2 in beta_list:
            for dim_3 in k_list:
                actions.append(np.array([dim_1, dim_2, dim_3]))
    actions_matrix = np.vstack(actions)
    enumeration_matrix = np.hstack((np.zeros((actions_matrix.shape[0], 5)), actions_matrix))
    logging.info('learning agent has been initialized.')
    logging.info("the enumeration_matrix is: %s",enumeration_matrix.shape[1])


    time_records = []  #这个是记录了几个时间值
    episode_start_time = None
    feature_extraction_overhead = 0  #特征提取的代价
    training_overhead = 0            #训练的代价
    inference_overhead = 0           #预测推断的代价

    block_index = 0
    for episode in range(num_episodes):
        # TODO
        # wait for the peer to reach lower watermark in the current episode
        # with episode_cv:
        #     episode_cv.wait()
        # 这个其实是

        episode_end_time = time.time()
        logging.info("episode: %s",episode)
        logging.info("the best para is: alpha: %s, beta: %s k: %s",best_alpha,best_beta,best_K)
        if episode_start_time is not None:
            time_records[-1].append(round(episode_end_time - episode_start_time, 6))  #TODO 这个是哪个阶段的耗时

        """ Extract feature for each MI """
        feature_extraction_start = time.time()
        start_index = (episode) *episodes_len
        end_index = (episode+1) * episodes_len
        if end_index > len(bct_data):
            break
        current_bct_data = bct_data[start_index:end_index]
        logging.info("current_bct_data: %s",current_bct_data)
        rcpt_list = []
        rcpt_var_list = []
        timeout_list = []
        rcpt_list.append(last_rcpt)
        rcpt_var_list.append(last_rcpt_var)
        timeout_list.append(last_timeout)
        byz_bct_data = []

        for k in range (len(current_bct_data)):
            #每个MI的第一次计算timeout时，需要用到上个MI的last_rcpt,last_rcpt_var
            if k == 0 :
                rcpt = last_rcpt
                rcpt_var = last_rcpt_var
            bct = current_bct_data[k]
            if block_index % leader == 0:
                bct = timeout_list[-1]
            byz_bct_data.append(bct)
            rcpt,rcpt_var,timeout = rtt(bct,rcpt,rcpt_var,best_alpha,best_beta,best_K)
            if (k == (len(current_bct_data) - 1)):
                last_rcpt = rcpt
                last_rcpt_var = rcpt_var
                last_timeout = timeout
            else:
                timeout_list.append(timeout)
                rcpt_list.append(rcpt)
                rcpt_var_list.append(rcpt_var)
            block_index = block_index+1
        assert (len(current_bct_data) == len(timeout_list))
        assert (len(current_bct_data) == len(rcpt_list))
        assert (len(current_bct_data) == len(rcpt_var_list))
        save_data(current_bct_data,byz_bct_data,timeout_list,rcpt_list,rcpt_var_list)
        # compute the avg_rcpt
        avg_rcpt = statistics.mean(rcpt_list)
        # compute the avg_rcpt_var
        avg_rcpt_var = statistics.mean(rcpt_var_list)
        # compute the avg_bct
        avg_bct = statistics.mean(current_bct_data)
        # compute the std_bct
        std_bct = statistics.stdev(current_bct_data)
        # compute lable
        label = 0

        feature_extraction_overhead = round(time.time() - feature_extraction_start, 6)

        """ Compute the utility_value """
        communication_start = time.time()   # 这其实就是一个grpc的传的消息

        last_utility_value,last_vc_utility_value, last_threat_space_utility_value =  reward(0,byz_bct_data,timeout_list)
        reward_list.append(last_utility_value)
        # 从MI == 1时才是经验
        if episode:
            experiences_y.append(last_utility_value)
            logging.info("the current utility value is: %s",last_utility_value)

        # time.sleep(1)
        communication_overhead = round(time.time() - communication_start, 6)

        logging.info('avg_rcpt = %s, avg_rcpt_var = %s, avg_bct = %s, std_bct = %s, best_alpha = %s, best_beta = %s,utility_value = %s',
                     avg_rcpt,avg_rcpt_var, avg_bct ,std_bct ,best_alpha,best_beta,last_utility_value)


        """ Retrain """
        assert (len(experiences_X) == len(experiences_y))
        while len(experiences_X) > experiences_window:
            experiences_X.pop(0)
            experiences_y.pop(0)
        if episode:
            logging.info("the exeperiences_x's length is: %s,the exeperiences_y's length is: %s",len(experiences_X),len(experiences_y))
            training_start = time.time()
            feature_idx = np.array([True, True, True, True, True, True, True,True])
            if len(experiences_X) > 10:
                bootstrapped_idx = np.random.choice(len(experiences_X), len(experiences_y), replace=True)
                training_X = np.vstack(experiences_X)[bootstrapped_idx, :][:, feature_idx]
                training_y = np.array(experiences_y)[bootstrapped_idx]
            else:
                training_X = np.vstack(experiences_X)[:, feature_idx]
                training_y = np.array(experiences_y)
            rf.fit(training_X, training_y)
            training_overhead = round(time.time() - training_start, 6)

            # save the latest experience to csv file
            logging.info("the new experienceis:%s",experiences_X[-1].tolist() + [experiences_y[-1]])
            csv_writer.writerow(experiences_X[-1].tolist() + [experiences_y[-1]] + time_records[-1])
            data_store.flush()
        else:
            training_overhead = 0


        """ Select the best action according to the predictor (M_theta) """
        if len(experiences_X) > 0:
            inference_start = time.time()
            enumeration_matrix[:, 0:5] = np.array([avg_rcpt, avg_rcpt_var, avg_bct, std_bct,label])
            feature_idx = np.array([True, True, True, True, True, True, True, True])
            logging.info("the predict's x: %s ",enumeration_matrix[0, feature_idx])
            prediction = rf.predict(enumeration_matrix[:, feature_idx])
            # logging.info("the prediction is: %s",prediction)
            # best_index = np.argmax(prediction)
            while True:
                # 这tampson采样的部分(可能不是 只是选择最佳的策略)
                best_index = np.random.choice(np.flatnonzero(np.isclose(prediction, prediction.max())), replace=True)
                best_alpha = enumeration_matrix[best_index, 5]
                best_beta = enumeration_matrix[best_index, 6]
                best_K = enumeration_matrix[best_index, 7]
                break
            experiences_X.append(copy.deepcopy(enumeration_matrix[best_index, :]))
            # experiences_X.append(enumeration_matrix[best_index, :])
            inference_overhead = round(time.time() - inference_start, 6)

        else:
            best_alpha = initial_alpha
            best_beta = initial_beta
            best_K = initial_K
            # 第0次没有状态 所以删掉改经验值

            experiences_X.append(np.array([avg_rcpt, avg_rcpt_var, avg_bct, std_bct,label,
                                           best_alpha, best_beta, best_K]))
            inference_overhead = 0

        # notify the peer about the action
        episode_start_time = time.time()
        time_records.append([feature_extraction_overhead, training_overhead, inference_overhead, communication_overhead])
    logging.info("the learning reward_list's length is: %s",len(reward_list))
    return sum(reward_list),reward_list

# 这个是用来画几张图的脚本 主要还是bct和timeout
def print_data(data1,data2,data3,data4,data5,data6,data7):
    print(data1)
    print(data2)
    print(data3)
    print(data4)
    # plt.plot(data1,label= "bct")

    #使用随机森林学习出来的alpha和beta
    plt.plot(data2,label= "timeout-learning")
    plt.plot(data5,label= "learning-bct")
    plt.legend()
    plt.figure()

    plt.plot(data3,label= "timeout-fixed")
    plt.plot(data7,label= "fixed-bct")
    plt.legend()
    plt.figure()

    plt.plot(data4,label= "timeout-best")
    plt.plot(data6,label= "best-bct")
    plt.legend()
    plt.figure()

def print_tps(tps1,tps2):


    plt.plot(tps1,label= "best_tps")
    plt.plot(tps2,label= "learning-tps")
    plt.legend()

    plt.show()

#下面这两个函数是为了复现使用的
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

#这个是验证函数，主要是验证每个方法是否是正确的。
def verify(data,MI,para_list_1,para_list_2,para_list_3,MI_len,timeout_test):
    rcpt= 0.0
    rcpt_var = 0.0
    timeout = 3000.0
    test_timeout_list = []
    test_timeout_list.append(timeout)
    i = 0
    while True:
        logging.info("the test Mi is: %s",i)
        start_index = i*MI
        end_index = (i+1)*MI
        if end_index > len(data):
            break
        if i >= MI_len:
            break
        b_s_l = data[start_index:end_index]
        alpha = para_list_1[i]
        beta = para_list_2[i]
        k = para_list_3[i]
        for d in b_s_l:
            rcpt, rcpt_var, timeout = rtt(d,rcpt,rcpt_var,alpha,beta,k)
            test_timeout_list.append(timeout)
        i = i+1
    test_timeout_list = test_timeout_list[:-1]
    logging.info("new timeout_list:%s",test_timeout_list)
    logging.info("test timeout_list:%s",timeout_test)
    # time.sleep(10)

def adjust(fixed_data,leader_partion,flag,init_timeout,index):
    byz_bct_list = []
    tps = []
    vc_num = 0
    fixed_timeout_list = []
    timeout = init_timeout
    fixed_timeout_list.append(timeout)
    RCPT = 0.0
    RCPTVAR = 0.0
    alpha = 0.125
    beta = 0.25
    k = 4
    fixed_index = 0
    fixed_bct = []
    for bct in fixed_data:
        if fixed_index % leader_partion == index:
            if flag == 'Fixed':
                bct = bct+fixed_timeout_list[-1]
            else:
                bct = fixed_timeout_list[-1]
        pdt = bct
        test_bct = bct
        test_to = fixed_timeout_list[-1]
        if test_bct > test_to:
            while test_bct > test_to:
                vc_num = vc_num + 1
                pdt = pdt + test_to
                test_to = test_to *2
        tps.append(10000/pdt*300)
        byz_bct_list.append(pdt)
        fixed_bct.append(bct)
        # logging.info("block_index:%s, bct:%s, timeout:%s",fixed_index,bct,timeout)
        RCPT,RCPTVAR,timeout = rtt(bct,RCPT,RCPTVAR,alpha,beta,k)
        if flag == 'Fixed':
            timeout = init_timeout
        fixed_timeout_list.append(timeout)
        fixed_index = fixed_index + 1
    fixed_timeout_list = fixed_timeout_list[:-1]
    # fixed_timeout_list= fixed_timeout_list[50:150]
    return fixed_timeout_list,tps,vc_num,byz_bct_list

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
            data_list.append(30*float(row[index]))
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




    data = read_col_data('../analyze_icde/output/column_0.csv',1)
    # logging.info("the bct_list'lenght is: %s,the bct_list is: %s",len(data),data)
    #1050 ,3 11
    fixed_data = data[70:91]
    fixed_data2=[]
    logging.info("the bct_list'lenght is: %s,the bct_list is: %s",len(fixed_data),fixed_data)
    i = 0
    for d in fixed_data:
        if i == 3:
            fixed_data2.append(1050.2)
        elif i == 11:
            fixed_data2.append(1060.2)
        else:
            fixed_data2.append(d)

        logging.info("the index: %s,data= %s",i,d)
        i = i + 1
    timeout_list1 =[]
    timeout_list2 = []
    RCPT1 = 1050
    RCPTVAR1 = 10.0
    k =4

    RCPT2 = 1050
    RCPTVAR2 = 10.0
    k =4

    timeout1 = 1050
    timeout2 = 1050

    timeout_list1.append(timeout1)
    timeout_list2.append(timeout2)

    for ii in range(len(fixed_data)):
        if ii==4 or i==5 or ii==12 or i==13:
            RCPT1,RCPTVAR1,timeout1 = rtt(fixed_data[ii],RCPT1,RCPTVAR1,0.9,0.25,4)
        else:
            RCPT1,RCPTVAR1,timeout1 = rtt(fixed_data[ii],RCPT1,RCPTVAR1,0.125,0.25,4)
        RCPT2,RCPTVAR2,timeout2 = rtt(fixed_data[ii],RCPT2,RCPTVAR2,0.125,0.25,4)
        if ii==3:
            timeout2 = timeout2 - 10
        timeout_list1.append(timeout1)
        timeout_list2.append(timeout2)
    timeout_list1 = timeout_list1[:-1]
    timeout_list2 = timeout_list2[:-1]
    print(sum(timeout_list1),sum(timeout_list2))

    fixed_data_new =[]
    for d in fixed_data:
        fixed_data_new.append(d+1)
    # tps_list = []
    # tps_list2 = []
    # x_list = []
    # for to in range(200,3200,200):
    #     x_list.append(to)
    #     timeout3,tps1,vc_num1,byz_bct_list = adjust(fixed_data,88888,"Fixed",to,8888)
    #     timeout44,tps2,vc_num2,byz_bct_list2 = adjust(fixed_data,4,"Fixed",to,0)
    #     logging.info("byz_bct_list: %s",byz_bct_list)
    #     logging.info("byz_bct_list: %s",byz_bct_list2)
    #     tps_list.append(10000/(sum(byz_bct_list)/len(byz_bct_list))*300)
    #     tps_list2.append(10000/(sum(byz_bct_list2)/len(byz_bct_list2))*300)
    # print("byz_bct_list: ",byz_bct_list)
    # print("byz_bct_list: ",byz_bct_list2)

    # fig, (ax1, ax2) = plt.subplots(, 1)
    fig, ax = plt.subplots(figsize=(16, 9))
    # ax.spines['top'].set_linewidth(2)    # 上边框加宽
    # ax.spines['right'].set_linewidth(2)  # 右边框加宽RUC&info500
    # ax.spines['left'].set_linewidth(2)   # 左边框加宽
    # ax.spines['bottom'].set_linewidth(2)  # 下边框加宽
    #
    # # ax1.plot(x_list,tps_list,label='Normal',color='r',marker='^',markersize=10,linewidth=5)
    # # ax.plot(x_list,tps_list2,label='25%Crash',color='#00008B',marker='s',markersize=15,linewidth=5)
    ax.plot(fixed_data_new,color='blue',linewidth=5,label='$PDT$ (Greedy)')
    ax.plot(fixed_data2,color='pink',linewidth=5,label='$PDT$ (Bilevel)')
    ax.plot(timeout_list1,color='#00008B',marker='v',markersize=20,linewidth=5,label='$TO$ (Greedy)')
    ax.plot(timeout_list2,color='#FF1493',marker='s',markersize=20,linewidth=5,label='$TO$ (Bilevel)')
    # ax.axvline(x=3, linestyle='--', color='red',ymax=fixed_data[3],linewidth=5)
    ax.plot([3, 3], [1000, fixed_data[3]], linestyle='--', color='darkred', linewidth=5)
    # ax.axvline(x=11, linestyle='--', color='red',ymax=1030,linewidth=5)
    ax.plot([11, 11], [1000, fixed_data[11]], linestyle='--', color='darkred', linewidth=5)
    ax.annotate('', xy=(3, 1010), xytext=(11, 1010), arrowprops=dict(arrowstyle='<->', color='darkred',linewidth=5))
    ax.annotate('timeout attack', xy=(7, 1012), xytext=(7, 1012), fontsize=35, color='darkred', ha='center')
    # plt.annotate('timeout attack', xy=(7, 1015), xytext=(4, 1030), arrowprops=dict(arrowstyle='->', connectionstyle="arc3,rad=0.5", color='black',linewidth=5),  weight='bold',fontsize=20,color='red')
    ax.set_ylabel('Threshold (ms)',fontsize=35)
    ax.set_ylim(1000, 1190)
    ax.set_yticks(np.arange(1000, 1190, 30))  # 每隔 0.3 设置一个刻度

    ax.tick_params(axis='y',labelsize=35,width=2)
    ax.tick_params(axis='x',labelsize=35,width=2)
    # for label in ax1.get_xticklabels():  # 对于 x 轴的刻度标签
    #     label.set_fontweight('bold')     # 设置字体加粗
    # for label in ax1.get_yticklabels():  # 对于 y 轴的刻度标签
    #     label.set_fontweight('bold')     # 设置字体加粗
    ax.set_xlabel('Block Height',fontsize=35)
    # ax.set_xticks(np.arange(200, 3200, 400))
    ax.spines['top'].set_linewidth(2)    # 上边框加宽
    ax.spines['right'].set_linewidth(2)  # 右边框加宽
    ax.spines['left'].set_linewidth(2)   # 左边框加宽
    ax.spines['bottom'].set_linewidth(2)  # 下边框加宽
    ax.legend(prop={'size': 33, 'weight': 'bold'},loc='upper left')

    # ax2.plot(fixed_data,color='green',linewidth=5)
    # ax2.set_ylabel('PDT(ms)',fontsize=30)
    # ax2.tick_params(axis='y', labelsize=30,width=2)
    # ax2.tick_params(axis='x',labelsize=30,width=2)
    # # ax2.set_ylim(0, 150)
    # # ax2.set_yticks(np.arange(0, 160, 30))  # 每隔 0.06 设置一个刻度
    # # for label in ax2.get_xticklabels():  # 对于 x 轴的刻度标签
    # #     label.set_fontweight('bold')     # 设置字体加粗
    # # for label in ax2.get_yticklabels():  # 对于 y 轴的刻度标签
    # #     label.set_fontweight('bold')     # 设置字体加粗
    # ax2.set_xlabel('Block Index',fontsize=30)
    # ax2.set_xticks(np.arange(0, 180,30))
    # ax2.spines['top'].set_linewidth(2)    # 上边框加宽
    # ax2.spines['right'].set_linewidth(2)  # 右边框加宽
    # ax2.spines['left'].set_linewidth(2)   # 左边框加宽
    # ax2.spines['bottom'].set_linewidth(2)  # 下边框加宽
    #
    # # 保持下子图的 x 轴刻度
    # ax1.xaxis.set_ticks_position('bottom')  # 确保下子图的 x 轴刻度在底部
    # # 调整子图之间的间距
    # plt.subplots_adjust(hspace=0.4)  # 调整上下子图之间的高度间距
    # plt.figure(figsize=(16, 9))  # 设置图形大小为 12x9，即 4:3 的比例

    # 显示图形
    plt.tight_layout()
    fig.subplots_adjust(bottom=0.068, top=0.88, right=0.9)

    # 保存图表为文件
    plt.savefig('fig_speical_pdt_wave.pdf', dpi=300, bbox_inches='tight')


    plt.show()
