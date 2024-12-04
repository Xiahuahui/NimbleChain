import matplotlib
matplotlib.use('Agg')  # 使用非图形界面后端
import matplotlib.pyplot as plt
import numpy as np

matplotlib.rcParams['pdf.fonttype'] = 42
matplotlib.rcParams['ps.fonttype'] = 42
matplotlib.rcParams['figure.facecolor'] = 'white'  # 确保背景颜色是白色

# 蓝色系
colors_a = ['#e8effd', '#bed1ef', '#8aa9e3', '#516ac7', '#203086']
# 粉色系
colors_b = ['#f4dde5', '#ecaabe', '#d67290', '#b24866', '#8e233d']

# 数据示例
nodes = ['Node 1', 'Node 2', 'Node 3']
experiment_labels = ['Node 1', 'Node 2', 'Node 3']

# 完成时间数据
completion_times = [
    [69.2, 81.3, 84.7],  # Node 1
    [67, 46.5171, 61],  # Node 2
    [46, 42.7, 31]   # Node 3
]

# 设置柱状图的宽度和位置
bar_width = 0.28
index = np.arange(len(nodes)) * 1.1

# 创建柱状图
fig, ax = plt.subplots(figsize=(12.5, 9))

# 分别添加每个实验的柱状图，手动设置颜色和稀疏底纹
bars1 = ax.bar(index, completion_times[0], bar_width, label=experiment_labels[0], color='#8aa9e3', edgecolor='black', linewidth=3, hatch='')
bars2 = ax.bar(index + bar_width, completion_times[1], bar_width, label=experiment_labels[1], color='#ecaabf', edgecolor='black', linewidth=3, hatch='')
bars3 = ax.bar(index + 2 * bar_width, completion_times[2], bar_width, label=experiment_labels[2], color='#e8e8e8', edgecolor='black', linewidth=3, hatch='')

# 手动添加稀疏的底纹
def add_sparse_hatch(ax, bars, color='gray', density=2):
    for bar in bars:
        bar_height = bar.get_height()
        bar_x = bar.get_x()
        bar_width = bar.get_width()

        x_coords = np.linspace(bar_x, bar_x + bar_width, int(density))
        y_coords = np.linspace(0, bar_height, int(density))

        for y in y_coords:
            ax.scatter(x_coords, [y] * len(x_coords), color=color, s=10)

add_sparse_hatch(ax, bars1, color='gray', density=5)
add_sparse_hatch(ax, bars2, color='gray', density=5)
add_sparse_hatch(ax, bars3, color='gray', density=5)

# 设置轴标签和标题
ax.set_ylabel('Completion Time (ms)', fontsize=42)
ax.set_ylim(25, max(max(completion_times[0]), max(completion_times[1]), max(completion_times[2])) + 6)  # 设置y轴从20开始

# 修改x轴标签并设置字体大小
xtick_labels = ['Node 1  ', 'Node 2  ', '      Node 3 as leader']
ax.set_xticks(index + bar_width)
ax.set_xticklabels(xtick_labels, fontsize=38)
ax.tick_params(axis='y', labelsize=36)

# 添加横向虚线并标出取值
for j in range(len(nodes)):
    value = completion_times[0][j]  # 每个 leader 第一个柱子的值
    ax.text(index[j] - bar_width * 0.6, value + 1, f'{value}', color='black', fontsize=44, zorder=4)

# 设置图例为横向布局并放置在图表上方
ax.legend(fontsize=42, loc='upper center', ncol=3, bbox_to_anchor=(0.535, 1.18), labelspacing=0.1, columnspacing=0.9, handlelength=1.4, handletextpad=0.4, frameon=False)

# 去掉右边和上边的框线
ax.spines['right'].set_visible(False)
ax.spines['top'].set_visible(False)
# 加粗底部和左侧框线
ax.spines['bottom'].set_linewidth(3)
ax.spines['left'].set_linewidth(3)

# 调整图表范围和边距
plt.tight_layout()
fig.subplots_adjust(bottom=0.068, top=0.88, right=0.9)  # 调整上边距和右边距

# 保存图表为文件
plt.savefig('popularity.pdf', dpi=300, bbox_inches='tight')  # 保存高 DPI 的 PDF 文件
print("save pdf over")
