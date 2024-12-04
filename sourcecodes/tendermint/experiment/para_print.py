[4684.05,3057.97 ,3732.32 ,2539.14,1962.44,1727.80,1731,79,1958.03,2127.68]
import matplotlib.pyplot as plt
pdt = [4684.05,3741.97 ,3732.32 ,2539.14,1962.44,1727.80,1731.79,1958.03,2127.68]
a = [0.1,0.5,1,1.5,2,2.5,3,3.5,4]
# y1 = [3273.8579808006316,2952.0759070471777,2691.875985700316,2518.309600946867,2249.9115207636946,2050.478097899565]
# y2 = [3217.983492011131,3045.300554580985,2878.3588980963295,2830.1490034251447,2656.779390054986,2505.7874107745697]
# y3 = [3528.031388855546,3153.177907982046,3053.6448506992074,3019.275757815654,2758.952089589118,2704.812269799489]
y1 = []
for p in pdt:
    y1.append(10000*1000 / (p + 2000))
# 创建图形和子图
fig, ax = plt.subplots()
print(len(a),len(y1))
# 绘制两条折线
ax.plot(y1, color='r', marker='o', markersize=15, linewidth=5,)
# ax.plot(range(6), y2, color='#00008B', marker='s', markersize=15, linewidth=5, label="Jacobson")
# ax.plot(range(6), y3, color='green', marker='*', markersize=15, linewidth=5, label="RL-Jacobson")

# 设置标签和刻度
ax.tick_params(axis='y', labelsize=30, width=2)
ax.tick_params(axis='x', labelsize=30, width=2)
ax.set_xlabel('Italic Font: a', fontsize=30, labelpad=20,style='italic')  # 调整 X 轴标签与图形的距离
ax.set_ylabel("TPS", fontsize=30, labelpad=20)  # 调整 Y 轴标签与图形的距离
plt.xticks([0,1,2,3,4,5,6,7,8,], ['0.1','0.5','1','1.5','2','2.5','3','3.5','4'])  # 指定刻度和标签
# ax.set_xticks(range(6))
# ax.set_xticklabels([f"{i}" for i in range(0,12,2)])
ax.grid(color='gray', linestyle='--', linewidth=0.8)  # 调整网格线样式

# 添加图例
# ax.legend(prop={'size': 20, 'weight': 'bold'}, loc='upper right', frameon=True, framealpha=0.9, edgecolor='black', borderpad=1)

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

# plt.savefig("Mean of PDT", dpi=300)
plt.show()
