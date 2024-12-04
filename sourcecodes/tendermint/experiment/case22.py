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

leader = 4
def rtt(sample, RCPT,alpha):  # rtt是根据五个变量预测下一个时间的值
    RCPT = (1 - alpha) * RCPT + alpha * float(sample)  # 低通滤波计算下一个RCPT
    timeout = RCPT
    return RCPT, timeout

# TODO 解释一下这个函数 这个函数为什么要这么实现,这个要仔细检查一下是个错的目前
def reward(bct_data, timeout_data):
    logging.info("len(bct_data): %s","len(timeout): ",len(bct_data),len(timeout_data))
    assert (len(bct_data) == len(timeout_data)==1)
    reward_list = []
    view_change_list = []
    for k in range(len(timeout_data)):
        reward = 0
        view_change = 0
        bct = bct_data[k]
        to = timeout_data[k]
        if bct > to:
            temp = to
            while temp < bct:
                reward = reward + temp
                view_change = view_change + temp
                temp = temp * 2
            reward = reward + bct
        else:
            reward = reward + timeout_data[k]
        reward_list.append(1/reward)
        if view_change == 0:
            view_change_list.append(0)
        else:
            view_change_list.append(1/view_change)
    r_v =  0
    for r in reward_list:
        r_v = r_v + 1 / r
    r_v = 10000/r_v*300
    return r_v,reward_list,view_change_list

# 产生不同的网络波形的脚本
def generatedata( high, low, std1, std2, step):
    # 第一阶段，从low经过步数step提升到high
    step1 = random.randint(1,step)  #TODO 产生不均匀的波形
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
    return [], data2, [], data4

# 遍历每一个组合得出最优的alpha和beta 所谓的最优是说根据效用值计算出来的最优(遍历出最优的组合)
# TODO 其实不需要timeout的，MI=1,其实就能解决
def test_best_rtt(data,bct_data,RCPT,RCPTVAR):
    # TODO 下面的变量是说明当前区块的索引值
    # global best_block_index,best_bct_list #在最优的alpha和beta组合下，真实的bct的组合

    rcpt = RCPT    # 当前开始的rcpt
    rcptvar = RCPTVAR   # 当前开始的 rcpt_var
    logging.info("erer")


    # 下面的这几个辅助量是用来返回最优值。
    best_alpha=0
    best_beta=0
    best_k = 0
    best_rcpt = 0
    best_rcpt_var = 0
    best_byz_bct = []
    best_timeout_list =[]
    best_reward_list = []

    trace_r = 0

    my_dict = {}  #TODO 这个字典的用处在哪里
    alpha = 0
    alpha_list = [0.1,0.5,0.9]
    beta_list = [0.1,0.5,0.9]
    for kkk in range(len(alpha_list)):
        alpha = alpha_list[kkk]
        # beta = 0
        # alpha+=0.1
        # # alpha = 0.125
        for t in range(1):
            beta = beta_list[kkk]
            k_list = [4]
            for k in k_list :
                trace_predict = []
                # trace_predict.append(timeout0)
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
                    # block_index = best_block_index + j
                    # if block_index % leader == 0:
                    #     bct = trace_predict[-1]
                    # 这是因为要遍历多次 所以rcpt,rcpt_var都会变
                    byz_bct.append(bct)
                    if j == 0 :
                        rcpt,rcptvar,timeout = rtt(float(bct),RCPT,RCPTVAR,alpha_new,beta_new,k)
                    else:
                        rcpt,rcptvar,timeout = rtt(float(bct),rcpt,rcptvar,alpha_new,beta_new,k)
                    trace_predict.append(timeout)
                # logging.info("byz_bct: %s,timeout: %s",byz_bct,trace_predict)
                byz_bct.append(bct_data)
                byz_bct = byz_bct[1:]
                assert (len(trace_predict) == 1)
                # if data[0] ==680.0 and bct_data == 780.0:
                #     logging.info("the timeout is: %s,the rcptvar is %s,the rcpt is %s,the alpha is %s,the beta is %s, the k is %s ",trace_predict,RCPTVAR,RCPT,alpha,beta,k)
                r_v,r_list,v_r_list=reward(byz_bct,trace_predict)
                # logging.info("timeout is: %s the alpha is: %s, the beta is: %s the best k is: %s,the reward is: %s",timeout,alpha_new,beta_new,k,r_v)
                if r_v > trace_r:
                    trace_r = r_v
                    best_alpha = alpha_new
                    best_beta = beta_new
                    best_k = k
                    best_rcpt = rcpt
                    best_rcpt_var = rcptvar
                    best_byz_bct = byz_bct

                    best_timeout_list = trace_predict
                    # if data[0] == 680.0 and bct_data == 780.0:
                    #     logging.info("the bct_data1 is: %s,the bct_data2 is: %sthe best timeout is: %s",data,byz_bct,best_timeout_list)
                    best_reward_list = r_list
    best_bct_list.extend(copy.deepcopy(best_byz_bct))
    # best_block_index = best_block_index + len(data)
    # if best_timeout_list[0] < bct_data:
    #     logging.info("the timeout is: %s,the bct_data is: %s,the rcptvar is %s,the rcpt is %s,the alpha is %s,the beta is %s, the k is %s ",best_timeout_list,bct_data,RCPTVAR,RCPT,best_alpha,best_beta,best_k)
    return best_alpha,best_beta,best_k,best_rcpt,best_rcpt_var,best_timeout_list,trace_r,best_reward_list

