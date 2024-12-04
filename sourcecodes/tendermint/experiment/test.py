import matplotlib.pyplot as plt
import numpy as np

# 示例数据
x = np.linspace(0, 10, 100)
y1 = np.sin(x)
y2 = np.cos(x)

# 创建图形和轴
fig, ax = plt.subplots()

# 绘制折线
line1, = ax.plot(x, y1, label='Sine Wave', color='blue')
line2, = ax.plot(x, y2, label='Cosine Wave', color='orange')

# 添加标题和标签
ax.set_title('Sine and Cosine Waves with Table Legend')
ax.set_xlabel('X-axis')
ax.set_ylabel('Y-axis')

# 创建表格数据
data = [
    ['Line', 'Color'],
    ['Sine Wave', 'Blue'],
    ['Cosine Wave', 'Orange']
]

# 插入表格作为图例
table = ax.table(cellText=data, loc='upper right', cellLoc='center', colLabels=None)

# 调整表格的大小
table.scale(1, 1.5)  # 调整行高

# 设置表格的背景颜色
table.auto_set_font_size(False)
table.set_fontsize(12)
table.scale(1, 1.5)

# 调整图形的布局
plt.subplots_adjust(right=0.75)  # 留出空间给表格

# 显示图形
plt.show()
