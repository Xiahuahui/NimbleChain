import numpy as np
import matplotlib.pyplot as plt

# 数据集大小和时间成本
dataset_sizes = ['static', 'ideal']
time_costs = {
    'network1': [2698.6066130505437, 1764.6545303096018],
    'network-delay': [3066.3826331360656, 2955.520974067999]
}
colors = ['#155084', 'orange']

# 设置柱状图宽度
bar_width = 0.25
x = np.arange(len(dataset_sizes))

# 创建图表
fig, ax1 = plt.subplots(figsize=(8, 6))

# 绘制柱状图
for i, (label, times) in enumerate(time_costs.items()):
    bars = ax1.bar(x + i * bar_width, times, bar_width, label=label, color=colors[i], edgecolor='black', linewidth=1)
    i  = 0
    for bar in bars:
        yval = bar.get_height()
        tt = ''
        if i ==0:
            tt ='TO=1s'
        if i ==1:
            tt ='TO=2s'
        if i ==2:
            tt ='TO=1s'
        if i ==3:
            tt ='TO=3s'
        i= i+1
        ax1.text(bar.get_x() + bar.get_width()/2, yval, f'{tt}', ha='center', va='bottom', fontsize=30)

# 配置第一个Y轴
ax1.set_xlabel('经验池规模', fontsize=30)
ax1.set_ylabel('时间成本（毫秒）', fontsize=30)
ax1.set_xticks(x + bar_width / 2)
ax1.set_xticklabels(dataset_sizes, fontsize=30)
ax1.tick_params(axis='y', labelsize=30, width=2)
ax1.tick_params(axis='x', labelsize=30, width=2)

# 添加图例
ax1.legend(prop={'size': 20, 'weight': 'bold'}, loc='upper right')
ax1.spines['top'].set_linewidth(2)
ax1.spines['right'].set_linewidth(2)
ax1.spines['left'].set_linewidth(2)
ax1.spines['bottom'].set_linewidth(2)

# 设置Y轴范围
ax1.set_ylim(0, 3500)  # 根据数据调整范围
ax1.set_yticks(np.arange(0, 3600, 500))

plt.show()
