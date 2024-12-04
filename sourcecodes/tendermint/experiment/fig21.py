#构造一个例子说明adjust的方法，也不能应对
import random
import statistics
import time
import csv
import logging
import numpy as np
import copy
import matplotlib.pyplot as plt

y1=[0.6666666,0.833333333,0.777777778,0.705882353,0.555555556]

x = ["0.06","0.12","0.19","0.25","0.31"]

if __name__ == '__main__':
    # fig, (ax1, ax2) = plt.subplots(, 1)
    fig, ax = plt.subplots()
    ax.spines['top'].set_linewidth(2)    # 上边框加宽
    ax.spines['right'].set_linewidth(2)  # 右边框加宽RUC&info500
    ax.spines['left'].set_linewidth(2)   # 左边框加宽
    ax.spines['bottom'].set_linewidth(2)  # 下边框加宽

    # ax1.plot(x_list,tps_list,label='Normal',color='r',marker='^',markersize=10,linewidth=5)
    ax.plot(y1,label='Max/2',color = 'b',marker='d',markersize=15,linewidth=5)
    # ax.plot(y2,label='Bilivel',marker='p',markersize=15,linewidth=5)
    ax.set_ylabel('Recall',fontsize=24)
    ax.set_ylim(0.5, 0.9)
    # ax.set_yticks(np.arao.5nge(1000, 1900, 200))  # 每隔 0.3 设置一个刻度

    ax.tick_params(axis='y',labelsize=20,width=2)
    ax.tick_params(axis='x',labelsize=20,width=2)
    # for label in ax1.get_xticklabels():  # 对于 x 轴的刻度标签
    #     label.set_fontweight('bold')     # 设置字体加粗
    # for label in ax1.get_yticklabels():  # 对于 y 轴的刻度标签
    #     label.set_fontweight('bold')     # 设置字体加粗
    ax.set_xlabel('Ratio of Byzantine Nodes',fontsize=24)
    ax.set_xticklabels(x)
    ax.set_xlim(-1,4)
    ax.set_xticks(np.arange(0.0, 6, 1))
    ax.spines['top'].set_linewidth(2)    # 上边框加宽
    ax.spines['right'].set_linewidth(2)  # 右边框加宽
    ax.spines['left'].set_linewidth(2)   # 左边框加宽
    ax.spines['bottom'].set_linewidth(2)  # 下边框加宽
    # ax.legend(prop={'size': 20, 'weight': 'bold'},loc='upper left')


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
    plt.tight_layout()
    fig.subplots_adjust(bottom=0.068, top=0.88, right=0.9)

    # 保存图表为文件
    plt.savefig('rtl_abnormal_detection_breakdown.pdf', dpi=300, bbox_inches='tight')  # 保存高 DPI 的 PDF 文件
    # plt.savefig("fig_observation_network.pdf", dpi=300)
    plt.show()
