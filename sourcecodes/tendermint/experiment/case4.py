import matplotlib.pyplot as plt
import numpy as np

# 数据
dataset_sizes = [50,100, 200, 300, 400,500]
time_costs = {
    'PredictingTO(LSTM)': [117.476*11.487,131.9933*12.355, 140.12477*14.290, 134.66302*11.238, 146.94533*12.752,168.38678*11.477],
    'PredictingTO(RF)': [119.44546,118.40181, 120.42268, 121.00276, 123.63075,124.06951],
    'RL-Jacobson': [117.476,131.9933, 140.12477, 134.66302, 146.94533,168.38678]
}
Recall_values = {
    'PredictingTO(LSTM)': [591.00*0.8,568.84*1.2,619.46*0.8, 719.50*0.9,722.65*0.85,685.48*0.9],
    'PredictingTO(RF)': [1187.67,1178.49, 1204.31, 1228.11, 1109.41,1197.25],
    'RL-Jacobson': [591.00,568.84,619.46, 719.50,722.65,685.48]
}
colors = ['#155084', 'orange', 'green']  # 每种方法的颜色

# 设置柱状图宽度
bar_width = 0.25
x = np.arange(len(dataset_sizes))

# 创建图表
fig, ax1 = plt.subplots(figsize=(8, 6))

# 绘制柱状图
for i, (label, times) in enumerate(time_costs.items()):
    ax1.bar(x + i * bar_width, times, bar_width, label=label, color = colors[i],edgecolor='black', linewidth=5)

# 配置第一个Y轴
ax1.set_xlabel(' The scale of experience pool', fontsize=30)
ax1.set_ylabel('Time Cost (ms)', fontsize=30)
ax1.set_xticks(x + bar_width)
ax1.set_xticklabels(dataset_sizes, fontsize=30)
ax1.tick_params(axis='y',labelsize=30,width=2)
ax1.tick_params(axis='x',labelsize=30,width=2)
# ax1.legend(
#     loc='upper center',
#     bbox_to_anchor=(0.5, 0.95),  # 调整图例位置到顶部
#     ncol=3,  # 设置图例为一行三列
#     fontsize=30,
#     prop={'weight': 'bold'}
# )
ax1.legend(prop={'size': 20, 'weight': 'bold'},loc='upper right')
ax1.spines['top'].set_linewidth(2)
ax1.spines['right'].set_linewidth(2)
ax1.spines['left'].set_linewidth(2)
ax1.spines['bottom'].set_linewidth(2)
# 设置第一个Y轴范围（不从零开始）
ax1.set_ylim(0, 2500)
ax1.set_yticks(np.arange(0, 2600, 500))  # 每隔 0.3 设置一个刻度

# 创建第二个Y轴
ax2 = ax1.twinx()
line_styles = ['-', '--', '-.']  # 每种方法的线型
markers = ['o', 's', 'D']  # 每种方法的标记样式

# 调整折线图的横坐标，使其对齐到柱状图的中心
x_center = x + bar_width * (len(time_costs) - 1) / 2

for i, (label, recall) in enumerate(Recall_values.items()):
    ax2.plot(
        x_center, recall, linestyle=line_styles[i], marker=markers[i],
        markersize=15, label=f'{label} Recall', linewidth=5, color='black',
        markerfacecolor=colors[i], markeredgecolor='black', markeredgewidth=2
    )

ax2.set_ylabel('|TO - PDT| (ms)', fontsize=30)
# 设置第二个Y轴范围（不从零开始）
ax2.set_ylim(0, 2000)
ax2.set_yticks(np.arange(0, 2100, 400))  # 每隔 0.06 设置一个刻度
ax2.tick_params(axis='y',labelsize=30,width=2)
ax2.tick_params(axis='x',labelsize=30,width=2)
# ax2.legend(loc='upper right', fontsize=12)
ax2.spines['top'].set_linewidth(2)
ax2.spines['right'].set_linewidth(2)
ax2.spines['left'].set_linewidth(2)
ax2.spines['bottom'].set_linewidth(2)

# 添加网格和标题
ax1.grid(True, linestyle='--', alpha=0.6)
# plt.title('Time Cost and Recall vs Dataset Size', fontsize=30)

# 调整布局
plt.tight_layout()

# 保存和显示图表
# plt.savefig("time_cost_recall_styled_chart.png", dpi=300)
plt.show()
