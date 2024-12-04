import numpy as np
import matplotlib.pyplot as plt

# 数据集大小和时间成本
dataset_sizes = ['Network1', 'Network2']
time_costs = {
    'Static(timeout = 3s)': [1158.942529327685, 1203.234293781685],  # Static 在两个网络都显示
    'Ideal(timeout = 1.2s)': [1747.6746218226308, None],  # Ideal(timeout = 1.2s) 仅在 Network1
    'Ideal(timeout = 1.4s)': [None, 1359.5494755160241]   # Ideal(timeout = 1.4s) 仅在 Network2
}
patterns = [None, '\\', '//']  # 纹理样式
colors = ['#155084', '#8aa9e3', '#8aa9e3']  # Static 和 Ideal 的颜色

# 设置柱状图宽度
bar_width = 0.35
x = np.arange(len(dataset_sizes))

# 创建图表
fig, ax1 = plt.subplots(figsize=(8, 6))

# 绘制 Static(timeout = 3s)
bars_static = ax1.bar(
    x, time_costs['Static(timeout = 3s)'], bar_width,
    label='Static(timeout = 3s)', color=colors[0], edgecolor='black', linewidth=5,
    hatch=patterns[0]
)

# 绘制 Ideal(timeout = 1.2s) 和 Ideal(timeout = 1.4s)
if time_costs['Ideal(timeout = 1.2s)'][0] is not None:
    bars_ideal_1_2 = ax1.bar(
        x[0] + bar_width, time_costs['Ideal(timeout = 1.2s)'][0], bar_width,
        label='Ideal(timeout = 1.2s)', color=colors[1], edgecolor='black', linewidth=5,
        hatch=patterns[1]
    )

if time_costs['Ideal(timeout = 1.4s)'][1] is not None:
    bars_ideal_1_4 = ax1.bar(
        x[1] + bar_width, time_costs['Ideal(timeout = 1.4s)'][1], bar_width,
        label='Ideal(timeout = 1.4s)', color=colors[2], edgecolor='black', linewidth=5,
        hatch=patterns[2]
    )

# 配置第一个Y轴
ax1.set_xlabel("", fontsize=20)
ax1.set_ylabel('Transactions per Second', fontsize=20)
ax1.set_xticks(x + bar_width / 2)  # 调整X轴刻度位置
ax1.set_xticklabels(dataset_sizes, fontsize=24)
ax1.tick_params(axis='y', labelsize=24, width=2)
ax1.tick_params(axis='x', labelsize=24, width=2)

ax1.legend(prop={'size': 24, 'weight': 'bold'}, loc='upper right')
ax1.spines['top'].set_linewidth(2)
ax1.spines['right'].set_linewidth(2)
ax1.spines['left'].set_linewidth(2)
ax1.spines['bottom'].set_linewidth(2)

# 设置第一个Y轴范围（不从零开始）
ax1.set_ylim(0, 2800)
ax1.set_yticks(np.arange(0, 2800, 500))

# 调整图表范围和边距
plt.tight_layout()
fig.subplots_adjust(bottom=0.068, top=0.88, right=0.9)

# 保存图表为文件
plt.savefig('fig_observation_network.pdf', dpi=300, bbox_inches='tight')  # 保存高 DPI 的 PDF 文件
# plt.savefig("fig_observation_network.pdf", dpi=300)
plt.show()
