# # server.py
import time
from concurrent import futures
import grpc
import pcc_pb2 as pcc_pb2
import pcc_pb2_grpc as pcc_pb2_grpc
import pcc as pcc
import logging
import model as m
import spc as spc

logging.basicConfig(level=logging.INFO, format='%(asctime)s - %(levelname)s - %(message)s')

global_test = 0

class PccServicer(pcc_pb2_grpc.PccServiceServicer):
    logging.info("init pcc service")
    def InitPCC(self, request, context):
        self.wait_write_data = ''
        self.is_byz_flag = request.is_byz                               #表该节点是否还是拜占庭节点
        self.version = request.version
        self.delay_time = request.delay_time
        logging.info("the init delay_time is: %s",self.delay_time)
        self.cold_start_phase = request.cold_start_phase
        MI = 4   #每个MI的大小
        init_timeout = request.init_timeout  #初始化的timeout
        cold_start_phase = request.cold_start_phase   #冷启动阶段的长度
        method = request.method
        self.method = method
        para_a = request.para_a
        experiences_window = 300  #经验池的大小
        logging.info("the is a malicious node? %s , the could_start_phase'len is: %s the version is: %s,para_a: %s",self.is_byz_flag,request.cold_start_phase,self.version,para_a)
        # 这个是诚实节点设置偏大的TOu

        model,state,action = m.generatemodel('honest',version=self.version,MI=MI,init_timeout=init_timeout,cold_start_phase=cold_start_phase,experiences_window = experiences_window,method=method,para_a=para_a)
        self.model_with_posioned = model              #每种节点都有
        self.action_with_posioned = action
        self.state_with_posioned = state

        # if self.is_byz_flag:
        model_byz,state_byz,action_byz = m.generatemodel('byz',version=self.version,MI=MI,init_timeout=init_timeout,cold_start_phase=cold_start_phase,experiences_window= experiences_window,method=method,para_a=para_a)  #这个是拜占庭节点估计出来的正常的网络延迟
        self.model_without_posioned = model_byz           #只有拜占庭节点有
        self.action_without_posioned = action_byz
        self.state_without_posioned = state_byz

        # 返回值应该包含timeout
        # logging.info("init pcc service")
        mi = request.m_i
        smi = request.s_m_i
        epoch_size = request.epoch_size
        init_timeout = request.init_timeout
        logging.info("MI: %s, SMI: %s, EpochSize: %s, Init_Timeout: %s",mi,smi,epoch_size,init_timeout)
        # 初始化PCC
        self.p = pcc.Pcc(mi,epoch_size,init_timeout)
        # 初始化SPC
        logging.info("MI: %s, EpochSize: %s, Init_Timeout: %s",mi,epoch_size,init_timeout)
        self.s = spc.Spc(smi)
        #构造响应
        response = pcc_pb2.InitResponse()
        response.m_i = self.p.MI
        response.mi_of_number = self.p.mi_of_number  #这个是表明有几个mi
        for size in self.p.epoch_size_list:
            response.epoch_size_list.append(size)

        response.init_timeout = self.p.init_timeout
        self.last_latency = 0
        response.delay_time = self.delay_time
        self.start_time = time.time()
        return response

    def RunPcc(self, request, context):
        global global_test
        global_test = global_test + 1
        # 取请求中数据
        mi_data_list = request.m_i_data

        # 执行pcc
        self.p.run(mi_data_list)
        # 构造响应结构体
        response = pcc_pb2.RunResponse()

        response.m_i = self.p.MI
        response.mi_of_number = self.p.mi_of_number
        for size in self.p.epoch_size_list:
            response.epoch_size_list.append(size)
        response.init_timeout = int(self.p.init_timeout)
        return response
    def RunSpc(self, request, context):
        logging.info("Runing Spc service to apdate alpha、bate")
        smi_data = request.m_i_data
        code = self.s.run(smi_data)
        response = pcc_pb2.RunSpcResponse()
        response.code = code
        return response

    def RunRLJ(self,request,context):
        # 在这调用RLJ的算法，
        logging.info("the height of block is: %s",request.height_data.height)
        bct_data = request.height_data.actaul_timeout
        byz_node_flag = request.height_data.is_byz_node
        is_leader_flag = request.height_data.is_leader
        pdt_data = request.height_data.pdt
        td = request.height_data.delay_time
        min_value = request.height_data.min
        max_value = request.height_data.max
        avg_value = request.height_data.avg
        std_value = request.height_data.std
        # 这是诚实节点
        if td > 0.0:
            byz_node_flag = False
            is_leader_flag = False
        self.state_with_posioned,self.action_with_posioned = self.model_with_posioned.new_height(bct_data=bct_data,pdt_data=pdt_data,state=self.state_with_posioned,action=self.action_with_posioned,byz_node_flag=byz_node_flag,is_leader_flag=is_leader_flag,last_latency=self.last_latency,min_value=min_value,max_value=max_value,avg_value=avg_value,std_value=std_value)
        timeout = self.model_with_posioned.input_data.Timeout
        byz_timeout = timeout
        # 这是作恶节点模拟正确的情况
        logging.info("starting malicous phase")
        byz_bct_data = bct_data-td
        if self.is_byz_flag:
            if td < 0.0:
                byz_bct_data = bct_data

        self.state_without_posioned,self.action_without_posioned = self.model_without_posioned.new_height(bct_data=byz_bct_data,pdt_data=pdt_data,state=self.state_without_posioned,action=self.action_without_posioned,byz_node_flag=byz_node_flag,is_leader_flag=is_leader_flag,last_latency=self.last_latency,min_value=min_value,max_value=max_value,avg_value=avg_value,std_value=std_value)

        if self.is_byz_flag:
            byz_timeout = self.model_without_posioned.input_data.Timeout
            if self.cold_start_phase > request.height_data.height:
                td = self.delay_time
            else:
                td = timeout - byz_timeout
        else:
            td = timeout - byz_timeout

        logging.info("the self.cold_start_phase is: %s ,the request.height_data.height is: %s",self.cold_start_phase,request.height_data.height)
        response = pcc_pb2.RunRLJResponse()
        response.code = request.height_data.height
        logging.info("the height is: %s,the timeout is: %s",request.height_data.height+1,timeout)
        response.timeout = timeout
        response.byz_timeout = byz_timeout
        if self.method == "Fixed":
            td = self.delay_time
        response.delay_time = td
        self.end_time = time.time()
        self.last_latency = self.end_time - self.start_time
        self.model_with_posioned.savedataofheight(request.height_data)
        self.model_without_posioned.savedataofheight(request.height_data)
        self.start_time = time.time()
        return response

def serve():
    server = grpc.server(futures.ThreadPoolExecutor(max_workers=10))
    pcc_pb2_grpc.add_PccServiceServicer_to_server(PccServicer(), server)
    server.add_insecure_port('localhost:50051')
    server.start()
    logging.info("the pcc service is starting ,the service's adderss:port is localhost:50051")
    server.wait_for_termination()
    logging.info("the pcc service ")

if __name__ == '__main__':
    serve()