#这个好像也不是一个最优的参数组合
def find_best(data_list,MI,init_length):
    #这个是个冷启动的阶段
    RCPT = 0.0
    RCPTVAR = 0.0
    alpha = 0.1
    beta = 0.1
    k = 4
    for index in range(init_length-1):
        bct = data_list[index]
        RCPT,timeout = rtt(bct,alpha,k)

    # 说明一下下面的原因
    best_rcpt= RCPT
    best_rcpt_list  = []
    best_rcpt_list.append(best_rcpt)

    best_rcpt_var = RCPTVAR
    best_rcpt_var_list = []
    best_rcpt_var_list.append(best_rcpt_var)

    best_timeout = 3000.0
    reward_list_MI = []
    timeout_list = [best_timeout]*init_length

    best_alpha_list = [0.1]*init_length
    best_beta_list = [0.1]*init_length
    best_k_list = [4]*init_length

    reward_list = []
    r_v,r_list,v_r_list = reward(data_list[0:init_length],timeout_list[0:init_length])
    reward_list.extend(r_list)
    reward_list_MI.append(r_v)
    # 这有个冷启动的时间
    i = 0
    while True:
        logging.info("the Mi is: %s",i)
        start_index = i*MI+init_length-1   #这应该修改一下
        end_index = (i+1)*MI+init_length-1
        if end_index >= len(data_list):
            break
        b_s_l = data_list[start_index:end_index]
        logging.info("the bct time list: %s",b_s_l)
        # 是使用第i个bct计算第i+1个timeout
        best_alpha,best_beta,best_k,best_rcpt,best_rcpt_var,best_timeout_list,best_reward,best_reward_list = test_best_rtt(b_s_l,data_list[end_index],best_rcpt,best_rcpt_var)
        best_alpha_list.append(best_alpha)
        best_beta_list.append(best_beta)
        best_k_list.append(best_k)
        timeout_list.extend(best_timeout_list)
        reward_list.append(best_reward)
        reward_list_MI.append(best_reward)
        i = i+1
    logging.info("the data_list's len is:%s ,the timeout_list's len is: %s the para's len is: %s",len(data_list),len(timeout_list),[len(best_alpha_list),len(best_beta_list),len(best_k_list)])
    assert (len(data_list)==len(timeout_list))
    logging.info("the best reward_list's length is: %s",len(reward_list))
    return timeout_list,reward_list,reward_list_MI,[best_alpha_list,best_beta_list,best_k_list]



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



# 固定参数时的reward的计算
def fixed_para_method(data,alpha,beta,k,init_lenght):
    timeout_list = []  #返回的timeout序列
    reward_list  = []  #返回的reward序列
    RCPT = 0.0
    RCPTVAR = 0.0
    para_list = []
    timeout = 3000.0
    timeout_list.append(timeout)
    para_list.append([alpha,beta,k])
    i = 0
    for bct in data:
        r,r_list,v_r= reward([bct],[timeout])
        reward_list.extend(r_list)
        RCPT,timeout = rtt(bct,RCPT,alpha)
        if i < init_lenght-1:  #这个是个冷启动时期
            timeout = 3000.0
        timeout_list.append(timeout)
        para_list.append([alpha,beta,k])
        i = i + 1
    timeout_list = timeout_list[:-1]
    para_list = para_list[:-1]
    assert (len(reward_list)==len(data))
    assert (len(timeout_list)==len(data))
    assert (len(para_list)==len(data))
    return timeout_list,reward_list,para_list

#
def find_best_para(data,init_lenghth):
    timeout_list,reward_list,reward_list_MI,para_list = find_best(data,MI=1,init_length=init_lenghth)
    colors = parastranscolors(20,1,para_list)
    assert(len(data)==len(para_list[0]))
    assert(len(reward_list)==len(data))
    assert(len(reward_list)==len(timeout_list))
    return colors,timeout_list,reward_list

