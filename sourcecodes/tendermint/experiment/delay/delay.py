import matplotlib.pyplot as plt
import numpy as  np
y1 = [3273.8579808006316,3130.096275528949,3042.7595356231623,2867.8987193637204,2816.9923185577613,2759.1587177625565]
y2 = [3273.8579808006316,3273-13.8579808006316,3273.8579808006316-20,3273.8579808006316-24,3273.8579808006316-30,3273.8579808006316-35]

# y1 = pdt_means
# y2 = fixed_delay_means

# 创建图形和子图
fig, ax = plt.subplots()
# '#155084', '#8aa9e3'
# 绘制两条折线
ax.plot(range(6), y1, color='#155084', marker='o', markersize=15, linewidth=5, label="static")
ax.plot(range(6), y2, color='#8aa9e3', marker='s', markersize=15, linewidth=5, label="ideal")

# 设置标签和刻度
ax.tick_params(axis='y', labelsize=30, width=2)
ax.tick_params(axis='x', labelsize=30, width=2)
ax.set_xlabel("# of Byzantine nodes", fontsize=30, labelpad=20)  # 调整 X 轴标签与图形的距离
ax.set_ylabel("TPS", fontsize=30, labelpad=20)  # 调整 Y 轴标签与图形的距离
ax.set_xticks(range(6))
ax.set_xticklabels([f"{i}" for i in range(0,12,2)])
ax.set_ylim(2500, 3600)
ax.set_yticks(np.arange(2500, 3700, 200))
ax.grid(color='gray', linestyle='--', linewidth=0.8)  # 调整网格线样式

# 添加图例
ax.legend(prop={'size': 20, 'weight': 'bold'}, loc='upper right', frameon=True, framealpha=0.9, edgecolor='black', borderpad=1)

# 调整边框线宽
ax.spines['top'].set_linewidth(2)
ax.spines['right'].set_linewidth(2)
ax.spines['left'].set_linewidth(2)
ax.spines['bottom'].set_linewidth(2)

# 添加标题
# ax.set_title("Comparison of TPS with Different Crash Nodes", fontsize=35, pad=30)

# 自动调整布局
# plt.tight_layout()

# 如果需要手动调整边距
# plt.subplots_adjust(left=0.3, right=0.95, top=0.9, bottom=0.5)  # 左、右、上、下边距
plt.show()