def parastranscolors(k_number,init_k,para_list):
    dict_Map = {}
    for i in range(k_number):
        dict_Map[str(init_k+i)] = (i+1)/(k_number+1)
    alpha_list = para_list[0]
    beta_list = para_list[1]
    k_list = []
    for para_k in para_list[2]:
        k_list.append(dict_Map[str(para_k)])
    colors = [np.array(alpha_list),np.array(beta_list),np.array(k_list)]
    return colors


def print_diff_para(start_index,end_index,low,mid,high,best,bct_data,low_timeout_list,mid_timeout_list,high_timeout_list,best_timeout_list,best_para_list):
    # 生成示例数据 生成横坐标
    x = np.linspace(0, end_index-start_index, end_index-start_index)

    # 创建一个图形和两个子图
    fig, (ax1, ax2) = plt.subplots(2, 1, sharex=True)
    colors = np.vstack((best_para_list[0][start_index:end_index], best_para_list[1][start_index:end_index], best_para_list[2][start_index:end_index])).T# 转换为 [0, 1] 范围的 RGB 颜色

    # 在第一个子图中绘制正弦波
    # ax1.plot(x, low[start_index:end_index], color='red', label='low')
    # ax1.plot(x, mid[start_index:end_index],color='blue' , label='mid')
    # ax1.plot(x, high[start_index:end_index], color='purple', label='high')
    logging.info("the x is: %s",x)
    # ax1.plot(x, best[start_index:end_index], color='green', label='best')
    # 绘制每个线段
    corlor_list = ['r','#FFA500','purple','#00008B']
    # corlor_map = {}

    for i in range(end_index-start_index-1):
        # 生成随机颜色
        # color = (random.random(), random.random(), random.random())
        # logging.info("i is %s",i,x[start_index+i:start_index+i+2])
        c = colors[i]
        k = 0
        if c[0] == 0.125:
            k = 0
        # if c[0] == 0.1:
        #     k = 1
        if c[0] == 0.5:
            k = 1
        if c[0] == 0.9:
            k = 2
        ax1.plot(x[i:i+2], best[start_index+i:start_index+i+2],color=corlor_list[k],linewidth=5)
    # ax1.plot([], [], color='gray', label='',linewidth=30)
    # ax1.plot([], [], color='gray', label='best',linewidth=30)
    # ax1.plot([], [], color='gray', label='best',linewidth=30)
    # ax1.plot([], [], color='gray', label='best',linewidth=30)
    # 创建表格数据
    # data = [
    #     ['alpha', 'beta','k',''],
    #     ['0.125', '0.25','4',''],
    #     ['0.5', '0.5','4',''],
    #     ['0.9', '0.9','4','']
    # ]
    data = [
        [r'$\alpha$',''],
        ['0.1',''],
        ['0.5',''],
        ['0.9','']
    ]

    # 插入表格作为图例
    table = ax1.table(cellText=data, loc='lower right', cellLoc='center', colLabels=None)

    # 调整表格的大小
    table.scale(1, 1.5)  # 调整行高

    # 设置表格的背景颜色
    table.auto_set_font_size(False)
    for (i, j), cell in table.get_celld().items():
        cell.set_text_props(fontsize=20, color='black')
        if j == 1:
            if i == 1:
                cell.set_facecolor('r')
            if i == 2:
                cell.set_facecolor('#FFA500')
            if i == 3:
                cell.set_facecolor('purple')
            if i == 4:
                cell.set_facecolor('#00008B')
    # 插入表格，指定坐标
    # table = ax1.table(cellText=data, loc='lower right', cellLoc='center')
    table.scale(0.35,0.35*1.5 )
    # table.scale(1, 1.5)  # 调整行高
    # table.auto_set_column_width([0, 1])  # 自动设置列宽
    ax1.set_ylabel('TPS',fontsize=30)

    ax1.tick_params(axis='y',labelsize=30)
    ax1.spines['top'].set_linewidth(2)    # 上边框加宽
    ax1.spines['right'].set_linewidth(2)  # 右边框加宽
    ax1.spines['left'].set_linewidth(2)   # 左边框加宽
    ax1.spines['bottom'].set_linewidth(2)  # 下边框加宽
    # 设置第一个Y轴范围（不从零开始）
    ax1.set_ylim(0, 3100)
    ax1.set_yticks(np.arange(0, 3100, 1000))  # 每隔 0.3 设置一个刻度
    # ax1.legend()

    # 隐藏上方子图的 x 轴刻度
    # ax1.xaxis.set_ticks([])  # 隐藏 x 轴刻度
    # 在第二个子图中绘制余弦波
    # 将 (a, b, c) 标准化到 [0, 1] 范围
    logging.info("the best para list is: %s",best_para_list)
    colors = np.vstack((best_para_list[0][start_index:end_index], best_para_list[1][start_index:end_index], best_para_list[2][start_index:end_index])).T# 转换为 [0, 1] 范围的 RGB 颜色
    print(colors)


    ax2.plot(x, bct_data[start_index:end_index],color='green',label='PDT',linewidth=5)
    # ax2.scatter(x, bct_data[start_index:end_index], c=colors, s=50, edgecolor='none')
    # ax2.plot(x,best_timeout_list[start_index:end_index],color='green',label='best_timeout')
    # ax2.plot(x,low_timeout_list[start_index:end_index],color='red',label='low_timeout')
    # ax2.plot(x[start_index:end_index],mid_timeout_list[start_index:end_index],color='blue',label='mid_timeout')
    # ax2.plot(x[start_index:end_index],high_timeout_list[start_index:end_index],color='purple',label='high_timeout')
    ax2.set_ylabel('PDT (ms)',fontsize=30)
    ax2.set_xticks(np.arange(0, 180,30))
    ax2.tick_params(axis='y',labelsize=30)
    ax2.tick_params(axis='x',labelsize=30)
    ax2.set_xlabel('Block Index',fontsize=30)
    ax2.spines['top'].set_linewidth(2)    # 上边框加宽
    ax2.spines['right'].set_linewidth(2)  # 右边框加宽
    ax2.spines['left'].set_linewidth(2)   # 左边框加宽
    ax2.spines['bottom'].set_linewidth(2)  # 下边框加宽

    # ax2.legend()

    # 保持下子图的 x 轴刻度
    ax2.xaxis.set_ticks_position('bottom')  # 确保下子图的 x 轴刻度在底部
    # 调整子图之间的间距
    plt.subplots_adjust(hspace=0)  # 调整上下子图之间的高度间距
    # 显示图形
    plt.show()
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

    '''
    ***************************模拟生成数据***************************
    '''

    MI = 5
    # 这个是生成模拟数据的函数
    step = 30
    high = 780
    low =  580
    std1 = 20
    std2 = 40
    data = []
    data_list = []
    for i in range (10):
        d1,d2,d3,d4 = generatedata(high,low,std1,std2,step)
        data.extend(d1)
        data.extend(d2)
        data.extend(d3)
        data.extend(d4)
        data_list.append(d1)
        data_list.append(d2)
        data_list.append(d3)
        data_list.append(d4)
    # 这设置一个能够控制重复原来波形的控制
    ReNewFlag = True
    if ReNewFlag:
        save_model_data(data,'mode_data.csv')
    data = read_col_data('../analyze_icde/output/column_0.csv',2)
    logging.info("the bct_list'lenght is: %s,the bct_list is: %s",len(data),data)

    low_timeout_list,low_reward_list,_ = fixed_para_method(data,0.1,0.1,4,5)
    mid_timeout_list,mid_reward_list,_ = fixed_para_method(data,0.5,0.5,4,5)
    high_timeout_list,high_reward_list,_ = fixed_para_method(data,0.6,0.6,4,5)
    logging.info("low reward list is: %s",low_reward_list)
    logging.info("mid reward list is: %s",mid_reward_list)
    logging.info("high reward list is: %s",high_reward_list)
    colors,best_timeout_list,best_reward_list = find_best_para(data,init_lenghth=5)
    print("colors",colors)
    print_diff_para(start_index=0,end_index=150,low=low_reward_list,mid=mid_reward_list,high=high_reward_list,best=best_reward_list,bct_data=data,low_timeout_list=low_timeout_list,mid_timeout_list=mid_timeout_list,high_timeout_list=high_timeout_list,best_timeout_list=best_timeout_list,best_para_list = colors)
    assert (len(data) == len(best_timeout_list))
    for ii in range(len(best_timeout_list)):
        logging.info("the bct_data is: %s,the timeout is: %s",data[ii],best_timeout_list[ii])

    # bct_data 表示的是bct的时间序列，episodes_len表示的是MI的到校,num_episodes表示的是最大遍历的MI的数量
    # # logging.info("***************************下面是最优参数的过程***************************")

    #下面就是统计了几个变量，比如vc的数量，tps/reward的情况等；
    # test_bct_data_list=[674.9180600060571]
    # next_bct_data = 683.877586832632
    # RCPTVAR=0.3544159437502943
    # RCPT=674.6089637554302
    # test_best_rtt(data=test_bct_data_list,bct_data=next_bct_data,RCPTVAR=RCPTVAR,RCPT=RCPT)
